package plugin

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/paularlott/jsonrpc"
	"github.com/paularlott/logger"
)

// TransportMode restricts which plugin transport protocols a Manager or scope
// will accept when LoadPath or LoadURL is called.
type TransportMode int

const (
	// TransportAll permits both stdio/executable and HTTP(S) plugins (default).
	TransportAll TransportMode = iota
	// TransportHTTP permits only HTTP(S) endpoints; loading executables fails.
	TransportHTTP
	// TransportStdio permits only stdio executables; loading HTTP URLs fails.
	TransportStdio
)

// ScopeOption configures a scoped Manager created by NewScope.
type ScopeOption func(*Manager)

type Manager struct {
	parent                *Manager
	transportMode         TransportMode
	httpTransport         *http.Transport // shared pooled TLS-verified transport
	httpInsecureTransport *http.Transport // shared pooled TLS-skip-verify transport

	dirs         []string
	clients      map[string]*Client
	warnings     []string
	crashHandler func(name string, err error)
	logger       logger.Logger
	mu           sync.RWMutex
}

// NewManager creates an empty plugin manager. If log is not nil, plugin log
// records emitted through Logger(ctx) are forwarded to it. If crashHandler is
// provided, it is called when a loaded plugin process exits unexpectedly.
func NewManager(log logger.Logger, crashHandler ...func(name string, err error)) *Manager {
	manager := &Manager{
		clients:               make(map[string]*Client),
		logger:                log,
		httpTransport:         newSharedHTTPTransport(false),
		httpInsecureTransport: newSharedHTTPTransport(true),
	}
	if len(crashHandler) > 0 {
		manager.crashHandler = crashHandler[0]
	}
	return manager
}

// newSharedHTTPTransport returns a pooled, HTTP/2-capable transport. When
// insecureSkipVerify is true the transport skips TLS certificate validation —
// intended for development against self-signed certificates only.
func newSharedHTTPTransport(insecureSkipVerify bool) *http.Transport {
	tlsCfg := &tls.Config{
		InsecureSkipVerify: insecureSkipVerify, //nolint:gosec // intentional when insecureSkipVerify=true
	}
	if !insecureSkipVerify {
		tlsCfg.MinVersion = tls.VersionTLS12
	}
	return &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSClientConfig:     tlsCfg,
		ForceAttemptHTTP2:   true,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

// WithTransport sets the transport restriction for a scoped Manager. Use
// TransportHTTP to permit only HTTP(S) plugins, TransportStdio for only
// stdio executables, or TransportAll (default) to allow both.
func WithTransport(mode TransportMode) ScopeOption {
	return func(m *Manager) { m.transportMode = mode }
}

// NewScope creates a child Manager that inherits the logger and shared HTTP
// transports from this Manager. Plugins loaded into the scope are invisible to
// the parent and to other scopes. When the scope is closed, only its locally
// loaded plugins are unloaded; the parent's plugins are unaffected.
//
// Calling Get or List on the scope chains to the parent for fallback: the
// scope sees its own plugins first and parent plugins where there is no clash.
//
// The scope does not inherit the parent's dirs or crash handler.
func (m *Manager) NewScope(opts ...ScopeOption) *Manager {
	scope := &Manager{
		parent:                m,
		clients:               make(map[string]*Client),
		logger:                m.logger,
		httpTransport:         m.httpTransport,         // shared — connections pooled with parent
		httpInsecureTransport: m.httpInsecureTransport, // shared — connections pooled with parent
	}
	for _, opt := range opts {
		opt(scope)
	}
	return scope
}

// httpTransportFor returns the appropriate shared transport for the given TLS
// skip-verify preference. Both transports are pooled and shared with child
// scopes so connections are reused across executions.
func (m *Manager) httpTransportFor(insecureSkipTLS bool) *http.Transport {
	if insecureSkipTLS {
		return m.httpInsecureTransport
	}
	return m.httpTransport
}

// AddDir adds a directory whose executable files should be loaded as plugins.
func (m *Manager) AddDir(dir string) {
	if dir != "" {
		m.dirs = append(m.dirs, dir)
	}
}

// Load eagerly starts all executable plugins in configured plugin directories.
func (m *Manager) Load(ctx context.Context) error {
	for _, dir := range m.dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			m.addWarning("plugin dir %s: %v", dir, err)
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			info, err := entry.Info()
			if err != nil {
				m.addWarning("plugin %s: %v", path, err)
				continue
			}
			if info.Mode()&0111 == 0 {
				continue
			}
			client, err := startClient(ctx, path, nil)
			if err != nil {
				m.addWarning("plugin %s failed to load: %v", path, err)
				continue
			}
			m.mu.RLock()
			client.setLogger(m.logger)
			m.mu.RUnlock()
			name := client.Metadata().Name
			m.mu.Lock()
			if _, exists := m.clients[name]; exists {
				m.warnings = append(m.warnings, fmt.Sprintf("plugin %s ignored: duplicate library %s", path, name))
				m.mu.Unlock()
				_ = client.Close()
				continue
			}
			m.clients[name] = client
			m.mu.Unlock()
			m.installCrashHandler(name, client)
		}
	}
	return nil
}

