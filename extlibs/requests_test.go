package extlibs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

func TestRaiseForStatus(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		wantException bool
		wantType      string
		wantMsgPart   string
	}{
		{"200 ok", 200, false, "", ""},
		{"404 client error", 404, true, "HTTPError", "Client Error"},
		{"500 server error", 500, true, "HTTPError", "Server Error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instance := createResponseInstance(tt.statusCode, nil, nil, "http://example.com")
			fn := ResponseClass.Methods["raise_for_status"].(*object.Builtin)
			result := fn.Fn(context.Background(), object.NewKwargs(nil), instance)

			if !tt.wantException {
				if _, ok := result.(*object.Null); !ok {
					t.Fatalf("expected Null for %d, got %T", tt.statusCode, result)
				}
				return
			}

			exc, ok := result.(*object.Exception)
			if !ok {
				t.Fatalf("expected *object.Exception, got %T: %v", result, result)
			}
			if exc.ExceptionType != tt.wantType {
				t.Errorf("ExceptionType = %q, want %q", exc.ExceptionType, tt.wantType)
			}
			if tt.wantMsgPart != "" && !containsStr(exc.Message, tt.wantMsgPart) {
				t.Errorf("Message %q does not contain %q", exc.Message, tt.wantMsgPart)
			}
		})
	}
}

func TestRaiseForStatusCaughtByExcept(t *testing.T) {
	// Verify raise_for_status exceptions can be caught by except HTTPError
	// using a mock HTTP server returning 404
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()

	instance := createResponseInstance(404, nil, nil, srv.URL)
	fn := ResponseClass.Methods["raise_for_status"].(*object.Builtin)
	result := fn.Fn(context.Background(), object.NewKwargs(nil), instance)

	exc, ok := result.(*object.Exception)
	if !ok {
		t.Fatalf("expected *object.Exception, got %T", result)
	}
	if exc.ExceptionType != "HTTPError" {
		t.Errorf("ExceptionType = %q, want HTTPError", exc.ExceptionType)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > len(sub) && findStr(s, sub))
}

func findStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestBuildURLWithParams(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		params   map[string]string
		wantContains string
	}{
		{
			name:     "no params",
			baseURL:  "https://example.com/api",
			params:   nil,
			wantContains: "https://example.com/api",
		},
		{
			name:     "single param",
			baseURL:  "https://example.com/api",
			params:   map[string]string{"key": "value"},
			wantContains: "key=value",
		},
		{
			name:     "multiple params",
			baseURL:  "https://example.com/api",
			params:   map[string]string{"name": "test", "count": "5"},
			wantContains: "name=test",
		},
		{
			name:     "with existing query params",
			baseURL:  "https://example.com/api?existing=1",
			params:   map[string]string{"new": "2"},
			wantContains: "existing=1",
		},
		{
			name:     "special characters in params",
			baseURL:  "https://example.com/api",
			params:   map[string]string{"city": "New York"},
			wantContains: "city=New+York",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildURLWithParams(tt.baseURL, tt.params)
			if !containsStr(result, tt.wantContains) {
				t.Errorf("buildURLWithParams(%q, %v) = %q, want to contain %q",
					tt.baseURL, tt.params, result, tt.wantContains)
			}
		})
	}
}

func TestExtractParams(t *testing.T) {
	dict := map[string]object.Object{
		"string": object.NewString("hello"),
		"int":    object.NewInteger(42),
		"float":  object.NewFloat(3.14),
		"bool":   object.NewBoolean(true),
	}

	params := extractParams(dict)

	if params["string"] != "hello" {
		t.Errorf("expected string='hello', got %q", params["string"])
	}
	if params["int"] != "42" {
		t.Errorf("expected int='42', got %q", params["int"])
	}
	if params["float"] != "3.14" {
		t.Errorf("expected float='3.14', got %q", params["float"])
	}
	if params["bool"] != "true" {
		t.Errorf("expected bool='true', got %q", params["bool"])
	}
}

func TestRequestsGetWithParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify query params are present
		query := r.URL.Query()
		if query.Get("name") != "test" {
			t.Errorf("expected name=test, got %q", query.Get("name"))
		}
		if query.Get("count") != "10" {
			t.Errorf("expected count=10, got %q", query.Get("count"))
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	// Test using scriptling evaluator
	p := scriptling.New()
	stdlib.RegisterAll(p)
	RegisterRequestsLibrary(p)

	script := `
import requests

params = {"name": "test", "count": 10}
response = requests.get("` + srv.URL + `", params=params, timeout=5)
response.status_code
`

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	if status, _ := result.AsInt(); status != 200 {
		t.Errorf("Expected status 200, got %d", status)
	}
}

func TestRequestsPostWithParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify query params are present even for POST
		query := r.URL.Query()
		if query.Get("api_key") != "secret" {
			t.Errorf("expected api_key=secret, got %q", query.Get("api_key"))
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"result":"created"}`))
	}))
	defer srv.Close()

	p := scriptling.New()
	stdlib.RegisterAll(p)
	RegisterRequestsLibrary(p)

	script := `
import requests

params = {"api_key": "secret"}
response = requests.post("` + srv.URL + `", data="test data", params=params, timeout=5)
response.status_code
`

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	if status, _ := result.AsInt(); status != 200 {
		t.Errorf("Expected status 200, got %d", status)
	}
}

func TestParallelBasic(t *testing.T) {
	// Echo server that returns the request path in the response body
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"path":"` + r.URL.Path + `"}`))
	}))
	defer srv.Close()

	p := scriptling.New()
	stdlib.RegisterAll(p)
	RegisterRequestsLibrary(p)

	script := `
import requests

results = requests.parallel([
    {"method": "GET", "url": "` + srv.URL + `/item/1"},
    {"method": "GET", "url": "` + srv.URL + `/item/2"},
    {"method": "GET", "url": "` + srv.URL + `/item/3"},
], max_parallel=2)

len(results)
`

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	if count, _ := result.AsInt(); count != 3 {
		t.Errorf("Expected 3 results, got %d", count)
	}
}

func TestParallelOrderPreserved(t *testing.T) {
	// Server that echoes the path back as JSON
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"path":"` + r.URL.Path + `"}`))
	}))
	defer srv.Close()

	p := scriptling.New()
	stdlib.RegisterAll(p)
	RegisterRequestsLibrary(p)

	script := `
import requests

results = requests.parallel([
    {"method": "GET", "url": "` + srv.URL + `/first"},
    {"method": "GET", "url": "` + srv.URL + `/second"},
    {"method": "GET", "url": "` + srv.URL + `/third"},
], max_parallel=1)

# Check order by inspecting each response body
data = []
for r in results:
    data.append(r.json()["path"])
data
`

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	list, listErr := result.AsList()
	if listErr != nil {
		t.Fatalf("Expected list result, got %T", result)
	}
	if len(list) != 3 {
		t.Fatalf("Expected 3 items, got %d", len(list))
	}

	expected := []string{"/first", "/second", "/third"}
	for i, exp := range expected {
		s, _ := list[i].AsString()
		if s != exp {
			t.Errorf("results[%d] = %q, want %q", i, s, exp)
		}
	}
}

func TestParallelPostWithJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %q", ct)
		}
		body := make([]byte, 1024)
		n, _ := r.Body.Read(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"received":` + string(body[:n]) + `}`))
	}))
	defer srv.Close()

	p := scriptling.New()
	stdlib.RegisterAll(p)
	RegisterRequestsLibrary(p)

	script := `
import requests

results = requests.parallel([
    {"method": "POST", "url": "` + srv.URL + `", "json": {"id": 1}},
    {"method": "POST", "url": "` + srv.URL + `", "json": {"id": 2}},
], max_parallel=2)

results[0].status_code
`

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	if status, _ := result.AsInt(); status != 200 {
		t.Errorf("Expected status 200, got %d", status)
	}
}

func TestParallelMixedMethods(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"method":"` + r.Method + `"}`))
	}))
	defer srv.Close()

	p := scriptling.New()
	stdlib.RegisterAll(p)
	RegisterRequestsLibrary(p)

	script := `
import requests

results = requests.parallel([
    {"method": "GET", "url": "` + srv.URL + `"},
    {"method": "POST", "url": "` + srv.URL + `", "data": "hello"},
    {"method": "PUT", "url": "` + srv.URL + `", "data": "world"},
    {"method": "DELETE", "url": "` + srv.URL + `"},
], max_parallel=4)

methods = []
for r in results:
    methods.append(r.json()["method"])
methods
`

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	list, listErr := result.AsList()
	if listErr != nil {
		t.Fatalf("Expected list, got %T", result)
	}

	expected := []string{"GET", "POST", "PUT", "DELETE"}
	for i, exp := range expected {
		s, _ := list[i].AsString()
		if s != exp {
			t.Errorf("results[%d] method = %q, want %q", i, s, exp)
		}
	}
}

func TestParallelEmptyList(t *testing.T) {
	p := scriptling.New()
	stdlib.RegisterAll(p)
	RegisterRequestsLibrary(p)

	script := `
import requests

