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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wolvever/litefaas/internal/manifest"
	"github.com/wolvever/litefaas/internal/proxy"
	"github.com/wolvever/litefaas/internal/runner"
	"github.com/wolvever/litefaas/internal/secret"
	"github.com/wolvever/litefaas/internal/store"
	"github.com/wolvever/litefaas/internal/types"
	"github.com/wolvever/litefaas/internal/version"
)

var nameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// Server is the control-plane HTTP handler.
type Server struct {
	mux      *http.ServeMux
	store    *store.Store
	secrets  *secret.Store
	token    string
	runner   runner.Runner
	idleTTL  time.Duration
	idleTick time.Duration
	started  time.Time
	metrics  counters
	lastUsed sync.Map
	stop     chan struct{}
	stopOnce sync.Once
}

type Options struct {
	Store     *store.Store
	Secrets   *secret.Store
	Token     string
	Runner    runner.Runner
	IdleTTL   time.Duration // 0 disables function idle stop (default for tests)
	IdleEvery time.Duration
}

func New(opts Options) *Server {
	s := &Server{
		mux:      http.NewServeMux(),
		store:    opts.Store,
		secrets:  opts.Secrets,
		token:    opts.Token,
		runner:   opts.Runner,
		idleTTL:  opts.IdleTTL,
		idleTick: opts.IdleEvery,
		started:  time.Now(),
		stop:     make(chan struct{}),
	}
	if s.idleTick <= 0 {
		s.idleTick = 30 * time.Second
	}
	s.routes()
	if s.idleTTL > 0 {
		go s.reapLoop()
	}
	return s
}

func (s *Server) Close() {
	s.stopOnce.Do(func() { close(s.stop) })
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("GET /version", s.handleVersion)
	s.mux.HandleFunc("POST /v1/functions", s.auth(s.handleCreate))
	s.mux.HandleFunc("GET /v1/functions", s.auth(s.handleList))
	s.mux.HandleFunc("GET /v1/functions/{name}", s.auth(s.handleGet))
	s.mux.HandleFunc("GET /v1/functions/{name}/logs", s.auth(s.handleLogs))
	s.mux.HandleFunc("PUT /v1/functions/{name}", s.auth(s.handleUpdate))
	s.mux.HandleFunc("DELETE /v1/functions/{name}", s.auth(s.handleDelete))
	s.mux.HandleFunc("POST /v1/functions/{name}/deploy", s.auth(s.handleDeploy))
	s.mux.HandleFunc("POST /v1/invoke/{name}", s.auth(s.handleInvoke))
	s.mux.HandleFunc("GET /v1/metrics", s.auth(s.handleMetrics))
	s.mux.HandleFunc("GET /v1/routes", s.auth(s.handleRoutes))
	s.mux.HandleFunc("PUT /v1/routes", s.auth(s.handlePutRoutes))
	s.mux.HandleFunc("DELETE /v1/routes", s.auth(s.handleClearRoutes))
	s.mux.HandleFunc("GET /v1/edge-rules", s.auth(s.handleEdgeRules))
	s.mux.HandleFunc("PUT /v1/edge-rules", s.auth(s.handlePutEdgeRules))
	s.mux.HandleFunc("DELETE /v1/edge-rules", s.auth(s.handleClearEdgeRules))
	s.mux.HandleFunc("GET /v1/secrets", s.auth(s.handleSecretList))
	s.mux.HandleFunc("PUT /v1/secrets/{name}", s.auth(s.handleSecretPut))
	s.mux.HandleFunc("GET /v1/secrets/{name}", s.auth(s.handleSecretGet))
	s.mux.HandleFunc("DELETE /v1/secrets/{name}", s.auth(s.handleSecretDelete))
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
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		if s.token != "" {
			got := requestToken(r)
			if subtle.ConstantTimeCompare([]byte(got), []byte(s.token)) != 1 {
				rec.Header().Set("WWW-Authenticate", `Bearer realm="litefaas"`)
				writeJSON(rec, http.StatusUnauthorized, errorBody{Error: "unauthorized"})
				s.metrics.requests.Add(1)
				s.metrics.errors.Add(1)
				return
			}
		}
		next(rec, r)
		s.metrics.requests.Add(1)
		if rec.status >= 400 {
			s.metrics.errors.Add(1)
		}
	}
}

