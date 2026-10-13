package secretprovider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeConnect is a minimal 1Password Connect server: one vault and a few
// items, counting every request.
type fakeConnect struct {
	mu       sync.Mutex
	requests []string
	items    map[string]onePasswordItem // by item id
	titles   map[string]string          // title -> item id
}

func newFakeConnect(t *testing.T) (*fakeConnect, *httptest.Server) {
	t.Helper()
	f := &fakeConnect{
		items: map[string]onePasswordItem{
			"itm1": {ID: "itm1", Title: "Database", Fields: []onePasswordItemField{
				{ID: "username", Label: "username", Purpose: "USERNAME", Value: "admin"},
				{ID: "password", Label: "password", Purpose: "PASSWORD", Value: "hunter2"},
				{ID: "abc123", Label: "Port", Value: "5432"},
			}},
			"itm2": {ID: "itm2", Title: "Notes Only", Fields: []onePasswordItemField{
				{ID: "notesPlain", Label: "notesPlain", Purpose: "NOTES", Value: "just notes"},
			}},
		},
		titles: map[string]string{"Database": "itm1", "Notes Only": "itm2"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.requests = append(f.requests, r.URL.Path)

		if r.Header.Get("Authorization") != "Bearer connect-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		switch {
		case r.URL.Path == "/v1/vaults":
			json.NewEncoder(w).Encode([]onePasswordVault{{ID: "vlt1", Name: "Engineering"}})
		case r.URL.Path == "/v1/vaults/vlt1/items":
			filter := r.URL.Query().Get("filter")
			out := []onePasswordItem{}
			for title, id := range f.titles {
				if strings.Contains(filter, `"`+title+`"`) {
					out = append(out, onePasswordItem{ID: id, Title: title})
				}
			}
			json.NewEncoder(w).Encode(out)
		case strings.HasPrefix(r.URL.Path, "/v1/vaults/vlt1/items/"):
			item, ok := f.items[strings.TrimPrefix(r.URL.Path, "/v1/vaults/vlt1/items/")]
			if !ok {
				http.Error(w, `{"status":404,"message":"item not found"}`, http.StatusNotFound)
				return
			}
			json.NewEncoder(w).Encode(item)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return f, srv
}

func (f *fakeConnect) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests)
}

func newTestConnect(t *testing.T, address string) *onePasswordProvider {
	t.Helper()
	p, err := NewProvider(Config{Provider: "onepassword", Address: address, Token: "connect-token"})
	if err != nil {
		t.Fatal(err)
	}
	return p.(*onePasswordProvider)
}

func TestOnePasswordConnectResolve(t *testing.T) {
	_, srv := newFakeConnect(t)
	p := newTestConnect(t, srv.URL)
	ctx := context.Background()

	for _, tt := range []struct{ path, field, want string }{
		{"Engineering/Database", "", "hunter2"},
		{"Engineering/Database", "username", "admin"},
		{"Engineering/Database", "PORT", "5432"},
		{"Engineering/Notes Only", "", "just notes"},
		{"Engineering/Notes Only", "notesPlain", "just notes"},
	} {
		got, err := p.Resolve(ctx, tt.path, tt.field)
		if err != nil || got != tt.want {
			t.Errorf("Resolve(%q, %q) = %q, %v; want %q", tt.path, tt.field, got, err, tt.want)
		}
	}

	for _, tt := range []struct{ path, field, want string }{
		{"Missing/Database", "", `vault "Missing" not found`},
		{"Engineering/Missing", "", `item "Missing" not found`},
		{"Engineering/Database", "nope", `field "nope" not found`},
	} {
		_, err := p.Resolve(ctx, tt.path, tt.field)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("Resolve(%q, %q) err = %v, want %q", tt.path, tt.field, err, tt.want)
		}
	}
}

func TestOnePasswordConnectRemembersIDs(t *testing.T) {
	f, srv := newFakeConnect(t)
	p := newTestConnect(t, srv.URL)
	ctx := context.Background()

	if _, err := p.Resolve(ctx, "Engineering/Database", "password"); err != nil {
		t.Fatal(err)
	}
	first := f.count()
	if first != 3 {
		t.Fatalf("first read made %d requests, want 3 (vault, item lookup, item)", first)
	}

	if _, err := p.Resolve(ctx, "Engineering/Database", "username"); err != nil {
		t.Fatal(err)
	}
	if got := f.count() - first; got != 1 {
		t.Fatalf("second read made %d requests, want 1", got)
	}
}

func TestOnePasswordConnectRelooksUpGoneItem(t *testing.T) {
	f, srv := newFakeConnect(t)
	p := newTestConnect(t, srv.URL)
	ctx := context.Background()

	if _, err := p.Resolve(ctx, "Engineering/Database", ""); err != nil {
		t.Fatal(err)
	}

	// The item is recreated under a new id.
	f.mu.Lock()
	item := f.items["itm1"]
	delete(f.items, "itm1")
	item.ID = "itm9"
	f.items["itm9"] = item
	f.titles["Database"] = "itm9"
	f.mu.Unlock()

	got, err := p.Resolve(ctx, "Engineering/Database", "")
	if err != nil || got != "hunter2" {
		t.Fatalf("after the item moved: %q, %v", got, err)
	}
}

func TestOnePasswordConnectIDsExpire(t *testing.T) {
	f, srv := newFakeConnect(t)
	p := newTestConnect(t, srv.URL)
	now := time.Now()
	p.now = func() time.Time { return now }
	ctx := context.Background()

	p.Resolve(ctx, "Engineering/Database", "")
	first := f.count()
	now = now.Add(onePasswordIDTTL + time.Minute)
	p.Resolve(ctx, "Engineering/Database", "")
	if got := f.count() - first; got != 3 {
		t.Fatalf("after expiry made %d requests, want a full lookup of 3", got)
	}
}
