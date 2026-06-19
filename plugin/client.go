package plugin

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/paularlott/logger"
)

type Manager struct {
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
		clients: make(map[string]*Client),
		logger:  log,
	}
	if len(crashHandler) > 0 {
		manager.crashHandler = crashHandler[0]
	}
	return manager
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

// LoadPath starts a single executable and registers it under name. Identity
// is by absolute path: calling LoadPath twice with the same path AND the same
// name returns the existing client without respawning. A second LoadPath of
// an already-loaded path with a different name is rejected, as is any
// LoadPath that would register a name already in use by a different path.
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
	absPath, err := resolveExecutablePath(path)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	for _, existing := range m.clients {
		if existing.Path() == absPath {
			if existing.Metadata().Name == normalisedName {
				m.mu.Unlock()
				return existing, nil
			}
			m.mu.Unlock()
			return nil, fmt.Errorf("plugin %s already loaded as %s", absPath, existing.Metadata().Name)
		}
	}
	if _, exists := m.clients[normalisedName]; exists {
		m.mu.Unlock()
		return nil, fmt.Errorf("plugin name %s already in use", normalisedName)
	}
	m.mu.Unlock()

	var client *Client
	if scriptling {
		client, err = startClient(ctx, absPath, args)
	} else {
		client, err = spawnClient(ctx, absPath, args)
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
		if existing.Path() == absPath {
			m.mu.Unlock()
			_ = client.Close()
			if existing.Metadata().Name == normalisedName {
				return existing, nil
			}
			return nil, fmt.Errorf("plugin %s already loaded as %s", absPath, existing.Metadata().Name)
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

// Close shuts down all loaded plugin processes.
func (m *Manager) Close() error {
	m.mu.RLock()
	clients := make([]*Client, 0, len(m.clients))
	for _, client := range m.clients {
		clients = append(clients, client)
	}
	m.mu.RUnlock()

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
func (m *Manager) List() []Metadata {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Metadata, 0, len(m.clients))
	for _, client := range m.clients {
		out = append(out, client.Metadata())
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
func (m *Manager) Get(name string) (*Client, bool) {
	normalized := NormalizeLibraryName(name)
	m.mu.RLock()
	defer m.mu.RUnlock()
	client, ok := m.clients[normalized]
	return client, ok
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

type Client struct {
	path     string
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	stdout   io.ReadCloser
	encoder  *json.Encoder
	metadata Metadata

	handshakeDone bool

	nextID         atomic.Int64
	pending        map[int64]*pendingCall
	callbackOwners map[string]*pendingCall
	mu             sync.Mutex
	writeMu        sync.Mutex
	done           chan struct{}
	waitDone       chan struct{}
	stateMu        sync.Mutex
	readErr        error
	waitErr        error
	closing        atomic.Bool
	exitNotified   atomic.Bool
	exitHandler    func(error)
	logger         logger.Logger
}

type pendingCall struct {
	id        int64
	response  chan rpcResponse
	callbacks chan callbackInbound
	set       *callbackSet
	done      chan struct{}
}

type callbackInbound struct {
	request  rpcRequest
	response chan rpcResponse
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

// spawnClient starts an executable as a subprocess wired up for JSON-RPC over
// stdio, but does not perform the plugin handshake. Callers that want the
// plugin protocol handshake should use LoadClient (or call handshake next);
// callers that want to skip the handshake may use the returned client directly.
// args, if non-empty, are passed as command-line arguments to the executable.
func spawnClient(ctx context.Context, path string, args []string) (*Client, error) {
	// The subprocess must outlive any single request context — the evaluation
	// context that reaches load() may be cancelled after the builtin returns.
	// The process lifecycle is managed by the client's Close() (stdin close +
	// process wait) and the manager's Close()/Unload(), not by a context.
	cmd := exec.Command(path, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	client := &Client{
		path:           path,
		cmd:            cmd,
		stdin:          stdin,
		stdout:         stdout,
		encoder:        json.NewEncoder(stdin),
		pending:        make(map[int64]*pendingCall),
		callbackOwners: make(map[string]*pendingCall),
		done:           make(chan struct{}),
		waitDone:       make(chan struct{}),
	}
	go client.readLoop()
	go client.waitLoop()
	return client, nil
}

// LoadClient spawns an executable and performs the plugin protocol handshake.
// The returned client has Metadata populated from the handshake result.
// args, if non-empty, are passed as command-line arguments to the executable.
func LoadClient(ctx context.Context, path string, args []string) (*Client, error) {
	return startClient(ctx, path, args)
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
// meaningful before the client is added to a Manager and is intended for raw
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
	calls := make([]*pendingCall, len(requests))
	wire := make([]rpcRequest, len(requests))

	c.mu.Lock()
	for i, req := range requests {
		id := c.nextID.Add(1)
		call := &pendingCall{
			id:       id,
			response: make(chan rpcResponse, 1),
			done:     make(chan struct{}),
		}
		c.pending[id] = call
		calls[i] = call
		wire[i] = rpcRequest{JSONRPC: "2.0", ID: id, Method: req.Method, Params: req.Params}
	}
	c.mu.Unlock()
	defer func() {
		for _, call := range calls {
			c.removeCall(call)
		}
	}()

	c.writeMu.Lock()
	err := c.encoder.Encode(wire)
	c.writeMu.Unlock()
	if err != nil {
		return nil, err
	}

	results := make([]json.RawMessage, len(calls))
	for i, call := range calls {
		select {
		case resp := <-call.response:
			if resp.Error != nil {
				return nil, fmt.Errorf("batch call %d (%s): %w", i, requests[i].Method, resp.Error)
			}
			results[i] = resp.Result
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-c.done:
			return nil, fmt.Errorf("plugin process exited")
		}
	}
	return results, nil
}

// Close shuts down this plugin process. The plugin.shutdown notification is
// best-effort: peers that do not implement the plugin protocol (e.g. raw
// JSON-RPC executables loaded via LoadPath(scriptling=false)) will return a
// method-not-found error which is intentionally ignored. Handshaken Scriptling
// plugins still report shutdown RPC errors. Real failures — the process not
// exiting, or exiting with a non-zero status — are also reported.
func (c *Client) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	c.closing.Store(true)
	shutdownErr := c.call(ctx, "plugin.shutdown", nil, nil, nil)
	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	var first error
	if shutdownErr != nil && c.HandshakeDone() {
		first = shutdownErr
	}
	select {
	case <-c.waitDone:
		if err := c.waitError(); err != nil && first == nil {
			first = err
		}
	case <-ctx.Done():
		if first == nil {
			first = ctx.Err()
		}
	}
	return first
}

// Health reports whether this plugin process and stdio transport are healthy.
func (c *Client) Health() error {
	select {
	case <-c.waitDone:
		if err := c.waitError(); err != nil {
			return err
		}
		return errors.New("plugin process exited")
	default:
	}
	select {
	case <-c.done:
		if err := c.readError(); err != nil {
			return err
		}
		return errors.New("plugin stdio closed")
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

func (c *Client) call(ctx context.Context, method string, params any, callbacks *callbackSet, result any) error {
	id := c.nextID.Add(1)
	call := &pendingCall{
		id:       id,
		response: make(chan rpcResponse, 1),
		set:      callbacks,
		done:     make(chan struct{}),
	}
	if callbacks != nil {
		call.callbacks = make(chan callbackInbound)
	}

	c.mu.Lock()
	c.pending[id] = call
	if callbacks != nil {
		for callbackID := range callbacks.callbacks {
			c.callbackOwners[callbackID] = call
		}
	}
	c.mu.Unlock()
	defer c.removeCall(call)

	req := rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}
	c.writeMu.Lock()
	err := c.encoder.Encode(req)
	c.writeMu.Unlock()
	if err != nil {
		return err
	}

	for {
		select {
		case resp := <-call.response:
			if resp.Error != nil {
				return resp.Error
			}
			if result != nil && len(resp.Result) > 0 {
				if err := json.Unmarshal(resp.Result, result); err != nil {
					return err
				}
			}
			return nil
		case inbound := <-call.callbacks:
			resp := c.handleCallback(ctx, call, inbound.request)
			inbound.response <- resp
		case <-ctx.Done():
			return ctx.Err()
		case <-c.done:
			return fmt.Errorf("plugin process exited")
		}
	}
}

func (c *Client) handleCallback(ctx context.Context, call *pendingCall, req rpcRequest) rpcResponse {
	var params callbackCallParams
	if err := decodeParams(req.Params, &params); err != nil {
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32000, Message: err.Error()}}
	}
	result, err := callHostCallback(ctx, call.set, params)
	resp := rpcResponse{JSONRPC: "2.0", ID: req.ID}
	if err != nil {
		resp.Error = &RPCError{Code: -32000, Message: err.Error()}
		return resp
	}
	raw, err := json.Marshal(result)
	if err != nil {
		resp.Error = &RPCError{Code: -32000, Message: err.Error()}
		return resp
	}
	resp.Result = raw
	return resp
}

func (c *Client) removeCall(call *pendingCall) {
	c.mu.Lock()
	delete(c.pending, call.id)
	if call.set != nil {
		for callbackID := range call.set.callbacks {
			delete(c.callbackOwners, callbackID)
		}
	}
	c.mu.Unlock()
	close(call.done)
}

func (c *Client) readLoop() {
	defer close(c.done)
	decoder := json.NewDecoder(bufio.NewReader(c.stdout))
	for {
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			if err != io.EOF {
				c.setReadErr(err)
			}
			return
		}
		if len(bytes.TrimSpace(raw)) == 0 {
			continue
		}
		if bytes.TrimSpace(raw)[0] == '[' {
			var batch []rpcMessage
			if err := json.Unmarshal(raw, &batch); err != nil {
				c.setReadErr(err)
				return
			}
			var responses []rpcResponse
			for _, msg := range batch {
				if resp, ok := c.handleInboundMessage(msg); ok {
					responses = append(responses, resp)
				}
			}
			if len(responses) > 0 {
				c.writeMu.Lock()
				_ = c.encoder.Encode(responses)
				c.writeMu.Unlock()
			}
			continue
		}
		var msg rpcMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			c.setReadErr(err)
			return
		}
		if resp, ok := c.handleInboundMessage(msg); ok {
			c.writeMu.Lock()
			_ = c.encoder.Encode(resp)
			c.writeMu.Unlock()
		}
	}
}

func (c *Client) handleInboundMessage(msg rpcMessage) (rpcResponse, bool) {
	if msg.Method != "" {
		req := rpcRequest{JSONRPC: msg.JSONRPC, ID: msg.ID, Method: msg.Method, Params: msg.Params}
		resp := c.routeRequest(req)
		return resp, true
	}
	c.mu.Lock()
	call := c.pending[msg.ID]
	c.mu.Unlock()
	if call != nil {
		call.response <- rpcResponse{JSONRPC: msg.JSONRPC, ID: msg.ID, Result: msg.Result, Error: msg.Error}
	}
	return rpcResponse{}, false
}

func (c *Client) waitLoop() {
	defer close(c.waitDone)
	if c.cmd == nil {
		return
	}
	err := c.cmd.Wait()
	if err != nil {
		c.setWaitErr(err)
	}
	c.notifyExit()
}

func (c *Client) setReadErr(err error) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	c.readErr = err
}

func (c *Client) readError() error {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	return c.readErr
}

func (c *Client) setWaitErr(err error) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	c.waitErr = err
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

	select {
	case <-c.waitDone:
		c.notifyExit()
	default:
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
	if !c.exitNotified.CompareAndSwap(false, true) {
		return
	}
	if err == nil {
		err = errors.New("plugin process exited")
	}
	handler(err)
}

func (c *Client) routeRequest(req rpcRequest) rpcResponse {
	switch req.Method {
	case "callback.call":
		return c.routeCallbackRequest(req)
	case "host.log":
		return c.routeLogRequest(req)
	default:
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32000, Message: "unknown method " + req.Method}}
	}
}

