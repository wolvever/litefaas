// Package api is the litefaasd HTTP surface (RFC-0001 §9).
package api

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/wolvever/litefaas/internal/proxy"
	"github.com/wolvever/litefaas/internal/runner"
	"github.com/wolvever/litefaas/internal/store"
	"github.com/wolvever/litefaas/internal/types"
	"github.com/wolvever/litefaas/internal/version"
)

var nameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

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
	s.mux.HandleFunc("DELETE /v1/functions/{name}", s.auth(s.handleDelete))
	s.mux.HandleFunc("POST /v1/functions/{name}/deploy", s.auth(s.handleDeploy))
	s.mux.HandleFunc("POST /invoke/{name}", s.auth(s.handleInvoke))
	s.mux.HandleFunc("POST /v1/invoke/{name}", s.auth(s.handleInvoke))
	s.mux.HandleFunc("GET /v1/routes", s.auth(s.handleRoutes))
}

func isControlPath(p string) bool {
	switch {
	case p == "/healthz", p == "/version":
		return true
	case strings.HasPrefix(p, "/v1/"), strings.HasPrefix(p, "/invoke/"):
		return true
	default:
		return false
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if isControlPath(r.URL.Path) {
		s.mux.ServeHTTP(w, r)
		return
	}
	if s.handleEdge(w, r) {
		return
	}
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
	_, err := s.store.Get(in.Name)
	exists := err == nil
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	out, err := s.store.Upsert(in)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	status := http.StatusCreated
	if exists {
		status = http.StatusOK
	}
	writeJSON(w, status, out)
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
	view := types.ResourceView{Resource: res, Revisions: revs}
	if inst, err := s.store.GetInstance(name); err == nil {
		view.Instance = &inst
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "store not configured"})
		return
	}
	name := r.PathValue("name")
	if s.runner != nil {
		_ = s.runner.Stop(name)
	}
	_ = s.store.DeleteInstance(name)
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
	res.Image = image
	if _, err := s.store.Update(res); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	status := "recorded"
	if s.runner != nil {
		inst, err := s.runner.Deploy(res)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, errorBody{Error: err.Error()})
			return
		}
		if err := s.store.PutInstance(inst); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
			return
		}
		status = inst.Status
	}
	rev, err := s.store.AddRevision(name, image, status)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, rev)
}

func (s *Server) handleInvoke(w http.ResponseWriter, r *http.Request) {
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
	inst, err := s.store.GetInstance(name)
	if errors.Is(err, store.ErrNotFound) || inst.Endpoint == "" {
		writeJSON(w, http.StatusConflict, errorBody{Error: "resource is not deployed"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: err.Error()})
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	req, err := http.NewRequest(http.MethodPost, inst.Endpoint+path, bytes.NewReader(body))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	if ct := r.Header.Get("Content-Type"); ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	client := &http.Client{Timeout: res.TimeoutDuration()}
	resp, err := client.Do(req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorBody{Error: err.Error()})
		return
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	for _, h := range []string{"Content-Type", "Content-Length"} {
		if v := resp.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(out)
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
		if r.Kind == types.KindFrontend {
			r.Health = "/"
		} else {
			r.Health = "/healthz"
		}
	}
	return nil
}

func (s *Server) routesFromStore() ([]proxy.Route, error) {
	if s.store == nil {
		return nil, nil
	}
	list, err := s.store.List()
	if err != nil {
		return nil, err
	}
	insts := map[string]types.Instance{}
	for _, res := range list {
		if inst, err := s.store.GetInstance(res.Name); err == nil {
			insts[res.Name] = inst
		}
	}
	return proxy.FromResources(list, insts), nil
}

func (s *Server) handleRoutes(w http.ResponseWriter, _ *http.Request) {
	routes, err := s.routesFromStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	if routes == nil {
		routes = []proxy.Route{}
	}
	writeJSON(w, http.StatusOK, routes)
}

func (s *Server) handleEdge(w http.ResponseWriter, r *http.Request) bool {
	routes, err := s.routesFromStore()
	if err != nil || len(routes) == 0 {
		return false
	}
	route, ok := proxy.Match(r.URL.Path, routes)
	if !ok {
		return false
	}
	proxy.Handler(route).ServeHTTP(w, r)
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
