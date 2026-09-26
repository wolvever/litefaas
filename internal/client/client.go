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

// Client talks to litefaasd.
type Client struct {
	Gateway    string
	Token      string
	HTTPClient *http.Client
}

func (c *Client) http() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 15 * time.Second}
}

func (c *Client) url(path string) string {
	return strings.TrimRight(c.Gateway, "/") + path
}

// Health calls GET /healthz.
func (c *Client) Health() (types.HealthResponse, error) {
	var out types.HealthResponse
	err := c.do(http.MethodGet, "/healthz", nil, http.StatusOK, &out)
	return out, err
}

// Version calls GET /version.
func (c *Client) Version() (types.VersionResponse, error) {
	var out types.VersionResponse
	err := c.do(http.MethodGet, "/version", nil, http.StatusOK, &out)
	return out, err
}

// List calls GET /v1/functions.
func (c *Client) List() (types.FunctionList, error) {
	var out types.FunctionList
	err := c.do(http.MethodGet, "/v1/functions", nil, http.StatusOK, &out)
	return out, err
}

// Get calls GET /v1/functions/{name}.
func (c *Client) Get(name string) (types.Resource, error) {
	var out types.Resource
	err := c.do(http.MethodGet, "/v1/functions/"+name, nil, http.StatusOK, &out)
	return out, err
}

// Create calls POST /v1/functions.
func (c *Client) Create(r types.Resource) (types.Resource, error) {
	var out types.Resource
	err := c.do(http.MethodPost, "/v1/functions", r, http.StatusCreated, &out)
	return out, err
}

// Delete calls DELETE /v1/functions/{name}.
func (c *Client) Delete(name string) error {
	return c.do(http.MethodDelete, "/v1/functions/"+name, nil, http.StatusNoContent, nil)
}

func (c *Client) do(method, path string, body any, want int, out any) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.url(path), rdr)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.http().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != want {
		var er types.ErrorResponse
		if json.Unmarshal(raw, &er) == nil && er.Error != "" {
			return fmt.Errorf("gateway %s: %s", resp.Status, er.Error)
		}
		if len(bytes.TrimSpace(raw)) == 0 {
			return fmt.Errorf("gateway %s", resp.Status)
		}
		return fmt.Errorf("gateway %s: %s", resp.Status, bytes.TrimSpace(raw))
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}