// LoadPath starts a single executable, or connects to an http(s) JSON-RPC
// endpoint, and registers it under name. Executable identity is by absolute
// path; HTTP identity is by URL.
//
// If scriptling is true, the plugin protocol handshake is performed and the
// client can be driven through CallFunction / CallMethod / call_function /
// call_method. If scriptling is false, the handshake is skipped and
// call_function sends the function name directly as the JSON-RPC method.
//
// args, if non-empty, are passed as command-line arguments to the executable
// (e.g. ["--json-rpc", "./setup.py"] when spawning `scriptling` itself).
//
// name is normalised into the plugin.* namespace (e.g. "widgets" becomes
// "plugin.widgets"); the returned client's Metadata().Name reflects that.
func (m *Manager) LoadPath(ctx context.Context, name, path string, scriptling bool, args []string) (*Client, error) {
	normalisedName := NormalizeLibraryName(name)
	resolvedPath := path
	isHTTP := isHTTPURL(path)
	var err error
	if !isHTTP {
		resolvedPath, err = resolveExecutablePath(path)
		if err != nil {
			return nil, err
		}
	}

	// Enforce transport restriction for this scope.
	switch m.transportMode {
	case TransportHTTP:
		if !isHTTP {
			return nil, fmt.Errorf("stdio/executable plugins are not permitted in this scope (http/https only)")
		}
	case TransportStdio:
		if isHTTP {
			return nil, fmt.Errorf("http/https plugins are not permitted in this scope (stdio only)")
		}
	}

	// Parent-chain check: a child scope may not shadow a name that already
	// exists in any ancestor. Loading the exact same path under the same name
	// is idempotent and returns the parent's client unchanged. Loading a
	// different path under an already-taken name is an error — the parent's
	// plugin remains the canonical binding for that name.
	if m.parent != nil {
		if parentClient, ok := m.parent.Get(normalisedName); ok {
			if parentClient.Path() == resolvedPath {
				return parentClient, nil // same endpoint — idempotent via parent
			}
			return nil, fmt.Errorf("plugin name %s is already loaded in a parent scope; child scopes cannot shadow parent plugins", normalisedName)
		}
	}

	m.mu.Lock()
	for _, existing := range m.clients {
		if existing.Path() == resolvedPath {
			if existing.Metadata().Name == normalisedName {
				m.mu.Unlock()
				return existing, nil
			}
			m.mu.Unlock()
			return nil, fmt.Errorf("plugin %s already loaded as %s", resolvedPath, existing.Metadata().Name)
		}
	}
	if _, exists := m.clients[normalisedName]; exists {
		m.mu.Unlock()
		return nil, fmt.Errorf("plugin name %s already in use", normalisedName)
	}
	m.mu.Unlock()

	var client *Client
	if isHTTP {
		client, err = newHTTPClient(ctx, resolvedPath, false, scriptling, m.httpTransport)
	} else if scriptling {
		client, err = startClient(ctx, resolvedPath, args)
	} else {
		client, err = spawnClient(ctx, resolvedPath, args)
	}
	if err != nil {
		return nil, err
	}
	m.mu.RLock()
	client.setLogger(m.logger)
	m.mu.RUnlock()
	client.SetName(normalisedName)

	m.mu.Lock()
	// Re-check under the write lock: a concurrent LoadPath may have won the race.
	for _, existing := range m.clients {
		if existing.Path() == resolvedPath {
			m.mu.Unlock()
			_ = client.Close()
			if existing.Metadata().Name == normalisedName {
				return existing, nil
			}
			return nil, fmt.Errorf("plugin %s already loaded as %s", resolvedPath, existing.Metadata().Name)
		}
	}
	if _, exists := m.clients[normalisedName]; exists {
		m.mu.Unlock()
		_ = client.Close()
		return nil, fmt.Errorf("plugin name %s already in use", normalisedName)
	}
	m.clients[normalisedName] = client
	m.mu.Unlock()
	m.installCrashHandler(normalisedName, client)
	return client, nil
}

