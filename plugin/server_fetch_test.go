package plugin

import (
	"context"
	"errors"
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

func (f *countingFetcher) Read(ctx context.Context, source, path string) (FetchResult, error) {
	f.reads++
	if f.notFound || (f.onlyPath != "" && path != "" && path != f.onlyPath) {
		return FetchResult{}, fmtNotFound(source, path)
	}
	return FetchResult{Data: []byte(f.content)}, nil
}

func (f *countingFetcher) List(ctx context.Context, source, path string) ([]FetchEntry, error) {
	f.lists++
	if f.notFound {
		return nil, fmtNotFound(source, path)
	}
	return []FetchEntry{{Name: "lib", IsDir: true}, {Name: "manifest.toml"}}, nil
}

func fmtNotFound(source, path string) error {
	return errors.Join(ErrFetchNotFound, errors.New(source+" "+path))
}

func TestServerHandshakeAdvertisesFetchCapability(t *testing.T) {
	server := NewServer("fetchy", "1.0.0", "fetch test")
	result := sendServerRequest[handshakeResult](t, server, "scriptling.handshake", handshakeParams{})
	if result.Scheme != "" {
		t.Fatalf("expected no scheme without a fetcher, got %q", result.Scheme)
	}
	for _, c := range result.Capabilities {
		if c == CapabilityFetch {
			t.Fatal("fetch capability advertised without any fetcher")
		}
	}

	fetcher := &countingFetcher{content: "x"}
	server.RegisterFetcher("ftest", fetcher)
	result = sendServerRequest[handshakeResult](t, server, "scriptling.handshake", handshakeParams{})
	if result.Scheme != "ftest" {
		t.Fatalf("expected scheme ftest, got %q", result.Scheme)
	}
	found := false
	for _, c := range result.Capabilities {
		if c == CapabilityFetch {
			found = true
		}
	}
	if !found {
		t.Fatal("expected fetch capability with a registered fetcher")
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

func TestServerFetchListDispatch(t *testing.T) {
	fetcher := &countingFetcher{content: "x"}
	server := NewServer("fetchy", "1.0.0", "fetch test")
	server.RegisterFetcher("ftest", fetcher)

	res := sendServerRequest[fetchListResult](t, server, "fetch.list", fetchListParams{Source: "ftest://libs"})
	if len(res.Entries) != 2 || res.Entries[0].Name != "lib" || !res.Entries[0].IsDir || res.Entries[1].Name != "manifest.toml" || res.Entries[1].IsDir {
		t.Fatalf("unexpected entries: %+v", res.Entries)
	}

	fetcher.notFound = true
	rpcErr := sendServerRequestExpectError(t, server, "fetch.list", fetchListParams{Source: "ftest://libs", Path: "missing"})
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
