// Package api is the litefaasd HTTP surface (RFC-0001 §9).
package api

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/wolvever/litefaas/internal/runner"
	"github.com/wolvever/litefaas/internal/store"
	"github.com/wolvever/litefaas/internal/types"
	"github.com/wolvever/litefaas/internal/version"
)

// Server is the control-plane HTTP handler.
type Server struct {
	mux    *http.ServeMux
	store  *store.Store
	token  string
	runner runner.Runner
}

type Options struct {
	Store  *store.Store
	Token  string
	Runner runner.Runner
}

func New(opts Options) *Server {
	s := &Server{mux: http.NewServeMux(), store: opts.Store, token: opts.Token, runner: opts.Runner}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("GET /version", s.handleVersion)
	s.mux.HandleFunc("POST /v1/functions", s.auth(s.handleCreate))
	s.mux.HandleFunc("GET /v1/functions", s.auth(s.handleList))
	s.mux.HandleFunc("GET /v1/functions/{name}", s.auth(s.handleGet))
	s.mux.HandleFunc("PUT /v1/functions/{name}", s.auth(s.handleUpdate))
	s.mux.HandleFunc("DELETE /v1/functions/{name}", s.auth(s.handleDelete))
	s.mux.HandleFunc("POST /v1/functions/{name}/deploy", s.auth(s.handleDeploy))
	s.mux.HandleFunc("POST /v1/invoke/{name}", s.auth(s.handleInvoke))
	s.mux.HandleFunc("POST /invoke/{name}", s.auth(s.handleInvoke))
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

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "store not configured"})
		return
	}
	name := r.PathValue("name")
	var in types.Resource
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid json: " + err.Error()})
		return
	}
	if in.Name == "" {
		in.Name = name
	}
	if in.Name != name {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "name in body must match URL"})
		return
	}
	if err := validateResource(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: err.Error()})
		return
	}
	out, err := s.store.Update(in)
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorBody{Error: "not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "store not configured"})
		return
	}
	name := r.PathValue("name")
	if s.runner != nil {
		_ = s.runner.Stop(r.Context(), name)
	}
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

type deployResponse struct {
	types.Revision
	Endpoint string `json:"endpoint,omitempty"`
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
	if image != res.Image {
		res.Image = image
		if _, err := s.store.Update(res); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
			return
		}
	}

	status := "recorded"
	endpoint := ""
	if s.runner != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
		defer cancel()
		inst, err := s.runner.Deploy(ctx, res, image)
		if err != nil {
			_, _ = s.store.AddRevision(name, image, "failed")
			writeJSON(w, http.StatusBadGateway, errorBody{Error: err.Error()})
			return
		}
		status = "running"
		endpoint = inst.Endpoint
	}
	rev, err := s.store.AddRevision(name, image, status)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, deployResponse{Revision: rev, Endpoint: endpoint})
}

func (s *Server) handleInvoke(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "store not configured"})
		return
	}
	if s.runner == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "runner not configured"})
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
	if res.Kind != types.KindFunction {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invoke is only supported for kind=function"})
		return
	}
	inst, err := s.runner.Lookup(r.Context(), name)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errorBody{Error: err.Error()})
		return
	}
	timeout := parseTimeout(res.Timeout)
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	payload, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: err.Error()})
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(inst.Endpoint, "/")+"/", bytes.NewReader(payload))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	if ct := r.Header.Get("Content-Type"); ct != "" {
		req.Header.Set("Content-Type", ct)
	} else {
		req.Header.Set("Content-Type", "application/octet-stream")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorBody{Error: "invoke: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorBody{Error: err.Error()})
		return
	}
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(out)
}

func parseTimeout(s string) time.Duration {
	if s == "" {
		return 60 * time.Second
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return 60 * time.Second
	}
	return d
}

func validateResource(r *types.Resource) error {
	if !types.ValidName(r.Name) {
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
