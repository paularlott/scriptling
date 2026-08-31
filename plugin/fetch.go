package plugin

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/paularlott/jsonrpc"
)

// Fetchers let a plugin serve sources such as demo://libs or knot://scripts/foo
// on demand. The host resolves the source's scheme to a plugin (via the schemes
// the plugin advertises in its handshake), then calls fetch.read / fetch.glob
// as files are needed — nothing is transferred up front except the bytes
// actually read.

const (
	// FetchNotFoundCode is the JSON-RPC error code a fetcher returns for a
	// missing source or path, mapped back to ErrFetchNotFound on the host.
	FetchNotFoundCode = -32001
	// FetchDeniedCode reports an access the fetcher refused: credentials,
	// permissions. It is permanent, and the host never retries it.
	FetchDeniedCode = -32002
	// FetchUnavailableCode reports a backend that could not answer right now
	// (network blip, upstream 503). The host retries these.
	FetchUnavailableCode = -32003
)

// DefaultFetchTimeout bounds fetch.read / fetch.glob calls when the caller
// provides no deadline of its own.
const DefaultFetchTimeout = 30 * time.Second

// Fetch retries: fetch operations are idempotent reads, so a transport hiccup
// or an unavailable backend is retried a bounded number of times with a short
// linear backoff. Permanent errors (not found, denied) are never retried.
const (
	FetchRetryAttempts = 3
	FetchRetryDelay    = 150 * time.Millisecond
)

// Fetch error sentinels. Fetcher implementations wrap them
// (fmt.Errorf("%w: ...", ErrFetchNotFound)) and the server transports each as
// its error code above; the client maps the codes back, so hosts can tell a
// plain miss from a refusal from a flaky backend.
var (
	ErrFetchNotFound    = errors.New("fetch source not found")
	ErrFetchDenied      = errors.New("fetch access denied")
	ErrFetchUnavailable = errors.New("fetch backend unavailable")
)

// FetchEntry is one match returned by Fetcher.Glob: Name is the entry's slash
// path relative to the source root (full path, not a bare base name).
type FetchEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
}

// Fetcher serves file content for sources under a registered scheme. Read is
// called with the full source string and a slash path relative to it (empty for
// a source that denotes a single file, such as a script) and returns the file's
// bytes; an error wrapping ErrFetchNotFound is a miss. Data travels
// base64-encoded on the wire so binary assets survive intact. There is no
// conditional-read machinery — the host does not cache what a plugin serves, so
// a plugin whose backend is slow caches behind its own Read, where the
// freshness rules live; the host stays a dumb pipe.
//
// Glob matches a pattern in the fetch glob language (see MatchGlob) against
// the source's tree and returns every match, directories included. It answers
// in one call what a directory-by-directory walk would need one round trip
// per level for, which is the point: existence is a wildcard-free pattern, a
// listing is "<dir>/*", a whole subtree is "<dir>/**". No match is an empty
// result, never an error; errors mean the fetcher could not answer. The
// MatchGlob and GlobDisk helpers implement the semantics for in-memory and
// disk-backed fetchers respectively.
type Fetcher interface {
	Read(ctx context.Context, source, path string) ([]byte, error)
	Glob(ctx context.Context, source, pattern string) ([]FetchEntry, error)
}

// =========================================================================
// Server side
// =========================================================================

// RegisterFetcher registers f as this plugin's fetcher: the host routes
// <scheme>:// sources here, attaches the plugin's library automatically, and
// asks for files only as imports resolve. The whole fetcher contract is this
// one call — one plugin serves one scheme, with the standard layout (modules
// under lib/, scripts as bare scheme:// sources). It must be called before
// Run / ServeHTTP, like the other registration methods. Built-in schemes
// (http, https, file) are rejected, as is a second registration.
func (s *Server) RegisterFetcher(scheme string, f Fetcher) *Server {
	if !validScheme(scheme) {
		panic(fmt.Sprintf("plugin: invalid fetcher scheme %q", scheme))
	}
	if s.fetcher != nil {
		panic(fmt.Sprintf("plugin: fetcher already registered for scheme %q (one scheme per plugin)", s.fetcherScheme))
	}
	s.fetcherScheme = scheme
	s.fetcher = f
	return s
}

// fetcherFor resolves the fetcher that owns source. The plugin has exactly
// one scheme, so this is a prefix check.
func (s *Server) fetcherFor(source string) (Fetcher, error) {
	if s.fetcher == nil || !strings.HasPrefix(source, s.fetcherScheme+"://") {
		return nil, fmt.Errorf("no fetcher registered for %q", source)
	}
	return s.fetcher, nil
}

func (s *Server) callFetchRead(ctx context.Context, params any) (any, error) {
	var p fetchReadParams
	if err := decodeParams(params, &p); err != nil {
		return nil, err
	}
	fetcher, err := s.fetcherFor(p.Source)
	if err != nil {
		return nil, err
	}
	data, err := fetcher.Read(ctx, p.Source, p.Path)
	if err != nil {
		return nil, fetchError(err)
	}
	return fetchReadResult{Data: data}, nil
}

// fetchError maps a fetcher's sentinel-wrapped error to its wire code;
// anything else travels as a plain (opaque) error.
func fetchError(err error) error {
	switch {
	case errors.Is(err, ErrFetchNotFound):
		return jsonrpc.NewError(FetchNotFoundCode, err.Error(), nil)
	case errors.Is(err, ErrFetchDenied):
		return jsonrpc.NewError(FetchDeniedCode, err.Error(), nil)
	case errors.Is(err, ErrFetchUnavailable):
		return jsonrpc.NewError(FetchUnavailableCode, err.Error(), nil)
	}
	return err
}

