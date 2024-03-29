package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/rschio/rinha/internal/core/client"
	"github.com/rschio/rinha/internal/web"
)

func getID(r *http.Request) (int, error) {
	sID := r.PathValue("id")
	return strconv.Atoi(sID)
}

func serveJSON[Req any, Resp any](
	s *Server,
	w http.ResponseWriter,
	r *http.Request,
	fn func(ctx context.Context, id int, req Req) (Resp, error),
) {
	ctx, span := web.AddSpan(r.Context(), "internal.handlers.serveJSON")
	defer span.End()

	var req Req
	if r.Method == http.MethodPost {
		if r.Header.Get("Content-Type") != "application/json" {
			s.log.Error("request must be a json")
			http.Error(w, "request must be a json", http.StatusBadRequest)
			return
		}

		err := json.NewDecoder(r.Body).Decode(&req)
		r.Body.Close()
		if err != nil {
			s.log.Error("decoding json", "ERROR", err)
			// TODO: this error is incorrect.
			http.Error(w, "bad request", http.StatusUnprocessableEntity)
			//http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
	}

	id, err := getID(r)
	if err != nil {
		s.log.Error("getID", "ERROR", err)
		http.Error(w, "invalid id", http.StatusNotFound)
		return
	}

	resp, err := fn(ctx, id, req)
	if err != nil {
		s.log.Error("fn", "ERROR", err)
		switch {
		case errors.Is(err, client.ErrNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
			return

		case errors.Is(err, client.ErrInvalidArgument):
			// TODO: I think this should return bad request,
			// but the tests aks for 422.
			fallthrough
			//http.Error(w, err.Error(), http.StatusBadRequest)
			//return

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
