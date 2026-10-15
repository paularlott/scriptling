package extlibs

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

func TestRandbitsBounds(t *testing.T) {
	p := scriptling.New()
	RegisterSecretsLibrary(p)
	result, err := p.Eval(`
import secrets
ok = []
for k in [1, 8, 62]:
    v = secrets.randbits(k)
    ok.append(v >= 0 and v < (1 << k))
ok.append(secrets.randbits(63) >= 0)
errors = []
for k in [64, 1 << 40]:
    try:
        secrets.randbits(k)
        errors.append("no error")
    except OverflowError:
        errors.append("overflow")
[ok, errors]
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); got != "[[True, True, True, True], ['overflow', 'overflow']]" {
		t.Fatalf("got %s", got)
	}
}

// requests.parallel runs a fixed pool of max_parallel workers, not a goroutine
// per request.
func TestParallelRequestsUseBoundedWorkers(t *testing.T) {
	var active, maxActive, maxGoroutines atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := active.Add(1)
		defer active.Add(-1)
		for {
			m := maxActive.Load()
			if n <= m || maxActive.CompareAndSwap(m, n) {
				break
			}
		}
		g := int64(runtime.NumGoroutine())
		for {
			m := maxGoroutines.Load()
			if g <= m || maxGoroutines.CompareAndSwap(m, g) {
				break
			}
		}
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	p := scriptling.New()
	stdlib.RegisterAll(p)
	RegisterRequestsLibrary(p)
	baseline := int64(runtime.NumGoroutine())
	result, err := p.Eval(`
import requests
reqs = [{"method": "GET", "url": "` + srv.URL + `/x"} for i in range(1000)]
results = requests.parallel(reqs, max_parallel=2)
[len(results), all([r.status_code == 200 for r in results])]
`)
	if err != nil {
		t.Fatal(err)
	}
	if result.Inspect() != "[1000, True]" {
		t.Fatalf("got %s", result.Inspect())
	}
	if maxActive.Load() > 2 {
		t.Fatalf("%d requests at once under max_parallel=2", maxActive.Load())
	}
	// A goroutine per request would put about 1000 above the baseline.
	if extra := maxGoroutines.Load() - baseline; extra > 100 {
		t.Fatalf("%d goroutines above the baseline for 1000 requests", extra)
	}
}

// Responses fetched by parallel workers are held to the calling interpreter's
// value limit, not the process default.
func TestParallelRequestsUseCallerLimits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/big" {
			w.Write([]byte(strings.Repeat("b", 4096)))
			return
		}
		w.Write([]byte("small"))
	}))
	defer srv.Close()

	p := scriptling.New(scriptling.WithMaxValueBytes(1024))
	stdlib.RegisterAll(p)
	RegisterRequestsLibrary(p)
	result, err := p.Eval(`
import requests
requests.parallel([
    {"method": "GET", "url": "` + srv.URL + `/small"},
    {"method": "GET", "url": "` + srv.URL + `/big"},
    "not a dict",
], max_parallel=2)
`)
	if err != nil {
		t.Fatal(err)
	}
	list, ok := result.(*object.List)
	if !ok || len(list.Elements) != 3 {
		t.Fatalf("got %s", result.Inspect())
	}
	if _, isErr := list.Elements[0].(*object.Error); isErr {
		t.Fatalf("small response failed: %s", list.Elements[0].Inspect())
	}
	// In a batch a refused response is a status 0 response naming the limit.
	big, isResp := list.Elements[1].(*object.Instance)
	if !isResp {
		t.Fatalf("big response is not a Response: %s", list.Elements[1].Inspect())
	}
	code, _ := big.GetField("status_code")
	text, _ := big.GetField("text")
	if code.Inspect() != "0" || !strings.Contains(text.Inspect(), "too large") {
		t.Fatalf("big response was not refused under the caller's limit: %s %s", code.Inspect(), text.Inspect())
	}
	if _, isErr := list.Elements[2].(*object.Error); !isErr {
		t.Fatalf("a non-dict request did not give an error: %s", list.Elements[2].Inspect())
	}

	// With the caller's memory limit on, the workers' allocations are charged
	// to the caller.
	q := scriptling.New(scriptling.WithMemoryLimit(1<<40), scriptling.WithMaxValueBytes(0))
	stdlib.RegisterAll(q)
	RegisterRequestsLibrary(q)
	if _, err := q.Eval(`
import requests
held = requests.parallel([{"method": "GET", "url": "` + srv.URL + `/big"} for i in range(50)], max_parallel=4)
`); err != nil {
		t.Fatal(err)
	}
	if got := q.ResourceUsage().AllocatedBytes; got < 50*4096/2 {
		t.Fatalf("parallel responses charged only %d bytes to the caller", got)
	}
}

// Encoded tokens are sized by their encoded length, not as containers.
func TestSecretsTokenSizes(t *testing.T) {
	p := scriptling.New(scriptling.WithMaxValueBytes(1024))
	RegisterSecretsLibrary(p)
	result, err := p.Eval(`
import secrets
out = [len(secrets.token_hex(512)), len(secrets.token_urlsafe(700))]
for f in [lambda: secrets.token_hex(513), lambda: secrets.token_urlsafe(800), lambda: secrets.token_bytes(65)]:
    try:
        f()
        out.append("no error")
    except MemoryError:
        out.append("caught")
out.append(len(secrets.token_bytes(64)))
out
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); got != "[1024, 934, 'caught', 'caught', 'caught', 64]" {
		t.Fatalf("got %s", got)
	}
}

