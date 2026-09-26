// Package api is the litefaasd HTTP surface (RFC-0001 §9).
package api

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/wolvever/litefaas/internal/store"
	"github.com/wolvever/litefaas/internal/types"
	"github.com/wolvever/litefaas/internal/version"
)

var nameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// Server is the control-plane HTTP handler.
type Server struct {
	mux   *http.ServeMux
	store *store.Store
	token string
}

type Options struct {
	Store *store.Store
	Token string
}

func New(opts Options) *Server {
	s := &Server{mux: http.NewServeMux(), store: opts.Store, token: opts.Token}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("GET /version", s.handleVersion)
	s.mux.HandleFunc("POST /v1/functions", s.auth(s.handleCreate))
	s.mux.HandleFunc("GET /v1/functions", s.auth(s.handleList))
	s.mux.HandleFunc("GET /v1/functions/{name}", s.auth(s.handleGet))
	s.mux.HandleFunc("DELETE /v1/functions/{name}", s.auth(s.handleDelete))
	s.mux.HandleFunc("POST /v1/functions/{name}/deploy", s.auth(s.handleDeploy))
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.token == "" {
			next(w, r)
			return
		}
		got := bearer(r.Header.Get("Authorization"))
		if subtle.ConstantTimeCompare([]byte(got), []byte(s.token)) != 1 {
			writeJSON(w, http.StatusUnauthorized, errorBody{Error: "unauthorized"})
			return
		}
		next(w, r)
	}
}

func bearer(h string) string {
	const p = "Bearer "
	if strings.HasPrefix(h, p) {
		return strings.TrimSpace(h[len(p):])
	}
	return strings.TrimSpace(h)
}

type healthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

type errorBody struct {
	Error string `json:"error"`
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		Status:  "ok",
		Version: version.Version,
	})
}

func (s *Server) handleVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, version.Get())
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "store not configured"})
		return
	}
	var in types.Resource
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid json: " + err.Error()})
		return
	}
	if err := validateResource(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: err.Error()})
		return
	}
	out, err := s.store.Create(in)
	if errors.Is(err, store.ErrExists) {
		writeJSON(w, http.StatusConflict, errorBody{Error: "resource already exists"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) handleList(w http.ResponseWriter, _ *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "store not configured"})
		return
	}
	list, err := s.store.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "store not configured"})
		return
	}
	name := r.PathValue("name")
	res, err := s.store.Get(name)
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorBody{Error: "not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	revs, err := s.store.ListRevisions(name)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, types.ResourceView{Resource: res, Revisions: revs})
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "store not configured"})
		return
	}
	name := r.PathValue("name")
	err := s.store.Delete(name)
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorBody{Error: "not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type deployRequest struct {
	Image string `json:"image"`
}

func (s *Server) handleDeploy(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "store not configured"})
		return
	}
	name := r.PathValue("name")
	res, err := s.store.Get(name)
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorBody{Error: "not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	var req deployRequest
	if r.Body != nil {
		_ = json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req)
	}
	image := req.Image
	if image == "" {
		image = res.Image
	}
	if image == "" {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "image is required"})
		return
	}
	// Stub runner: record the revision only. Docker starts in Phase 2.
	rev, err := s.store.AddRevision(name, image, "recorded")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, rev)
}

func validateResource(r *types.Resource) error {
	if !nameRE.MatchString(r.Name) {
		return errors.New("name must match [A-Za-z0-9][A-Za-z0-9._-]{0,63}")
	}
	kind, err := types.ParseKind(string(r.Kind))
	if err != nil {
		return err
	}
	r.Kind = kind
	rt, err := types.ParseRuntime(string(r.Runtime))
	if err != nil {
		return err
	}
	r.Runtime = rt
	if r.Port == 0 {
		r.Port = 8080
	}
	if r.Health == "" {
		r.Health = "/healthz"
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
