package resolve

import (
	"errors"
	"net"
	"testing"

	"github.com/paularlott/scriptling"
)

var errTestLookup = errors.New("lookup failed")

// mockResolver implements Resolver for testing.
type mockResolver struct {
	ips        []string
	ipErr      error
	tcpAddrs   []*net.TCPAddr
	srvErr     error
	srvHTTPURL string
}

func (m *mockResolver) LookupIP(host string) ([]string, error) {
	if m.ipErr != nil {
		return nil, m.ipErr
	}
	return m.ips, nil
}

func (m *mockResolver) LookupSRV(service string) ([]*net.TCPAddr, error) {
	if m.srvErr != nil {
		return nil, m.srvErr
	}
	return m.tcpAddrs, nil
}

func (m *mockResolver) ResolveSRVHttp(uri string) string {
	return m.srvHTTPURL
}

var noopResolver = &mockResolver{}

func TestResolveLibraryRegistration(t *testing.T) {
	p := scriptling.New()
	Register(p, noopResolver)

	_, err := p.Eval(`
import scriptling.net.resolve as resolve
`)
	if err != nil {
		t.Fatalf("Failed to import resolve library: %v", err)
	}
}

func TestLookupIP(t *testing.T) {
	p := scriptling.New()
	Register(p, &mockResolver{ips: []string{"1.2.3.4", "5.6.7.8"}})

	_, err := p.Eval(`
import scriptling.net.resolve as resolve
result = resolve.lookup_ip("example.com")
count = len(result)
first = result[0]
`)
	if err != nil {
		t.Fatalf("Failed to run lookup_ip: %v", err)
	}

	count, _ := p.GetVar("count")
	if count.(int64) != 2 {
		t.Errorf("Expected 2 IPs, got %d", count)
	}

	first, _ := p.GetVar("first")
	if first.(string) != "1.2.3.4" {
		t.Errorf("Expected first IP '1.2.3.4', got %s", first)
	}
}

func TestLookupIPError(t *testing.T) {
	p := scriptling.New()
	Register(p, &mockResolver{ipErr: errTestLookup})

	_, err := p.Eval(`
import scriptling.net.resolve as resolve
try:
    resolve.lookup_ip("nonexistent.invalid")
    error_caught = False
except:
    error_caught = True
`)
	if err != nil {
		t.Fatalf("Failed to run lookup_ip error test: %v", err)
	}

	caught, _ := p.GetVar("error_caught")
	if caught.(bool) != true {
		t.Errorf("Expected error to be caught")
	}
}

func TestLookupSRV(t *testing.T) {
	p := scriptling.New()
	Register(p, &mockResolver{tcpAddrs: []*net.TCPAddr{
		{IP: net.ParseIP("10.0.0.1"), Port: 8080},
		{IP: net.ParseIP("10.0.0.2"), Port: 8081},
	}})

	_, err := p.Eval(`
import scriptling.net.resolve as resolve
addrs = resolve.lookup_srv("_svc._tcp.example.com")
count = len(addrs)
first_ip = addrs[0]["ip"]
first_port = addrs[0]["port"]
`)
	if err != nil {
		t.Fatalf("Failed to run lookup_srv: %v", err)
	}

	count, _ := p.GetVar("count")
	if count.(int64) != 2 {
		t.Errorf("Expected 2 addrs, got %d", count)
	}

	ip, _ := p.GetVar("first_ip")
	if ip.(string) != "10.0.0.1" {
		t.Errorf("Expected first IP '10.0.0.1', got %s", ip)
	}

	port, _ := p.GetVar("first_port")
	if port.(int64) != 8080 {
		t.Errorf("Expected first port 8080, got %d", port)
	}
}

func TestLookupSRVEmpty(t *testing.T) {
	p := scriptling.New()
	Register(p, &mockResolver{srvErr: errTestLookup})

	_, err := p.Eval(`
import scriptling.net.resolve as resolve
try:
    resolve.lookup_srv("_missing._tcp.example.com")
    error_caught = False
except:
    error_caught = True
`)
	if err != nil {
		t.Fatalf("Failed to run lookup_srv error test: %v", err)
	}

	caught, _ := p.GetVar("error_caught")
	if caught.(bool) != true {
		t.Errorf("Expected error to be caught")
	}
}

func TestResolveSRVHttp(t *testing.T) {
	p := scriptling.New()
	Register(p, &mockResolver{srvHTTPURL: "https://api.example.com:8443/v1"})

	_, err := p.Eval(`
import scriptling.net.resolve as resolve
url = resolve.resolve_srv_http("srv+https://api.example.com/v1")
`)
	if err != nil {
		t.Fatalf("Failed to run resolve_srv_http: %v", err)
	}

	url, _ := p.GetVar("url")
	if url.(string) != "https://api.example.com:8443/v1" {
		t.Errorf("Expected resolved URL 'https://api.example.com:8443/v1', got %s", url)
	}
}

func TestResolveSRVHttpPassthrough(t *testing.T) {
	p := scriptling.New()
	Register(p, &mockResolver{srvHTTPURL: "https://api.example.com:8443/v1"})

	_, err := p.Eval(`
import scriptling.net.resolve as resolve
url = resolve.resolve_srv_http("https://plain.example.com/path")
`)
	if err != nil {
		t.Fatalf("Failed to run resolve_srv_http passthrough: %v", err)
	}

	url, _ := p.GetVar("url")
	if url.(string) != "https://api.example.com:8443/v1" {
		t.Errorf("Expected mock URL, got %s", url)
	}
}

func TestLookupIPWrongArgCount(t *testing.T) {
	p := scriptling.New()
	Register(p, noopResolver)

	_, err := p.Eval(`
import scriptling.net.resolve as resolve
try:
    resolve.lookup_ip()
    error_caught = False
except:
    error_caught = True
`)
	if err != nil {
		t.Fatalf("Failed to run wrong arg count test: %v", err)
	}

	caught, _ := p.GetVar("error_caught")
	if caught.(bool) != true {
		t.Errorf("Expected error for missing argument")
	}
}

func TestLookupSRVWrongArgCount(t *testing.T) {
	p := scriptling.New()
	Register(p, noopResolver)

	_, err := p.Eval(`
import scriptling.net.resolve as resolve
try:
    resolve.lookup_srv()
    error_caught = False
except:
    error_caught = True
`)
	if err != nil {
		t.Fatalf("Failed to run wrong arg count test: %v", err)
	}

	caught, _ := p.GetVar("error_caught")
	if caught.(bool) != true {
		t.Errorf("Expected error for missing argument")
	}
}

func TestResolveSRVHttpWrongArgCount(t *testing.T) {
	p := scriptling.New()
	Register(p, noopResolver)

	_, err := p.Eval(`
import scriptling.net.resolve as resolve
try:
    resolve.resolve_srv_http()
    error_caught = False
except:
    error_caught = True
`)
	if err != nil {
		t.Fatalf("Failed to run wrong arg count test: %v", err)
	}

	caught, _ := p.GetVar("error_caught")
	if caught.(bool) != true {
		t.Errorf("Expected error for missing argument")
	}
}
