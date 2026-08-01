package scriptling_test

import (
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/stdlib"
)

const routerSetup = `
endpoints = {
    "server-a": {"load": 0.30, "models": ["gpt-4", "gpt-3.5", "gpt-4o"]},
    "server-b": {"load": 0.62, "models": ["gpt-4", "gpt-4o"]},
    "server-c": {"load": 0.10, "models": ["gpt-3.5"]},
    "server-d": {"load": 0.45, "models": ["gpt-4", "gpt-3.5", "gpt-4o", "o1"]},
}
def route(request):
    model = request["model"]
    best = None
    best_load = 2.0
    for name, info in endpoints.items():
        if model in info["models"]:
            if info["load"] < best_load:
                best_load = info["load"]
                best = name
    if best == None:
        return {"error": "no server", "model": model}
    return {"server": best, "load": best_load, "model": model}
`

func BenchmarkAIRouter(b *testing.B) {
	p := scriptling.New()
	stdlib.RegisterAll(p)
	p.Eval(routerSetup)
	req := map[string]interface{}{"model": "gpt-4", "messages": []interface{}{map[string]interface{}{"role": "user", "content": "hi"}}}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.CallFunction("route", req)
	}
}

func BenchmarkItemsUnpack(b *testing.B) {
	p := scriptling.New()
	stdlib.RegisterAll(p)
	p.Eval(`d = {"a":1, "b":2, "c":3, "d":4}`)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Eval(`s = 0
for k, v in d.items():
    s += v`)
	}
}
