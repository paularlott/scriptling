package extlibs

import (
	"fmt"
	"os"
	"path/filepath"
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

// Libraries whose input is not a script value (files, directories, parsed
// records) build their results under the caller's limits: an over-limit call
// stops with a MemoryError instead of materialising everything first, and the
// process heap stays near the limit while it runs.

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

// writeMatchFiles writes n files of lines matching "match" and returns the
// directory and the bytes of matches written.
func writeMatchFiles(t *testing.T, n, linesPerFile int) (string, int) {
	t.Helper()
	dir := t.TempDir()
	line := strings.Repeat("match ", 16) + "\n"
	perFile := strings.Repeat(line, linesPerFile)
	for i := range n {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%03d.txt", i)), []byte(perFile), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir, n * len(perFile)
}

func TestGrepBoundedByMemoryLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("writes 48 MB of files")
	}
	dir, total := writeMatchFiles(t, 64, 8000)
	const limit = 8 << 20
	if total < 4*limit {
		t.Fatalf("test data of %d bytes is not well over the limit", total)
	}
	p := scriptling.New(scriptling.WithMemoryLimit(limit))
	RegisterGrepLibrary(p, nil)
	p.SetVar("dir", dir)

	sampler := startPeakHeapSampler()
	result, err := p.Eval(`
import scriptling.grep as grep
try:
    r = len(grep.pattern("match", dir, recursive=True))
except MemoryError as e:
    r = "caught: " + str(e)
r
`)
	grown := sampler.grown()
	overLimit(t, result, err)
	if grown > 4*limit {
		t.Fatalf("heap grew %d MiB while grep stopped at an 8 MiB limit", grown>>20)
	}

	// Within the limit the same call works, in a directory and in one file.
	small, _ := writeMatchFiles(t, 2, 10)
	p = scriptling.New(scriptling.WithMemoryLimit(limit))
	RegisterGrepLibrary(p, nil)
	p.SetVar("small", small)
	result, err = p.Eval(`
import scriptling.grep as grep
a = grep.pattern("match", small, recursive=True)
b = grep.pattern("match", small + "/f000.txt")
[len(a), len(b), a[0]["text"] == b[0]["text"]]
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); got != "[20, 10, True]" {
		t.Fatalf("got %s", got)
	}
}

func TestGrepSingleFileBoundedByMemoryLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("writes 24 MB of matches")
	}
	dir, _ := writeMatchFiles(t, 1, 250_000)
	p := scriptling.New(scriptling.WithMemoryLimit(8 << 20))
	RegisterGrepLibrary(p, nil)
	p.SetVar("file", filepath.Join(dir, "f000.txt"))
	sampler := startPeakHeapSampler()
	result, err := p.Eval(`
import scriptling.grep as grep
try:
    r = len(grep.pattern("match", file, max_size=1 << 30))
except MemoryError as e:
    r = "caught: " + str(e)
r
`)
	grown := sampler.grown()
	overLimit(t, result, err)
	if grown > 4*(8<<20) {
		t.Fatalf("heap grew %d MiB", grown>>20)
	}
}

func TestSedExtractBoundedByMemoryLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("writes 48 MB of files")
	}
	dir, _ := writeMatchFiles(t, 64, 8000)
	p := scriptling.New(scriptling.WithMemoryLimit(8 << 20))
	RegisterSedLibrary(p, nil)
	p.SetVar("dir", dir)
	sampler := startPeakHeapSampler()
	result, err := p.Eval(`
import scriptling.sed as sed
try:
    r = len(sed.extract("(match) (match)", dir, recursive=True))
except MemoryError as e:
    r = "caught: " + str(e)
[r, len(sed.extract("^(match) (match)", dir + "/f000.txt"))]
`)
	grown := sampler.grown()
	overLimit(t, result, err)
	if err == nil && !strings.HasSuffix(result.Inspect(), ", 8000]") {
		t.Fatalf("got %s", result.Inspect())
	}
	if grown > 4*(8<<20) {
		t.Fatalf("heap grew %d MiB while sed stopped at an 8 MiB limit", grown>>20)
	}
}

func TestFindBoundedByMemoryLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("creates 6000 files")
	}
	dir := t.TempDir()
	name := strings.Repeat("n", 200)
	for i := range 6000 {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%s%04d", name, i)), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p := scriptling.New(scriptling.WithMemoryLimit(1 << 20))
	RegisterFindLibrary(p, nil)
	p.SetVar("dir", dir)
	result, err := p.Eval(`
import scriptling.find as find
out = []
for call in [lambda: find.path(dir, recursive=True), lambda: find.entries(dir, recursive=True)]:
    try:
        out.append(len(call()))
    except MemoryError as e:
        out.append("caught" if "memory limit exceeded" in str(e) else str(e))
out
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); got != "['caught', 'caught']" {
		t.Fatalf("6000 long paths under a 1 MiB limit: %s", got)
	}

	// A smaller search in the same tree works.
	result, err = p.Eval(`len(find.path(dir, name="*0000", recursive=True))`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); got != "1" {
		t.Fatalf("got %s", got)
	}
}

func TestListdirBoundedByValueLimit(t *testing.T) {
	dir := t.TempDir()
	for i := range 500 {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%03d", i)), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	small := t.TempDir()
	for i := range 3 {
		if err := os.WriteFile(filepath.Join(small, fmt.Sprintf("f%d", i)), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p := scriptling.New(scriptling.WithMaxValueBytes(1024))
	RegisterOSLibrary(p, nil)
	p.SetVar("dir", dir)
	p.SetVar("small", small)
	result, err := p.Eval(`
import os
try:
    r = len(os.listdir(dir))
except MemoryError:
    r = "caught"
[r, sorted(os.listdir(small))]
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); got != "['caught', ['f0', 'f1', 'f2']]" {
		t.Fatalf("got %s", got)
	}
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
	RegisterCsvLibrary(p)
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
	RegisterCsvLibrary(p)
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
	RegisterCsvLibrary(p)
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