// A fire-and-forget task outlives the call that started it, even with a
// memory limit (which wraps each call in a cancellable run).
func TestBackgroundTaskOutlivesItsCall(t *testing.T) {
	for _, opts := range [][]scriptling.Option{nil, {scriptling.WithMemoryLimit(1 << 30)}} {
		ResetRuntime()
		SetSandboxFactory(nil)
		p := scriptling.New(opts...)
		stdlib.RegisterAll(p)
		RegisterRuntimeLibraryAll(p, nil)
		if _, err := p.Eval(`
import scriptling.runtime as runtime
import time
def task():
    time.sleep(0.1)
    n = 0
    for i in range(1000):
        n += 1
    return n
pr = runtime.background("outlives", "task")
`); err != nil {
			t.Fatal(err)
		}
		result, err := p.Eval("pr.get()")
		if err != nil || result.Inspect() != "1000" {
			t.Fatalf("memory limit %v: task result %v, error %v", opts != nil, result, err)
		}
	}
}

// A detached task still counts against the shared memory budget, and is
// stopped when it goes over it.
func TestBackgroundTaskStoppedByMemoryLimit(t *testing.T) {
	ResetRuntime()
	SetSandboxFactory(nil)
	p := scriptling.New(scriptling.WithMemoryLimit(8<<20), scriptling.WithMaxValueBytes(0))
	stdlib.RegisterAll(p)
	RegisterRuntimeLibraryAll(p, nil)
	if _, err := p.Eval(`
import scriptling.runtime as runtime
def hog():
    held = []
    n = 1000
    i = 0
    while True:
        held.append(str(i) * n)
        i += 1
pr = runtime.background("hog", "hog")
`); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, err := p.Eval("pr.get()")
	if err == nil {
		t.Fatal("task holding more than the memory limit ran to completion")
	}
	if time.Since(start) > 20*time.Second {
		t.Fatal("task was slow to stop")
	}
	if p.ResourceUsage().MemoryLimitCancellations == 0 {
		t.Fatalf("no cancellation counted: %v", err)
	}
}

