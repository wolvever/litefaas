// Package api is the litefaasd HTTP surface (RFC-0001 §9).
package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/wolvever/litefaas/internal/manifest"
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
	s.mux.HandleFunc("PUT /v1/functions/{name}", s.auth(s.handleUpdate))
	s.mux.HandleFunc("DELETE /v1/functions/{name}", s.auth(s.handleDelete))
	s.mux.HandleFunc("POST /v1/functions/{name}/deploy", s.auth(s.handleDeploy))
	s.mux.HandleFunc("POST /v1/invoke/{name}", s.auth(s.handleInvoke))
	s.mux.HandleFunc("GET /v1/routes", s.auth(s.handleRoutes))
	s.mux.HandleFunc("PUT /v1/routes", s.auth(s.handlePutRoutes))
	s.mux.HandleFunc("DELETE /v1/routes", s.auth(s.handleClearRoutes))
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
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "name in body must match path"})
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
		_ = s.runner.Remove(r.Context(), name)
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
	Endpoint  string `json:"endpoint,omitempty"`
	Container string `json:"container,omitempty"`
}

func (s *Server) handleDeploy(w http.ResponseWriter, r *http.Request) {
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
	var req deployRequest
	if r.Body != nil {
		_ = json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req)
	}
	if req.Image != "" {
		res.Image = req.Image
	}
	if res.Image == "" {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "image is required"})
		return
	}
	if _, err := s.store.Update(res); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	out, err := s.runner.Deploy(r.Context(), res)
	if err != nil {
		_, _ = s.store.AddRevision(name, res.Image, "failed")
		writeJSON(w, http.StatusBadGateway, errorBody{Error: "deploy: " + err.Error()})
		return
	}
	rev, err := s.store.AddRevision(name, res.Image, "deployed")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, deployResponse{Revision: rev, Endpoint: out.Endpoint, Container: out.Container})
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
	ep, err := s.runner.Endpoint(r.Context(), name)
	if errors.Is(err, runner.ErrNotDeployed) {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "function is not deployed"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorBody{Error: err.Error()})
		return
	}
	timeout, err := manifest.ParseTimeout(res.Timeout)
	if err != nil {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	target := strings.TrimRight(ep, "/") + "/"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	if ct := r.Header.Get("Content-Type"); ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeJSON(w, http.StatusGatewayTimeout, errorBody{Error: "invoke: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	for _, h := range []string{"Content-Type", "Content-Length"} {
		if v := resp.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(resp.Body, 1<<20))
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

func (s *Server) endpoints(ctx context.Context, names ...string) map[string]string {
	eps := map[string]string{}
	if s.runner == nil {
		return eps
	}
	seen := map[string]struct{}{}
	for _, name := range names {
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		ep, err := s.runner.Endpoint(ctx, name)
		if err != nil {
			continue
		}
		eps[name] = ep
	}
	return eps
}

func (s *Server) edgeRoutes(ctx context.Context) ([]proxy.Route, error) {
	if s.store == nil {
		return nil, nil
	}
	override, ok, err := s.store.GetRouteOverride()
	if err != nil {
		return nil, err
	}
	if ok {
		names := make([]string, 0, len(override))
		for _, r := range override {
			names = append(names, r.Name)
		}
		eps := s.endpoints(ctx, names...)
		out := make([]proxy.Route, 0, len(override))
		for _, r := range override {
			out = append(out, proxy.Route{
				Path:        r.Path,
				Name:        r.Name,
				Endpoint:    eps[r.Name],
				StripPrefix: r.StripPrefix,
				SPA:         r.SPA,
			})
		}
		return out, nil
	}
	list, err := s.store.List()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(list))
	for _, res := range list {
		names = append(names, res.Name)
	}
	return proxy.FromResources(list, s.endpoints(ctx, names...)), nil
}

func (s *Server) handleRoutes(w http.ResponseWriter, r *http.Request) {
	routes, err := s.edgeRoutes(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	if routes == nil {
		routes = []proxy.Route{}
	}
	writeJSON(w, http.StatusOK, routes)
}

func (s *Server) handlePutRoutes(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "store not configured"})
		return
	}
	var in []store.RouteSpec
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid json: " + err.Error()})
		return
	}
	if in == nil {
		in = []store.RouteSpec{}
	}
	for i, rt := range in {
		if strings.TrimSpace(rt.Path) == "" {
			writeJSON(w, http.StatusBadRequest, errorBody{Error: fmt.Sprintf("routes[%d]: path is required", i)})
			return
		}
		if !strings.HasPrefix(rt.Path, "/") {
			writeJSON(w, http.StatusBadRequest, errorBody{Error: fmt.Sprintf("routes[%d]: path must start with /", i)})
			return
		}
		if strings.TrimSpace(rt.Name) == "" {
			writeJSON(w, http.StatusBadRequest, errorBody{Error: fmt.Sprintf("routes[%d]: name is required", i)})
			return
		}
	}
	if err := s.store.SetRouteOverride(in); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	routes, err := s.edgeRoutes(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, routes)
}

func (s *Server) handleClearRoutes(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "store not configured"})
		return
	}
	if err := s.store.ClearRouteOverride(); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleEdge(w http.ResponseWriter, r *http.Request) bool {
	routes, err := s.edgeRoutes(r.Context())
	if err != nil || len(routes) == 0 {
		return false
	}
	route, ok := proxy.Match(r.URL.Path, routes)
	if !ok || route.Endpoint == "" {
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
