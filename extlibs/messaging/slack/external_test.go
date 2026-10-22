package slack

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
)

// recorder serves a fixed body and records the Authorization header of
// every request it receives.
type recorder struct {
	mu    sync.Mutex
	auths []string
}

func (r *recorder) handler(w http.ResponseWriter, req *http.Request) {
	r.mu.Lock()
	r.auths = append(r.auths, req.Header.Get("Authorization"))
	r.mu.Unlock()
	w.Write([]byte("file-data"))
}

func (r *recorder) seen() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.auths...)
}

// usePlatformDomains makes the test server's "localhost" name a Slack host.
func usePlatformDomains(t *testing.T, domains ...string) {
	t.Helper()
	old := platformDomains
	platformDomains = domains
	t.Cleanup(func() { platformDomains = old })
}

func evalSlack(t *testing.T, cfg *netsecurity.Config, script string) (string, error) {
	t.Helper()
	p := scriptling.New()
	if cfg != nil {
		Register(p, nil, cfg)
	} else {
		Register(p, nil)
	}
	result, err := p.Eval("import scriptling.messaging.slack as slack\nc = slack.client(\"xoxb-secret\", \"xapp-secret\")\n" + script)
	if err != nil {
		return "", err
	}
	return result.Inspect(), nil
}

func TestSlackDownloadTokenOnlyToPlatform(t *testing.T) {
	usePlatformDomains(t, "localhost")
	rec := &recorder{}
	srv := httptest.NewServer(http.HandlerFunc(rec.handler))
	defer srv.Close()
	byName := strings.Replace(srv.URL, "127.0.0.1", "localhost", 1)

	if _, err := evalSlack(t, nil, `slack.download(c, "`+srv.URL+`/f")`); err != nil {
		t.Fatalf("foreign download: %v", err)
	}
	if _, err := evalSlack(t, nil, `slack.download(c, "`+byName+`/f")`); err != nil {
		t.Fatalf("platform download: %v", err)
	}
	got := rec.seen()
	if len(got) != 2 || got[0] != "" || got[1] != "Bearer xoxb-secret" {
		t.Errorf("Authorization headers = %q, want [\"\" \"Bearer xoxb-secret\"]", got)
	}
}

func TestSlackNetworkPolicy(t *testing.T) {
	usePlatformDomains(t, "localhost")
	rec := &recorder{}
	srv := httptest.NewServer(http.HandlerFunc(rec.handler))
	defer srv.Close()
	byName := strings.Replace(srv.URL, "127.0.0.1", "localhost", 1)

	// Foreign hosts are denied by the policy, for downloads and responses.
	for _, script := range []string{
		`slack.download(c, "` + srv.URL + `/f")`,
		`slack.answer_callback(c, "id", "` + srv.URL + `/respond", "done")`,
	} {
		if _, err := evalSlack(t, &netsecurity.Config{}, script); err == nil || !strings.Contains(err.Error(), "network policy") {
			t.Errorf("%s: expected network policy error, got: %v", script, err)
		}
	}
	if n := len(rec.seen()); n != 0 {
		t.Fatalf("POLICY-BYPASS [slack]: denied requests reached the server %d times", n)
	}

	// Platform hosts are trusted even though the policy denies loopback.
	if _, err := evalSlack(t, &netsecurity.Config{}, `slack.download(c, "`+byName+`/f")`); err != nil {
		t.Errorf("platform download under policy: %v", err)
	}
}
