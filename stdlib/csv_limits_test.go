package stdlib_test

import (
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

// overLimit reports whether a call ended for the memory limit: either the
// script caught the MemoryError (result is the caught message) or the sampled
// accounting cancelled the run.
func overLimit(t *testing.T, result object.Object, err error) {
	t.Helper()
	if err != nil {
		if !strings.Contains(err.Error(), "memory limit exceeded") {
			t.Fatalf("unexpected error: %v", err)
		}
		return
	}
	if !strings.Contains(result.Inspect(), "caught: memory limit exceeded") {
		t.Fatalf("call over the memory limit finished: %s", result.Inspect())
	}
}

// peakHeapSampler polls heap use and keeps the high-water mark above the
// heap in use when it started.
type peakHeapSampler struct {
	base    int64
	peak    atomic.Int64
	stop    chan struct{}
	stopped chan struct{}
}

func startPeakHeapSampler() *peakHeapSampler {
	s := &peakHeapSampler{stop: make(chan struct{}), stopped: make(chan struct{})}
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	s.base = int64(ms.HeapAlloc)
	s.peak.Store(s.base)
	go func() {
		defer close(s.stopped)
		var ms runtime.MemStats
		for {
			select {
			case <-s.stop:
				return
			default:
			}
			runtime.ReadMemStats(&ms)
			for {
				cur := s.peak.Load()
				if int64(ms.HeapAlloc) <= cur || s.peak.CompareAndSwap(cur, int64(ms.HeapAlloc)) {
					break
				}
			}
			time.Sleep(time.Millisecond)
		}
	}()
	return s
}

// grown stops sampling and returns how far the heap grew above its start,
// or 0 under the race detector, whose heap sizes say nothing about the code.
func (s *peakHeapSampler) grown() int64 {
	close(s.stop)
	<-s.stopped
	if raceDetector {
		return 0
	}
	return s.peak.Load() - s.base
}

func TestCsvLoadsBoundedByMemoryLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("parses 5 MB of CSV")
	}
	// Short cells: the values a row becomes cost far more than its text, so
	// 5 MB of text within the limit does not fit as values.
	row := strings.Repeat("a,", 63) + "a\n"
	text := strings.Repeat(row, 40_000)
	const limit = 8 << 20
	p := scriptling.New(scriptling.WithMemoryLimit(limit))
	stdlib.RegisterAll(p)
	p.SetVar("text", text)
	sampler := startPeakHeapSampler()
	result, err := p.Eval(`
import csv
try:
    r = len(csv.loads(text))
except MemoryError as e:
    r = "caught: " + str(e)
r
`)
	grown := sampler.grown()
	overLimit(t, result, err)
	if grown > 4*limit {
		t.Fatalf("heap grew %d MiB while csv.loads stopped at an 8 MiB limit", grown>>20)
	}

	p = scriptling.New(scriptling.WithMemoryLimit(limit))
	registerCSV(p)
	result, err = p.Eval(`
import csv
csv.loads("a,b\n1,\"2,3\"\n")`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); got != "[['a', 'b'], ['1', '2,3']]" {
		t.Fatalf("got %s", got)
	}
}

func TestCsvLoadsBoundedByValueLimit(t *testing.T) {
	p := scriptling.New(scriptling.WithMaxValueBytes(512))
	registerCSV(p)
	// Inputs over the limit can only come from the host.
	p.SetVar("rows", strings.Repeat("a,b\n", 200))
	p.SetVar("wide", strings.Repeat("a,", 200)+"\n")
	result, err := p.Eval(`
import csv
out = []
for text in [rows, wide, "a,b\nc,d\n"]:
    try:
        out.append(len(csv.loads(text)))
    except MemoryError:
        out.append("caught")
out
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); got != "['caught', 'caught', 2]" {
		t.Fatalf("got %s", got)
	}
}

// registerCSV registers the csv library under both of its names, as
// stdlib.RegisterAll does, without the rest of the stdlib.
func registerCSV(p *scriptling.Scriptling) {
	p.RegisterLibrary(stdlib.CSVLibrary)
	p.RegisterLibrary(stdlib.CSVAliasLibrary)
}
