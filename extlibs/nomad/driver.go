// Package nomad implements the scriptling.nomad extended library: a thin
// client over the HashiCorp Nomad HTTP API covering CSI volumes and jobs.
package nomad

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/paularlott/scriptling/pool"
)

const (
	// DefaultAddr is used when no address is supplied to Client().
	DefaultAddr = "http://127.0.0.1:4646"

	// DefaultTimeout is the per-request HTTP timeout used when none is given
	// to Client().
	DefaultTimeout = 10 * time.Second
)

// client talks to the Nomad HTTP API.
type client struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

// newClient builds a client from an address, ACL token, TLS skip-verify flag,
// and per-request timeout (0 = use DefaultTimeout).
//
// The underlying connection pool (transport) is shared via the scriptling
// pool package, keeping the two entirely separate depending on
// insecureSkipVerify: a Client() call that opts into skipping TLS
// verification for its own Nomad cluster never affects the transport used by
// TLS-verified clients (nomad or otherwise) elsewhere in the process. Only
// the per-request timeout is customized per instance.
func newClient(addr, token string, insecureSkipVerify bool, timeout time.Duration) *client {
	if addr == "" {
		addr = DefaultAddr
	}
	addr = strings.TrimRight(addr, "/")

	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	var transport http.RoundTripper
	if insecureSkipVerify {
		transport = pool.GetInsecureHTTPClient().Transport
	} else {
		transport = pool.GetHTTPClient().Transport
	}

	return &client{
		httpClient: &http.Client{Transport: transport, Timeout: timeout},
		baseURL:    addr,
		token:      token,
	}
}

func (c *client) url(path string, query map[string]string) string {
	u := c.baseURL + path
	if len(query) > 0 {
		values := make([]string, 0, len(query))
		for k, v := range query {
			if v == "" {
				continue
			}
			values = append(values, k+"="+v)
		}
		if len(values) > 0 {
			u += "?" + strings.Join(values, "&")
		}
	}
	return u
}

func (c *client) do(ctx context.Context, method, path string, query map[string]string, body any) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.url(path, query), bodyReader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("X-Nomad-Token", c.token)
	}
	return c.httpClient.Do(req)
}

// doJSON performs a request and decodes a JSON response body into out (if non-nil).
// Non-2xx responses are turned into an error containing the response body text.
func (c *client) doJSON(ctx context.Context, method, path string, query map[string]string, body, out any) error {
	resp, err := c.do(ctx, method, path, query, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("nomad API error (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("failed to decode nomad response: %w", err)
		}
	}
	return nil
}

// ── CSI Volumes ──────────────────────────────────────────────────────────────

// CSIVolumeListEntry is a summary entry as returned by the volume list endpoint.
type CSIVolumeListEntry struct {
	ID                 string `json:"ID"`
	Name               string `json:"Name"`
	Namespace          string `json:"Namespace"`
	PluginID           string `json:"PluginID"`
	Provider           string `json:"Provider"`
	Schedulable        bool   `json:"Schedulable"`
	ControllersHealthy int    `json:"ControllersHealthy"`
	NodesHealthy       int    `json:"NodesHealthy"`
}

