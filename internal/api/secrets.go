package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/wolvever/litefaas/internal/secret"
)

type secretValueRequest struct {
	Value string `json:"value"`
}

type secretNameResponse struct {
	Name string `json:"name"`
}

type secretValueResponse struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (s *Server) handleSecretList(w http.ResponseWriter, r *http.Request) {
	if s.secrets == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "secrets store not configured"})
		return
	}
	names, err := s.secrets.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: "list secrets failed"})
		return
	}
	out := make([]secretNameResponse, 0, len(names))
	for _, n := range names {
		out = append(out, secretNameResponse{Name: n})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleSecretPut(w http.ResponseWriter, r *http.Request) {
	if s.secrets == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "secrets store not configured"})
		return
	}
	name := r.PathValue("name")
	if err := secret.ValidateName(name); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: err.Error()})
		return
	}
	var req secretValueRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid JSON body"})
		return
	}
	if err := s.secrets.Set(name, req.Value); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, secretNameResponse{Name: name})
}

func (s *Server) handleSecretGet(w http.ResponseWriter, r *http.Request) {
	if s.secrets == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "secrets store not configured"})
		return
	}
	name := r.PathValue("name")
	val, err := s.secrets.Get(name)
	if errors.Is(err, secret.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorBody{Error: "not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, secretValueResponse{Name: name, Value: val})
}

func (s *Server) handleSecretDelete(w http.ResponseWriter, r *http.Request) {
	if s.secrets == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "secrets store not configured"})
		return
	}
	name := r.PathValue("name")
	err := s.secrets.Delete(name)
	if errors.Is(err, secret.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorBody{Error: "not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