// requests.parallel checks its batch against the caller's limits before
// sending anything.
func TestParallelRequestBatchChecked(t *testing.T) {
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()
	p := scriptling.New(scriptling.WithMaxValueBytes(1024))
	stdlib.RegisterAll(p)
	RegisterRequestsLibrary(p)
	result, err := p.Eval(`
import requests
try:
    requests.parallel([{"method": "GET", "url": "` + srv.URL + `"} for i in range(100)])
    r = "no error"
except MemoryError:
    r = "caught"
r
`)
	if err != nil {
		t.Fatal(err)
	}
	if result.Inspect() != "caught" {
		t.Fatalf("a 100-request batch under a 1024 byte value limit: %s", result.Inspect())
	}
	if hits.Load() != 0 {
		t.Fatalf("%d requests sent before the batch was refused", hits.Load())
	}
}

// Extreme token sizes are refused cleanly, not overflowed or allocated.
func TestSecretsExtremeTokenSizes(t *testing.T) {
	p := scriptling.New()
	RegisterSecretsLibrary(p)
	result, err := p.Eval(`
import secrets
out = []
for f in [secrets.token_bytes, secrets.token_hex, secrets.token_urlsafe]:
    for n in [4611686018427387904, 9223372036854775807, 1 << 41]:
        try:
            f(n)
            out.append("no error")
        except MemoryError:
            out.append("refused")
out
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); strings.Count(got, "refused") != 9 {
		t.Fatalf("got %s", got)
	}
}

// A token whose raw bytes and encoding together do not fit in the memory
// left is refused before it is built.
func TestSecretsTokenPeakMemory(t *testing.T) {
	p := scriptling.New(scriptling.WithMemoryLimit(4<<20), scriptling.WithMaxValueBytes(0))
	RegisterSecretsLibrary(p)
	result, err := p.Eval(`
import secrets
n = len(secrets.token_hex(1 << 20))
try:
    secrets.token_hex(3 << 20)
    r = "no error"
except MemoryError:
    r = "refused"
[n, r]
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); got != "[2097152, 'refused']" {
		t.Fatalf("got %s", got)
	}
}

// A batch whose bookkeeping would not fit in the memory limit is refused
// before anything is sent.
func TestParallelRequestBatchMemoryChecked(t *testing.T) {
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
	}))
	defer srv.Close()
	p := scriptling.New(scriptling.WithMemoryLimit(2<<20), scriptling.WithMaxValueBytes(0))
	stdlib.RegisterAll(p)
	RegisterRequestsLibrary(p)
	result, err := p.Eval(`
import requests
req = {"method": "GET", "url": "` + srv.URL + `"}
try:
    requests.parallel([req] * 10000)
    r = "no error"
except MemoryError:
    r = "caught"
r
`)
	if err != nil {
		t.Fatal(err)
	}
	if result.Inspect() != "caught" || hits.Load() != 0 {
		t.Fatalf("result %s, %d requests sent", result.Inspect(), hits.Load())
	}
}

// token_bytes lists stay charged for as long as they are held.
func TestSecretsTokenBytesRetainedIsCharged(t *testing.T) {
	p := scriptling.New(scriptling.WithMemoryLimit(8<<20), scriptling.WithMaxValueBytes(0))
	RegisterSecretsLibrary(p)
	_, err := p.Eval(`
import secrets
held = [secrets.token_bytes(200000) for i in range(3)]
`)
	if err == nil {
		t.Fatalf("three 3.2 MB lists held under an 8 MB limit; usage %+v", p.ResourceUsage())
	}
	if !strings.Contains(err.Error(), "memory limit exceeded") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// A shared task, like an isolated one, outlives the call that started it.
func TestSharedTaskOutlivesItsCall(t *testing.T) {
	for _, opts := range [][]scriptling.Option{nil, {scriptling.WithMemoryLimit(1 << 30)}} {
		ResetRuntime()
		SetSandboxFactory(nil)
		p := scriptling.New(opts...)
		stdlib.RegisterAll(p)
		RegisterRuntimeLibraryAll(p, nil)
		if _, err := p.Eval(`
import scriptling.runtime as runtime
import time
def task():
    time.sleep(0.1)
    return 42
pr = runtime.background("shared-outlives", "task", shared=True)
`); err != nil {
			t.Fatal(err)
		}
		result, err := p.Eval("pr.get()")
		if err != nil || result.Inspect() != "42" {
			t.Fatalf("memory limit %v: result %v, error %v", opts != nil, result, err)
		}
	}
}

// Wide request dicts are counted entry by entry in the batch's memory check.
func TestParallelRequestWideDictsChecked(t *testing.T) {
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
	}))
	defer srv.Close()
	p := scriptling.New(scriptling.WithMemoryLimit(2<<20), scriptling.WithMaxValueBytes(0))
	stdlib.RegisterAll(p)
	RegisterRequestsLibrary(p)
	result, err := p.Eval(`
import requests
req = {"method": "GET", "url": "` + srv.URL + `"}
for i in range(2000):
    req["k" + str(i)] = i
try:
    requests.parallel([req] * 40)
    r = "no error"
except MemoryError:
    r = "caught"
r
`)
	if err != nil {
		t.Fatal(err)
	}
	if result.Inspect() != "caught" || hits.Load() != 0 {
		t.Fatalf("40 requests of 2000 entries under 2 MB: %s, %d requests sent", result.Inspect(), hits.Load())
	}
}