// LoadURL connects to an HTTP(S) JSON-RPC endpoint and registers it under name.
// If scriptling is true, the plugin protocol handshake is performed. If
// insecureSkipTLS is true, HTTPS certificate verification is skipped. Optional
// headers are sent with every HTTP request.
func (m *Manager) LoadURL(ctx context.Context, name, rawURL string, scriptling, insecureSkipTLS bool, headers ...map[string]string) (*Client, error) {
	if !isHTTPURL(rawURL) {
		return nil, fmt.Errorf("plugin URL must use http or https")
	}
	// Enforce transport restriction: stdio-only scopes may not load HTTP plugins.
	if m.transportMode == TransportStdio {
		return nil, fmt.Errorf("http/https plugins are not permitted in this scope (stdio only)")
	}
	normalisedName := NormalizeLibraryName(name)

	// Parent-chain check: same rule as LoadPath — a child scope may not shadow
	// a name that already exists in any ancestor. Same URL under same name is
	// idempotent; different URL under same name is an error.
	if m.parent != nil {
		if parentClient, ok := m.parent.Get(normalisedName); ok {
			if parentClient.Path() == rawURL {
				return parentClient, nil // same endpoint — idempotent via parent
			}
			return nil, fmt.Errorf("plugin name %s is already loaded in a parent scope; child scopes cannot shadow parent plugins", normalisedName)
		}
	}

	m.mu.Lock()
	for _, existing := range m.clients {
		if existing.Path() == rawURL {
			if existing.Metadata().Name == normalisedName {
				m.mu.Unlock()
				return existing, nil
			}
			m.mu.Unlock()
			return nil, fmt.Errorf("plugin %s already loaded as %s", rawURL, existing.Metadata().Name)
		}
	}
	if _, exists := m.clients[normalisedName]; exists {
		m.mu.Unlock()
		return nil, fmt.Errorf("plugin name %s already in use", normalisedName)
	}
	m.mu.Unlock()

	client, err := newHTTPClient(ctx, rawURL, insecureSkipTLS, scriptling, m.httpTransportFor(insecureSkipTLS), firstHeaderMap(headers))
	if err != nil {
		return nil, err
	}
	m.mu.RLock()
	client.setLogger(m.logger)
	m.mu.RUnlock()
	client.SetName(normalisedName)

	m.mu.Lock()
	for _, existing := range m.clients {
		if existing.Path() == rawURL {
			m.mu.Unlock()
			_ = client.Close()
			if existing.Metadata().Name == normalisedName {
				return existing, nil
			}
			return nil, fmt.Errorf("plugin %s already loaded as %s", rawURL, existing.Metadata().Name)
		}
	}
	if _, exists := m.clients[normalisedName]; exists {
		m.mu.Unlock()
		_ = client.Close()
		return nil, fmt.Errorf("plugin name %s already in use", normalisedName)
	}
	m.clients[normalisedName] = client
	m.mu.Unlock()
	return client, nil
}

// Unload closes a client registered via LoadPath and removes it from the
// manager. It is intended for runtime-loaded executables; calling Unload on a
// plugin discovered via Load also works but the plugin will not be restarted.
// Returns an error if no client is registered under name (after normalisation).
func (m *Manager) Unload(name string) error {
	normalized := NormalizeLibraryName(name)
	m.mu.Lock()
	client, ok := m.clients[normalized]
	if ok {
		delete(m.clients, normalized)
	}
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("plugin not found: %s", name)
	}
	return client.Close()
}

func resolveExecutablePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("plugin path is required")
	}
	resolved := path
	if !filepath.IsAbs(path) && !strings.Contains(path, string(filepath.Separator)) {
		found, err := exec.LookPath(path)
		if err != nil {
			return "", err
		}
		resolved = found
	}
	absPath, err := filepath.Abs(resolved)
	if err != nil {
		return "", err
	}
	return absPath, nil
}

func isHTTPURL(ref string) bool {
	return strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://")
}

