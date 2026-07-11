package pool

import (
	"crypto/tls"
	"net/http"
	"sync"
	"time"

	"golang.org/x/net/http2"
)

// Config holds configuration for a scriptling HTTP pool.
// Optimized for short-lived, diverse HTTP requests (requests library, wait_for).
//
// InsecureSkipVerify is no longer part of this struct: secure and insecure
// traffic are served by two entirely separate pools (see GetHTTPClient and
// GetInsecureHTTPClient) so that opting one caller into skipping TLS
// verification can never affect requests made by any other caller sharing
// the pool.
type Config struct {
	// Connection pool settings
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	IdleConnTimeout     time.Duration

	// Default timeout for requests
	Timeout time.Duration
}

// DefaultConfig returns sensible defaults, applied to both the secure and
// insecure pools. Optimized for short-lived scripting HTTP requests.
func DefaultConfig() *Config {
	return &Config{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
		Timeout:             30 * time.Second, // Shorter timeout for general HTTP requests
	}
}

var (
	poolCfgMu sync.RWMutex // Protects poolCfg for GetConfig/SetConfig
	poolCfg   *Config

	secureOnce   sync.Once
	secureInst   *scriptlingPool
	insecureOnce sync.Once
	insecureInst *scriptlingPool
)

// scriptlingPool implements HTTPPool interface
type scriptlingPool struct {
	httpClient *http.Client
}

// SetConfig sets up the shared connection pool configuration (connection
// limits and timeout, applied to both the secure and insecure pools).
// Must be called before any HTTP calls are made (before GetHTTPClient /
// GetInsecureHTTPClient), since each pool is initialized lazily on first use.
func SetConfig(config *Config) {
	poolCfgMu.Lock()
	defer poolCfgMu.Unlock()
	poolCfg = config
}

// GetConfig returns the current pool configuration.
// Returns a copy to prevent external modification of internal state.
func GetConfig() Config {
	poolCfgMu.RLock()
	defer poolCfgMu.RUnlock()

	if poolCfg == nil {
		return *DefaultConfig()
	}
	return *poolCfg
}

func currentConfig() *Config {
	poolCfgMu.RLock()
	cfg := poolCfg
	poolCfgMu.RUnlock()
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return cfg
}

// GetHTTPClient returns the shared, TLS-verified HTTP client (lazy
// initialization). This is the pool used by default for all scripting HTTP
// requests (requests library, wait_for, etc).
func GetHTTPClient() *http.Client {
	secureOnce.Do(func() {
		secureInst = newScriptlingPool(currentConfig(), false)
	})
	return secureInst.httpClient
}

// GetInsecureHTTPClient returns a separate shared HTTP client with TLS
// certificate verification disabled (lazy initialization). This pool is
// entirely independent of GetHTTPClient's pool: enabling insecure mode for
// one caller (e.g. a script explicitly opting in with insecure=True) never
// affects the TLS behavior of any other caller using the default pool.
func GetInsecureHTTPClient() *http.Client {
	insecureOnce.Do(func() {
		insecureInst = newScriptlingPool(currentConfig(), true)
	})
	return insecureInst.httpClient
}

// GetPool returns the shared, TLS-verified pool as an HTTPPool interface.
func GetPool() *scriptlingPool {
	secureOnce.Do(func() {
		secureInst = newScriptlingPool(currentConfig(), false)
	})
	return secureInst
}

// GetInsecurePool returns the shared, TLS-skip-verify pool as an HTTPPool interface.
func GetInsecurePool() *scriptlingPool {
	insecureOnce.Do(func() {
		insecureInst = newScriptlingPool(currentConfig(), true)
	})
	return insecureInst
}

// GetHTTPClient returns the shared HTTP client
// Implements HTTPPool interface
func (p *scriptlingPool) GetHTTPClient() *http.Client {
	return p.httpClient
}

// newScriptlingPool creates a new scriptling pool with the given configuration.
// insecureSkipVerify controls only this pool's TLS verification; it never
// affects any other pool.
func newScriptlingPool(cfg *Config, insecureSkipVerify bool) *scriptlingPool {
	tlsCfg := &tls.Config{
		InsecureSkipVerify: insecureSkipVerify, //nolint:gosec // isolated to the insecure pool only
	}
	if !insecureSkipVerify {
		tlsCfg.MinVersion = tls.VersionTLS13
	}

	transport := &http.Transport{
		TLSClientConfig:     tlsCfg,
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