func requestToken(r *http.Request) string {
	if t := bearer(r.Header.Get("Authorization")); t != "" {
		return t
	}
	return strings.TrimSpace(r.Header.Get("X-Litefaas-Token"))
}

func bearer(h string) string {
	const p = "Bearer "
	if strings.HasPrefix(h, p) {
		return strings.TrimSpace(h[len(p):])
	}
	return ""
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
	prune := queryBool(r, "prune_volumes")
	var vols []types.VolumeMount
	if res, err := s.store.Get(name); err == nil {
		vols = res.Volumes
	}
	if s.runner != nil {
		opts := runner.RemoveOpts{PruneVolumes: prune, Volumes: vols}
		if err := s.runner.Remove(r.Context(), name, opts); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
			return
		}
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
	deployRes, err := s.withResolvedSecrets(res)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: err.Error()})
		return
	}
	out, err := s.runner.Deploy(r.Context(), deployRes)
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
	s.metrics.deploys.Add(1)
	s.touch(name)
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
	ep, err := s.ensureEndpoint(r.Context(), res)
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
		timeout = types.DefaultTimeout
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
	s.metrics.invokes.Add(1)
	s.touch(name)
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
	if r.Memory == 0 {
		r.Memory = types.DefaultMemoryMiB
	}
	if err := manifest.ValidateMemory(r.Memory); err != nil {
		return err
	}
	if r.Timeout != "" {
		if _, err := manifest.ParseTimeout(r.Timeout); err != nil {
			return err
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
	if name, ok := proxy.ParseDraftPath(r.URL.Path); ok {
		return s.handleDraftEdge(w, r, name)
	}
	rules, _ := s.loadEdgeRules()
	origPath := r.URL.Path
	proxy.MergeResponseHeaders(w.Header(), origPath, rules)
	if handled, _ := proxy.ApplyEdgeRules(w, r, rules); handled {
		return true
	}
	routes, err := s.edgeRoutes(r.Context())
	if err != nil || len(routes) == 0 {
		return false
	}
	route, ok := proxy.Match(r.URL.Path, routes)
	if !ok || route.Endpoint == "" {
		return false
	}
	proxy.Handler(route).ServeHTTP(&headerInjectWriter{ResponseWriter: w, path: origPath, rules: rules}, r)
	return true
}

// handleDraftEdge serves /--draft/<name>/… by stripping the draft prefix onto the resource endpoint.
func (s *Server) handleDraftEdge(w http.ResponseWriter, r *http.Request, name string) bool {
	if s.store == nil {
		return false
	}
	res, err := s.store.Get(name)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return true
	} else if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return true
	}
	ep, err := s.ensureEndpoint(r.Context(), res)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorBody{Error: err.Error()})
		return true
	}
	route := proxy.Route{
		Path:        proxy.DraftPrefix(name),
		Name:        name,
		Endpoint:    ep,
		StripPrefix: true,
	}
	proxy.Handler(route).ServeHTTP(w, r)
	return true
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "store not configured"})
		return
	}
	if s.runner == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "runner not configured"})
		return
	}
	name := r.PathValue("name")
	if _, err := s.store.Get(name); errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorBody{Error: "not found"})
		return
	} else if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	// Fail closed before streaming when the container is missing.
	if _, err := s.runner.Endpoint(r.Context(), name); errors.Is(err, runner.ErrNotDeployed) {
		writeJSON(w, http.StatusConflict, errorBody{Error: "not deployed"})
		return
	} else if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	opts := runner.LogsOptions{
		Follow: queryBool(r, "follow"),
		Tail:   queryInt(r, "tail", 100),
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	dest := io.Writer(w)
	if f, ok := w.(http.Flusher); ok {
		dest = flushWriter{w: w, f: f}
	}
	if err := s.runner.Logs(r.Context(), name, opts, dest); err != nil {
		if errors.Is(err, runner.ErrNotDeployed) {
			_, _ = io.WriteString(w, "not deployed\n")
			return
		}
		_, _ = io.WriteString(w, "logs: "+err.Error()+"\n")
	}
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	n := 0
	if s.store != nil {
		if list, err := s.store.List(); err == nil {
			n = len(list)
		}
	}
	idle := ""
	if s.idleTTL > 0 {
		idle = s.idleTTL.String()
	}
	writeJSON(w, http.StatusOK, snapshot{
		Started:       s.started.UTC().Truncate(time.Second),
		UptimeSeconds: int64(time.Since(s.started).Seconds()),
		Requests:      s.metrics.requests.Load(),
		Invokes:       s.metrics.invokes.Load(),
		Deploys:       s.metrics.deploys.Load(),
		Errors:        s.metrics.errors.Load(),
		Resources:     n,
		Auth:          s.token != "",
		IdleTTL:       idle,
	})
}

