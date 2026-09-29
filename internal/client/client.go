// Package client is the HTTP client lf uses against litefaasd.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/wolvever/litefaas/internal/proxy"
	"github.com/wolvever/litefaas/internal/types"
)

type Client struct {
	Gateway    string
	Token      string
	HTTPClient *http.Client
}

func New(gateway, token string) *Client {
	return &Client{
		Gateway:    strings.TrimRight(gateway, "/"),
		Token:      token,
		HTTPClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *Client) Healthz() (map[string]string, error) {
	var out map[string]string
	if err := c.do(http.MethodGet, "/healthz", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) List() ([]types.Resource, error) {
	var out []types.Resource
	if err := c.do(http.MethodGet, "/v1/functions", nil, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []types.Resource{}
	}
	return out, nil
}

func (c *Client) Get(name string) (types.Resource, error) {
	var out types.Resource
	if err := c.do(http.MethodGet, "/v1/functions/"+url.PathEscape(name), nil, &out); err != nil {
		return types.Resource{}, err
	}
	return out, nil
}

func (c *Client) Create(r types.Resource) (types.Resource, error) {
	var out types.Resource
	if err := c.do(http.MethodPost, "/v1/functions", r, &out); err != nil {
		return types.Resource{}, err
	}
	return out, nil
}

func (c *Client) Update(r types.Resource) (types.Resource, error) {
	var out types.Resource
	if err := c.do(http.MethodPut, "/v1/functions/"+r.Name, r, &out); err != nil {
		return types.Resource{}, err
	}
	return out, nil
}

func (c *Client) Delete(name string, pruneVolumes ...bool) error {
	path := "/v1/functions/" + name
	if len(pruneVolumes) > 0 && pruneVolumes[0] {
		path += "?prune_volumes=1"
	}
	return c.do(http.MethodDelete, path, nil, nil)
}

func (c *Client) Routes() ([]proxy.Route, error) {
	var out []proxy.Route
	if err := c.do(http.MethodGet, "/v1/routes", nil, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []proxy.Route{}
	}
	return out, nil
}

func (c *Client) PutRoutes(routes []proxy.Route) ([]proxy.Route, error) {
	if routes == nil {
		routes = []proxy.Route{}
	}
	var out []proxy.Route
	if err := c.do(http.MethodPut, "/v1/routes", routes, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []proxy.Route{}
	}
	return out, nil
}

func (c *Client) ClearRoutes() error {
	return c.do(http.MethodDelete, "/v1/routes", nil, nil)
}

func (c *Client) EdgeRules() ([]proxy.EdgeRule, error) {
	var out []proxy.EdgeRule
	if err := c.do(http.MethodGet, "/v1/edge-rules", nil, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []proxy.EdgeRule{}
	}
	return out, nil
}

func (c *Client) PutEdgeRules(rules []proxy.EdgeRule) ([]proxy.EdgeRule, error) {
	if rules == nil {
		rules = []proxy.EdgeRule{}
	}
	var out []proxy.EdgeRule
	if err := c.do(http.MethodPut, "/v1/edge-rules", rules, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []proxy.EdgeRule{}
	}
	return out, nil
}

func (c *Client) ClearEdgeRules() error {
	return c.do(http.MethodDelete, "/v1/edge-rules", nil, nil)
}

type SecretRef struct {
	Name string `json:"name"`
	Env  string `json:"env,omitempty"`
}

type Secret struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Env   string `json:"env,omitempty"`
}

func secretEnvQuery(env string) string {
	if env == "" {
		return ""
	}
	return "?env=" + url.QueryEscape(env)
}

func (c *Client) SecretList(env string) ([]SecretRef, error) {
	var out []SecretRef
	if err := c.do(http.MethodGet, "/v1/secrets"+secretEnvQuery(env), nil, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []SecretRef{}
	}
	return out, nil
}

func (c *Client) SecretPut(name, value string) (SecretRef, error) {
	return c.SecretPutEnv("", name, value)
}

func (c *Client) SecretPutEnv(env, name, value string) (SecretRef, error) {
	var out SecretRef
	if err := c.do(http.MethodPut, "/v1/secrets/"+url.PathEscape(name)+secretEnvQuery(env), map[string]string{"value": value}, &out); err != nil {
		return SecretRef{}, err
	}
	return out, nil
}

func (c *Client) SecretGet(name string) (Secret, error) {
	return c.SecretGetEnv("", name)
}

func (c *Client) SecretGetEnv(env, name string) (Secret, error) {
	var out Secret
	if err := c.do(http.MethodGet, "/v1/secrets/"+url.PathEscape(name)+secretEnvQuery(env), nil, &out); err != nil {
		return Secret{}, err
	}
	return out, nil
}

func (c *Client) SecretDelete(name string) error {
	return c.SecretDeleteEnv("", name)
}

func (c *Client) SecretDeleteEnv(env, name string) error {
	return c.do(http.MethodDelete, "/v1/secrets/"+url.PathEscape(name)+secretEnvQuery(env), nil, nil)
}

type Metrics struct {
	Started       time.Time `json:"started"`
	UptimeSeconds int64     `json:"uptime_seconds"`
	Requests      int64     `json:"requests"`
	Invokes       int64     `json:"invokes"`
	Deploys       int64     `json:"deploys"`
	Errors        int64     `json:"errors"`
	Resources     int       `json:"resources"`
	Auth          bool      `json:"auth"`
	IdleTTL       string    `json:"idle_ttl,omitempty"`
}

func (c *Client) Metrics() (Metrics, error) {
	var out Metrics
	if err := c.do(http.MethodGet, "/v1/metrics", nil, &out); err != nil {
		return Metrics{}, err
	}
	return out, nil
}

func (c *Client) Logs(ctx context.Context, name string, follow bool, tail int, w io.Writer) error {
	if ctx == nil {
		ctx = context.Background()
	}
	q := url.Values{}
	if follow {
		q.Set("follow", "1")
	}
	if tail > 0 {
		q.Set("tail", strconv.Itoa(tail))
	}
	path := "/v1/functions/" + name + "/logs"
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Gateway+path, nil)
	if err != nil {
		return err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	httpClient := c.HTTPClient
	if follow {
		httpClient = &http.Client{}
		if c.HTTPClient != nil && c.HTTPClient.Transport != nil {
			httpClient.Transport = c.HTTPClient.Transport
		}
	} else if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("gateway %s: %w", c.Gateway, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		msg := strings.TrimSpace(string(raw))
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("%s %s: %s", http.MethodGet, "/v1/functions/"+name+"/logs", msg)
	}
	if w == nil {
		w = io.Discard
	}
	_, err = io.Copy(w, resp.Body)
	return err
}

// DeployResult is the deploy API response (flat JSON: revision fields + endpoint/container).
type DeployResult struct {
	types.Revision
	Endpoint  string `json:"endpoint,omitempty"`
	Container string `json:"container,omitempty"`
}

func (c *Client) Deploy(name, image string) (DeployResult, error) {
	return c.DeployEnv(name, image, "")
}

func (c *Client) DeployEnv(name, image, env string) (DeployResult, error) {
	var out DeployResult
	body := map[string]string{}
	if image != "" {
		body["image"] = image
	}
	path := "/v1/functions/" + name + "/deploy" + secretEnvQuery(env)
	if err := c.do(http.MethodPost, path, body, &out); err != nil {
		return DeployResult{}, err
	}
	return out, nil
}

// InvokeResult is the function's HTTP response (not a gateway envelope).
type InvokeResult struct {
	StatusCode int
	Body       []byte
	Header     http.Header
}

func (c *Client) Invoke(name string, payload []byte, contentType string, timeout time.Duration) (InvokeResult, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	var rdr io.Reader
	if payload != nil {
		rdr = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(http.MethodPost, c.Gateway+"/v1/invoke/"+name, rdr)
	if err != nil {
		return InvokeResult{}, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	} else if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	httpClient := &http.Client{Timeout: timeout + 2*time.Second}
	if c.HTTPClient != nil && c.HTTPClient.Transport != nil {
		httpClient.Transport = c.HTTPClient.Transport
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return InvokeResult{}, fmt.Errorf("gateway %s: %w", c.Gateway, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return InvokeResult{}, err
	}
	if resp.StatusCode >= 300 && looksLikeGatewayError(raw) {
		msg := strings.TrimSpace(string(raw))
		return InvokeResult{StatusCode: resp.StatusCode, Body: raw, Header: resp.Header}, fmt.Errorf("%s %s: %s", http.MethodPost, "/v1/invoke/"+name, msg)
	}
	return InvokeResult{StatusCode: resp.StatusCode, Body: raw, Header: resp.Header}, nil
}

func looksLikeGatewayError(raw []byte) bool {
	var env struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(raw, &env) != nil {
		return false
	}
	return env.Error != ""
}

func IsConflict(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "already exists")
}

func (c *Client) do(method, path string, body any, dest any) error {
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, c.Gateway+path, rdr)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("gateway %s: %w", c.Gateway, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(raw))
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("%s %s: %s", method, path, msg)
	}
	if dest == nil || resp.StatusCode == http.StatusNoContent || len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, dest)
}

