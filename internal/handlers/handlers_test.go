package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rschio/rinha/internal/core/client"
	"github.com/rschio/rinha/internal/core/client/store/clientdb"
	"github.com/rschio/rinha/internal/data/dbtest"
	"go.opentelemetry.io/otel"
)

func TestTransactions(t *testing.T) {
	log, db, teardown := dbtest.NewUnit(t, dbtest.WithMigrations())
	t.Cleanup(teardown)

	server := NewServer(log, client.NewCore(clientdb.NewStore(log, db)))
	httpServer := httptest.NewServer(APIMux(server, otel.GetTracerProvider().Tracer("")))
	t.Cleanup(httpServer.Close)

	id := 1
	path := httpServer.URL + fmt.Sprintf("/clientes/%d/transacoes", id)
	data := `{"valor":1000,"tipo":"c","descricao":"descricao"}`
	contentType := "application/json"

	resp, err := http.Post(path, contentType, strings.NewReader(data))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got wrong status code: %v", resp.StatusCode)
	}

	var tresp TransactionsResp
	if err := json.NewDecoder(resp.Body).Decode(&tresp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if tresp.Limit == 0 {
		t.Fatalf("limit should be != 0, got %v", tresp.Limit)
	}
}

func TestTransactionsID(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		wantedCode int
	}{
		{"invalid string", "not_number", 404},
		{"invalid id", "-1", 404},
		{"id not found", "6", 404},
		{"good id", "1", 200},
	}

	log, db, teardown := dbtest.NewUnit(t, dbtest.WithMigrations())
	t.Cleanup(teardown)

	server := NewServer(log, client.NewCore(clientdb.NewStore(log, db)))
	httpServer := httptest.NewServer(APIMux(server, otel.GetTracerProvider().Tracer("")))
	t.Cleanup(httpServer.Close)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			path := httpServer.URL + fmt.Sprintf("/clientes/%s/transacoes", tt.id)
			data := `{"valor":1000,"tipo":"c","descricao":"descricao"}`
			contentType := "application/json"

			resp, err := http.Post(path, contentType, strings.NewReader(data))
			if err != nil {
				t.Fatalf("post: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantedCode {
				t.Fatalf("got wrong status code: %v, want: %v", resp.StatusCode, tt.wantedCode)
			}
		})
	}
}
