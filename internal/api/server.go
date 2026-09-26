package api

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/wolvever/litefaas/internal/store"
	"github.com/wolvever/litefaas/internal/types"
	"github.com/wolvever/litefaas/internal/version"
)

// Server is the litefaasd HTTP API.
type Server struct {
	store *store.Store
	token string
	mux   http.Handler
}

// New constructs a server. If token is empty, mutating/list routes are open (dev mode).
func New(st *store.Store, token string) *Server {
	s := &Server{store: st, token: token}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /version", s.version)
	mux.HandleFunc("GET /v1/functions", s.requireAuth(s.listFunctions))
	mux.HandleFunc("POST /v1/functions", s.requireAuth(s.createFunction))
	mux.HandleFunc("GET /v1/functions/{name}", s.requireAuth(s.getFunction))
	mux.HandleFunc("DELETE /v1/functions/{name}", s.requireAuth(s.deleteFunction))
	mux.HandleFunc("POST /v1/functions/{name}/deploy", s.requireAuth(s.deployFunction))
	s.mux = withLogging(mux)
	return s
}

// Handler returns the HTTP handler.
func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, types.HealthResponse{
		Status:  "ok",
		Version: version.Version,
	})
}

func (s *Server) version(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, types.VersionResponse{
		Name:    version.Name,
		Version: version.Version,
	})
}

func (s *Server) listFunctions(w http.ResponseWriter, _ *http.Request) {
	items, err := s.store.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, types.FunctionList{Items: items})
}

func (s *Server) createFunction(w http.ResponseWriter, r *http.Request) {
	var body types.Resource
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	created, err := s.store.Create(body)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrExists):
			writeError(w, http.StatusConflict, "resource already exists")
		case isClientErr(err):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) getFunction(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	got, err := s.store.Get(name)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, got)
}

func (s *Server) deleteFunction(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.store.Delete(name); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deployFunction(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var body types.DeployRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	rev, err := s.store.Deploy(name, body.Image)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrNotFound):
			writeError(w, http.StatusNotFound, "not found")
		case isClientErr(err):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, types.DeployResponse{Name: name, Revision: rev})
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.token == "" {
			next(w, r)
			return
		}
		got := bearerToken(r.Header.Get("Authorization"))
		if !tokenEqual(s.token, got) {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r)
	}
}

func bearerToken(h string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(h, prefix))
}

func tokenEqual(want, got string) bool {
	if subtle.ConstantTimeCompare([]byte(want), []byte(got)) == 1 {
		return true
	}
	return false
}

func isClientErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "must be") ||
		strings.Contains(msg, "required") ||
		strings.Contains(msg, "invalid")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	_ = enc.Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, types.ErrorResponse{Error: msg})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		slog.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"dur", time.Since(start),
		)
	})
}
