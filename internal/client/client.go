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
		HTTPClient: &http.Client{Timeout: 15 * time.Second},
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

var ErrNotFound = fmt.Errorf("not found")

func (c *Client) Create(r types.Resource) (types.Resource, error) {
	var out types.Resource
	if err := c.do(http.MethodPost, "/v1/functions", r, &out); err != nil {
		return types.Resource{}, err
	}
	return out, nil
}

func (c *Client) Get(name string) (types.ResourceView, error) {
	var out types.ResourceView
	if err := c.do(http.MethodGet, "/v1/functions/"+name, nil, &out); err != nil {
		return types.ResourceView{}, err
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

func (c *Client) Deploy(name, image string) (types.Revision, string, error) {
	var out struct {
		types.Revision
		Endpoint string `json:"endpoint"`
	}
	hc := c.clientWithTimeout(2 * time.Minute)
	if err := c.doWith(hc, http.MethodPost, "/v1/functions/"+name+"/deploy", map[string]string{"image": image}, &out); err != nil {
		return types.Revision{}, "", err
	}
	return out.Revision, out.Endpoint, nil
}

// Invoke POSTs payload to /v1/invoke/{name} and returns the function HTTP status and body.
func (c *Client) Invoke(name string, payload []byte, timeout time.Duration) (int, []byte, error) {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	hc := c.clientWithTimeout(timeout + 2*time.Second)
	req, err := http.NewRequest(http.MethodPost, c.Gateway+"/v1/invoke/"+name, bytes.NewReader(payload))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("gateway %s: %w", c.Gateway, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, raw, nil
}

func (c *Client) clientWithTimeout(d time.Duration) *http.Client {
	hc := &http.Client{Timeout: d}
	if c.HTTPClient != nil {
		hc.Transport = c.HTTPClient.Transport
	}
	return hc
}

func (c *Client) do(method, path string, body any, dest any) error {
	return c.doWith(c.HTTPClient, method, path, body, dest)
}

func (c *Client) doWith(hc *http.Client, method, path string, body any, dest any) error {
	if hc == nil {
		hc = http.DefaultClient
	}
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
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("gateway %s: %w", c.Gateway, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("%w: %s", ErrNotFound, strings.TrimSpace(string(raw)))
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
