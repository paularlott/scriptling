package plugin

import (
	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
)

// NetworkPolicy is the wire form of the host's outbound network policy. It
// carries exactly the fields of netsecurity.Config, so a Guard rebuilt from it
// enforces the same rules the host applies to requests and websockets,
// including DNS-rebinding protection via validated-IP dialing.
//
// A nil *NetworkPolicy means the host imposes no network restriction.
type NetworkPolicy struct {
	RequireHTTPS    bool     `json:"require_https,omitempty"`
	AllowIPLiterals bool     `json:"allow_ip_literals,omitempty"`
	AllowLoopback   bool     `json:"allow_loopback,omitempty"`
	AllowPrivateIPs bool     `json:"allow_private_ips,omitempty"`
	AllowHosts      []string `json:"allow_hosts,omitempty"`
	DenyHosts       []string `json:"deny_hosts,omitempty"`
	AllowedCIDRs    []string `json:"allowed_cidrs,omitempty"`
	DeniedCIDRs     []string `json:"denied_cidrs,omitempty"`
}

// Policy is the security context a host sends to plugins in the
// scriptling.handshake params. Plugins that understand it (advertised via the
// "policy" capability) enforce it on every operation that opens a file or a
// network connection; plugins that predate it simply ignore the field.
//
// A nil *Policy means no restrictions. A non-nil Policy with nil AllowedPaths
// leaves filesystem access unrestricted; a nil Network leaves network access
// unrestricted. This mirrors the nil-semantics of fssecurity.Config and
// netsecurity.Config.
type Policy struct {
	// AllowedPaths restricts filesystem locations (database files, storage
	// directories) exactly like the fs/pathlib libraries. nil = unrestricted.
	AllowedPaths []string `json:"allowed_paths,omitempty"`
	// Network restricts outbound connections. nil = unrestricted.
	Network *NetworkPolicy `json:"network,omitempty"`
}

// PolicyFromSecurity converts the host-side security configuration into the
// wire Policy form. A nil cfg and nil allowedPaths yield a nil Policy (no
// restrictions); otherwise only non-nil parts are carried.
func PolicyFromSecurity(cfg *netsecurity.Config, allowedPaths []string) *Policy {
	if cfg == nil && allowedPaths == nil {
		return nil
	}
	policy := &Policy{}
	if allowedPaths != nil {
		policy.AllowedPaths = allowedPaths
	}
	if cfg != nil {
		policy.Network = &NetworkPolicy{
			RequireHTTPS:    cfg.RequireHTTPS,
			AllowIPLiterals: cfg.AllowIPLiterals,
			AllowLoopback:   cfg.AllowLoopback,
			AllowPrivateIPs: cfg.AllowPrivateIPs,
			AllowHosts:      cfg.AllowHosts,
			DenyHosts:       cfg.DenyHosts,
			AllowedCIDRs:    cfg.AllowedCIDRs,
			DeniedCIDRs:     cfg.DeniedCIDRs,
		}
	}
	return policy
}

// PathAllowed reports whether path is within the policy's allowed paths. A
// nil policy (or nil AllowedPaths) allows everything, matching fssecurity.
func (p *Policy) PathAllowed(path string) bool {
	if p == nil {
		return true
	}
	cfg := &fssecurity.Config{AllowedPaths: p.AllowedPaths}
	return cfg.IsPathAllowed(path)
}

// NetworkEnabled reports whether the policy carries a network policy at all.
// Callers use this to decide between a guarded dialer and the default one.
func (p *Policy) NetworkEnabled() bool {
	return p != nil && p.Network != nil
}

// Guard returns a netsecurity.Guard enforcing the policy's network rules, or
// (nil, nil) when the network is unrestricted. The returned Guard validates
// every resolved IP at dial time, so connections made through its DialContext
// get the same SSRF and DNS-rebinding protection as host-side requests.
func (p *Policy) Guard() (*netsecurity.Guard, error) {
	if !p.NetworkEnabled() {
		return nil, nil
	}
	cfg := &netsecurity.Config{
		RequireHTTPS:    p.Network.RequireHTTPS,
		AllowIPLiterals: p.Network.AllowIPLiterals,
		AllowLoopback:   p.Network.AllowLoopback,
		AllowPrivateIPs: p.Network.AllowPrivateIPs,
		AllowHosts:      p.Network.AllowHosts,
		DenyHosts:       p.Network.DenyHosts,
		AllowedCIDRs:    p.Network.AllowedCIDRs,
		DeniedCIDRs:     p.Network.DeniedCIDRs,
	}
	return netsecurity.NewGuard(cfg)
}

// StaticPolicy is a PolicySource with a fixed value. It is the in-process
// counterpart of Server.Policy(): compiled-in plugins receive one built from
// the interpreter's security configuration, external plugins read the policy
// the handshake delivered.
type StaticPolicy struct {
	P *Policy
}

// Policy returns the fixed policy.
func (s *StaticPolicy) Policy() *Policy { return s.P }

// PolicySource supplies the effective security policy at call time. The host
// guarantees a plugin's handshake completes before its first function call,
// so external plugins can read Server.Policy() lazily inside connect/open.
type PolicySource interface {
	Policy() *Policy
}