func (c *Client) routeLogRequest(req rpcRequest) rpcResponse {
	var params logParams
	if err := decodeParams(req.Params, &params); err != nil {
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32000, Message: err.Error()}}
	}
	log := c.getLogger()
	if log != nil {
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
	raw, _ := json.Marshal(Value{Type: valueNull})
	return rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: raw}
}

func (c *Client) routeCallbackRequest(req rpcRequest) rpcResponse {
	var params callbackCallParams
	if err := decodeParams(req.Params, &params); err != nil {
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32000, Message: err.Error()}}
	}
	c.mu.Lock()
	call := c.callbackOwners[params.ID]
	c.mu.Unlock()
	if call == nil || call.callbacks == nil {
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32000, Message: "unknown callback " + params.ID}}
	}
	inbound := callbackInbound{request: req, response: make(chan rpcResponse, 1)}
	// call.callbacks is intentionally never closed; call.done signals expiry.
	// This lets routeRequest safely race with removeCall without send-on-closed panics.
	select {
	case call.callbacks <- inbound:
		select {
		case resp := <-inbound.response:
			return resp
		case <-call.done:
			return rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32000, Message: "callback call ended"}}
		case <-c.done:
			return rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32000, Message: "plugin process exited"}}
		}
	case <-call.done:
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32000, Message: "callback call ended"}}
	case <-c.done:
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &RPCError{Code: -32000, Message: "plugin process exited"}}
	}
}
