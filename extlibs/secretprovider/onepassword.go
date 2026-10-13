package secretprovider

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F-]{36}$`)

// onePasswordIDTTL bounds how long a vault name or item title is remembered
// as an id. Ids are stable; the bound only limits how long a renamed item
// keeps answering to its old title.
const onePasswordIDTTL = 10 * time.Minute

type onePasswordProvider struct {
	address      string
	token        string
	defaultField string
	client       *http.Client

	// ids remembers vault names and item titles resolved to ids, so a read
	// after the first costs one request (the item) instead of three.
	idsMu sync.Mutex
	ids   map[string]cachedID
	now   func() time.Time
}

type cachedID struct {
	id        string
	expiresAt time.Time
}

// errOnePasswordNotFound marks a 404 from Connect, keeping the error text
// unchanged.
var errOnePasswordNotFound = errors.New("not found")

type onePasswordNotFoundError struct{ error }

func (e onePasswordNotFoundError) Is(target error) bool { return target == errOnePasswordNotFound }

type onePasswordVault struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type onePasswordItem struct {
	ID     string                 `json:"id"`
	Title  string                 `json:"title"`
	Fields []onePasswordItemField `json:"fields"`
}

type onePasswordItemField struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Purpose string `json:"purpose"`
	Type    string `json:"type"`
	Value   string `json:"value"`
}

// NewOnePasswordProvider creates a 1Password Connect-backed secret provider.
func NewOnePasswordProvider(cfg Config) (Provider, error) {
	if cfg.Address == "" {
		return nil, fmt.Errorf("secret provider %q requires address", cfg.Provider)
	}
	if cfg.Token == "" {
		return nil, fmt.Errorf("onepassword secret provider requires token")
	}

	parsedURL, err := url.Parse(cfg.Address)
	if err != nil {
		return nil, fmt.Errorf("onepassword: invalid address: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("onepassword: address must use http or https")
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: cfg.InsecureSkipTLS,
			},
		},
	}

	provider := &onePasswordProvider{
		address:      strings.TrimRight(cfg.Address, "/"),
		token:        cfg.Token,
		defaultField: cfg.DefaultField,
		client:       client,
		ids:          map[string]cachedID{},
		now:          time.Now,
	}
	if provider.defaultField == "" {
		provider.defaultField = "password"
	}

	return provider, nil
}

func (p *onePasswordProvider) ID() string {
	return "onepassword"
}

func (p *onePasswordProvider) Resolve(ctx context.Context, path, field string) (string, error) {
	vaultRef, itemRef, err := splitOnePasswordPath(path)
	if err != nil {
		return "", err
	}
	if field == "" {
		field = p.defaultField
	}

	vaultID, err := p.resolveVaultID(ctx, vaultRef)
	if err != nil {
		return "", err
	}

	item, err := p.resolveItem(ctx, vaultID, itemRef)
	if err != nil {
		return "", err
	}

	value, err := p.resolveField(item, field)
	if err != nil {
		return "", fmt.Errorf("onepassword: %w", err)
	}

	return value, nil
}

func (p *onePasswordProvider) List(ctx context.Context, path string) ([]string, error) {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil, fmt.Errorf("onepassword: path must be a vault name or vault/item")
	}

	parts := strings.SplitN(path, "/", 2)
	vaultRef := parts[0]

	vaultID, err := p.resolveVaultID(ctx, vaultRef)
	if err != nil {
		return nil, err
	}

	// vault/item — list fields in the item
	if len(parts) == 2 && parts[1] != "" {
		item, err := p.resolveItem(ctx, vaultID, parts[1])
		if err != nil {
			return nil, err
		}
		names := make([]string, 0, len(item.Fields))
		for _, f := range item.Fields {
			if f.Label != "" {
				names = append(names, f.Label)
			}
		}
		return names, nil
	}

	// vault only — list items
	var items []onePasswordItem
	if err := p.getJSON(ctx, "/v1/vaults/"+url.PathEscape(vaultID)+"/items", &items); err != nil {
		return nil, fmt.Errorf("onepassword: list items in vault %q: %w", vaultRef, err)
	}

	titles := make([]string, 0, len(items))
	for _, item := range items {
		titles = append(titles, item.Title)
	}
	return titles, nil
}

func splitOnePasswordPath(path string) (string, string, error) {
	path = strings.Trim(path, "/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("onepassword: path must be vault/item, got %q", path)
	}
	return parts[0], parts[1], nil
}

func (p *onePasswordProvider) resolveVaultID(ctx context.Context, vaultRef string) (string, error) {
	if uuidPattern.MatchString(vaultRef) {
		return vaultRef, nil
	}

	key := "vault\x00" + vaultRef
	if id, ok := p.cachedID(key); ok {
		return id, nil
	}

	var vaults []onePasswordVault
	if err := p.getJSON(ctx, "/v1/vaults?filter="+url.QueryEscape(`name eq "`+escapeSCIMValue(vaultRef)+`"`), &vaults); err != nil {
		return "", fmt.Errorf("onepassword: list vaults: %w", err)
	}

	for _, vault := range vaults {
		if vault.Name == vaultRef {
			p.storeID(key, vault.ID)
			return vault.ID, nil
		}
	}

	return "", fmt.Errorf("onepassword: vault %q not found", vaultRef)
}

func (p *onePasswordProvider) resolveItem(ctx context.Context, vaultID, itemRef string) (*onePasswordItem, error) {
	if uuidPattern.MatchString(itemRef) {
		var item onePasswordItem
		if err := p.getJSON(ctx, "/v1/vaults/"+url.PathEscape(vaultID)+"/items/"+url.PathEscape(itemRef), &item); err != nil {
			return nil, fmt.Errorf("onepassword: get item %q: %w", itemRef, err)
		}
		return &item, nil
	}

	// A remembered id costs one request. If the item has gone since, look
	// it up by title again.
	key := "item\x00" + vaultID + "\x00" + itemRef
	if itemID, ok := p.cachedID(key); ok {
		var item onePasswordItem
		err := p.getJSON(ctx, "/v1/vaults/"+url.PathEscape(vaultID)+"/items/"+url.PathEscape(itemID), &item)
		if err == nil && item.Title == itemRef {
			return &item, nil
		}
		p.dropID(key)
		if err != nil && !errors.Is(err, errOnePasswordNotFound) {
			return nil, fmt.Errorf("onepassword: get item %q: %w", itemRef, err)
		}
	}

	var items []onePasswordItem
	filter := url.QueryEscape(`title eq "` + escapeSCIMValue(itemRef) + `"`)
	if err := p.getJSON(ctx, "/v1/vaults/"+url.PathEscape(vaultID)+"/items?filter="+filter, &items); err != nil {
		return nil, fmt.Errorf("onepassword: list items for %q: %w", itemRef, err)
	}

	for _, item := range items {
		if item.Title == itemRef {
			var fullItem onePasswordItem
			if err := p.getJSON(ctx, "/v1/vaults/"+url.PathEscape(vaultID)+"/items/"+url.PathEscape(item.ID), &fullItem); err != nil {
				return nil, fmt.Errorf("onepassword: get item %q: %w", itemRef, err)
			}
			p.storeID(key, item.ID)
			return &fullItem, nil
		}
	}

	return nil, fmt.Errorf("onepassword: item %q not found", itemRef)
}

func (p *onePasswordProvider) cachedID(key string) (string, bool) {
	p.idsMu.Lock()
	defer p.idsMu.Unlock()
	entry, ok := p.ids[key]
	if !ok || !p.now().Before(entry.expiresAt) {
		delete(p.ids, key)
		return "", false
	}
	return entry.id, true
}

func (p *onePasswordProvider) storeID(key, id string) {
	p.idsMu.Lock()
	defer p.idsMu.Unlock()
	p.ids[key] = cachedID{id: id, expiresAt: p.now().Add(onePasswordIDTTL)}
}

func (p *onePasswordProvider) dropID(key string) {
	p.idsMu.Lock()
	defer p.idsMu.Unlock()
	delete(p.ids, key)
}

func (p *onePasswordProvider) resolveField(item *onePasswordItem, field string) (string, error) {
	if value, ok := matchOnePasswordField(item.Fields, field); ok {
		return value, nil
	}

	if field == p.defaultField {
		for _, fallback := range []string{"password", "credential", "notesPlain"} {
			if fallback == field {
				continue
			}
			if value, ok := matchOnePasswordField(item.Fields, fallback); ok {
				return value, nil
			}
		}
	}

	return "", fmt.Errorf("field %q not found in item %q", field, item.Title)
}

func matchOnePasswordField(fields []onePasswordItemField, field string) (string, bool) {
	field = strings.ToLower(field)

	for _, itemField := range fields {
		for _, candidate := range []string{
			strings.ToLower(itemField.ID),
			strings.ToLower(itemField.Label),
			strings.ToLower(itemField.Purpose),
			strings.ToLower(strings.ReplaceAll(itemField.Purpose, "_", "")),
		} {
			if candidate == field && itemField.Value != "" {
				return itemField.Value, true
			}
		}
	}

	return "", false
}

func (p *onePasswordProvider) getJSON(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.address+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		err := fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(body)))
		if resp.StatusCode == http.StatusNotFound {
			err = onePasswordNotFoundError{err}
		}
		return err
	}

	return json.NewDecoder(resp.Body).Decode(out)
}

func escapeSCIMValue(value string) string {
	return strings.ReplaceAll(value, `"`, `\"`)
}
