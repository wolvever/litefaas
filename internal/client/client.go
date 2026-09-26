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
		HTTPClient: &http.Client{Timeout: 120 * time.Second},
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

func (c *Client) Delete(name string) error {
	return c.do(http.MethodDelete, "/v1/functions/"+name, nil, nil)
}

func (c *Client) Create(r types.Resource) (types.Resource, error) {
	var out types.Resource
	if err := c.do(http.MethodPost, "/v1/functions", r, &out); err != nil {
		return types.Resource{}, err
	}
	return out, nil
}

func (c *Client) Deploy(name, image string) (types.Revision, error) {
	var out types.Revision
	if err := c.do(http.MethodPost, "/v1/functions/"+name+"/deploy", types.DeployRequest{Image: image}, &out); err != nil {
		return types.Revision{}, err
	}
	return out, nil
}

func (c *Client) Invoke(name string, body []byte, contentType string) ([]byte, int, error) {
	req, err := http.NewRequest(http.MethodPost, c.Gateway+"/invoke/"+name, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	if contentType == "" {
		contentType = "application/json"
	}
	req.Header.Set("Content-Type", contentType)
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 70 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("gateway %s: %w", c.Gateway, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode >= 300 && looksJSONError(raw) {
		return raw, resp.StatusCode, fmt.Errorf("invoke %s: %s", name, strings.TrimSpace(string(raw)))
	}
	return raw, resp.StatusCode, nil
}

func looksJSONError(raw []byte) bool {
	return bytes.Contains(raw, []byte(`"error"`))
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
