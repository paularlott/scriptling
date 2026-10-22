package plugin

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/paularlott/scriptling/object"
)

// limitsPluginServer serves a plugin whose "hold" function records how many
// calls run at once.
func limitsPluginServer(t *testing.T, maxBatch, maxConcurrency int) (string, *atomic.Int64, *atomic.Int64) {
	t.Helper()
	var running, peak, calls atomic.Int64
	ps := NewServer("limits", "1.0.0", "limits test")
	ps.RegisterBuiltin("hold", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		n := running.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		running.Add(-1)
		calls.Add(1)
		return object.NewString("ok")
	})
	ps.SetHTTPLimits(maxBatch, maxConcurrency)
	srv := httptest.NewServer(ps)
	t.Cleanup(srv.Close)
	return srv.URL, &peak, &calls
}

func holdBatch(n int) string {
	var b strings.Builder
	b.WriteByte('[')
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"jsonrpc":"2.0","id":%d,"method":"function.call","params":{"name":"hold"}}`, i)
	}
	b.WriteByte(']')
	return b.String()
}

func postBatch(t *testing.T, url, body string) string {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return string(out)
}

// A plugin served over HTTP is reachable by any client of the server, so
// SetHTTPLimits bounds its batches and concurrent calls (strix 4250
// vuln-0012).
func TestPluginHTTPLimits(t *testing.T) {
	url, peak, calls := limitsPluginServer(t, 20, 3)

	out := postBatch(t, url, holdBatch(21))
	if !strings.Contains(out, "batch too large: 21 elements exceeds the limit of 20") || calls.Load() != 0 {
		t.Fatalf("21 elements, %d calls: %s", calls.Load(), out)
	}

	out = postBatch(t, url, holdBatch(20))
	if n := strings.Count(out, `"id":`); n != 20 || calls.Load() != 20 {
		t.Fatalf("20 elements: %d responses, %d calls: %s", n, calls.Load(), out)
	}
	if p := peak.Load(); p > 3 || p < 2 {
		t.Fatalf("peak concurrent calls = %d, want 2..3", p)
	}
}

// Without SetHTTPLimits, or with zero, the plugin is unbounded as before.
func TestPluginHTTPLimitsUnsetUnbounded(t *testing.T) {
	url, peak, calls := limitsPluginServer(t, 0, 0)
	postBatch(t, url, holdBatch(200))
	if calls.Load() != 200 || peak.Load() < 20 {
		t.Fatalf("%d calls, peak %d: want 200 calls running concurrently", calls.Load(), peak.Load())
	}
}
