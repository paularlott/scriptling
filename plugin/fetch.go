package plugin

import (
	"context"
	"errors"
	"fmt"
	"sort"
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

// FetchResult is the outcome of a Fetcher.Read. Data is the file content; a
// NotModified result carries no Data. ETag / LastModified are opaque validator
// strings the host stores alongside cached bytes and sends back on later reads
// to give the fetcher a chance to answer NotModified.
type FetchResult struct {
	NotModified  bool
	Data         []byte
	ETag         string
	LastModified string
}

// Fetcher serves file content for sources under a registered scheme. Read is
// called with the full source string, a slash path relative to it (empty for a
// source that denotes a single file, such as a script), and any validators the
// host held from a previous read. List enumerates one directory level.
type Fetcher interface {
	Read(ctx context.Context, source, path, etag, lastModified string) (FetchResult, error)
	List(ctx context.Context, source, path string) ([]FetchEntry, error)
}

// =========================================================================
// Server side
// =========================================================================

// RegisterFetcher registers f as the fetcher for sources with the given scheme.
// The scheme is advertised in the handshake so hosts route those sources here.
// It must be called before Run / ServeHTTP, like the other registration
// methods. Built-in schemes (http, https, file) are rejected.
func (s *Server) RegisterFetcher(scheme string, f Fetcher) *Server {
	if !validScheme(scheme) {
		panic(fmt.Sprintf("plugin: invalid fetcher scheme %q", scheme))
	}
	s.fetchers[scheme] = f
	return s
}

// DeclarePackage declares a package source (knot://libs) that the host
// attaches automatically whenever this plugin is loaded, so scripts import
// its modules without passing --package. The source must be served by one of
// this plugin's fetchers. Explicit --package sources shadow declared ones.
func (s *Server) DeclarePackage(source string) *Server {
	s.packages = append(s.packages, source)
	return s
}

// fetcherSchemes returns the registered schemes in stable order.
func (s *Server) fetcherSchemes() []string {
	schemes := make([]string, 0, len(s.fetchers))
	for scheme := range s.fetchers {
		schemes = append(schemes, scheme)
	}
	sort.Strings(schemes)
	return schemes
}

// fetcherFor resolves the fetcher that owns source by its scheme.
func (s *Server) fetcherFor(source string) (Fetcher, error) {
	scheme := sourceScheme(source)
	if scheme == "" {
		return nil, fmt.Errorf("fetch source %q has no <scheme>:// prefix", source)
	}
	f, ok := s.fetchers[scheme]
	if !ok {
		return nil, fmt.Errorf("no fetcher registered for scheme %q", scheme)
	}
	return f, nil
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
	res, err := fetcher.Read(ctx, p.Source, p.Path, p.ETag, p.LastModified)
	if err != nil {
		if errors.Is(err, ErrFetchNotFound) {
			return nil, jsonrpc.NewError(FetchNotFoundCode, err.Error(), nil)
		}
		return nil, err
	}
	return fetchReadResult{
		NotModified:  res.NotModified,
		Data:         res.Data,
		ETag:         res.ETag,
		LastModified: res.LastModified,
	}, nil
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
	if !c.handshakeDone {
		return false
	}
	for _, capability := range c.metadata.Capabilities {
		if capability == CapabilityFetch {
			return true
		}
	}
	return false
}

// Schemes returns the source schemes the peer's fetchers serve, as advertised
// in its handshake.
func (c *Client) Schemes() []string {
	return c.metadata.Schemes
}

// FetchFile reads one file from a source. path is a slash path relative to the
// source; an empty path denotes a source that is itself a single file (a
// script). etag / lastModified are validators from a previous FetchFile of the
// same source+path — the peer answers NotModified instead of resending
// unchanged bytes.
func (c *Client) FetchFile(ctx context.Context, source, path, etag, lastModified string) (FetchResult, error) {
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
		Source:       source,
		Path:         path,
		ETag:         etag,
		LastModified: lastModified,
	}, nil, &result); err != nil {
		return FetchResult{}, mapFetchError(err)
	}
	return FetchResult{
		NotModified:  result.NotModified,
		Data:         result.Data,
		ETag:         result.ETag,
		LastModified: result.LastModified,
	}, nil
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
func mapFetchError(err error) error {
	if err == nil {
		return nil
	}
	var rpcErr *RPCError
	if errors.As(err, &rpcErr) && rpcErr.Code == FetchNotFoundCode {
		return fmt.Errorf("%w: %s", ErrFetchNotFound, rpcErr.Message)
	}
	return err
}
