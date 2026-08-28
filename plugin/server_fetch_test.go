package plugin

import (
	"context"
	"errors"
	"testing"
)

// countingFetcher serves one file with mutable content and records the
// validators it was asked about. When onlyPath is set, every other path
// (except the source itself, path "") is a miss.
type countingFetcher struct {
	content   string
	etag      string
	lastMod   string
	notFound  bool
	onlyPath  string
	reads     int
	lists     int
	seenEtag  string
	seenLastM string
}

func (f *countingFetcher) Read(ctx context.Context, source, path, etag, lastModified string) (FetchResult, error) {
	f.reads++
	f.seenEtag = etag
	f.seenLastM = lastModified
	if f.notFound || (f.onlyPath != "" && path != "" && path != f.onlyPath) {
		return FetchResult{}, fmtNotFound(source, path)
	}
	if etag != "" && etag == f.etag {
		return FetchResult{NotModified: true, ETag: f.etag, LastModified: f.lastMod}, nil
	}
	return FetchResult{Data: []byte(f.content), ETag: f.etag, LastModified: f.lastMod}, nil
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
	if len(result.Schemes) != 0 {
		t.Fatalf("expected no schemes without fetchers, got %v", result.Schemes)
	}
	for _, c := range result.Capabilities {
		if c == CapabilityFetch {
			t.Fatal("fetch capability advertised without any fetcher")
		}
	}

	fetcher := &countingFetcher{content: "x", etag: "v1"}
	server.RegisterFetcher("ftest", fetcher)
	server.DeclarePackage("ftest://libs")
	result = sendServerRequest[handshakeResult](t, server, "scriptling.handshake", handshakeParams{})
	if len(result.Schemes) != 1 || result.Schemes[0] != "ftest" {
		t.Fatalf("expected schemes [ftest], got %v", result.Schemes)
	}
	if len(result.Packages) != 1 || result.Packages[0] != "ftest://libs" {
		t.Fatalf("expected packages [ftest://libs], got %v", result.Packages)
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
	fetcher := &countingFetcher{content: "print(1)", etag: "v1", lastMod: "Mon, 01 Jan 2026 00:00:00 GMT"}
	server := NewServer("fetchy", "1.0.0", "fetch test")
	server.RegisterFetcher("ftest", fetcher)

	res := sendServerRequest[fetchReadResult](t, server, "fetch.read", fetchReadParams{
		Source: "ftest://libs",
		Path:   "manifest.toml",
	})
	if string(res.Data) != "print(1)" {
		t.Fatalf("expected print(1), got %q", res.Data)
	}
	if res.ETag != "v1" || res.LastModified != fetcher.lastMod {
		t.Fatalf("expected validators, got %+v", res)
	}

	// Conditional read with the matching validator answers not_modified.
	res = sendServerRequest[fetchReadResult](t, server, "fetch.read", fetchReadParams{
		Source: "ftest://libs",
		Path:   "manifest.toml",
		ETag:   "v1",
	})
	if !res.NotModified || res.Data != nil {
		t.Fatalf("expected not_modified, got %+v", res)
	}
	if fetcher.seenEtag != "v1" {
		t.Fatalf("expected peer to receive the validator, got %q", fetcher.seenEtag)
	}
}

func TestServerFetchReadNotFound(t *testing.T) {
	fetcher := &countingFetcher{content: "x", etag: "v1", notFound: true}
	server := NewServer("fetchy", "1.0.0", "fetch test")
	server.RegisterFetcher("ftest", fetcher)

	rpcErr := sendServerRequestExpectError(t, server, "fetch.read", fetchReadParams{Source: "ftest://libs", Path: "nope.py"})
	if rpcErr.Code != FetchNotFoundCode {
		t.Fatalf("expected code %d, got %d (%s)", FetchNotFoundCode, rpcErr.Code, rpcErr.Message)
	}
}

func TestServerFetchListDispatch(t *testing.T) {
	fetcher := &countingFetcher{content: "x", etag: "v1"}
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
	fetcher := &countingFetcher{content: "x", etag: "v1"}
	server := NewServer("fetchy", "1.0.0", "fetch test")
	server.RegisterFetcher("ftest", fetcher)

	rpcErr := sendServerRequestExpectError(t, server, "fetch.read", fetchReadParams{Source: "other://libs"})
	if rpcErr.Code == FetchNotFoundCode {
		t.Fatal("unknown scheme must not be reported as not-found")
	}
}