// With only a memory limit set, reads, captured output and templates are
// still held to it.
func TestMemoryOnlyLimitBoundsBuffers(t *testing.T) {
	big := strings.Repeat("r", 6<<20)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(big))
	}))
	defer srv.Close()

	p := scriptling.New(scriptling.WithMemoryLimit(4<<20), scriptling.WithMaxValueBytes(0))
	stdlib.RegisterAll(p)
	RegisterRequestsLibrary(p)
	RegisterTemplateTextLibrary(p)
	p.EnableOutputCapture()
	result, err := p.Eval(`
import requests
import scriptling.template.text as tpl
out = []
try:
    requests.get("` + srv.URL + `")
    out.append("no error")
except MemoryError:
    out.append("read refused")
try:
    n = 1 << 20
    s = "o" * n
    for i in range(6):
        print(s)
    out.append("no error")
except MemoryError:
    out.append("output refused")
s = None
try:
    t = tpl.Set()
    t.add("{{range .L}}{{$.S}}{{end}}")
    t.render({"L": list(range(12)), "S": "t" * (1 << 19)})
    out.append("no error")
except MemoryError:
    out.append("render refused")
out
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); got != "['read refused', 'output refused', 'render refused']" {
		t.Fatalf("got %s", got)
	}
}

// Template data is converted under the interpreter's limits, so a small
// value sharing one list many times over cannot expand without bound.
func TestTemplateDataSharedExpansion(t *testing.T) {
	p := scriptling.New(scriptling.WithMaxValueBytes(1 << 20))
	stdlib.RegisterAll(p)
	RegisterTemplateTextLibrary(p)
	result, err := p.Eval(`
import scriptling.template.text as tpl
t = tpl.Set()
t.add("{{len .}}")
a = [0] * 100
b = [a] * 100
c = [b] * 100
d = [c] * 100
ok = t.render([a] * 10)
try:
    t.render(d)
    r = "no error"
except MemoryError:
    r = "caught"
[ok, r]
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); got != "['10', 'caught']" {
		t.Fatalf("got %s", got)
	}
}