// CSIVolumesList lists CSI volumes, optionally filtered by namespace ("*" for all)
// and/or plugin ID (empty = no plugin filter).
func (c *client) CSIVolumesList(ctx context.Context, namespace, pluginID string) ([]CSIVolumeListEntry, error) {
	query := map[string]string{"type": "csi"}
	if namespace != "" {
		query["namespace"] = namespace
	}
	if pluginID != "" {
		query["plugin_id"] = pluginID
	}
	var out []CSIVolumeListEntry
	if err := c.doJSON(ctx, http.MethodGet, "/v1/volumes", query, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CSIVolumeGet fetches full details for a single CSI volume.
func (c *client) CSIVolumeGet(ctx context.Context, id, namespace string) (map[string]any, error) {
	query := map[string]string{}
	if namespace != "" {
		query["namespace"] = namespace
	}
	var out map[string]any
	if err := c.doJSON(ctx, http.MethodGet, "/v1/volume/csi/"+id, query, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CSIVolumeRegister registers (creates) one or more CSI volumes. spec is the
// raw {"Volumes": [...]} payload, or a single volume spec (wrapped automatically).
func (c *client) CSIVolumeRegister(ctx context.Context, id, namespace string, volume map[string]any) error {
	body := map[string]any{"Volumes": []map[string]any{volume}}
	query := map[string]string{}
	if namespace != "" {
		query["namespace"] = namespace
	}
	return c.doJSON(ctx, http.MethodPut, "/v1/volume/csi/"+id, query, body, nil)
}

// CSIVolumeDeregister deregisters (deletes) a CSI volume. force detaches any
// remaining claims first.
func (c *client) CSIVolumeDeregister(ctx context.Context, id, namespace string, force bool) error {
	query := map[string]string{}
	if namespace != "" {
		query["namespace"] = namespace
	}
	if force {
		query["force"] = "true"
	}
	return c.doJSON(ctx, http.MethodDelete, "/v1/volume/csi/"+id, query, nil, nil)
}

// ── Jobs ─────────────────────────────────────────────────────────────────────

// JobListEntry is a summary entry as returned by the job list endpoint.
type JobListEntry struct {
	ID        string `json:"ID"`
	Name      string `json:"Name"`
	Namespace string `json:"Namespace"`
	Type      string `json:"Type"`
	Status    string `json:"Status"`
	Priority  int    `json:"Priority"`
}

// JobsList lists jobs, optionally filtered by namespace ("*" for all) and/or
// an ID prefix.
func (c *client) JobsList(ctx context.Context, namespace, prefix string) ([]JobListEntry, error) {
	query := map[string]string{}
	if namespace != "" {
		query["namespace"] = namespace
	}
	if prefix != "" {
		query["prefix"] = prefix
	}
	var out []JobListEntry
	if err := c.doJSON(ctx, http.MethodGet, "/v1/jobs", query, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// JobGet fetches the full job specification/status.
func (c *client) JobGet(ctx context.Context, id, namespace string) (map[string]any, error) {
	query := map[string]string{}
	if namespace != "" {
		query["namespace"] = namespace
	}
	var out map[string]any
	if err := c.doJSON(ctx, http.MethodGet, "/v1/job/"+id, query, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// JobRegister submits (creates or updates) a job. job is the job specification
// as a Go map (already in Nomad's JSON job format).
func (c *client) JobRegister(ctx context.Context, job map[string]any) (map[string]any, error) {
	body := map[string]any{"Job": job}
	var out map[string]any
	if err := c.doJSON(ctx, http.MethodPost, "/v1/jobs", nil, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// JobStop stops a job. purge fully removes the job from Nomad's state instead
// of just marking it stopped.
func (c *client) JobStop(ctx context.Context, id, namespace string, purge bool) (map[string]any, error) {
	query := map[string]string{}
	if namespace != "" {
		query["namespace"] = namespace
	}
	if purge {
		query["purge"] = "true"
	}
	var out map[string]any
	if err := c.doJSON(ctx, http.MethodDelete, "/v1/job/"+id, query, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// JobValidate validates a job specification without submitting it.
func (c *client) JobValidate(ctx context.Context, job map[string]any) (map[string]any, error) {
	body := map[string]any{"Job": job}
	var out map[string]any
	if err := c.doJSON(ctx, http.MethodPost, "/v1/validate/job", nil, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// JobPlan dry-runs a job registration and returns the resulting scheduler plan.
func (c *client) JobPlan(ctx context.Context, id string, job map[string]any, diff bool) (map[string]any, error) {
	body := map[string]any{"Job": job, "Diff": diff}
	var out map[string]any
	if err := c.doJSON(ctx, http.MethodPost, "/v1/job/"+id+"/plan", nil, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// JobsParse converts an HCL job specification into Nomad's JSON job format.
func (c *client) JobsParse(ctx context.Context, hcl string, canonicalize bool) (map[string]any, error) {
	body := map[string]any{"JobHCL": hcl, "Canonicalize": canonicalize}
	var out map[string]any
	if err := c.doJSON(ctx, http.MethodPost, "/v1/jobs/parse", nil, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// WaitJobStopped polls a job's status until it is "dead" (fully stopped), or
// the timeout elapses. Returns true if the job reached a dead/absent state.
func (c *client) WaitJobStopped(ctx context.Context, id, namespace string, timeout time.Duration) (bool, error) {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	check := func() (bool, error) {
		job, err := c.JobGet(ctx, id, namespace)
		if err != nil {
			// Job no longer exists (e.g. purged): treat as stopped.
			return true, nil
		}
		status, _ := job["Status"].(string)
		return status == "dead", nil
	}

	for {
		stopped, err := check()
		if err != nil {
			return false, err
		}
		if stopped {
			return true, nil
		}
		if time.Now().After(deadline) {
			return false, nil
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-ticker.C:
		}
	}
}

// parseBoolQuery is a small helper used by the library layer for kwargs that
// arrive as either bool or string.
func parseBoolQuery(s string) bool {
	b, _ := strconv.ParseBool(s)
	return b
}
