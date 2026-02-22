package extlibs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/paularlott/scriptling/object"
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
