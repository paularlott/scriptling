package scriptling_test

import (
	"io"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/extlibs/similarity"
	"github.com/paularlott/scriptling/extlibs/template"
	"github.com/paularlott/scriptling/extlibs/toml"
	"github.com/paularlott/scriptling/extlibs/yaml"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

// Benchmarks for the paths the output and context limits run through, each
// with no limits set (the default) and under a sandbox's limits.

const outputBenchSetup = `import json, yaml, toml, difflib
import scriptling.similarity as sim
import scriptling.template.text as text
small = [1, "a", 2.5, None, True]
medium = [{"id": i, "name": "item" + str(i), "tags": ["x", "y"], "score": i * 1.5} for i in range(200)]
cfg = {"name": "svc", "port": 8080, "debug": False, "tags": ["a", "b"], "db": {"host": "h", "pool": 4}}
t = text.Set()
t.add("{{range .}}{{.name}}={{.score}};{{end}}")
def add(a, b):
    return a + b
doc1 = "the quick brown fox jumps over the lazy dog\n" * 20
doc2 = "the quick brown cat jumps over the lazy dog\n" * 20
`

var outputBenches = []struct{ name, script string }{
	{"StrSmall", "for i in range(100):\n    s = str(small)\n"},
	{"ReprMedium", "s = repr(medium)\n"},
	{"FStringSmall", "for i in range(100):\n    s = f\"{small} {i} {cfg!r}\"\n"},
	{"PercentFormat", "for i in range(100):\n    s = \"%s %r %d\" % (small, cfg, i)\n"},
	{"PrintMulti", "for i in range(100):\n    print(small, i, cfg, sep=\", \")\n"},
	{"Join", "for i in range(100):\n    s = \",\".join([\"a\", \"bb\", \"ccc\", str(i)])\n"},
	{"JSONDumpsSmall", "for i in range(100):\n    s = json.dumps(cfg)\n"},
	{"JSONDumpsMedium", "s = json.dumps(medium)\n"},
	{"JSONDumpsIndent", "s = json.dumps(medium, indent=2)\n"},
	{"JSONLoads", "for i in range(100):\n    v = json.loads('{\"a\": [1, 2, {\"b\": null}], \"c\": \"d\"}')\n"},
	{"YAMLDumpSmall", "for i in range(20):\n    s = yaml.dump(cfg)\n"},
	{"YAMLDumpMedium", "s = yaml.dump(medium)\n"},
	{"TOMLDumps", "for i in range(20):\n    s = toml.dumps(cfg)\n"},
	{"ListRepeat", "for i in range(100):\n    l = [small] * 100\n"},
	{"TupleKeys", "d = {}\nfor i in range(200):\n    d[(i, (i, \"k\"))] = i\n"},
	{"CallLoop", "x = 0\nfor i in range(1000):\n    x = add(x, i)\n"},
	{"DifflibRatio", "r = difflib.ratio(doc1, doc2)\n"},
	{"DifflibUnified", "r = difflib.unified_diff(doc1, doc2)\n"},
	{"SimilarityScore", "for i in range(20):\n    r = sim.score(\"kitten sitting on the mat\", \"sitting kitten on a mat\")\n"},
	{"TemplateRender", "s = t.render(medium)\n"},
}

func outputBenchInterp(b *testing.B, limits bool) *scriptling.Scriptling {
	var p *scriptling.Scriptling
	if limits {
		p = scriptling.New(scriptling.WithMemoryLimit(256<<20), scriptling.WithMaxValueBytes(8<<20))
	} else {
		p = scriptling.New()
	}
	stdlib.RegisterAll(p)
	yaml.Register(p)
	toml.Register(p)
	similarity.Register(p)
	template.RegisterText(p)
	p.SetOutputWriter(io.Discard)
	if _, err := p.Eval(outputBenchSetup); err != nil {
		b.Fatal(err)
	}
	return p
}

func benchOutput(b *testing.B, limits bool) {
	for _, bench := range outputBenches {
		b.Run(bench.name, func(b *testing.B) {
			p := outputBenchInterp(b, limits)
			if _, err := p.Eval(bench.script); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := p.Eval(bench.script); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkOutputNoLimits(b *testing.B) { benchOutput(b, false) }
func BenchmarkOutputLimits(b *testing.B)   { benchOutput(b, true) }

func benchToGo(b *testing.B, limits bool) {
	p := outputBenchInterp(b, limits)
	v, err := p.Eval("medium")
	if err != nil {
		b.Fatal(err)
	}
	release := object.BindGuard(p.ResourceGuard())
	defer release()
	b.Run("ToGo", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = conversion.ToGo(v)
		}
	})
	b.Run("ToGoLimited", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, errObj := conversion.ToGoLimited(v); errObj != nil {
				b.Fatal(errObj)
			}
		}
	})
	b.Run("Inspect", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = v.Inspect()
		}
	})
}

func BenchmarkConvertNoLimits(b *testing.B) { benchToGo(b, false) }
func BenchmarkConvertLimits(b *testing.B)   { benchToGo(b, true) }
