package handlers

import (
	"context"
	"log/slog"
	"net/http"

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

type Server struct {
	log    *slog.Logger
	client *client.Core
}

func NewServer(log *slog.Logger, c *client.Core) *Server {
	return &Server{log: log, client: c}
}

func (s *Server) Transactions(w http.ResponseWriter, r *http.Request) {
	serveJSON(s, w, r,
		func(ctx context.Context, id int, req TransactionsReq) (TransactionsResp, error) {
			ctx, span := web.AddSpan(ctx, "internal.handlers.Server.Transactions")
			defer span.End()

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
	serveJSON(s, w, r,
		func(ctx context.Context, id int, _ struct{}) (BillingResp, error) {
			ctx, span := web.AddSpan(ctx, "internal.handlers.Server.Billing")
			defer span.End()

			b, err := s.client.Billing(ctx, id)
			if err != nil {
				return BillingResp{}, err
			}

			return toBillingResp(b), nil
		},
	)
}