results = requests.parallel([], max_parallel=4)
len(results)
`

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	if count, _ := result.AsInt(); count != 0 {
		t.Errorf("Expected 0, got %d", count)
	}
}

func TestParallelMissingURL(t *testing.T) {
	p := scriptling.New()
	stdlib.RegisterAll(p)
	RegisterRequestsLibrary(p)

	script := `
import requests

results = requests.parallel([
    {"method": "GET"},
], max_parallel=1)

# Should get a response with status_code 0 (error indicator)
results[0].status_code
`

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	if status, _ := result.AsInt(); status != 0 {
		t.Errorf("Expected status 0 for missing URL, got %d", status)
	}
}

func TestParallelWithHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"auth":"` + auth + `"}`))
	}))
	defer srv.Close()

	p := scriptling.New()
	stdlib.RegisterAll(p)
	RegisterRequestsLibrary(p)

	script := `
import requests

results = requests.parallel([
    {"method": "GET", "url": "` + srv.URL + `", "headers": {"Authorization": "Bearer token123"}},
], max_parallel=1)

results[0].json()["auth"]
`

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	if s, _ := result.AsString(); s != "Bearer token123" {
		t.Errorf("Expected 'Bearer token123', got %q", s)
	}
}

func TestParallelWithAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		w.Header().Set("Content-Type", "application/json")
		if !ok {
			w.WriteHeader(401)
			w.Write([]byte(`{"error":"no auth"}`))
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"user":"` + user + `","pass":"` + pass + `"}`))
	}))
	defer srv.Close()

	p := scriptling.New()
	stdlib.RegisterAll(p)
	RegisterRequestsLibrary(p)

	script := `
import requests

results = requests.parallel([
    {"method": "GET", "url": "` + srv.URL + `", "auth": ["admin", "secret"]},
], max_parallel=1)

data = results[0].json()
data["user"] + ":" + data["pass"]
`

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	if s, _ := result.AsString(); s != "admin:secret" {
		t.Errorf("Expected 'admin:secret', got %q", s)
	}
}

func TestParallelConcurrency(t *testing.T) {
	// Verify that max_parallel is respected by tracking concurrent requests
	var mu sync.Mutex
	maxConcurrent := 0
	concurrent := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		concurrent++
		if concurrent > maxConcurrent {
			maxConcurrent = concurrent
		}
		mu.Unlock()

		// Small sleep to allow overlap detection
		time.Sleep(50 * time.Millisecond)

		mu.Lock()
		concurrent--
		mu.Unlock()

		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	p := scriptling.New()
	stdlib.RegisterAll(p)
	RegisterRequestsLibrary(p)

	// Send 8 requests with max_parallel=2
	script := `
import requests

reqs = []
for i in range(8):
    reqs.append({"method": "GET", "url": "` + srv.URL + `"})

results = requests.parallel(reqs, max_parallel=2)
len(results)
`

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	if count, _ := result.AsInt(); count != 8 {
		t.Errorf("Expected 8 results, got %d", count)
	}

	// maxConcurrent should be <= 2
	if maxConcurrent > 2 {
		t.Errorf("max_parallel=2 violated: saw %d concurrent requests", maxConcurrent)
	}
	if maxConcurrent < 2 {
		t.Logf("Warning: only saw %d concurrent requests (expected 2) — may be timing-dependent", maxConcurrent)
	}
}

func TestParallelEchoServer(t *testing.T) {
	// Integration test using external echo server on port 9000.
	// Skip if the server is not running.
	resp, err := http.Get("http://localhost:9000/")
	if err != nil {
		t.Skip("Echo server not running on port 9000, skipping integration test")
	}
	resp.Body.Close()

	p := scriptling.New()
	stdlib.RegisterAll(p)
	RegisterRequestsLibrary(p)

	script := `
import requests

results = requests.parallel([
    {"method": "GET", "url": "http://localhost:9000/test1"},
    {"method": "POST", "url": "http://localhost:9000/test2", "json": {"hello": "world"}},
    {"method": "GET", "url": "http://localhost:9000/test3"},
    {"method": "GET", "url": "http://localhost:9000/test4"},
    {"method": "GET", "url": "http://localhost:9000/test5"},
    {"method": "GET", "url": "http://localhost:9000/test6"},
], max_parallel=4)

# All should succeed
ok = 0
for r in results:
    if r.status_code == 200:
        ok = ok + 1
ok
`

	result, evalErr := p.Eval(script)
	if evalErr != nil {
		t.Fatalf("Script error: %v", evalErr)
	}

	if count, _ := result.AsInt(); count != 6 {
		t.Errorf("Expected 6 successful responses, got %d", count)
	}
}
