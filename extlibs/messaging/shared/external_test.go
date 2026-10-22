package shared

import (
	"net/http"
	"testing"
	"time"

	"github.com/paularlott/scriptling/extlibs/netsecurity"
)

func TestExternalURLsIsPlatform(t *testing.T) {
	e := NewExternalURLs([]string{"slack.com"}, time.Second)
	for raw, want := range map[string]bool{
		"https://slack.com/x":             true,
		"https://files.slack.com/x":       true,
		"https://FILES.Slack.com./x":      true,
		"https://evilslack.com/x":         false,
		"https://slack.com.evil.test/x":   false,
		"https://attacker.test/slack.com": false,
		"http://127.0.0.1/x":              false,
		"://bad":                          false,
	} {
		if got := e.IsPlatform(raw); got != want {
			t.Errorf("IsPlatform(%q) = %v, want %v", raw, got, want)
		}
	}
}

func TestExternalURLsClient(t *testing.T) {
	platform := &http.Client{}

	// No policy: every URL uses the platform client.
	open := NewExternalURLs([]string{"slack.com"}, time.Second)
	if open.Client("http://127.0.0.1/", platform) != platform {
		t.Error("without a policy, foreign hosts should use the platform client")
	}

	// Policy: platform hosts stay on the platform client, others are guarded.
	guarded := NewExternalURLs([]string{"slack.com"}, time.Second, &netsecurity.Config{})
	if guarded.Client("https://files.slack.com/x", platform) != platform {
		t.Error("platform hosts should use the platform client under a policy")
	}
	c := guarded.Client("http://127.0.0.1/", platform)
	if c == platform {
		t.Fatal("foreign hosts must use the guarded client under a policy")
	}
	if c.Timeout != time.Second {
		t.Errorf("guarded client timeout = %v, want 1s", c.Timeout)
	}
}
