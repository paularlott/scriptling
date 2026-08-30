package plugin

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

// TestMatchGlob pins the fetch glob language: * and ? stay within a segment,
// ** crosses any number of segments (including none), a wildcard-free
// pattern is an exact-path probe.
func TestMatchGlob(t *testing.T) {
	cases := []struct {
		pattern string
		name    string
		want    bool
	}{
		{"lib", "lib", true},
		{"lib", "libs", false},
		{"lib", "lib/hello.py", false},
		{"*", "lib", true},
		{"*", "lib/hello.py", false},
		{"lib/*", "lib/hello.py", true},
		{"lib/*", "lib/sub/hello.py", false},
		{"lib/*.py", "lib/hello.py", true},
		{"lib/*.py", "lib/hello.txt", false},
		{"lib/he?lo.py", "lib/hello.py", true},
		{"lib/he?lo.py", "lib/helloo.py", false},
		{"lib/[hc]*", "lib/hello.py", true},
		{"lib/[hc]*", "lib/greet.py", false},
		{"lib/**", "lib", true},          // ** matches zero segments
		{"lib/**", "lib/a/b/c.py", true}, // and any depth
		{"lib/**", "other/x.py", false},
		{"**/*.py", "lib/a/b.py", true},
		{"**/*.py", "top.py", true},
		{"**/*.py", "lib/a/b.txt", false},
		{"**", "anything/at/all.py", true},
	}
	for _, tc := range cases {
		if got := MatchGlob(tc.pattern, tc.name); got != tc.want {
			t.Errorf("MatchGlob(%q, %q) = %v, want %v", tc.pattern, tc.name, got, tc.want)
		}
	}
}

