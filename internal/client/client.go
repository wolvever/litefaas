// Package client is the HTTP client lf uses against litefaasd.
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

func (c *Client) Delete(name string) error {
	return c.do(http.MethodDelete, "/v1/functions/"+name, nil, nil)
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

func (c *Client) Deploy(name, image string) (types.Revision, error) {
	var out types.Revision
	body := map[string]string{}
	if image != "" {
		body["image"] = image
	}
	if err := c.do(http.MethodPost, "/v1/functions/"+name+"/deploy", body, &out); err != nil {
		return types.Revision{}, err
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
