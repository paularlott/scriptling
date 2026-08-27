package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// muxMirror builds a ServeMux registering the handler keys the same way
// buildMux does, so the mux sees the same patterns the map is keyed by. Each
// matched request records the handlerForRequest/pathParams resolution.
func muxMirror(t *testing.T, s *Server) (*http.ServeMux, *resolved) {
	t.Helper()
	res := &resolved{}
	mux := http.NewServeMux()
	for key := range s.handlers {
		reg := key
		if strings.HasSuffix(reg, " /") {
			reg += "{$}"
		}
		mux.HandleFunc(reg, func(w http.ResponseWriter, r *http.Request) {
			res.ref, res.ok = s.handlerForRequest(r)
			res.params = pathParams(r)
		})
	}
	return mux, res
}

type resolved struct {
	ref    string
	ok     bool
	params map[string]string
}

// TestHandlerForRequestUsesMuxPattern routes requests through a mux built the
// same way buildMux builds it and checks that the matched pattern resolves
// the handler — including wildcard extraction, literal-beats-wildcard, the
// "/{$}" spelling of bare "/" routes, and HEAD dispatch to GET handlers.
func TestHandlerForRequestUsesMuxPattern(t *testing.T) {
	s := &Server{handlers: map[string]string{
		"GET /api/users/{id}":  "lib.get_user",
		"GET /api/users/new":   "lib.new_user",
		"GET /files/{path...}": "lib.file",
		"GET /":                "lib.index",
		"POST /api/users/{id}": "lib.update_user",
	}}

	tests := []struct {
		method     string
		path       string
		wantRef    string
		wantParams map[string]string
	}{
		{"GET", "/api/users/42", "lib.get_user", map[string]string{"id": "42"}},
		{"GET", "/api/users/new", "lib.new_user", nil}, // literal beats wildcard
		{"GET", "/api/users/john%20doe", "lib.get_user", map[string]string{"id": "john doe"}},
		{"GET", "/api/users/a%2Fb", "lib.get_user", map[string]string{"id": "a/b"}}, // %2F stays one segment
		{"GET", "/files/a/b/c.txt", "lib.file", map[string]string{"path": "a/b/c.txt"}},
		{"GET", "/files/", "lib.file", map[string]string{"path": ""}},
		{"GET", "/", "lib.index", nil}, // matched via "GET /{$}"
		{"HEAD", "/api/users/42", "lib.get_user", map[string]string{"id": "42"}},
		{"POST", "/api/users/9", "lib.update_user", map[string]string{"id": "9"}},
		{"PUT", "/api/users/9", "", nil}, // no PUT route registered
	}

	for _, tt := range tests {
		mux, res := muxMirror(t, s)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))

		if tt.wantRef == "" {
			// Negative case: the mux must not route it to a handler at all
			// (404, or 405 when the path exists under other methods).
			if res.ok {
				t.Errorf("%s %s: handlerForRequest = %q, want no match", tt.method, tt.path, res.ref)
			}
			continue
		}
		if rec.Code != http.StatusOK {
			t.Errorf("%s %s: mux did not route (status %d)", tt.method, tt.path, rec.Code)
			continue
		}
		if !res.ok || res.ref != tt.wantRef {
			t.Errorf("%s %s: handlerForRequest = %q ok=%v, want %q", tt.method, tt.path, res.ref, res.ok, tt.wantRef)
			continue
		}
		if !paramsEqual(res.params, tt.wantParams) {
			t.Errorf("%s %s: pathParams = %v, want %v", tt.method, tt.path, res.params, tt.wantParams)
		}
	}
}

func paramsEqual(got, want map[string]string) bool {
	if len(got) != len(want) {
		return false
	}
	for k, v := range want {
		if got[k] != v {
			return false
		}
	}
	return true
}

// Without a mux match (handleScriptRequest mounted directly), the lookup falls
// back to the exact literal key.
func TestHandlerForRequestWithoutMuxPattern(t *testing.T) {
	s := &Server{handlers: map[string]string{"GET /api/hello": "lib.hello"}}

	req := httptest.NewRequest(http.MethodGet, "/api/hello", nil)
	ref, ok := s.handlerForRequest(req)
	if !ok || ref != "lib.hello" {
		t.Errorf("handlerForRequest = %q ok=%v, want lib.hello", ref, ok)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/users/42", nil)
	if _, ok := s.handlerForRequest(req); ok {
		t.Error("handlerForRequest should not match a wildcard path without mux routing")
	}
}
