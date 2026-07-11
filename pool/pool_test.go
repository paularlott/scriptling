package pool

import (
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxIdleConns != 100 {
		t.Errorf("DefaultConfig() MaxIdleConns = %d, want 100", cfg.MaxIdleConns)
	}

	if cfg.MaxIdleConnsPerHost != 100 {
		t.Errorf("DefaultConfig() MaxIdleConnsPerHost = %d, want 100", cfg.MaxIdleConnsPerHost)
	}

	if cfg.IdleConnTimeout != 90*time.Second {
		t.Errorf("DefaultConfig() IdleConnTimeout = %v, want 90s", cfg.IdleConnTimeout)
	}

	if cfg.Timeout != 30*time.Second {
		t.Errorf("DefaultConfig() Timeout = %v, want 30s", cfg.Timeout)
	}
}

func TestGetConfig_Default(t *testing.T) {
	cfg := GetConfig()

	if cfg.MaxIdleConns != 100 {
		t.Errorf("GetConfig() MaxIdleConns = %d, want 100", cfg.MaxIdleConns)
	}
}

func TestSetAndGetConfig(t *testing.T) {
	// Save original config
	originalCfg := GetConfig()

	// Set a custom config
	customCfg := &Config{
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 25,
		IdleConnTimeout:     60 * time.Second,
		Timeout:             10 * time.Second,
	}
	SetConfig(customCfg)

	// Verify GetConfig returns the custom config
	gotCfg := GetConfig()
	if gotCfg.MaxIdleConns != 50 {
		t.Errorf("GetConfig() MaxIdleConns = %d, want 50", gotCfg.MaxIdleConns)
	}
	if gotCfg.MaxIdleConnsPerHost != 25 {
		t.Errorf("GetConfig() MaxIdleConnsPerHost = %d, want 25", gotCfg.MaxIdleConnsPerHost)
	}
	if gotCfg.IdleConnTimeout != 60*time.Second {
		t.Errorf("GetConfig() IdleConnTimeout = %v, want 60s", gotCfg.IdleConnTimeout)
	}
	if gotCfg.Timeout != 10*time.Second {
		t.Errorf("GetConfig() Timeout = %v, want 10s", gotCfg.Timeout)
	}

	// Restore original config
	SetConfig(&originalCfg)
}

func TestGetConfig_Copy(t *testing.T) {
	customCfg := &Config{
		MaxIdleConns:        200,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     120 * time.Second,
		Timeout:             60 * time.Second,
	}
	SetConfig(customCfg)

	gotCfg1 := GetConfig()
	gotCfg2 := GetConfig()

	// Verify they have the same values
	if gotCfg1.MaxIdleConns != gotCfg2.MaxIdleConns {
		t.Error("GetConfig() returned inconsistent values")
	}

	// Modify one should not affect the other (since we return a copy)
	gotCfg1.MaxIdleConns = 999
	if gotCfg2.MaxIdleConns == 999 {
		t.Error("GetConfig() returned a reference, not a copy")
	}
}

func TestGetHTTPClient(t *testing.T) {
	// This test may run after other tests, so we just verify the client exists
	client := GetHTTPClient()

	if client == nil {
		t.Fatal("GetHTTPClient() returned nil")
	}

	if client.Transport == nil {
		t.Error("GetHTTPClient() Transport is nil")
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("GetHTTPClient() Transport is not *http.Transport")
	}
	if transport.TLSClientConfig != nil && transport.TLSClientConfig.InsecureSkipVerify {
		t.Error("GetHTTPClient() should be TLS-verified, got InsecureSkipVerify=true")
	}
}

func TestGetInsecureHTTPClient(t *testing.T) {
	client := GetInsecureHTTPClient()

	if client == nil {
		t.Fatal("GetInsecureHTTPClient() returned nil")
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("GetInsecureHTTPClient() Transport is not *http.Transport")
	}
	if transport.TLSClientConfig == nil || !transport.TLSClientConfig.InsecureSkipVerify {
		t.Error("GetInsecureHTTPClient() should have InsecureSkipVerify=true")
	}
}

func TestSecureAndInsecurePools_AreIsolated(t *testing.T) {
	secure := GetHTTPClient()
	insecure := GetInsecureHTTPClient()

	if secure == insecure {
		t.Fatal("GetHTTPClient() and GetInsecureHTTPClient() returned the same client instance")
	}

	secureTransport, ok := secure.Transport.(*http.Transport)
	if !ok {
		t.Fatal("secure client Transport is not *http.Transport")
	}
	insecureTransport, ok := insecure.Transport.(*http.Transport)
	if !ok {
		t.Fatal("insecure client Transport is not *http.Transport")
	}

	if secureTransport.TLSClientConfig != nil && secureTransport.TLSClientConfig.InsecureSkipVerify {
		t.Error("secure pool's TLS config was affected by the insecure pool")
	}
	if insecureTransport.TLSClientConfig == nil || !insecureTransport.TLSClientConfig.InsecureSkipVerify {
		t.Error("insecure pool did not retain InsecureSkipVerify=true")
	}
}

