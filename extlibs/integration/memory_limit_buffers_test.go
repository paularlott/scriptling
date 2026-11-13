package integration

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/requests"
	"github.com/paularlott/scriptling/extlibs/template"
	"github.com/paularlott/scriptling/stdlib"
)

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
	requests.Register(p)
	template.RegisterText(p)
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
