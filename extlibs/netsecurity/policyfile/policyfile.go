// Package policyfile loads a netsecurity network policy from a TOML file.
// It is separate from netsecurity so the network libraries, which all import
// netsecurity, do not link a TOML parser: only an application that reads
// policy files (such as the scriptling CLI) imports this package.
package policyfile

import (
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
	"github.com/paularlott/scriptling/tomllimit"
)

// policyFile is the TOML schema accepted by LoadConfig.
type policyFile struct {
	HTTPSOnly        bool     `toml:"https_only"`
	AllowIPLiterals  bool     `toml:"allow_ip_literals"`
	AllowLoopback    bool     `toml:"allow_loopback"`
	AllowPrivateIPs  bool     `toml:"allow_private_ips"`
	AllowReservedIPs bool     `toml:"allow_reserved_ips"`
	AllowHosts       []string `toml:"allow_hosts"`
	DenyHosts        []string `toml:"deny_hosts"`
	AllowCIDRs       []string `toml:"allow_cidrs"`
	DenyCIDRs        []string `toml:"deny_cidrs"`
	DNSServers       []string `toml:"dns_servers"`
	// ClientTimeout caps each HTTP request end to end, e.g. "30s" or "2m".
	// Empty leaves requests to their own per-request timeouts.
	ClientTimeout string `toml:"client_timeout"`
}

// LoadConfig reads a TOML policy file. Every key is optional, for example:
//
//	https_only = true
//	allow_hosts = ["api.example.com", ".internal.corp"]
//	allow_cidrs = ["10.1.0.0/16"]
//	dns_servers = ["1.1.1.1", "8.8.8.8:53"]
//	client_timeout = "30s"
//
// The file must parse and compile: an invalid policy is an error, never an
// open policy.
func LoadConfig(path string) (*netsecurity.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f policyFile
	if err := tomllimit.Check(string(data)); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if _, err := toml.Decode(string(data), &f); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	clientTimeout := time.Duration(0)
	if f.ClientTimeout != "" {
		clientTimeout, err = time.ParseDuration(f.ClientTimeout)
		if err != nil || clientTimeout <= 0 {
			return nil, fmt.Errorf("parse %s: client_timeout %q must be a positive duration like \"30s\"", path, f.ClientTimeout)
		}
	}
	cfg := &netsecurity.Config{
		RequireHTTPS:     f.HTTPSOnly,
		AllowIPLiterals:  f.AllowIPLiterals,
		AllowLoopback:    f.AllowLoopback,
		AllowPrivateIPs:  f.AllowPrivateIPs,
		AllowReservedIPs: f.AllowReservedIPs,
		AllowHosts:       f.AllowHosts,
		DenyHosts:        f.DenyHosts,
		AllowedCIDRs:     f.AllowCIDRs,
		DeniedCIDRs:      f.DenyCIDRs,
		DNSServers:       f.DNSServers,
		ClientTimeout:    clientTimeout,
	}
	if _, err := netsecurity.NewGuard(cfg); err != nil {
		return nil, fmt.Errorf("invalid policy in %s: %w", path, err)
	}
	return cfg, nil
}
