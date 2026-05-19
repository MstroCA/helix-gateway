package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	helixmetrics "helix/internal/metrics"
	"helix/internal/store"
)

type Client struct {
	base string
	hc   *http.Client
	user string
	pass string
}

func New(baseURL, user, pass string) *Client {
	return &Client{
		base: baseURL,
		hc:   &http.Client{Timeout: 10 * time.Second},
		user: user,
		pass: pass,
	}
}

func (c *Client) ListRoutes() ([]*store.Route, error) {
	var out []*store.Route
	return out, c.do(http.MethodGet, "/admin/v1/routes", nil, &out)
}

func (c *Client) GetRoute(id string) (*store.Route, error) {
	var out store.Route
	return &out, c.do(http.MethodGet, "/admin/v1/routes/"+id, nil, &out)
}

func (c *Client) CreateRoute(r *store.Route) (*store.Route, error) {
	var out store.Route
	return &out, c.do(http.MethodPost, "/admin/v1/routes", r, &out)
}

func (c *Client) UpdateRoute(id string, r *store.Route) (*store.Route, error) {
	var out store.Route
	return &out, c.do(http.MethodPut, "/admin/v1/routes/"+id, r, &out)
}

func (c *Client) DeleteRoute(id string) error {
	return c.do(http.MethodDelete, "/admin/v1/routes/"+id, nil, nil)
}

func (c *Client) ToggleRoute(id string) (*store.Route, error) {
	var out store.Route
	return &out, c.do(http.MethodPatch, "/admin/v1/routes/"+id+"/toggle", nil, &out)
}

func (c *Client) ListUpstreams() ([]*store.Upstream, error) {
	var out []*store.Upstream
	return out, c.do(http.MethodGet, "/admin/v1/upstreams", nil, &out)
}

func (c *Client) GetUpstream(id string) (*store.Upstream, error) {
	var out store.Upstream
	return &out, c.do(http.MethodGet, "/admin/v1/upstreams/"+id, nil, &out)
}

func (c *Client) CreateUpstream(u *store.Upstream) (*store.Upstream, error) {
	var out store.Upstream
	return &out, c.do(http.MethodPost, "/admin/v1/upstreams", u, &out)
}

func (c *Client) UpdateUpstream(id string, u *store.Upstream) (*store.Upstream, error) {
	var out store.Upstream
	return &out, c.do(http.MethodPut, "/admin/v1/upstreams/"+id, u, &out)
}

func (c *Client) DeleteUpstream(id string) error {
	return c.do(http.MethodDelete, "/admin/v1/upstreams/"+id, nil, nil)
}

func (c *Client) HealthUpstream(id string) (map[string]any, error) {
	var out map[string]any
	return out, c.do(http.MethodGet, "/admin/v1/upstreams/"+id+"/health", nil, &out)
}

func (c *Client) ListIPPolicies() ([]*store.IPPolicy, error) {
	var out []*store.IPPolicy
	return out, c.do(http.MethodGet, "/admin/v1/policies", nil, &out)
}

func (c *Client) GetIPPolicy(id string) (*store.IPPolicy, error) {
	var out store.IPPolicy
	return &out, c.do(http.MethodGet, "/admin/v1/policies/"+id, nil, &out)
}

func (c *Client) CreateIPPolicy(p *store.IPPolicy) (*store.IPPolicy, error) {
	var out store.IPPolicy
	return &out, c.do(http.MethodPost, "/admin/v1/policies", p, &out)
}

func (c *Client) UpdateIPPolicy(id string, p *store.IPPolicy) (*store.IPPolicy, error) {
	var out store.IPPolicy
	return &out, c.do(http.MethodPut, "/admin/v1/policies/"+id, p, &out)
}

func (c *Client) DeleteIPPolicy(id string) error {
	return c.do(http.MethodDelete, "/admin/v1/policies/"+id, nil, nil)
}

func (c *Client) ToggleIPPolicy(id string) (*store.IPPolicy, error) {
	var out store.IPPolicy
	return &out, c.do(http.MethodPatch, "/admin/v1/policies/"+id+"/toggle", nil, &out)
}

func (c *Client) ListAPIKeys() ([]map[string]any, error) {
	var out []map[string]any
	return out, c.do(http.MethodGet, "/admin/v1/keys", nil, &out)
}

func (c *Client) GetAPIKey(id string) (map[string]any, error) {
	var out map[string]any
	return out, c.do(http.MethodGet, "/admin/v1/keys/"+id, nil, &out)
}

func (c *Client) DeleteAPIKey(id string) error {
	return c.do(http.MethodDelete, "/admin/v1/keys/"+id, nil, nil)
}

func (c *Client) ToggleAPIKey(id string) (map[string]any, error) {
	var out map[string]any
	return out, c.do(http.MethodPatch, "/admin/v1/keys/"+id+"/toggle", nil, &out)
}

func (c *Client) GetMetrics() (*helixmetrics.Snapshot, error) {
	var out helixmetrics.Snapshot
	return &out, c.do(http.MethodGet, "/admin/v1/metrics", nil, &out)
}

// Stream opens a long-lived GET to path (e.g. SSE endpoint) with no timeout.
// The caller is responsible for closing resp.Body.
func (c *Client) Stream(path string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, c.base+path, nil)
	if err != nil {
		return nil, err
	}
	if c.user != "" && c.pass != "" {
		req.SetBasicAuth(c.user, c.pass)
	}
	return http.DefaultClient.Do(req)
}

func (c *Client) do(method, path string, body, out any) error {
	var buf *bytes.Buffer
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		buf = bytes.NewBuffer(data)
	} else {
		buf = &bytes.Buffer{}
	}

	req, err := http.NewRequest(method, c.base+path, buf)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.user != "" && c.pass != "" {
		req.SetBasicAuth(c.user, c.pass)
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("http %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if resp.StatusCode >= 400 {
		var apiErr struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		return fmt.Errorf("admin api %s %s: %s — %s", method, path,
			apiErr.Error.Code, apiErr.Error.Message)
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}