// Close shuts down all loaded plugin processes and clears the local client map.
// For scoped managers this fully releases all locally loaded plugins without
// touching the parent's plugins. Close is safe to call more than once.
func (m *Manager) Close() error {
	m.mu.Lock()
	clients := make([]*Client, 0, len(m.clients))
	for _, client := range m.clients {
		clients = append(clients, client)
	}
	// Clear the local map immediately so concurrent Get/List calls see an empty
	// scope even while individual client shutdowns are still in progress.
	m.clients = make(map[string]*Client)
	m.mu.Unlock()

	var first error
	for _, client := range clients {
		if err := client.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// Warnings returns non-fatal plugin load warnings collected by the manager.
func (m *Manager) Warnings() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, len(m.warnings))
	copy(out, m.warnings)
	return out
}

// List returns metadata for all loaded plugins sorted by library name.
// If this Manager is a scope, parent plugins are included in the result.
// A child scope cannot load a name that an ancestor already owns, so name
// collisions between local and parent are not possible; the seen-map guard
// is kept as a safety net in case of direct manager manipulation.
func (m *Manager) List() []Metadata {
	// Collect local entries first so we can track which names they cover.
	m.mu.RLock()
	out := make([]Metadata, 0, len(m.clients))
	seen := make(map[string]bool, len(m.clients))
	for name, client := range m.clients {
		out = append(out, client.Metadata())
		seen[name] = true
	}
	m.mu.RUnlock()

	// Merge parent entries that are not shadowed by a local entry.
	if m.parent != nil {
		for _, meta := range m.parent.List() {
			if !seen[meta.Name] {
				out = append(out, meta)
			}
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Health returns loaded plugins whose process or stdio transport is unhealthy.
func (m *Manager) Health() map[string]error {
	m.mu.RLock()
	clients := make(map[string]*Client, len(m.clients))
	for name, client := range m.clients {
		clients[name] = client
	}
	m.mu.RUnlock()

	unhealthy := make(map[string]error)
	for name, client := range clients {
		if err := client.Health(); err != nil {
			unhealthy[name] = err
		}
	}
	return unhealthy
}

// SetCrashHandler installs a callback for loaded plugin processes that exit
// unexpectedly. The handler is not called for normal manager shutdown.
func (m *Manager) SetCrashHandler(handler func(name string, err error)) {
	m.mu.Lock()
	m.crashHandler = handler
	clients := make(map[string]*Client, len(m.clients))
	for name, client := range m.clients {
		clients[name] = client
	}
	m.mu.Unlock()

	for name, client := range clients {
		m.installCrashHandler(name, client)
	}
}

// SetLogger installs the host logger used for log records emitted by plugins.
func (m *Manager) SetLogger(log logger.Logger) {
	m.mu.Lock()
	m.logger = log
	clients := make([]*Client, 0, len(m.clients))
	for _, client := range m.clients {
		clients = append(clients, client)
	}
	m.mu.Unlock()

	for _, client := range clients {
		client.setLogger(log)
	}
}

// Get returns a loaded plugin client by short or fully-qualified library name.
// It checks the local map first; if not found and this is a scope with a parent,
// it falls back to the parent (and so on up the chain). Local always wins.
func (m *Manager) Get(name string) (*Client, bool) {
	normalized := NormalizeLibraryName(name)
	m.mu.RLock()
	client, ok := m.clients[normalized]
	m.mu.RUnlock()
	if ok {
		return client, true
	}
	if m.parent != nil {
		return m.parent.Get(name)
	}
	return nil, false
}

func (m *Manager) addWarning(format string, args ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.warnings = append(m.warnings, fmt.Sprintf(format, args...))
}

func (m *Manager) installCrashHandler(name string, client *Client) {
	if client == nil {
		return
	}
	client.setExitHandler(func(err error) {
		m.mu.RLock()
		handler := m.crashHandler
		m.mu.RUnlock()
		if handler != nil {
			handler(name, err)
		}
	})
}

// NormalizeLibraryName returns name in the host-owned plugin namespace.
func NormalizeLibraryName(name string) string {
	if strings.HasPrefix(name, NamespacePrefix) {
		return name
	}
	return NamespacePrefix + name
}

// Client is a connection to a loaded plugin. Over stdio it is a bidirectional
// JSON-RPC peer (built on [jsonrpc.Peer]) — outbound calls to the plugin and
// inbound host callbacks (callback.call, host.log) share one stream. Over HTTP
// it is a unidirectional JSON-RPC client (built on [jsonrpc.HTTPTransport]);
// callbacks are not available over HTTP.
type Client struct {
	path     string
	metadata Metadata

	handshakeDone bool

	// Exactly one transport is set: peer for stdio plugins, rpc for HTTP.
	peer *jsonrpc.Peer // bidirectional stdio transport (outbound + inbound callbacks)
	rpc  *jsonrpc.Client // unidirectional HTTP transport

	// callbackOwners routes an inbound "callback.call" to the in-flight plugin
	// call that registered the callback (stdio only). nil for HTTP clients.
	callbackOwners map[string]*callbackOwner
	mu             sync.Mutex

	// done is closed when the client is closed or the stdio process/transport
	// has gone away, so calls and Health fail fast.
	done     chan struct{}
	doneClose sync.Once

	closing atomic.Bool

	// Exit/crash handling. waitErr holds the subprocess exit error (stdio);
	// exitHandler is the manager-installed crash callback, fired once when the
	// process dies on its own (not during a normal Close).
	stateMu     sync.Mutex
	logger      logger.Logger
	waitErr     error
	exitHandler func(error)
	exitFired   atomic.Bool
}

// callbackOwner ties a registered host callback id to the plugin call that
// registered it, so an inbound callback.call is dispatched to the right
// callbackSet with the right (evaluator-bearing) context.
type callbackOwner struct {
	set *callbackSet
	ctx context.Context
}

func startClient(ctx context.Context, path string, args []string) (*Client, error) {
	client, err := spawnClient(ctx, path, args)
	if err != nil {
		return nil, err
	}
	if err := client.handshake(ctx); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

// newHTTPClient creates an HTTP plugin client. The caller is responsible for
// passing the appropriate transport (see Manager.httpTransportFor); no TLS
// policy decisions are made here. Pass nil to fall back to http.DefaultTransport.
func newHTTPClient(ctx context.Context, rawURL string, insecureSkipTLS bool, handshake bool, transport *http.Transport, headers ...map[string]string) (*Client, error) {
	opts := []jsonrpc.HTTPOption{jsonrpc.WithHTTPClient(&http.Client{Transport: transportOrDefault(transport)})}
	for key, value := range firstHeaderMap(headers) {
		opts = append(opts, jsonrpc.WithHeader(key, value))
	}
	client := &Client{
		path: rawURL,
		rpc:  jsonrpc.NewClient(jsonrpc.NewHTTPTransport(rawURL, opts...)),
		done: make(chan struct{}),
	}
	if handshake {
		if err := client.handshake(ctx); err != nil {
			_ = client.Close()
			return nil, err
		}
	}
	return client, nil
}

func transportOrDefault(t *http.Transport) http.RoundTripper {
	if t != nil {
		return t
	}
	return http.DefaultTransport
}

func firstHeaderMap(headers []map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	return headers[0]
}

// spawnClient starts an executable as a subprocess wired up as a bidirectional
// JSON-RPC peer over stdio, but does not perform the plugin handshake. Callers
// that want the plugin protocol handshake should use LoadClient (or call
// handshake next); callers that want to skip the handshake may use the returned
// client directly. args, if non-empty, are passed as command-line arguments to
// the executable.
func spawnClient(ctx context.Context, path string, args []string) (*Client, error) {
	// The subprocess must outlive any single request context — the evaluation
	// context that reaches load() may be cancelled after the builtin returns.
	// The process lifecycle is managed by the client's Close() (via the peer's
	// close function) and the manager's Close()/Unload(), not by a context.
	client := &Client{
		path:           path,
		callbackOwners: make(map[string]*callbackOwner),
		done:           make(chan struct{}),
	}
	server := newPluginPeerServer(client)
	peer, err := jsonrpc.NewProcessPeer(path, args, server,
		jsonrpc.WithStderr(os.Stderr),
		jsonrpc.WithOnExit(client.onProcessExit),
	)
	if err != nil {
		return nil, err
	}
	client.peer = peer
	return client, nil
}

// LoadClient spawns an executable and performs the plugin protocol handshake.
// The returned client has Metadata populated from the handshake result.
// args, if non-empty, are passed as command-line arguments to the executable.
func LoadClient(ctx context.Context, path string, args []string) (*Client, error) {
	return startClient(ctx, path, args)
}

// LoadClientFromIO connects to a plugin server over an existing bidirectional
// stream and performs the plugin protocol handshake. Use this when the server
// is already running and accessible via in-process pipes (e.g. tests, embedded
// servers). The caller is responsible for closing in/out when done.
func LoadClientFromIO(ctx context.Context, in io.ReadCloser, out io.WriteCloser) (*Client, error) {
	client := &Client{
		path:           "<pipe>",
		callbackOwners: make(map[string]*callbackOwner),
		done:           make(chan struct{}),
	}
	client.peer = jsonrpc.NewPeer(in, out, newPluginPeerServer(client), jsonrpc.WithPeerCloseFunc(func() error {
		// Closing the write end signals EOF to the server's reader (the original
		// client closed stdin here), so a stdio server's RunIO returns.
		return out.Close()
	}))
	go func() { _ = client.peer.Serve() }()
	// No subprocess to reap: treat reader EOF as the end of the client.
	go func() {
		<-client.peer.Done()
		client.markDone()
	}()
	if err := client.handshake(ctx); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

// SpawnClient spawns an executable without performing the plugin handshake.
// The caller is responsible for any handshake exchange via Call.
// args, if non-empty, are passed as command-line arguments to the executable.
func SpawnClient(ctx context.Context, path string, args []string) (*Client, error) {
	return spawnClient(ctx, path, args)
}

// handshake performs the scriptling plugin protocol handshake and populates the
// client metadata from the result. It is a no-op if the protocol/transport are
// already negotiated.
func (c *Client) handshake(ctx context.Context) error {
	handshakeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var result handshakeResult
	if err := c.call(handshakeCtx, "scriptling.handshake", handshakeParams{
		Protocol:     ProtocolVersion,
		Host:         "scriptling",
		HostVersion:  "dev",
		Transports:   []string{"json"},
		Capabilities: []string{"remote_objects"},
	}, nil, &result); err != nil {
		return err
	}
	if result.Protocol != ProtocolVersion {
		return fmt.Errorf("unsupported protocol %q", result.Protocol)
	}
	if result.Transport != "json" {
		return fmt.Errorf("unsupported transport %q", result.Transport)
	}
	c.metadata = Metadata{
		Name:         NormalizeLibraryName(result.Library.Name),
		Version:      result.Library.Version,
		Description:  result.Library.Description,
		Transport:    result.Transport,
		Capabilities: result.Capabilities,
		Schema:       result.Schema,
	}
	c.handshakeDone = true
	return nil
}

func (c *Client) Metadata() Metadata {
	return c.metadata
}

// Path returns the filesystem path of the executable this client runs.
func (c *Client) Path() string {
	return c.path
}

// HandshakeDone reports whether the plugin protocol handshake was completed.
// call_function uses this to route automatically: handshook clients use the
// typed plugin transport (function.call), non-handshook clients send the
// method name directly as a raw JSON-RPC request.
func (c *Client) HandshakeDone() bool {
	return c.handshakeDone
}

// SetName overrides the library name used to register this client. It is only
// meaningful before a client is added to a Manager and is intended for raw
// (non-plugin-handshake) clients whose name would otherwise be empty.
func (c *Client) SetName(name string) {
	c.metadata.Name = NormalizeLibraryName(name)
	if c.metadata.Transport == "" {
		c.metadata.Transport = "json"
	}
}

// Call sends a raw JSON-RPC request to the executable and unmarshals the result
// into out (which may be nil to ignore the result). params may be any
// JSON-marshalable value (struct, map, slice, scalar). It is the low-level
// building block for non-plugin JSON-RPC peers; plugin callers should prefer
// CallFunction / NewObject / CallMethod which use the plugin method names.
func (c *Client) Call(ctx context.Context, method string, params any, out any) error {
	return c.call(ctx, method, params, nil, out)
}

// Batch sends multiple raw JSON-RPC requests in one batch frame and returns
// results in the same order as requests. Batch does not support host callbacks.
func (c *Client) Batch(ctx context.Context, requests []batchRequest) ([]json.RawMessage, error) {
	if len(requests) == 0 {
		return nil, nil
	}
	client := c.rpc
	if client == nil {
		client = c.peer.Client()
	}
	calls := make([]jsonrpc.BatchCall, len(requests))
	for i, req := range requests {
		calls[i] = jsonrpc.BatchCall{Method: req.Method, Params: req.Params}
	}
	results := client.CallBatch(ctx, calls)
	out := make([]json.RawMessage, len(results))
	for i, r := range results {
		if r.Err != nil {
			return nil, fmt.Errorf("batch call %d (%s): %w", i, requests[i].Method, mapRPCError(r.Err))
		}
		out[i] = r.Result
	}
	return out, nil
}

// Close shuts down this plugin process. The plugin.shutdown notification is
// best-effort: peers that do not implement the plugin protocol (e.g. raw
// JSON-RPC executables loaded via LoadPath(scriptling=false)) will return a
// method-not-found error which is intentionally ignored. Handshaken Scriptling
// plugins still report shutdown RPC errors. Real failures — the process not
// exiting, or exiting with a non-zero status — are also reported.
func (c *Client) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c.closing.Store(true)
	shutdownErr := c.call(ctx, "plugin.shutdown", nil, nil, nil)
	if c.rpc != nil {
		c.doneClose.Do(func() { close(c.done) })
		if shutdownErr != nil && c.HandshakeDone() {
			return shutdownErr
		}
		return nil
	}
	// stdio: closing the Peer closes the child's stdin, waits for it to exit
	// (up to the shutdown timeout), reaps it, and fires onProcessExit.
	var first error
	if shutdownErr != nil && c.HandshakeDone() {
		first = shutdownErr
	}
	if err := c.peer.Close(); err != nil && first == nil {
		first = err
	}
	c.doneClose.Do(func() { close(c.done) })
	return first
}

// Health reports whether this plugin process and stdio transport are healthy.
func (c *Client) Health() error {
	if c.rpc != nil {
		select {
		case <-c.done:
			return errors.New("plugin http client closed")
		default:
			return nil
		}
	}
	select {
	case <-c.done:
		if err := c.waitError(); err != nil {
			return err
		}
		return errors.New("plugin process exited")
	default:
		return nil
	}
}

func (c *Client) CallFunction(ctx context.Context, name string, args []Value, kwargs map[string]Value) (Value, error) {
	var result Value
	err := c.call(ctx, "function.call", functionCallParams{
		Name:   name,
		Args:   args,
		Kwargs: kwargs,
	}, nil, &result)
	return result, err
}

func (c *Client) CallFunctionWithCallbacks(ctx context.Context, name string, args []Value, kwargs map[string]Value, callbacks *callbackSet) (Value, error) {
	var result Value
	err := c.call(ctx, "function.call", functionCallParams{
		Name:   name,
		Args:   args,
		Kwargs: kwargs,
	}, callbacks, &result)
	return result, err
}

func (c *Client) NewObject(ctx context.Context, class string, args []Value, kwargs map[string]Value) (*RemoteRef, error) {
	var result RemoteRef
	err := c.call(ctx, "object.new", objectNewParams{
		Class:  class,
		Args:   args,
		Kwargs: kwargs,
	}, nil, &result)
	return &result, err
}

func (c *Client) NewObjectWithCallbacks(ctx context.Context, class string, args []Value, kwargs map[string]Value, callbacks *callbackSet) (*RemoteRef, error) {
	var result RemoteRef
	err := c.call(ctx, "object.new", objectNewParams{
		Class:  class,
		Args:   args,
		Kwargs: kwargs,
	}, callbacks, &result)
	return &result, err
}

func (c *Client) CallMethod(ctx context.Context, objectID, method string, args []Value, kwargs map[string]Value) (Value, error) {
	var result Value
	err := c.call(ctx, "object.call_method", methodCallParams{
		ObjectID: objectID,
		Method:   method,
		Args:     args,
		Kwargs:   kwargs,
	}, nil, &result)
	return result, err
}

func (c *Client) CallMethodWithCallbacks(ctx context.Context, objectID, method string, args []Value, kwargs map[string]Value, callbacks *callbackSet) (Value, error) {
	var result Value
	err := c.call(ctx, "object.call_method", methodCallParams{
		ObjectID: objectID,
		Method:   method,
		Args:     args,
		Kwargs:   kwargs,
	}, callbacks, &result)
	return result, err
}

func (c *Client) DestroyObject(ctx context.Context, objectID string) error {
	return c.call(ctx, "object.destroy", objectDestroyParams{
		ObjectID: objectID,
	}, nil, nil)
}

// call sends a plugin method call. For stdio clients it registers any host
// callbacks for the call's lifetime so inbound callback.call requests can be
// routed back to this call while it is in flight. For HTTP clients callbacks
// are rejected (the transport is not bidirectional).
func (c *Client) call(ctx context.Context, method string, params any, callbacks *callbackSet, result any) error {
	if c.rpc != nil {
		return c.httpCall(ctx, method, params, callbacks, result)
	}
	if callbacks != nil {
		owner := &callbackOwner{set: callbacks, ctx: ctx}
		c.mu.Lock()
		for callbackID := range callbacks.callbacks {
			c.callbackOwners[callbackID] = owner
		}
		c.mu.Unlock()
		defer func() {
			c.mu.Lock()
			for callbackID := range callbacks.callbacks {
				delete(c.callbackOwners, callbackID)
			}
			c.mu.Unlock()
		}()
	}
	return mapRPCError(c.peer.Client().Call(ctx, method, params, result))
}

func (c *Client) httpCall(ctx context.Context, method string, params any, callbacks *callbackSet, result any) error {
	if callbacks != nil && len(callbacks.callbacks) > 0 {
		return fmt.Errorf("callbacks are not supported over http json-rpc transport")
	}
	select {
	case <-c.done:
		return fmt.Errorf("plugin http client closed")
	default:
	}
	return mapRPCError(c.rpc.Call(ctx, method, params, result))
}

// mapRPCError converts a jsonrpc error into the plugin-protocol RPCError so the
// existing error contract (message text, *RPCError type) is preserved. Other
// errors (transport, context) pass through unchanged.
func mapRPCError(err error) error {
	if err == nil {
		return nil
	}
	var rpcErr *jsonrpc.Error
	if errors.As(err, &rpcErr) {
		return &RPCError{Code: rpcErr.Code, Message: rpcErr.Message}
	}
	return err
}

// newPluginPeerServer builds the inbound method registry for a stdio peer: it
// routes the plugin protocol's reverse-direction requests (callback.call and
// host.log) back to the host. The server closes over the client, so it must be
// constructed before the peer starts reading.
func newPluginPeerServer(c *Client) *jsonrpc.Server {
	srv := jsonrpc.NewServer()
	srv.Handle("callback.call", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p callbackCallParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, jsonrpc.NewError(-32000, err.Error(), nil)
		}
		c.mu.Lock()
		owner := c.callbackOwners[p.ID]
		c.mu.Unlock()
		if owner == nil {
			return nil, jsonrpc.NewError(-32000, "unknown callback "+p.ID, nil)
		}
		result, err := callHostCallback(owner.ctx, owner.set, p)
		if err != nil {
			return nil, jsonrpc.NewError(-32000, err.Error(), nil)
		}
		return result, nil
	})
	srv.Handle("host.log", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p logParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, jsonrpc.NewError(-32000, err.Error(), nil)
		}
		c.routeLog(p)
		return Value{Type: valueNull}, nil
	})
	return srv
}

