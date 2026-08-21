package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadNetworkPolicyFileOnly(t *testing.T) {
	// Nothing set: no config at all.
	cfg, err := LoadNetworkPolicy("")
	if err != nil || cfg != nil {
		t.Errorf("empty path should be nil config, got %v, %v", cfg, err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "policy.toml")
	if err := os.WriteFile(path, []byte("dns_servers = [\"127.0.0.1:1\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err = LoadNetworkPolicy(path)
	if err != nil {
		t.Fatalf("LoadNetworkPolicy: %v", err)
	}
	if len(cfg.DNSServers) != 1 || cfg.DNSServers[0] != "127.0.0.1:1" {
		t.Errorf("servers = %v", cfg.DNSServers)
	}
	if cfg.AllowAll {
		t.Error("a policy file must keep its checks active")
	}

	// Invalid policy files abort rather than degrade.
	if err := os.WriteFile(path, []byte("allow_cidrs = [\"nope\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadNetworkPolicy(path); err == nil {
		t.Error("invalid policy file should error")
	}
}