// TestGlobDisk proves the reference disk implementation: subtree patterns in
// one call, directories as entries (empty ones included), and the root
// defense: symlinks resolving outside the root are not served.
func TestGlobDisk(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("lib/hello.py", "x = 1\n")
	write("lib/sub/deep.py", "y = 2\n")
	write("docs/intro.md", "# hi\n")
	if err := os.MkdirAll(filepath.Join(root, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("no"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "lib", "escape.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "lib", "hello.py"), filepath.Join(root, "lib", "alias.py")); err != nil {
		t.Fatal(err)
	}

	names := func(entries []FetchEntry) []string {
		out := make([]string, len(entries))
		for i, e := range entries {
			out[i] = e.Name
		}
		return out
	}

	// A whole subtree, one call, escape excluded, inside-link included.
	entries, err := GlobDisk(root, "lib/**")
	if err != nil {
		t.Fatalf("GlobDisk: %v", err)
	}
	got := names(entries)
	want := []string{"lib", "lib/alias.py", "lib/hello.py", "lib/sub", "lib/sub/deep.py"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("lib/** = %v, want %v", got, want)
	}

	// One level.
	entries, err = GlobDisk(root, "lib/*")
	if err != nil {
		t.Fatalf("GlobDisk: %v", err)
	}
	got = names(entries)
	want = []string{"lib/alias.py", "lib/hello.py", "lib/sub"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("lib/* = %v, want %v", got, want)
	}

	// Exact-path probes: a populated dir, an empty dir, a file, a miss.
	for _, tc := range []struct {
		pattern  string
		wantName string
		wantDir  bool
	}{
		{"lib", "lib", true},
		{"empty", "empty", true},
		{"docs/intro.md", "docs/intro.md", false},
	} {
		entries, err = GlobDisk(root, tc.pattern)
		if err != nil {
			t.Fatalf("GlobDisk(%q): %v", tc.pattern, err)
		}
		if len(entries) != 1 || entries[0].Name != tc.wantName || entries[0].IsDir != tc.wantDir {
			t.Fatalf("GlobDisk(%q) = %+v, want one entry {%s dir=%v}", tc.pattern, entries, tc.wantName, tc.wantDir)
		}
	}

	// The escaping symlink must not be served even when named exactly.
	entries, err = GlobDisk(root, "lib/escape.txt")
	if err != nil {
		t.Fatalf("GlobDisk: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("symlink out of the root was served: %+v", entries)
	}
}

// flakyFetcher fails the first failures reads with an unavailable backend,
// then succeeds; denied answers are permanent.
type flakyFetcher struct {
	failures int32
	calls    int32
	denied   bool
}

func (f *flakyFetcher) Read(ctx context.Context, source, path string) ([]byte, error) {
	atomic.AddInt32(&f.calls, 1)
	if f.denied {
		return nil, fmt.Errorf("%w: %s", ErrFetchDenied, path)
	}
	if atomic.AddInt32(&f.calls, 0) <= 0 {
		return []byte("ok"), nil
	}
	if n := atomic.AddInt32(&f.failures, -1); n >= 0 {
		return nil, fmt.Errorf("%w: upstream 503", ErrFetchUnavailable)
	}
	return []byte("recovered"), nil
}

func (f *flakyFetcher) Glob(ctx context.Context, source, pattern string) ([]FetchEntry, error) {
	return nil, nil
}

// TestFetchRetriesUnavailable proves the bounded retry: an unavailable
// backend is retried until it answers, while a denial comes back after
// exactly one attempt.
func TestFetchRetriesUnavailable(t *testing.T) {
	fetcher := &flakyFetcher{failures: 2}
	server := NewServer("flaky", "1.0.0", "flaky backend")
	server.RegisterFetcher("flaky", fetcher)
	srv := httptest.NewServer(server)
	t.Cleanup(srv.Close)

	manager := NewManager(nil)
	t.Cleanup(func() { _ = manager.Close() })
	client, err := manager.LoadURL(context.Background(), "flaky", srv.URL, true, false)
	if err != nil {
		t.Fatalf("LoadURL: %v", err)
	}

	data, err := client.FetchFile(context.Background(), "flaky://libs", "x.py")
	if err != nil {
		t.Fatalf("FetchFile after retries: %v", err)
	}
	if string(data) != "recovered" {
		t.Fatalf("unexpected content %q", data)
	}
	if calls := atomic.LoadInt32(&fetcher.calls); calls != FetchRetryAttempts {
		t.Fatalf("expected %d attempts, got %d", FetchRetryAttempts, calls)
	}

	fetcher.denied = true
	atomic.StoreInt32(&fetcher.calls, 0)
	_, err = client.FetchFile(context.Background(), "flaky://libs", "x.py")
	if !errors.Is(err, ErrFetchDenied) {
		t.Fatalf("expected ErrFetchDenied, got %v", err)
	}
	if calls := atomic.LoadInt32(&fetcher.calls); calls != 1 {
		t.Fatalf("a denial must not be retried, got %d attempts", calls)
	}
}

// TestFetchRetriesExhausted proves the retry is bounded: an always-unavailable
// backend fails after FetchRetryAttempts, not forever.
func TestFetchRetriesExhausted(t *testing.T) {
	fetcher := &flakyFetcher{failures: 1 << 30}
	server := NewServer("down", "1.0.0", "down backend")
	server.RegisterFetcher("down", fetcher)
	srv := httptest.NewServer(server)
	t.Cleanup(srv.Close)

	manager := NewManager(nil)
	t.Cleanup(func() { _ = manager.Close() })
	client, err := manager.LoadURL(context.Background(), "down", srv.URL, true, false)
	if err != nil {
		t.Fatalf("LoadURL: %v", err)
	}
	_, err = client.FetchFile(context.Background(), "down://libs", "x.py")
	if !errors.Is(err, ErrFetchUnavailable) {
		t.Fatalf("expected ErrFetchUnavailable, got %v", err)
	}
	if calls := atomic.LoadInt32(&fetcher.calls); calls != FetchRetryAttempts {
		t.Fatalf("expected the retry to stop at %d attempts, got %d", FetchRetryAttempts, calls)
	}
}