func TestGetHTTPClient_Singleton(t *testing.T) {
	client1 := GetHTTPClient()
	client2 := GetHTTPClient()

	if client1 != client2 {
		t.Error("GetHTTPClient() returned different instances")
	}
}

func TestGetInsecureHTTPClient_Singleton(t *testing.T) {
	client1 := GetInsecureHTTPClient()
	client2 := GetInsecureHTTPClient()

	if client1 != client2 {
		t.Error("GetInsecureHTTPClient() returned different instances")
	}
}

func TestGetPool(t *testing.T) {
	pool := GetPool()

	if pool == nil {
		t.Fatal("GetPool() returned nil")
	}

	client := pool.GetHTTPClient()
	if client == nil {
		t.Error("GetPool().GetHTTPClient() returned nil")
	}
}

func TestGetInsecurePool(t *testing.T) {
	pool := GetInsecurePool()

	if pool == nil {
		t.Fatal("GetInsecurePool() returned nil")
	}

	client := pool.GetHTTPClient()
	if client == nil {
		t.Error("GetInsecurePool().GetHTTPClient() returned nil")
	}
}

func TestGetHTTPClient_Concurrent(t *testing.T) {
	// Test concurrent access to GetHTTPClient
	var wg sync.WaitGroup
	clients := make([]*http.Client, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			clients[idx] = GetHTTPClient()
		}(i)
	}

	wg.Wait()

	// All clients should be the same instance
	for i := 1; i < len(clients); i++ {
		if clients[i] != clients[0] {
			t.Error("GetHTTPClient() returned different instances concurrently")
			break
		}
	}
}

func TestSetConfig_Concurrent(t *testing.T) {
	// Test concurrent access to SetConfig and GetConfig
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if idx%2 == 0 {
				cfg := &Config{
					MaxIdleConns:        idx,
					MaxIdleConnsPerHost: idx / 2,
					Timeout:             time.Duration(idx) * time.Second,
				}
				SetConfig(cfg)
			} else {
				_ = GetConfig()
			}
		}(i)
	}

	wg.Wait()

	// Just verify no race conditions occurred
	_ = GetConfig()
}

func TestNewScriptlingPool_Secure(t *testing.T) {
	cfg := &Config{
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 25,
		IdleConnTimeout:     60 * time.Second,
		Timeout:             10 * time.Second,
	}

	pool := newScriptlingPool(cfg, false)

	if pool == nil {
		t.Fatal("newScriptlingPool() returned nil")
	}

	client := pool.GetHTTPClient()
	if client == nil {
		t.Fatal("newScriptlingPool().GetHTTPClient() returned nil")
	}

	if client.Timeout != 10*time.Second {
		t.Errorf("newScriptlingPool().GetHTTPClient() Timeout = %v, want 10s", client.Timeout)
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Client Transport is not *http.Transport")
	}
	if transport.TLSClientConfig != nil && transport.TLSClientConfig.InsecureSkipVerify {
		t.Error("secure pool should not set InsecureSkipVerify")
	}
}

func TestConfig_AllFields(t *testing.T) {
	cfg := Config{
		MaxIdleConns:        200,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     120 * time.Second,
		Timeout:             60 * time.Second,
	}

	pool := newScriptlingPool(&cfg, true)
	client := pool.GetHTTPClient()

	// Verify client uses config values
	if client.Timeout != 60*time.Second {
		t.Errorf("Config Timeout not applied: got %v, want 60s", client.Timeout)
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Client Transport is not *http.Transport")
	}

	if transport.MaxIdleConns != 200 {
		t.Errorf("Config MaxIdleConns not applied: got %d, want 200", transport.MaxIdleConns)
	}

	if transport.MaxIdleConnsPerHost != 100 {
		t.Errorf("Config MaxIdleConnsPerHost not applied: got %d, want 100", transport.MaxIdleConnsPerHost)
	}

	if transport.IdleConnTimeout != 120*time.Second {
		t.Errorf("Config IdleConnTimeout not applied: got %v, want 120s", transport.IdleConnTimeout)
	}

	if transport.TLSClientConfig == nil || !transport.TLSClientConfig.InsecureSkipVerify {
		t.Error("insecureSkipVerify=true was not applied")
	}
}