// A parallel request's JSON body is converted and encoded under the
// caller's value limit.
func TestParallelRequestBodyLimited(t *testing.T) {
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()
	p := scriptling.New(scriptling.WithMaxValueBytes(4096))
	stdlib.RegisterAll(p)
	RegisterRequestsLibrary(p)
	// A value over the limit can only come from the host.
	p.SetVar("big", strings.Repeat("x", 5000))
	result, err := p.Eval(`
import requests
a = [0] * 100
b = [a] * 100
c = [b] * 100
rs = requests.parallel([
    {"method": "POST", "url": "` + srv.URL + `", "json": {"k": "v"}},
    {"method": "POST", "url": "` + srv.URL + `", "json": {"big": big}},
    {"method": "POST", "url": "` + srv.URL + `", "json": [c] * 100},
])
[r.status_code for r in rs]
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); got != "[200, 0, 0]" {
		t.Fatalf("statuses %s", got)
	}
	if hits.Load() != 1 {
		t.Fatalf("%d requests sent; only the small body should be", hits.Load())
	}
}

// Request headers, parameters and the encoded URL are held to the caller's
// value limit, on single requests and in parallel batches.
func TestRequestFieldsLimited(t *testing.T) {
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()
	p := scriptling.New(scriptling.WithMaxValueBytes(4096))
	stdlib.RegisterAll(p)
	RegisterRequestsLibrary(p)
	// 5,000 bytes: over the limit, so it can only come from the host.
	p.SetVar("big", strings.Repeat("k", 2500)+strings.Repeat("=", 2500))
	result, err := p.Eval(`
import requests
out = []
try:
    requests.get("` + srv.URL + `", params={big: "v"})
    out.append("no error")
except MemoryError:
    out.append("params refused")
try:
    requests.get("` + srv.URL + `", headers={"X-Big": big})
    out.append("no error")
except MemoryError:
    out.append("headers refused")
out.append(requests.get("` + srv.URL + `", params={"a": "b"}).status_code)
rs = requests.parallel([
    {"method": "GET", "url": "` + srv.URL + `", "params": {big: "v"}},
    {"method": "GET", "url": "` + srv.URL + `", "headers": {"X-Big": big}},
    {"method": "GET", "url": "` + srv.URL + `", "params": {"a": "b"}},
])
out.append([r.status_code for r in rs])
out
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); got != "['params refused', 'headers refused', 200, [0, 0, 200]]" {
		t.Fatalf("got %s", got)
	}
	if hits.Load() != 2 {
		t.Fatalf("%d requests reached the server; only the two small ones should", hits.Load())
	}
}

