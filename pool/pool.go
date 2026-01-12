package pool

import (
	"crypto/tls"
	"net/http"
	"sync"
	"time"

	"golang.org/x/net/http2"
)

// Config holds configuration for the scriptling HTTP pool
// Optimized for short-lived, diverse HTTP requests (requests library, wait_for)
type Config struct {
	// InsecureSkipVerify allows self-signed certificates
	// WARNING: This should be false in production for security
	InsecureSkipVerify bool

	// Connection pool settings
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	IdleConnTimeout     time.Duration

	// Default timeout for requests
	Timeout time.Duration
}

// DefaultConfig returns sensible defaults (secure by default)
// Optimized for short-lived scripting HTTP requests
func DefaultConfig() *Config {
	return &Config{
		InsecureSkipVerify: false, // Reject self-signed certs by default (secure)
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
		Timeout:             30 * time.Second, // Shorter timeout for general HTTP requests
	}
}

var (
	poolOnce  sync.Once
	poolCfg   *Config
	poolCfgMu sync.RWMutex // Protects poolCfg for GetConfig/SetConfig
	poolInst *scriptlingPool
)

// scriptlingPool implements HTTPPool interface
type scriptlingPool struct {
	httpClient *http.Client
}

// SetConfig sets up the shared connection pool configuration
// Must be called before any HTTP calls are made (before GetHTTPClient)
func SetConfig(config *Config) {
	poolCfgMu.Lock()
	defer poolCfgMu.Unlock()
	poolCfg = config
}

// GetConfig returns the current pool configuration
// Returns a copy to prevent external modification of internal state
func GetConfig() Config {
	poolCfgMu.RLock()
	defer poolCfgMu.RUnlock()

	if poolCfg == nil {
		return *DefaultConfig()
	}
	return *poolCfg
}

// ensurePoolInitialized initializes the pool if not already done
func ensurePoolInitialized() {
	poolOnce.Do(func() {
		poolCfgMu.RLock()
		cfg := poolCfg
		poolCfgMu.RUnlock()

		if cfg == nil {
			cfg = DefaultConfig()
		}

		poolInst = newScriptlingPool(cfg)
	})
}

// GetHTTPClient returns the shared HTTP client (lazy initialization)
// The pool is created on first call with either configured or default values
func GetHTTPClient() *http.Client {
	ensurePoolInitialized()
	return poolInst.httpClient
}

// GetPool returns the scriptling pool as an HTTPPool interface
func GetPool() *scriptlingPool {
	ensurePoolInitialized()
	return poolInst
}

// GetHTTPClient returns the shared HTTP client
// Implements HTTPPool interface
func (p *scriptlingPool) GetHTTPClient() *http.Client {
	return p.httpClient
}

// newScriptlingPool creates a new scriptling pool with the given configuration
func newScriptlingPool(cfg *Config) *scriptlingPool {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.InsecureSkipVerify,
			MinVersion:         tls.VersionTLS13,
		},
		MaxIdleConns:        cfg.MaxIdleConns,
		MaxIdleConnsPerHost: cfg.MaxIdleConnsPerHost,
		IdleConnTimeout:     cfg.IdleConnTimeout,
		ForceAttemptHTTP2:   true,
	}

	http2.ConfigureTransport(transport)
	// Note: HTTP/2 configuration errors are non-fatal, client will fall back to HTTP/1.1

	return &scriptlingPool{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   cfg.Timeout,
		},
	}
}
