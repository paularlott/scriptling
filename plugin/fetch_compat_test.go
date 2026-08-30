package plugin

import (
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/paularlott/scriptling/object"
)

// TestFetchCoexistsWithFullPluginSurface proves the fetch additions did not
// disturb the original plugin contract: one peer registers a fetcher AND the
// classic surface (functions, classes, constants), and every kind of call —
// function calls, object construction and method calls, batches, and fetch
// reads — works over the same connection.
func TestFetchCoexistsWithFullPluginSurface(t *testing.T) {
	// One server with both surfaces registered.
	server := NewServer("combi", "1.0.0", "fetcher plus functions and classes")
	server.RegisterFetcher("combi", staticFetcher{})
	greet := object.NewFunctionBuilder()
	greet.Function(func(name string) string { return "Hello, " + name })
	server.RegisterFunc("greet", greet)
	counter := object.NewClassBuilder("Counter").
		Method("__init__", func(self *object.Instance, start int) {
			self.SetField("value", object.NewInteger(int64(start)))
		}).
		Method("inc", func(self *object.Instance, amount int) int {
			current := self.Field("value").(*object.Integer).IntValue()
			next := current + int64(amount)
			self.SetField("value", object.NewInteger(next))
			return int(next)
		})
	server.RegisterClass(counter)
	server.Constant("default_name", "World")

	// Wire the server to a client over in-process pipes.
	clientToPluginR, clientToPluginW := io.Pipe()
	pluginToClientR, pluginToClientW := io.Pipe()
	go func() { _ = server.RunIO(clientToPluginR, pluginToClientW) }()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := LoadClientFromIO(ctx, pluginToClientR, clientToPluginW)
	if err != nil {
		t.Fatalf("LoadClientFromIO: %v", err)
	}
	defer func() {
		_ = client.Close()
		_ = clientToPluginW.Close()
		_ = pluginToClientR.Close()
	}()

	// The handshake reports both surfaces.
	meta := client.Metadata()
	if meta.Scheme != "combi" || !client.SupportsFetch() {
		t.Fatalf("expected scheme combi with fetch support, got %+v", meta)
	}
	if len(meta.Schema.Functions) == 0 || len(meta.Schema.Classes) == 0 {
		t.Fatalf("expected the classic schema alongside the fetcher, got %+v", meta.Schema)
	}

	// Function calls still work.
	result, err := client.CallFunction(ctx, "greet", []Value{{Type: valueString, Value: "Ada"}}, nil)
	if err != nil {
		t.Fatalf("CallFunction: %v", err)
	}
	if result.Type != valueString || result.Value != "Hello, Ada" {
		t.Fatalf("unexpected function result: %+v", result)
	}

	// Objects still work: construct, call a method.
	ref, err := client.NewObject(ctx, "Counter", []Value{{Type: valueInt, Value: int64(10)}}, nil)
	if err != nil {
		t.Fatalf("NewObject: %v", err)
	}
	result, err = client.CallMethod(ctx, ref.ID, "inc", []Value{{Type: valueInt, Value: int64(5)}}, nil)
	if err != nil {
		t.Fatalf("CallMethod: %v", err)
	}
	if result.Type != valueInt || result.Value.(float64) != 15 {
		t.Fatalf("unexpected method result: %+v", result)
	}
	if err := client.DestroyObject(ctx, ref.ID); err != nil {
		t.Fatalf("DestroyObject: %v", err)
	}

	// Batch calls mixing a function call with a fetch read.
	raw, err := client.Batch(ctx, []batchRequest{
		{Method: "function.call", Params: functionCallParams{Name: "greet", Args: []Value{{Type: valueString, Value: "Batch"}}}},
		{Method: "fetch.read", Params: fetchReadParams{Source: "combi://libs", Path: "lib/hello.py"}},
	})
	if err != nil {
		t.Fatalf("Batch: %v", err)
	}
	if len(raw) != 2 {
		t.Fatalf("expected 2 batch results, got %d", len(raw))
	}
	var fnResult struct {
		Type  string `json:"type"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(raw[0], &fnResult); err != nil || fnResult.Value != "Hello, Batch" {
		t.Fatalf("batch function result: %v (%v)", string(raw[0]), err)
	}
	var fetchResult fetchReadResult
	if err := json.Unmarshal(raw[1], &fetchResult); err != nil || string(fetchResult.Data) != "print('served by combi')\n" {
		t.Fatalf("batch fetch result: %+v (%v)", fetchResult, err)
	}

	// A plain fetch read over the same connection.
	data, err := client.FetchFile(ctx, "combi://libs", "lib/hello.py")
	if err != nil {
		t.Fatalf("FetchFile: %v", err)
	}
	if string(data) != "print('served by combi')\n" {
		t.Fatalf("unexpected fetch content: %q", data)
	}

	// Entries: the classic listing surface still enumerates the plugin.
	if listing := client.Metadata(); listing.Name != "plugin.combi" {
		t.Fatalf("unexpected library name %q", listing.Name)
	}
}

// staticFetcher serves two small files.
type staticFetcher struct{}

func (staticFetcher) Read(ctx context.Context, source, path string) ([]byte, error) {
	switch path {
	case "lib/hello.py":
		return []byte("print('served by combi')\n"), nil
	case "lib/util.py":
		return []byte("x = 1\n"), nil
	}
	return nil, ErrFetchNotFound
}

func (staticFetcher) Glob(ctx context.Context, source, pattern string) ([]FetchEntry, error) {
	tree := map[string]bool{"lib": true, "lib/hello.py": false, "lib/util.py": false}
	entries := []FetchEntry{}
	for name, isDir := range tree {
		if MatchGlob(pattern, name) {
			entries = append(entries, FetchEntry{Name: name, IsDir: isDir})
		}
	}
	return entries, nil
}
