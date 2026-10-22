package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

// handlerProbe is a Go library scripts call to record how many JSON-RPC
// handlers run at once.
type handlerProbe struct {
	running, peak, calls atomic.Int64
	hold                 time.Duration
}

func (h *handlerProbe) lib(p *scriptling.Scriptling) {
	p.RegisterLibrary(object.NewLibrary("probe", map[string]*object.Builtin{
		"hold": {Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			n := h.running.Add(1)
			for {
				p := h.peak.Load()
				if n <= p || h.peak.CompareAndSwap(p, n) {
					break
				}
			}
			time.Sleep(h.hold)
			h.running.Add(-1)
			h.calls.Add(1)
			return object.NewString("ok")
		}},
	}, nil, ""))
}

const probeHandlerSrc = `import probe

def work(params):
    return probe.hold()
`

func probeServer(t *testing.T, hold time.Duration, maxBatch, maxConcurrency int) (*Server, *handlerProbe) {
	t.Helper()
	probe := &handlerProbe{hold: hold}
	s, _ := jsonrpcTestServer(t, probeHandlerSrc, map[string]string{"work": "rpcmod.work"}, nil)
	s.config.ExtraLibs = probe.lib
	s.config.JSONRPCMaxBatch = maxBatch
	s.config.JSONRPCMaxConcurrency = maxConcurrency
	return s, probe
}

func workBatch(n int) string {
	var b strings.Builder
	b.WriteByte('[')
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"jsonrpc":"2.0","id":%d,"method":"work"}`, i)
	}
	b.WriteByte(']')
	return b.String()
}

func postRPCBody(t *testing.T, url, body string) string {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Error(err)
		return ""
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return string(out)
}

// One batch of hundreds of thousands of elements started a goroutine and an
// interpreter per element, so a 28 MiB request drove the server to 5 GiB and
// out of memory (CWE-770, strix 4250 vuln-0012). By default a batch over 1000
// elements is now refused before any of it runs.
func TestJSONRPCBatchLimitDefault(t *testing.T) {
	s, probe := probeServer(t, 0, 0, 0)
	srv := httptest.NewServer(http.HandlerFunc(s.handleJSONRPCHTTP))
	defer srv.Close()

	out := postRPCBody(t, srv.URL, workBatch(1001))
	if !strings.Contains(out, "batch too large: 1001 elements exceeds the limit of 1000") || !strings.Contains(out, `"code":-32600`) {
		t.Fatalf("1001 elements: %s", out)
	}
	if n := probe.calls.Load(); n != 0 {
		t.Fatalf("%d handlers ran for a refused batch", n)
	}

	var resps []map[string]any
	if err := json.Unmarshal([]byte(postRPCBody(t, srv.URL, workBatch(1000))), &resps); err != nil || len(resps) != 1000 {
		t.Fatalf("1000 elements: %d responses, %v", len(resps), err)
	}
	if n := probe.calls.Load(); n != 1000 {
		t.Fatalf("1000 elements: %d calls", n)
	}
}

// Many concurrent clients, each sending a batch, multiplied fan-out even with
// a batch cap. The concurrency limit is server-wide: across every request no
// more than the limit run at once, and every call is still answered.
func TestJSONRPCConcurrencyLimitAcrossRequests(t *testing.T) {
	s, probe := probeServer(t, 10*time.Millisecond, 0, 3)
	srv := httptest.NewServer(http.HandlerFunc(s.handleJSONRPCHTTP))
	defer srv.Close()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body := `{"jsonrpc":"2.0","id":1,"method":"work"}`
			if i%2 == 0 {
				body = workBatch(10)
			}
			if out := postRPCBody(t, srv.URL, body); !strings.Contains(out, `"result":"ok"`) {
				t.Errorf("request %d: %s", i, out)
			}
		}(i)
	}
	wg.Wait()
	if p := probe.peak.Load(); p > 3 || p < 2 {
		t.Fatalf("peak concurrent handlers = %d, want 2..3", p)
	}
	if c := probe.calls.Load(); c != 110 {
		t.Fatalf("calls = %d, want 110", c)
	}
}

// With no setting the concurrency limit is 4 x CPUs.
func TestJSONRPCConcurrencyLimitDefault(t *testing.T) {
	limit := DefaultJSONRPCMaxConcurrency()
	s, probe := probeServer(t, 20*time.Millisecond, 0, 0)
	srv := httptest.NewServer(http.HandlerFunc(s.handleJSONRPCHTTP))
	defer srv.Close()

	postRPCBody(t, srv.URL, workBatch(min(3*limit, 1000)))
	if p := probe.peak.Load(); p > int64(limit) || p < 2 {
		t.Fatalf("peak concurrent handlers = %d, want 2..%d", p, limit)
	}
}

// Negative settings remove both limits, as before this fix.
func TestJSONRPCLimitsNegativeUnlimited(t *testing.T) {
	limit := DefaultJSONRPCMaxConcurrency()
	s, probe := probeServer(t, 50*time.Millisecond, -1, -1)
	srv := httptest.NewServer(http.HandlerFunc(s.handleJSONRPCHTTP))
	defer srv.Close()

	n := max(1200, 3*limit)
	var resps []map[string]any
	if err := json.Unmarshal([]byte(postRPCBody(t, srv.URL, workBatch(n))), &resps); err != nil || len(resps) != n {
		t.Fatalf("%d responses, want %d, %v", len(resps), n, err)
	}
	if p := probe.peak.Load(); p <= int64(limit) {
		t.Fatalf("unlimited peak concurrent handlers = %d, want above the default %d", p, limit)
	}
}

// The stdio transport is held to the same limits.
func TestJSONRPCStdioLimits(t *testing.T) {
	s, probe := probeServer(t, 5*time.Millisecond, 5, 2)
	var in bytes.Buffer
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&in, `{"jsonrpc":"2.0","id":%d,"method":"work"}`+"\n", i)
	}
	in.WriteString(workBatch(6) + "\n")
	var out bytes.Buffer
	var mu sync.Mutex
	w := writerFunc(func(p []byte) (int, error) { mu.Lock(); defer mu.Unlock(); return out.Write(p) })
	if err := s.runJSONRPC(context.Background(), &in, w); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	got := out.String()
	mu.Unlock()
	if n := strings.Count(got, `"result":"ok"`); n != 30 {
		t.Fatalf("%d results, want 30: %s", n, got)
	}
	if !strings.Contains(got, "batch too large: 6 elements exceeds the limit of 5") {
		t.Fatalf("oversized batch not refused: %s", got)
	}
	if p := probe.peak.Load(); p > 2 {
		t.Fatalf("peak concurrent handlers = %d, want <= 2", p)
	}
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

func TestJSONRPCLimitsResolve(t *testing.T) {
	cpus := 4 * runtime.NumCPU()
	for _, tc := range []struct{ batch, conc, wantBatch, wantConc int }{
		{0, 0, 1000, cpus},
		{50, 7, 50, 7},
		{-1, -1, 0, 0},
		{-5, 3, 0, 3},
	} {
		s := &Server{config: ServerConfig{JSONRPCMaxBatch: tc.batch, JSONRPCMaxConcurrency: tc.conc}}
		if b, c := s.jsonrpcLimits(); b != tc.wantBatch || c != tc.wantConc {
			t.Errorf("(%d, %d) -> (%d, %d), want (%d, %d)", tc.batch, tc.conc, b, c, tc.wantBatch, tc.wantConc)
		}
	}
}
