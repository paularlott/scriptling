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
// the plugin advertises in its handshake), then calls fetch.read / fetch.list
// for individual files as they are needed — nothing is transferred up front
// except the bytes actually read.

const (
	// CapabilityFetch is advertised by peers that implement the fetch methods.
	CapabilityFetch = "fetch"

	// FetchNotFoundCode is the JSON-RPC error code a fetcher returns for a
	// missing source or path, mapped back to ErrFetchNotFound on the host.
	FetchNotFoundCode = -32001
)

// DefaultFetchTimeout bounds fetch.read / fetch.list calls when the caller
// provides no deadline of its own.
const DefaultFetchTimeout = 30 * time.Second

// ErrFetchNotFound reports a source or path the fetcher does not have. Fetcher
// implementations wrap it (fmt.Errorf("%w: ...", ErrFetchNotFound)) and the
// server transports it as error code FetchNotFoundCode.
var ErrFetchNotFound = errors.New("fetch source not found")

// FetchEntry is one directory entry returned by Fetcher.List.
type FetchEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
}

// FetchResult is the outcome of a Fetcher.Read: the file content. Data travels
// base64-encoded on the wire so binary assets (a webroot image, a font) survive
// intact — a JSON string cannot carry arbitrary bytes.
//
// There is no conditional-read machinery. The host does not cache what a plugin
// serves, so there is nothing to revalidate: every Read returns bytes. A plugin
// whose backend is slow caches behind its own Read, where the freshness rules
// live; the host stays a dumb pipe.
type FetchResult struct {
	Data []byte
}

// Fetcher serves file content for sources under a registered scheme. Read is
// called with the full source string and a slash path relative to it (empty for
// a source that denotes a single file, such as a script). List enumerates one
// directory level.
type Fetcher interface {
	Read(ctx context.Context, source, path string) (FetchResult, error)
	List(ctx context.Context, source, path string) ([]FetchEntry, error)
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
	res, err := fetcher.Read(ctx, p.Source, p.Path)
	if err != nil {
		if errors.Is(err, ErrFetchNotFound) {
			return nil, jsonrpc.NewError(FetchNotFoundCode, err.Error(), nil)
		}
		return nil, err
	}
	return fetchReadResult{Data: res.Data}, nil
}

func (s *Server) callFetchList(ctx context.Context, params any) (any, error) {
	var p fetchListParams
	if err := decodeParams(params, &p); err != nil {
		return nil, err
	}
	fetcher, err := s.fetcherFor(p.Source)
	if err != nil {
		return nil, err
	}
	entries, err := fetcher.List(ctx, p.Source, p.Path)
	if err != nil {
		if errors.Is(err, ErrFetchNotFound) {
			return nil, jsonrpc.NewError(FetchNotFoundCode, err.Error(), nil)
		}
		return nil, err
	}
	return fetchListResult{Entries: entries}, nil
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

// sourceScheme extracts the scheme from a source such as knot://libs,
// returning "" when there is no <scheme>:// prefix.
func sourceScheme(source string) string {
	scheme, rest, found := strings.Cut(source, "://")
	if !found || scheme == "" || !validScheme(scheme) {
		return ""
	}
	if strings.ContainsAny(rest, " ") {
		return ""
	}
	return scheme
}

// =========================================================================
// Client side
// =========================================================================

// SupportsFetch reports whether this peer advertised the fetch capability in
// its handshake. Fetch calls are refused (without contacting the peer) when it
// did not, so older plugins keep working unchanged.
func (c *Client) SupportsFetch() bool {
	if !c.handshakeDone || c.metadata.Scheme == "" {
		return false
	}
	for _, capability := range c.metadata.Capabilities {
		if capability == CapabilityFetch {
			return true
		}
	}
	return false
}

// Scheme returns the source scheme the peer's fetcher serves, as advertised
// in its handshake. Empty when the plugin has no fetcher.
func (c *Client) Scheme() string {
	return c.metadata.Scheme
}

// FetchFile reads one file from a source. path is a slash path relative to the
// source; an empty path denotes a source that is itself a single file (a
// script).
func (c *Client) FetchFile(ctx context.Context, source, path string) (FetchResult, error) {
	if !c.SupportsFetch() {
		return FetchResult{}, fmt.Errorf("plugin %s does not support fetch", c.metadata.Name)
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, DefaultFetchTimeout)
		defer cancel()
	}
	var result fetchReadResult
	if err := c.call(ctx, "fetch.read", fetchReadParams{
		Source: source,
		Path:   path,
	}, nil, &result); err != nil {
		return FetchResult{}, mapFetchError(err)
	}
	return FetchResult{Data: result.Data}, nil
}

// FetchList enumerates one directory level of a source. A missing directory
// returns an error wrapping ErrFetchNotFound.
func (c *Client) FetchList(ctx context.Context, source, path string) ([]FetchEntry, error) {
	if !c.SupportsFetch() {
		return nil, fmt.Errorf("plugin %s does not support fetch", c.metadata.Name)
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, DefaultFetchTimeout)
		defer cancel()
	}
	var result fetchListResult
	if err := c.call(ctx, "fetch.list", fetchListParams{
		Source: source,
		Path:   path,
	}, nil, &result); err != nil {
		return nil, mapFetchError(err)
	}
	return result.Entries, nil
}

// mapFetchError converts a FetchNotFoundCode RPC error into an error wrapping
// ErrFetchNotFound so hosts can treat it as a plain miss.
//
// Fetchers are told to wrap ErrFetchNotFound themselves, so the message
// arriving over the wire usually already begins with the sentinel's text.
// Re-prefixing it would read "fetch source not found: fetch source not found:
// knot://x", so the sentinel is only prepended when it is actually missing.
func mapFetchError(err error) error {
	if err == nil {
		return nil
	}
	var rpcErr *RPCError
	if errors.As(err, &rpcErr) && rpcErr.Code == FetchNotFoundCode {
		msg := strings.TrimSpace(rpcErr.Message)
		if detail, ok := trimNotFoundPrefix(msg); ok {
			if detail == "" {
				return ErrFetchNotFound
			}
			return fmt.Errorf("%w: %s", ErrFetchNotFound, detail)
		}
		if msg == "" {
			return ErrFetchNotFound
		}
		return fmt.Errorf("%w: %s", ErrFetchNotFound, msg)
	}
	return err
}

// trimNotFoundPrefix strips a leading copy of ErrFetchNotFound's text (and any
// following ": ") from msg, reporting whether it was present.
func trimNotFoundPrefix(msg string) (string, bool) {
	sentinel := ErrFetchNotFound.Error()
	if !strings.HasPrefix(msg, sentinel) {
		return msg, false
	}
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(msg, sentinel), ":")), true
}
