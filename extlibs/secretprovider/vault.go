package secretprovider

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type vaultProvider struct {
	address      string
	token        string
	namespace    string
	defaultField string
	kvVersion    int
	client       *http.Client
}

// NewVaultProvider creates a Vault-backed secret provider.
func NewVaultProvider(cfg Config) (Provider, error) {
	if cfg.Address == "" {
		return nil, fmt.Errorf("secret provider %q requires address", cfg.Provider)
	}
	if cfg.Token == "" && (cfg.AppRoleID == "" || cfg.AppRoleSecret == "") {
		return nil, fmt.Errorf("vault secret provider requires token or app_role credentials")
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: cfg.InsecureSkipTLS,
			},
		},
	}

	provider := &vaultProvider{
		address:      strings.TrimRight(cfg.Address, "/"),
		token:        cfg.Token,
		namespace:    cfg.Namespace,
		defaultField: cfg.DefaultField,
		kvVersion:    cfg.KVVersion,
		client:       client,
	}

	if provider.defaultField == "" {
		provider.defaultField = "value"
	}

	if provider.token == "" {
		token, err := provider.loginWithAppRole(context.Background(), cfg.AppRoleID, cfg.AppRoleSecret)
		if err != nil {
			return nil, err
		}
		provider.token = token
	}

	return provider, nil
}

func (v *vaultProvider) ID() string {
	return "vault"
}

func (v *vaultProvider) Resolve(ctx context.Context, path, field string) (string, error) {
	if field == "" {
		field = v.defaultField
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.address+"/v1/"+strings.TrimLeft(path, "/"), nil)
	if err != nil {
		return "", fmt.Errorf("vault: create request: %w", err)
	}
	v.decorate(req)

	resp, err := v.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("vault: read %q: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("vault: read %q returned %s: %s", path, resp.Status, strings.TrimSpace(string(body)))
	}

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("vault: decode %q: %w", path, err)
	}

	data, err := extractVaultData(payload, path, v.kvVersion)
	if err != nil {
		return "", err
	}

	value, ok := data[field]
	if !ok {
		return "", fmt.Errorf("vault: field %q not found at %q", field, path)
	}

	resolved, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("vault: field %q at %q is not a string", field, path)
	}

	return resolved, nil
}

func (v *vaultProvider) List(ctx context.Context, path string) ([]string, error) {
	reqPath := strings.TrimLeft(path, "/")

	// Try reading the secret first — if it exists, return its field names.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.address+"/v1/"+reqPath, nil)
	if err != nil {
		return nil, fmt.Errorf("vault: create list request: %w", err)
	}
	v.decorate(req)

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vault: list %q: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var payload map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return nil, fmt.Errorf("vault: decode list %q: %w", path, err)
		}
		data, err := extractVaultData(payload, path, v.kvVersion)
		if err != nil {
			return nil, err
		}
		keys := make([]string, 0, len(data))
		for key := range data {
			keys = append(keys, key)
		}
		return keys, nil
	}

	// Not a leaf secret — try listing sub-paths via the metadata endpoint.
	listPath := reqPath
	if v.kvVersion != 1 {
		if idx := strings.Index(listPath, "/data/"); idx >= 0 {
			listPath = listPath[:idx] + "/metadata/" + listPath[idx+6:]
		} else if strings.HasSuffix(listPath, "/data") {
			listPath = listPath[:len(listPath)-5] + "/metadata"
		}
	}

	req, err = http.NewRequestWithContext(ctx, http.MethodGet, v.address+"/v1/"+listPath+"?list=true", nil)
	if err != nil {
		return nil, fmt.Errorf("vault: create list request: %w", err)
	}
	v.decorate(req)

	resp, err = v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vault: list %q: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("vault: list %q returned %s: %s", path, resp.Status, strings.TrimSpace(string(body)))
	}

	var listPayload struct {
		Data struct {
			Keys []string `json:"keys"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listPayload); err != nil {
		return nil, fmt.Errorf("vault: decode list %q: %w", path, err)
	}

	return listPayload.Data.Keys, nil
}

func (v *vaultProvider) loginWithAppRole(ctx context.Context, roleID, secret string) (string, error) {
	body, err := json.Marshal(map[string]string{
		"role_id":   roleID,
		"secret_id": secret,
	})
	if err != nil {
		return "", fmt.Errorf("vault: encode approle login: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.address+"/v1/auth/approle/login", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("vault: create approle login request: %w", err)
	}
	v.decorate(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("vault: approle login: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("vault: approle login returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Auth struct {
			ClientToken string `json:"client_token"`
		} `json:"auth"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("vault: decode approle login: %w", err)
	}
	if payload.Auth.ClientToken == "" {
		return "", fmt.Errorf("vault: approle login returned empty client token")
	}

	return payload.Auth.ClientToken, nil
}

func (v *vaultProvider) decorate(req *http.Request) {
	if v.token != "" {
		req.Header.Set("X-Vault-Token", v.token)
	}
	if v.namespace != "" {
		req.Header.Set("X-Vault-Namespace", v.namespace)
	}
}

func extractVaultData(payload map[string]any, path string, kvVersion int) (map[string]any, error) {
	rawData, ok := payload["data"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("vault: missing data for %q", path)
	}

	if kvVersion == 1 {
		return rawData, nil
	}
	if kvVersion == 2 {
		nested, ok := rawData["data"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("vault: kv v2 secret %q missing nested data", path)
		}
		return nested, nil
	}

	if nested, ok := rawData["data"].(map[string]any); ok {
		return nested, nil
	}

	return rawData, nil
}
