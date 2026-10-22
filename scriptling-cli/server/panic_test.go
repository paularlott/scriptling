package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	logslog "github.com/paularlott/logger/slog"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

// A Go panic in an HTTP handler used to escape CallFunctionWithContext to
// net/http, which dropped the connection and printed its own trace. It now
// returns a 500 whose body is the one-line panic message, the error is logged
// as before, and the stack trace is logged at debug level only.
func TestHTTPHandlerPanicReturns500AndLogsStack(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "handlers.py"), []byte("import nat\n\ndef handle(request):\n    return nat.explode()\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	saved := Log
	Log = logslog.New(logslog.Config{Level: "debug", Format: "json", Writer: &logs})
	defer func() { Log = saved }()

	s := &Server{config: ServerConfig{
		LibDirs: []string{dir},
		ExtraLibs: func(p *scriptling.Scriptling) {
			p.RegisterLibrary(object.NewLibrary("nat", map[string]*object.Builtin{
				"explode": {Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					var s []int
					return object.NewInteger(int64(s[3]))
				}},
			}, nil, ""))
		},
	}}
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	resp := s.runHandler(context.Background(), "handlers.handle", s.createRequestObject(req, nil, nil))
	if resp == nil {
		t.Fatal("no response")
	}
	rec := httptest.NewRecorder()
	s.writeResponse(rec, resp)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "script panic: runtime error: index out of range [3] with length 0") {
		t.Fatalf("body = %q", body)
	}
	if strings.Contains(body, "goroutine") || strings.Contains(body, ".go:") {
		t.Fatalf("response body leaks the stack trace: %q", body)
	}

	out := logs.String()
	if !strings.Contains(out, `"msg":"Handler error"`) {
		t.Errorf("handler error not logged: %s", out)
	}
	if !strings.Contains(out, `"msg":"Script panic stack trace"`) || !strings.Contains(out, "panic_test.go") || !strings.Contains(out, `"level":"DEBUG"`) {
		t.Errorf("stack trace not logged at debug level: %s", out)
	}

	// Above debug level the error is still logged, the stack trace is not.
	logs.Reset()
	Log = logslog.New(logslog.Config{Level: "info", Format: "json", Writer: &logs})
	s.runHandler(context.Background(), "handlers.handle", s.createRequestObject(req, nil, nil))
	if out := logs.String(); !strings.Contains(out, `"msg":"Handler error"`) || strings.Contains(out, "stack") {
		t.Errorf("at info level: %s", out)
	}
}
