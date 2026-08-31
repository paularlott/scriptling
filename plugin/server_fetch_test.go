package plugin

import (
	"context"
	"errors"
	"sort"
	"testing"
)

// countingFetcher serves one file with mutable content and counts the calls it
// receives. When onlyPath is set, every other path (except the source itself,
// path "") is a miss.
type countingFetcher struct {
	content  string
	notFound bool
	onlyPath string
	reads    int
	lists    int
}

func (f *countingFetcher) Read(ctx context.Context, source, path string) ([]byte, error) {
	f.reads++
	if f.notFound || (f.onlyPath != "" && path != "" && path != f.onlyPath) {
		return nil, fmtNotFound(source, path)
	}
	return []byte(f.content), nil
}

func (f *countingFetcher) Glob(ctx context.Context, source, pattern string) ([]FetchEntry, error) {
	f.lists++
	if f.notFound {
		return nil, fmtNotFound(source, pattern)
	}
	tree := map[string]bool{"lib": true, "lib/hello.py": false, "manifest.toml": false}
	entries := []FetchEntry{}
	for name, isDir := range tree {
		if MatchGlob(pattern, name) {
			entries = append(entries, FetchEntry{Name: name, IsDir: isDir})
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries, nil
}

func fmtNotFound(source, path string) error {
	return errors.Join(ErrFetchNotFound, errors.New(source+" "+path))
}

func TestServerHandshakeAdvertisesTheScheme(t *testing.T) {
	server := NewServer("fetchy", "1.0.0", "fetch test")
	result := sendServerRequest[handshakeResult](t, server, "scriptling.handshake", handshakeParams{})
	if result.Scheme != "" {
		t.Fatalf("expected no scheme without a fetcher, got %q", result.Scheme)
	}

	fetcher := &countingFetcher{content: "x"}
	server.RegisterFetcher("ftest", fetcher)
	result = sendServerRequest[handshakeResult](t, server, "scriptling.handshake", handshakeParams{})
	if result.Scheme != "ftest" {
		t.Fatalf("expected scheme ftest, got %q", result.Scheme)
	}
}

func TestServerFetchReadDispatch(t *testing.T) {
	fetcher := &countingFetcher{content: "print(1)"}
	server := NewServer("fetchy", "1.0.0", "fetch test")
	server.RegisterFetcher("ftest", fetcher)

	res := sendServerRequest[fetchReadResult](t, server, "fetch.read", fetchReadParams{
		Source: "ftest://libs",
		Path:   "manifest.toml",
	})
	if string(res.Data) != "print(1)" {
		t.Fatalf("expected print(1), got %q", res.Data)
	}

	// A second read is another fetch — there is no conditional path.
	res = sendServerRequest[fetchReadResult](t, server, "fetch.read", fetchReadParams{
		Source: "ftest://libs",
		Path:   "manifest.toml",
	})
	if string(res.Data) != "print(1)" {
		t.Fatalf("expected print(1) on re-read, got %q", res.Data)
	}
	if fetcher.reads != 2 {
		t.Fatalf("expected 2 reads, got %d", fetcher.reads)
	}
}

func TestServerFetchReadNotFound(t *testing.T) {
	fetcher := &countingFetcher{content: "x", notFound: true}
	server := NewServer("fetchy", "1.0.0", "fetch test")
	server.RegisterFetcher("ftest", fetcher)

	rpcErr := sendServerRequestExpectError(t, server, "fetch.read", fetchReadParams{Source: "ftest://libs", Path: "nope.py"})
	if rpcErr.Code != FetchNotFoundCode {
		t.Fatalf("expected code %d, got %d (%s)", FetchNotFoundCode, rpcErr.Code, rpcErr.Message)
	}
}

func TestServerFetchGlobDispatch(t *testing.T) {
	fetcher := &countingFetcher{content: "x"}
	server := NewServer("fetchy", "1.0.0", "fetch test")
	server.RegisterFetcher("ftest", fetcher)

	// A wildcard-free pattern is the existence probe: the directory entry
	// itself comes back, so an empty directory is distinguishable from a
	// missing one.
	res := sendServerRequest[fetchGlobResult](t, server, "fetch.glob", fetchGlobParams{Source: "ftest://libs", Pattern: "lib"})
	if len(res.Entries) != 1 || res.Entries[0].Name != "lib" || !res.Entries[0].IsDir {
		t.Fatalf("unexpected exact-path entries: %+v", res.Entries)
	}

	res = sendServerRequest[fetchGlobResult](t, server, "fetch.glob", fetchGlobParams{Source: "ftest://libs", Pattern: "*"})
	if len(res.Entries) != 2 || res.Entries[0].Name != "lib" || !res.Entries[0].IsDir || res.Entries[1].Name != "manifest.toml" || res.Entries[1].IsDir {
		t.Fatalf("unexpected entries: %+v", res.Entries)
	}

	fetcher.notFound = true
	rpcErr := sendServerRequestExpectError(t, server, "fetch.glob", fetchGlobParams{Source: "ftest://libs", Pattern: "missing"})
	if rpcErr.Code != FetchNotFoundCode {
		t.Fatalf("expected code %d, got %d", FetchNotFoundCode, rpcErr.Code)
	}
}

func TestServerFetchUnknownScheme(t *testing.T) {
	fetcher := &countingFetcher{content: "x"}
	server := NewServer("fetchy", "1.0.0", "fetch test")
	server.RegisterFetcher("ftest", fetcher)

	rpcErr := sendServerRequestExpectError(t, server, "fetch.read", fetchReadParams{Source: "other://libs"})
	if rpcErr.Code == FetchNotFoundCode {
		t.Fatal("unknown scheme must not be reported as not-found")
	}
}
