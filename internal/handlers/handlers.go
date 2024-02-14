package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
)

func APIMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /clientes/{id}/transacoes", Transactions)
	mux.HandleFunc("GET /clientes/{id}/extrato", Billing)

	return mux
}

type TransactionsReq struct {
	Value       int    `json:"valor"`
	Type        string `json:"tipo"`
	Description string `json:"descricao"`
}

type TransactionsResp struct {
	Limit   int `json:"limite"`
	Balance int `json:"saldo"`
}

func Transactions(w http.ResponseWriter, r *http.Request) {
	serveJSON(w, r,
		func(ctx context.Context, id int, req TransactionsReq) (TransactionsResp, error) {
			return TransactionsResp{Limit: 1000, Balance: 1000}, nil
		},
	)
}

func Billing(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusInternalServerError)
}

func getID(r *http.Request) (int, error) {
	sID := r.PathValue("id")
	return strconv.Atoi(sID)
}

func serveJSON[Req any, Resp any](
	//	s *Server,
	w http.ResponseWriter,
	r *http.Request,
	fn func(ctx context.Context, id int, req Req) (Resp, error),
) {
	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "request must be a json", http.StatusBadRequest)
		return
	}

	var req Req
	err := json.NewDecoder(r.Body).Decode(&req)
	r.Body.Close()
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	id, err := getID(r)
	if err != nil {
		// TODO: Return 404 or other error?
		http.Error(w, "invalid id", http.StatusNotFound)
		return
	}

	resp, err := fn(r.Context(), id, req)
	if err != nil {
		// TODO: change to represent the correct errors.
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	bs, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, "failed encode response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(bs)
}