// An ordinary request's JSON body is converted and encoded under the value
// limit, and nothing is sent when it is over.
func TestSingleRequestJSONBodyLimited(t *testing.T) {
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()
	p := scriptling.New(scriptling.WithMaxValueBytes(4096))
	stdlib.RegisterAll(p)
	RegisterRequestsLibrary(p)
	p.SetVar("big", strings.Repeat("xyz", 2000))
	result, err := p.Eval(`
import requests
out = []
a = [0] * 100
b = [a] * 100
c = [b] * 100
for body in [{"big": big}, [c] * 100]:
    try:
        requests.post("` + srv.URL + `", json=body)
        out.append("no error")
    except MemoryError:
        out.append("refused")
out.append(requests.post("` + srv.URL + `", json={"k": "v"}).status_code)
out
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); got != "['refused', 'refused', 200]" {
		t.Fatalf("got %s", got)
	}
	if hits.Load() != 1 {
		t.Fatalf("%d requests reached the server; only the small one should", hits.Load())
	}
}

// A response is held as the read buffer plus two copies (text and bytes);
// the copies are admitted together before they are made.
func TestResponseCopiesAdmitted(t *testing.T) {
	env := object.NewEnvironment()
	g := object.NewResourceGuard()
	g.SetMemoryLimit(2<<20 + 512<<10)
	g.SetMaxValueBytes(0)
	env.SetResourceGuard(g)
	env.EnterGIL()
	defer env.ExitGIL()

	body := make([]byte, 1<<20)
	ok := createResponseInstance(200, nil, body, "u")
	if code, _ := ok.GetField("status_code"); code.Inspect() != "200" {
		t.Fatalf("1 MiB body with nothing held: status %s", code.Inspect())
	}

	held := object.NewString(string(make([]byte, 1<<20))) // 1 MiB charged
	if _, errObj := newResponseInstance(200, nil, body, "u"); errObj == nil || !strings.Contains(errObj.Inspect(), "memory limit") {
		t.Fatalf("a single request over the limit did not fail: %v", errObj)
	}
	refused := createResponseInstance(200, nil, body, "u")
	code, _ := refused.GetField("status_code")
	text, _ := refused.GetField("text")
	if code.Inspect() != "0" || !strings.Contains(text.Inspect(), "memory limit") {
		t.Fatalf("two more 1 MiB copies with 1 MiB held under 2.5 MiB: status %s, %s", code.Inspect(), text.Inspect())
	}
	runtime.KeepAlive(held)
}

// Each part of an outgoing request is a value of its own: the Basic Auth
// header is counted exactly as it is sent, and parts within the limit are
// not refused for their sum.
func TestOutgoingRequestAuthSizedExactly(t *testing.T) {
	user, pass := "alice", "s3cret"
	exact := int64(len("Authorization: Basic ")) + int64(len(base64.StdEncoding.EncodeToString([]byte(user+":"+pass))))
	env := object.NewEnvironment()
	g := object.NewResourceGuard()
	g.SetMaxValueBytes(exact) // set before taking the lock, which binds to a guard with a limit
	env.SetResourceGuard(g)
	env.EnterGIL()
	defer env.ExitGIL()

	url := strings.Repeat("u", int(exact))
	headers := map[string]string{"X": strings.Repeat("h", int(exact)-1)}
	if errObj := checkOutgoingRequest(url, url, headers, user, pass); errObj != nil {
		t.Fatalf("request whose parts are each within the limit refused: %v", errObj)
	}
	g.SetMaxValueBytes(exact - 1)
	errObj := checkOutgoingRequest("", "", nil, user, pass)
	if errObj == nil || !strings.Contains(errObj.Inspect(), "request authorization") {
		t.Fatalf("authorization header one byte over the limit: %v", errObj)
	}
	// Without a user no Authorization header is sent, so none is counted.
	if errObj := checkOutgoingRequest("", "", nil, "", "ignored"); errObj != nil {
		t.Fatalf("request without auth refused: %v", errObj)
	}
	g.SetMaxValueBytes(exact - 2)
	if errObj := checkOutgoingRequest(url[:int(exact)-1], "", nil, "", ""); errObj == nil || !strings.Contains(errObj.Inspect(), "request URL") {
		t.Fatalf("URL over the limit: %v", errObj)
	}
}

// Response headers are a value: over the limit, a single request raises
// MemoryError and a request in a batch becomes a status 0 response.
func TestResponseHeadersLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Big", strings.Repeat("h", 200))
		w.Write([]byte("ok"))
	}))
	defer srv.Close()
	p := scriptling.New(scriptling.WithMaxValueBytes(128))
	stdlib.RegisterAll(p)
	RegisterRequestsLibrary(p)
	result, err := p.Eval(`
import requests
try:
    requests.get("` + srv.URL + `")
    single = "no error"
except MemoryError as e:
    single = "too large" in str(e)
r = requests.parallel([{"method": "GET", "url": "` + srv.URL + `"}])[0]
[single, r.status_code, "too large" in r.text, len(r.headers)]
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); got != "[True, 0, True, 0]" {
		t.Fatalf("200-byte header under a 128-byte limit: %s", got)
	}
}

// A converted dict a script keeps hold of is charged to the interpreter.
func TestConvertedMapCharged(t *testing.T) {
	p := scriptling.New(scriptling.WithMemoryLimit(1<<40), scriptling.WithMaxValueBytes(0))
	if _, err := p.Eval("d = {}\nfor i in range(4096):\n    d[str(i)] = i"); err != nil {
		t.Fatal(err)
	}
	d, _ := p.GetVarAsObject("d")
	env := object.NewEnvironment()
	env.SetResourceGuard(p.ResourceGuard())
	env.EnterGIL()
	defer env.ExitGIL()
	before := p.ResourceUsage().AllocatedBytes
	converted, errObj := conversion.ToGoLimited(d)
	if errObj != nil {
		t.Fatal(errObj)
	}
	if got := p.ResourceUsage().AllocatedBytes - before; got < 4096*64 {
		t.Fatalf("a 4,096-entry map was charged %d bytes", got)
	}
	runtime.KeepAlive(converted)
}