func (s *Server) callFetchGlob(ctx context.Context, params any) (any, error) {
	var p fetchGlobParams
	if err := decodeParams(params, &p); err != nil {
		return nil, err
	}
	fetcher, err := s.fetcherFor(p.Source)
	if err != nil {
		return nil, err
	}
	entries, err := fetcher.Glob(ctx, p.Source, p.Pattern)
	if err != nil {
		return nil, fetchError(err)
	}
	return fetchGlobResult{Entries: entries}, nil
}

// validScheme reports whether scheme is a plausible URI scheme: a letter
// followed by letters, digits, "+" "-" "." — and not a built-in source scheme.
func validScheme(scheme string) bool {
	if scheme == "" || scheme == "http" || scheme == "https" || scheme == "file" {
		return false
	}
	for i, r := range scheme {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case i > 0 && (r >= '0' && r <= '9' || r == '+' || r == '-' || r == '.'):
		default:
			return false
		}
	}
	return true
}

// =========================================================================
// Client side
// =========================================================================

// SupportsFetch reports whether this peer has a fetcher: a non-empty scheme
// in the handshake says so, which is the whole advertisement. Fetch calls are
// refused (without contacting the peer) when it does not, so plugins without
// fetchers keep working unchanged.
func (c *Client) SupportsFetch() bool {
	return c.handshakeDone && c.metadata.Scheme != ""
}

// Scheme returns the source scheme the peer's fetcher serves, as advertised
// in its handshake. Empty when the plugin has no fetcher.
func (c *Client) Scheme() string {
	return c.metadata.Scheme
}

// FetchFile reads one file from a source. path is a slash path relative to the
// source; an empty path denotes a source that is itself a single file (a
// script). Transport failures and unavailable backends are retried a bounded
// number of times (fetch reads are idempotent); permanent errors are not.
func (c *Client) FetchFile(ctx context.Context, source, path string) ([]byte, error) {
	if !c.SupportsFetch() {
		return nil, fmt.Errorf("plugin %s does not support fetch", c.metadata.Name)
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, DefaultFetchTimeout)
		defer cancel()
	}
	var result fetchReadResult
	err := c.fetchCall(ctx, func() error {
		return c.call(ctx, "fetch.read", fetchReadParams{Source: source, Path: path}, nil, &result)
	})
	if err != nil {
		return nil, mapFetchError(err)
	}
	return result.Data, nil
}

// FetchGlob returns the entries of a source whose paths match pattern in the
// fetch glob language (see MatchGlob). No match is an empty result; a missing
// source is an error wrapping ErrFetchNotFound. Retried like FetchFile.
func (c *Client) FetchGlob(ctx context.Context, source, pattern string) ([]FetchEntry, error) {
	if !c.SupportsFetch() {
		return nil, fmt.Errorf("plugin %s does not support fetch", c.metadata.Name)
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, DefaultFetchTimeout)
		defer cancel()
	}
	var result fetchGlobResult
	err := c.fetchCall(ctx, func() error {
		return c.call(ctx, "fetch.glob", fetchGlobParams{Source: source, Pattern: pattern}, nil, &result)
	})
	if err != nil {
		return nil, mapFetchError(err)
	}
	return result.Entries, nil
}

// fetchCall runs a fetch RPC, retrying retryable failures: an unavailable
// backend (ErrFetchUnavailable over the wire) or a transport error (anything
// that is not a JSON-RPC error object). Permanent answers — not found, denied,
// or any other coded error — come back immediately.
func (c *Client) fetchCall(ctx context.Context, call func() error) error {
	var err error
	for attempt := 0; attempt < FetchRetryAttempts; attempt++ {
		if attempt > 0 {
			delay := time.Duration(attempt) * FetchRetryDelay
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return err
			case <-timer.C:
			}
		}
		err = call()
		if err == nil || !retryableFetchError(err) {
			return err
		}
	}
	return err
}

// retryableFetchError reports whether a fetch failure is worth another
// attempt: unavailable backends and transport-level failures yes; JSON-RPC
// error objects carry the plugin's considered answer and are final, except
// the unavailable code.
func retryableFetchError(err error) bool {
	var rpcErr *RPCError
	if errors.As(err, &rpcErr) {
		return rpcErr.Code == FetchUnavailableCode
	}
	return true
}

// mapFetchError converts the fetch error codes into errors wrapping their
// sentinels, so hosts can treat a miss as a miss and a refusal as a refusal
// without parsing messages.
//
// Fetchers wrap the sentinels themselves, so the message arriving over the
// wire usually already begins with the sentinel's text. Re-prefixing it would
// read "fetch source not found: fetch source not found: knot://x", so the
// sentinel is only prepended when it is actually missing.
func mapFetchError(err error) error {
	if err == nil {
		return nil
	}
	var rpcErr *RPCError
	if !errors.As(err, &rpcErr) {
		return err
	}
	var sentinel error
	switch rpcErr.Code {
	case FetchNotFoundCode:
		sentinel = ErrFetchNotFound
	case FetchDeniedCode:
		sentinel = ErrFetchDenied
	case FetchUnavailableCode:
		sentinel = ErrFetchUnavailable
	default:
		return err
	}
	detail := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(rpcErr.Message), sentinel.Error()))
	detail = strings.TrimSpace(strings.TrimPrefix(detail, ":"))
	if detail == "" {
		return sentinel
	}
	return fmt.Errorf("%w: %s", sentinel, detail)
}
