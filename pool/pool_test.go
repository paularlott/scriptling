package pool

import (
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.InsecureSkipVerify {
		t.Error("DefaultConfig() InsecureSkipVerify should be false")
	}

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

	if cfg.InsecureSkipVerify {
		t.Error("GetConfig() InsecureSkipVerify should be false by default")
	}

	if cfg.MaxIdleConns != 100 {
		t.Errorf("GetConfig() MaxIdleConns = %d, want 100", cfg.MaxIdleConns)
	}
}

func TestSetAndGetConfig(t *testing.T) {
	// Save original config
	originalCfg := GetConfig()

	// Set a custom config
	customCfg := &Config{
		InsecureSkipVerify:  true,
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 25,
		IdleConnTimeout:     60 * time.Second,
		Timeout:             10 * time.Second,
	}
	SetConfig(customCfg)

	// Verify GetConfig returns the custom config
	gotCfg := GetConfig()
	if gotCfg.InsecureSkipVerify != true {
		t.Error("GetConfig() InsecureSkipVerify = false, want true")
	}
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
		InsecureSkipVerify: false,
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
}

func TestGetHTTPClient_CustomConfig(t *testing.T) {
	// Note: Once GetHTTPClient() is called, the pool is initialized
	// This test verifies the config is stored correctly even if pool is already initialized

	// Set custom config (this updates the stored config)
	customCfg := &Config{
		InsecureSkipVerify: true,
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 25,
		IdleConnTimeout:     60 * time.Second,
		Timeout:             10 * time.Second,
	}
	SetConfig(customCfg)

	// Verify config is stored
	gotCfg := GetConfig()
	if gotCfg.MaxIdleConns != 50 {
		t.Errorf("GetConfig() MaxIdleConns = %d, want 50", gotCfg.MaxIdleConns)
	}

	// Reset to default
	SetConfig(DefaultConfig())
}

func TestGetHTTPClient_Singleton(t *testing.T) {
	// Reset pool state by setting config
	SetConfig(DefaultConfig())

	client1 := GetHTTPClient()
	client2 := GetHTTPClient()

	if client1 != client2 {
		t.Error("GetHTTPClient() returned different instances")
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

func TestNewScriptlingPool(t *testing.T) {
	cfg := &Config{
		InsecureSkipVerify: true,
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 25,
		IdleConnTimeout:     60 * time.Second,
		Timeout:             10 * time.Second,
	}

	pool := newScriptlingPool(cfg)

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
}

func TestConfig_AllFields(t *testing.T) {
	cfg := Config{
		InsecureSkipVerify:  true,
		MaxIdleConns:        200,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     120 * time.Second,
		Timeout:             60 * time.Second,
	}

	pool := newScriptlingPool(&cfg)
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

	if !transport.TLSClientConfig.InsecureSkipVerify {
		t.Error("Config InsecureSkipVerify not applied")
	}
}