// routeLog forwards a plugin log record to the host logger, if one is set.
func (c *Client) routeLog(params logParams) {
	log := c.getLogger()
	if log == nil {
		return
	}
	args := make([]any, 0, len(params.Args))
	for _, arg := range params.Args {
		args = append(args, transportValueToAny(arg))
	}
	switch strings.ToLower(params.Level) {
	case "trace":
		log.Trace(params.Message, args...)
	case "debug":
		log.Debug(params.Message, args...)
	case "warn", "warning":
		log.Warn(params.Message, args...)
	case "error":
		log.Error(params.Message, args...)
	case "fatal":
		log.Fatal(params.Message, args...)
	default:
		log.Info(params.Message, args...)
	}
}

func (c *Client) setLogger(log logger.Logger) {
	c.stateMu.Lock()
	c.logger = log
	c.stateMu.Unlock()
}

func (c *Client) getLogger() logger.Logger {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	return c.logger
}

// onProcessExit is the jsonrpc WithOnExit callback: it records the subprocess
// exit status, signals done, and fires the crash handler when the process died
// on its own (not during a normal Close).
func (c *Client) onProcessExit(waitErr error) {
	c.stateMu.Lock()
	c.waitErr = waitErr
	c.stateMu.Unlock()
	c.markDone()
}

// markDone closes the done channel (idempotent) and notifies any crash handler.
func (c *Client) markDone() {
	c.doneClose.Do(func() { close(c.done) })
	c.notifyExit()
}

func (c *Client) waitError() error {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	return c.waitErr
}

func (c *Client) setExitHandler(handler func(error)) {
	c.stateMu.Lock()
	c.exitHandler = handler
	c.stateMu.Unlock()
	// If the process already exited before the handler was installed, fire it.
	select {
	case <-c.done:
		c.notifyExit()
	default:
	}
}

// notifyExit fires the crash handler once, unless this is a normal Close
// (closing is set) or no handler is installed. A nil waitErr is reported as
// "plugin process exited".
func (c *Client) notifyExit() {
	if c.closing.Load() {
		return
	}
	c.stateMu.Lock()
	handler := c.exitHandler
	err := c.waitErr
	c.stateMu.Unlock()
	if handler == nil {
		return
	}
	if !c.exitFired.CompareAndSwap(false, true) {
		return
	}
	if err == nil {
		err = errors.New("plugin process exited")
	}
	handler(err)
}
