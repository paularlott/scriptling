package shared

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/paularlott/scriptling/extlibs/netsecurity"
)

// ExternalURLs decides how a client fetches a URL handed to it by a script
// (download refs, Slack response URLs). The platform's own hosts are trusted:
// they get the bot credential and the platform client. Any other host gets no
// credential and, when a network policy is configured, the guarded client.
type ExternalURLs struct {
	domains []string
	guarded *http.Client // nil = no network policy
}

// NewExternalURLs builds the rule for a platform whose hosts are the given
// domains and their subdomains. cfg is the optional network policy.
func NewExternalURLs(domains []string, timeout time.Duration, cfg ...*netsecurity.Config) *ExternalURLs {
	e := &ExternalURLs{domains: domains}
	if len(cfg) > 0 && cfg[0] != nil {
		guard, err := netsecurity.NewGuard(cfg[0])
		if err != nil {
			guard = netsecurity.FailClosed(err)
		}
		client := guard.HTTPClient()
		if client.Timeout == 0 || client.Timeout > timeout {
			client.Timeout = timeout
		}
		e.guarded = client
	}
	return e
}

// IsPlatform reports whether rawURL's host is one of the platform's domains
// or a subdomain of one. Unparseable URLs are not platform URLs.
func (e *ExternalURLs) IsPlatform(rawURL string) bool {
	if e == nil {
		return false
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.TrimRight(strings.ToLower(u.Hostname()), ".")
	for _, d := range e.domains {
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}

// Client returns the client for rawURL: platform for platform hosts, the
// guarded client for other hosts when a policy is configured.
func (e *ExternalURLs) Client(rawURL string, platform *http.Client) *http.Client {
	if e == nil || e.guarded == nil || e.IsPlatform(rawURL) {
		return platform
	}
	return e.guarded
}
