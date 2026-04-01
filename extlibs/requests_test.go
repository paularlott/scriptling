package extlibs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

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
		"string": &object.String{Value: "hello"},
		"int":    &object.Integer{Value: 42},
		"float":  &object.Float{Value: 3.14},
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
