package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/rschio/rinha/internal/core/client"
	"github.com/rschio/rinha/internal/web"
	"go.opentelemetry.io/otel/trace"
)

func APIMux(s *Server, tracer trace.Tracer) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("POST /clientes/{id}/transacoes", middlewareWeb(tracer, s.Transactions))
	mux.Handle("GET /clientes/{id}/extrato", middlewareWeb(tracer, s.Billing))

	return mux
}

func middlewareWeb(tracer trace.Tracer, h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer.Start(r.Context(), "web")

		v := web.Values{
			TraceID: span.SpanContext().TraceID().String(),
			Tracer:  tracer,
			Now:     time.Now().UTC(),
		}
		ctx = web.SetValues(ctx, &v)
		r = r.WithContext(ctx)

		h(w, r)
	})
}

type Server struct {
	log    *slog.Logger
	client *client.Core
}

func NewServer(log *slog.Logger, c *client.Core) *Server {
	return &Server{log: log, client: c}
}

func (s *Server) Transactions(w http.ResponseWriter, r *http.Request) {
	serveJSON(w, r, s,
		func(ctx context.Context, id int, req TransactionsReq) (TransactionsResp, error) {
			nt := client.NewTransaction{
				Value:       req.Value,
				Type:        req.Type,
				Description: req.Description,
			}

			c, err := s.client.AddTransaction(ctx, id, nt)
			if err != nil {
				return TransactionsResp{}, err
			}

			return TransactionsResp{
				Limit:   c.Limit,
				Balance: c.Balance,
			}, nil
		},
	)
}

func (s *Server) Billing(w http.ResponseWriter, r *http.Request) {
	serveJSON(w, r, s,
		func(ctx context.Context, id int, req struct{}) (BillingResp, error) {
			b, err := s.client.Billing(ctx, id)
			if err != nil {
				return BillingResp{}, err
			}

			return toBillingResp(b), nil
		},
	)
}

func getID(r *http.Request) (int, error) {
	sID := r.PathValue("id")
	return strconv.Atoi(sID)
}

func serveJSON[Req any, Resp any](
	w http.ResponseWriter,
	r *http.Request,
	s *Server,
	fn func(ctx context.Context, id int, req Req) (Resp, error),
) {
	if r.Header.Get("Content-Type") != "application/json" {
		s.log.Error("request must be a json")
		http.Error(w, "request must be a json", http.StatusBadRequest)
		return
	}

	var req Req
	if r.Method != http.MethodGet {
		err := json.NewDecoder(r.Body).Decode(&req)
		r.Body.Close()
		if err != nil {
			s.log.Error("decoding json", "ERROR", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
	}

	id, err := getID(r)
	if err != nil {
		s.log.Error("getID", "ERROR", err)
		http.Error(w, "invalid id", http.StatusNotFound)
		return
	}

	resp, err := fn(r.Context(), id, req)
	if err != nil {
		s.log.Error("fn", "ERROR", err)
		switch {
		case errors.Is(err, client.ErrNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
			return

		case errors.Is(err, client.ErrInvalidArgument):
			http.Error(w, err.Error(), http.StatusBadRequest)
			return

		case errors.Is(err, client.ErrTransactionDenied):
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return

		default:
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}

	bs, err := json.Marshal(resp)
	if err != nil {
		s.log.Error("failed to encode response", "ERROR", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(bs)
}