func (s *Server) ensureEndpoint(ctx context.Context, res types.Resource) (string, error) {
	ep, err := s.runner.Endpoint(ctx, res.Name)
	if err == nil {
		return ep, nil
	}
	if !errors.Is(err, runner.ErrNotDeployed) {
		return "", err
	}
	if res.Image == "" {
		return "", runner.ErrNotDeployed
	}
	deployRes, err := s.withResolvedSecrets(res)
	if err != nil {
		return "", err
	}
	out, err := s.runner.Deploy(ctx, deployRes)
	if err != nil {
		return "", err
	}
	s.metrics.deploys.Add(1)
	return out.Endpoint, nil
}

// withResolvedSecrets expands ${secret:name} in env. Stored metadata keeps refs.
func (s *Server) withResolvedSecrets(res types.Resource) (types.Resource, error) {
	if len(res.Env) == 0 {
		return res, nil
	}
	hasRef := false
	for _, v := range res.Env {
		if strings.Contains(v, "${secret:") {
			hasRef = true
			break
		}
	}
	if !hasRef {
		return res, nil
	}
	if s.secrets == nil {
		return res, fmt.Errorf("secrets store not configured")
	}
	resolved, err := secret.ResolveEnv(res.Env, s.secrets.Get)
	if err != nil {
		return res, err
	}
	res.Env = resolved
	return res, nil
}

func (s *Server) touch(name string) {
	if s.idleTTL <= 0 || name == "" {
		return
	}
	s.lastUsed.Store(name, time.Now())
}

func (s *Server) reapLoop() {
	t := time.NewTicker(s.idleTick)
	defer t.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-t.C:
			s.reapIdle()
		}
	}
}

func (s *Server) reapIdle() {
	if s.store == nil || s.runner == nil || s.idleTTL <= 0 {
		return
	}
	list, err := s.store.List()
	if err != nil {
		return
	}
	now := time.Now()
	for _, res := range list {
		if res.Kind != types.KindFunction {
			continue
		}
		v, ok := s.lastUsed.Load(res.Name)
		if !ok {
			continue
		}
		last, _ := v.(time.Time)
		if last.IsZero() || now.Sub(last) < s.idleTTL {
			continue
		}
		_ = s.runner.Remove(context.Background(), res.Name)
		s.lastUsed.Delete(res.Name)
	}
}

func queryBool(r *http.Request, key string) bool {
	v := strings.TrimSpace(strings.ToLower(r.URL.Query().Get(key)))
	return v == "1" || v == "true" || v == "yes"
}

func queryInt(r *http.Request, key string, fallback int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return n
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}


// headerInjectWriter adds edge header rules when the backend response is written.
type headerInjectWriter struct {
	http.ResponseWriter
	path     string
	rules    []proxy.EdgeRule
	wroteHdr bool
}

func (h *headerInjectWriter) WriteHeader(code int) {
	if !h.wroteHdr {
		proxy.MergeResponseHeaders(h.ResponseWriter.Header(), h.path, h.rules)
		h.wroteHdr = true
	}
	h.ResponseWriter.WriteHeader(code)
}

func (h *headerInjectWriter) Write(b []byte) (int, error) {
	if !h.wroteHdr {
		h.WriteHeader(http.StatusOK)
	}
	return h.ResponseWriter.Write(b)
}
