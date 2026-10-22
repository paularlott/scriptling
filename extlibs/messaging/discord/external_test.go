package discord

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
)

func TestDiscordDownloadTokenAndPolicy(t *testing.T) {
	old := platformDomains
	platformDomains = []string{"localhost"}
	t.Cleanup(func() { platformDomains = old })

	var mu sync.Mutex
	var auths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		auths = append(auths, r.Header.Get("Authorization"))
		mu.Unlock()
		w.Write([]byte("file-data"))
	}))
	defer srv.Close()
	byName := strings.Replace(srv.URL, "127.0.0.1", "localhost", 1)

	eval := func(cfg *netsecurity.Config, url string) error {
		p := scriptling.New()
		if cfg != nil {
			Register(p, nil, cfg)
		} else {
			Register(p, nil)
		}
		_, err := p.Eval(`import scriptling.messaging.discord as discord
c = discord.client("bot-secret")
discord.download(c, "` + url + `")`)
		return err
	}

	// Under a policy a foreign host is denied before any request is made.
	if err := eval(&netsecurity.Config{}, srv.URL+"/f"); err == nil || !strings.Contains(err.Error(), "network policy") {
		t.Errorf("expected network policy error, got: %v", err)
	}
	// Without a policy it is fetched, but without the token; platform hosts
	// get the token and are trusted under a policy.
	for _, tc := range []struct {
		cfg *netsecurity.Config
		url string
	}{{nil, srv.URL + "/f"}, {&netsecurity.Config{}, byName + "/f"}} {
		if err := eval(tc.cfg, tc.url); err != nil {
			t.Errorf("download %s: %v", tc.url, err)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(auths) != 2 || auths[0] != "" || auths[1] != "Bot bot-secret" {
		t.Errorf("Authorization headers = %q, want [\"\" \"Bot bot-secret\"]", auths)
	}
}
