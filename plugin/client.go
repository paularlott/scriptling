package plugin

import (
	"bufio"
	"context"
	"encoding/json"
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
)

type Manager struct {
	dirs     []string
	clients  map[string]*Client
	warnings []string
	mu       sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		clients: make(map[string]*Client),
	}
}

func (m *Manager) AddDir(dir string) {
	if dir != "" {
		m.dirs = append(m.dirs, dir)
	}
}

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
			client, err := startClient(ctx, path)
			if err != nil {
				m.addWarning("plugin %s failed to load: %v", path, err)
				continue
			}
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
		}
	}
	return nil
}

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

func (m *Manager) Warnings() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, len(m.warnings))
	copy(out, m.warnings)
	return out
}

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

	nextID  atomic.Int64
	pending map[int64]chan rpcResponse
	mu      sync.Mutex
	writeMu sync.Mutex
	done    chan struct{}
}

func startClient(ctx context.Context, path string) (*Client, error) {
	cmd := exec.CommandContext(ctx, path)
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
		path:    path,
		cmd:     cmd,
		stdin:   stdin,
		stdout:  stdout,
		encoder: json.NewEncoder(stdin),
		pending: make(map[int64]chan rpcResponse),
		done:    make(chan struct{}),
	}
	go client.readLoop()

	handshakeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var result handshakeResult
	err = client.call(handshakeCtx, "scriptling.handshake", handshakeParams{
		Protocol:     ProtocolVersion,
		Host:         "scriptling",
		HostVersion:  "dev",
		Transports:   []string{"json"},
		Capabilities: []string{"remote_objects"},
	}, &result)
	if err != nil {
		_ = client.Close()
		return nil, err
	}
	if result.Protocol != ProtocolVersion {
		_ = client.Close()
		return nil, fmt.Errorf("unsupported protocol %q", result.Protocol)
	}
	if result.Transport != "json" {
		_ = client.Close()
		return nil, fmt.Errorf("unsupported transport %q", result.Transport)
	}
	client.metadata = Metadata{
		Name:         NormalizeLibraryName(result.Library.Name),
		Version:      result.Library.Version,
		Description:  result.Library.Description,
		Transport:    result.Transport,
		Capabilities: result.Capabilities,
		Schema:       result.Schema,
	}
	return client, nil
}

func (c *Client) Metadata() Metadata {
	return c.metadata
}

func (c *Client) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = c.call(ctx, "plugin.shutdown", nil, nil)
	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Wait()
	}
	return nil
}

func (c *Client) CallFunction(ctx context.Context, name string, args []Value, kwargs map[string]Value) (Value, error) {
	var result Value
	err := c.call(ctx, "function.call", functionCallParams{
		Name:   name,
		Args:   args,
		Kwargs: kwargs,
	}, &result)
	return result, err
}

func (c *Client) NewObject(ctx context.Context, class string, args []Value, kwargs map[string]Value) (*RemoteRef, error) {
	var result RemoteRef
	err := c.call(ctx, "object.new", objectNewParams{
		Class:  class,
		Args:   args,
		Kwargs: kwargs,
	}, &result)
	return &result, err
}

func (c *Client) CallMethod(ctx context.Context, objectID, method string, args []Value, kwargs map[string]Value) (Value, error) {
	var result Value
	err := c.call(ctx, "object.call_method", methodCallParams{
		ObjectID: objectID,
		Method:   method,
		Args:     args,
		Kwargs:   kwargs,
	}, &result)
	return result, err
}

func (c *Client) DestroyObject(ctx context.Context, objectID string) error {
	return c.call(ctx, "object.destroy", objectDestroyParams{
		ObjectID: objectID,
	}, nil)
}

func (c *Client) call(ctx context.Context, method string, params any, result any) error {
	id := c.nextID.Add(1)
	ch := make(chan rpcResponse, 1)

	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()

	req := rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}
	c.writeMu.Lock()
	err := c.encoder.Encode(req)
	c.writeMu.Unlock()
	if err != nil {
		c.removePending(id)
		return err
	}

	select {
	case resp := <-ch:
		if resp.Error != nil {
			return resp.Error
		}
		if result != nil && len(resp.Result) > 0 {
			if err := json.Unmarshal(resp.Result, result); err != nil {
				return err
			}
		}
		return nil
	case <-ctx.Done():
		c.removePending(id)
		return ctx.Err()
	case <-c.done:
		c.removePending(id)
		return fmt.Errorf("plugin process exited")
	}
}

func (c *Client) removePending(id int64) {
	c.mu.Lock()
	delete(c.pending, id)
	c.mu.Unlock()
}

func (c *Client) readLoop() {
	defer close(c.done)
	decoder := json.NewDecoder(bufio.NewReader(c.stdout))
	for {
		var resp rpcResponse
		if err := decoder.Decode(&resp); err != nil {
			return
		}
		c.mu.Lock()
		ch := c.pending[resp.ID]
		delete(c.pending, resp.ID)
		c.mu.Unlock()
		if ch != nil {
			ch <- resp
		}
	}
}
