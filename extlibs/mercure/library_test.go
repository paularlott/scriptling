package mercure

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/paularlott/gossiper"
	"github.com/paularlott/gossiper/auth"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
	"github.com/paularlott/scriptling/stdlib"
)

const testKey = "mercure-test-key-0123456789abcdef0123456789"

// testHub is an in-process gossiper hub, over HTTP/1.1 and HTTP/2 without TLS.
type testHub struct {
	url     string
	streams atomic.Int64 // streaming publishes opened
	forms   atomic.Int64 // form POSTs
	query   atomic.Value // the query of the last subscription
	gets    atomic.Int64 // subscriptions opened (and reopened)
}

func newHub(t testing.TB, mod func(*gossiper.Config)) *testHub {
	t.Helper()
	return newHubWith(t, mod, false)
}

// newHubWith is newHub, refusing streaming publishes as any other Mercure hub
// does (it reads the body as a form) when noStreams is set.
func newHubWith(t testing.TB, mod func(*gossiper.Config), noStreams bool) *testHub {
	t.Helper()
	cfg := gossiper.Config{PublisherKey: auth.KeyConfig{Key: testKey, Algorithm: "HS256"},
		TrustedIssuers: []string{"https://localhost"}}
	if mod != nil {
		mod(&cfg)
	}
	h, err := gossiper.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	th := &testHub{}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			th.query.Store(r.URL.RawQuery)
			th.gets.Add(1)
		}
		if r.Method == http.MethodPost {
			if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-ndjson") {
				th.streams.Add(1)
				if noStreams {
					http.Error(w, `Missing "topic" parameter`, http.StatusBadRequest)
					return
				}
			} else {
				th.forms.Add(1)
			}
		}
		h.ServeHTTP(w, r)
	}))
	srv.Config.Protocols = new(http.Protocols)
	srv.Config.Protocols.SetHTTP1(true)
	srv.Config.Protocols.SetUnencryptedHTTP2(true)
	srv.Start()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = h.Close(ctx)
		srv.Close()
	})
	th.url = srv.URL + "/.well-known/mercure"
	return th
}

func run(t *testing.T, register func(*scriptling.Scriptling), script string) (string, error) {
	t.Helper()
	p := scriptling.New()
	stdlib.RegisterAll(p)
	register(p)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	result, err := p.EvalWithContext(ctx, "import scriptling.mercure as mercure\n"+script)
	if err != nil {
		return "", err
	}
	return result.Inspect(), nil
}

func plain(p *scriptling.Scriptling) { Register(p) }

// prelude defines hub (a client with a token for every topic) for a script.
func prelude(h *testHub, clientArgs string) string {
	return `KEY = "` + testKey + `"
TOKEN = mercure.create_token(KEY, "` + h.url + `", publish=["*"], subscribe=["*"])
hub = mercure.Client("` + h.url + `", token=TOKEN` + clientArgs + `)
`
}

func TestPublishSubscribeDefaultHTTP(t *testing.T) {
	h := newHub(t, nil)
	out, err := run(t, plain, prelude(h, "")+`
sub = hub.subscribe(match_urlpattern="https://example.com/books/:id")
id1 = hub.publish("https://example.com/books/1", {"title": "Dune", "n": 1}, id="urn:book:1", type="book", retry=1.5)
id2 = hub.publish(["https://example.com/books/2", "https://example.com/alt"], "line 1\nline 2", private=True)
e1 = sub.receive(timeout=10)
e2 = sub.receive(timeout=10)
none = sub.receive(timeout=0.2)
result = [hub.transport, id1, e1.id, e1.type, e1.retry, e1.json()["title"], e1.json()["n"], e2.id == id2, e2.data, e2.type, e2.retry, none, sub.last_event_id == id2]
hub.close()
result
`)
	if err != nil {
		t.Fatal(err)
	}
	want := `['http', 'urn:book:1', 'urn:book:1', 'book', 1.5, 'Dune', 1, True, 'line 1\nline 2', None, None, None, True]`
	if out != want {
		t.Errorf("got  %s\nwant %s", out, want)
	}
	if h.streams.Load() != 0 || h.forms.Load() != 2 {
		t.Errorf("%d streams, %d form POSTs: the default is a POST per update", h.streams.Load(), h.forms.Load())
	}
}

func TestStreamPublishMany(t *testing.T) {
	for _, mode := range []string{"True", `"auto"`} {
		t.Run(mode, func(t *testing.T) {
			h := newHub(t, nil)
			out, err := run(t, plain, prelude(h, ", stream="+mode)+`
sub = hub.subscribe(["https://example.com/n"], buffer=2000)
ids = hub.publish_many([{"topic": "https://example.com/n", "data": str(i)} for i in range(1000)])
got = []
for ev in sub:
    got.append(ev.data)
    if len(got) == 1000:
        break
hub.publish("https://example.com/n", "last")
last = sub.receive(timeout=5).data
sub.close()
[hub.transport, len(ids), len(set(ids)), got == [str(i) for i in range(1000)], last, sub.receive(timeout=5)]
`)
			if err != nil {
				t.Fatal(err)
			}
			if want := `['stream', 1000, 1000, True, 'last', None]`; out != want {
				t.Errorf("got  %s\nwant %s", out, want)
			}
			if h.streams.Load() == 0 || h.forms.Load() != 0 {
				t.Errorf("%d streams, %d form POSTs", h.streams.Load(), h.forms.Load())
			}
		})
	}
}

func TestPublishManyHTTPKeepsOrder(t *testing.T) {
	h := newHub(t, nil)
	out, err := run(t, plain, prelude(h, "")+`
sub = hub.subscribe("https://example.com/n")
ids = hub.publish_many([{"topic": "https://example.com/n", "data": {"i": i}} for i in range(20)])
got = [sub.receive(timeout=10).json()["i"] for _ in range(20)]
got == list(range(20)) and len(ids) == 20
`)
	if err != nil || out != "True" {
		t.Fatalf("%s %v", out, err)
	}
	if h.forms.Load() != 20 {
		t.Errorf("%d form POSTs", h.forms.Load())
	}
}

// With stream="auto", a hub without streams gets a POST per update, one after
// another: the updates keep their order.
func TestAutoWithoutStreamsKeepsOrder(t *testing.T) {
	h := newHubWith(t, nil, true)
	out, err := run(t, plain, prelude(h, `, stream="auto"`)+`
sub = hub.subscribe("https://example.com/n", buffer=500)
ids = hub.publish_many([{"topic": "https://example.com/n", "data": str(i)} for i in range(300)])
got = [sub.receive(timeout=10).data for _ in range(300)]
[hub.transport, len(ids), got == [str(i) for i in range(300)]]
`)
	if err != nil || out != `['http', 300, True]` {
		t.Fatalf("%s %v", out, err)
	}
	if h.forms.Load() != 300 {
		t.Errorf("%d form POSTs", h.forms.Load())
	}
}

func TestH2C(t *testing.T) {
	h := newHub(t, nil)
	out, err := run(t, plain, prelude(h, ", stream=True, h2c=True")+`
sub = hub.subscribe("https://example.com/h")
hub.publish("https://example.com/h", "x")
sub.receive(timeout=10).data
`)
	if err != nil || out != `x` {
		t.Fatalf("%s %v", out, err)
	}
}

func TestErrors(t *testing.T) {
	h := newHub(t, nil)
	for name, tc := range map[string]struct{ script, want string }{
		"publish beyond the token's grants": {`
tok = mercure.create_token(KEY, hub.url, publish=["https://example.com/mine"])
c = mercure.Client(hub.url, token=tok)
try:
    c.publish("https://example.com/other", "x")
    r = "published"
except Exception as e:
    r = "Error: " + str(e)
r`, "Error: mercure: 403"},
		"anonymous publish": {`
try:
    mercure.Client(hub.url).publish("https://example.com/x")
    r = "published"
except Exception as e:
    r = "Error: " + str(e)
r`, "Error: mercure: 401"},
		"anonymous subscribe refused": {`
try:
    mercure.Client(hub.url).subscribe("*")
    r = "subscribed"
except Exception as e:
    r = "Error: " + str(e)
r`, "Error: mercure: 401"},
		"no topic": {`
try:
    hub.publish([])
except ValueError as e:
    r = str(e)
r`, "an update needs a topic"},
		"no topic to subscribe": {`
try:
    hub.subscribe()
except ValueError as e:
    r = str(e)
r`, "at least one topic"},
		"bad stream": {`
try:
    mercure.Client(hub.url, stream="sometimes")
except ValueError as e:
    r = str(e)
r`, "stream must be"},
		"bad url": {`
try:
    mercure.Client("ftp://example.com/hub")
except ValueError as e:
    r = str(e)
r`, "http or https"},
		"unknown publish argument": {`
try:
    hub.publish("https://example.com/x", colour="blue")
except ValueError as e:
    r = str(e)
r`, "unexpected argument"},
		"publish_many names the failure": {`
tok = mercure.create_token(KEY, hub.url, publish=["https://example.com/mine"])
c = mercure.Client(hub.url, token=tok, stream=True)
try:
    c.publish_many([{"topic": "https://example.com/mine"}, {"topic": "https://example.com/other"}])
except Exception as e:
    r = str(e)
r`, "publish_many: update 1: mercure: 403"},
		"closed client": {`
c = mercure.Client(hub.url, token=TOKEN)
c.close()
try:
    c.publish("https://example.com/x")
except Exception as e:
    r = str(e)
[c.closed, r]`, `[True, 'mercure: client closed']`},
	} {
		t.Run(name, func(t *testing.T) {
			out, err := run(t, plain, prelude(h, "")+tc.script)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out, tc.want) {
				t.Errorf("got %s, want %s", out, tc.want)
			}
		})
	}
}

func TestPrivateUpdates(t *testing.T) {
	h := newHub(t, func(c *gossiper.Config) { c.Anonymous = true })
	out, err := run(t, plain, prelude(h, "")+`
allowed = mercure.Client(hub.url, token=mercure.create_token(KEY, hub.url, subscribe=["https://example.com/p"]))
anon = mercure.Client(hub.url)
s1 = allowed.subscribe("https://example.com/p")
s2 = anon.subscribe("https://example.com/p")
hub.publish("https://example.com/p", "secret", private=True)
hub.publish("https://example.com/p", "public")
[s1.receive(timeout=10).data, s2.receive(timeout=10).data]
`)
	if err != nil || out != `['secret', 'public']` {
		t.Fatalf("%s %v", out, err)
	}
}

func TestResumeFromLastEventID(t *testing.T) {
	h := newHub(t, nil)
	out, err := run(t, plain, prelude(h, "")+`
first = hub.publish("https://example.com/r", "1")
hub.publish("https://example.com/r", "2")
hub.publish("https://example.com/r", "3")
sub = hub.subscribe("https://example.com/r", last_event_id=first)
[sub.receive(timeout=10).data, sub.receive(timeout=10).data]
`)
	if err != nil || out != `['2', '3']` {
		t.Fatalf("%s %v", out, err)
	}
}

func TestWithBlocksClose(t *testing.T) {
	h := newHub(t, nil)
	out, err := run(t, plain, prelude(h, "")+`
with mercure.Client(hub.url, token=TOKEN) as c:
    with c.subscribe("https://example.com/w") as sub:
        c.publish("https://example.com/w", "x")
        got = sub.receive(timeout=10).data
    after = sub.receive(timeout=1)
[got, after, c.closed]
`)
	if err != nil || out != `['x', None, True]` {
		t.Fatalf("%s %v", out, err)
	}
}

func TestEventHelpers(t *testing.T) {
	h := newHub(t, nil)
	out, err := run(t, plain, prelude(h, "")+`
sub = hub.subscribe("https://example.com/e")
hub.publish("https://example.com/e", "not json", id="urn:e")
ev = sub.receive(timeout=10)
try:
    ev.json()
    bad = "parsed"
except ValueError:
    bad = "ValueError"
[ev.to_dict(), bad, repr(ev)]
`)
	want := `[{'id': 'urn:e', 'type': None, 'data': 'not json', 'retry': None}, 'ValueError', 'Event(id="urn:e", data="not json")']`
	if err != nil || out != want {
		t.Fatalf("got  %s\nwant %s\n%v", out, want, err)
	}
}

func TestCreateToken(t *testing.T) {
	out, err := run(t, plain, `mercure.create_token("k", publish="https://example.com/a", subscribe=[], payload={"user": "u1"}, expires=60, subject="me", draft07=True)`)
	if err != nil {
		t.Fatal(err)
	}
	tok := out
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatalf("token %q", tok)
	}
	mac := hmac.New(sha256.New, []byte("k"))
	mac.Write([]byte(parts[0] + "." + parts[1]))
	if base64.RawURLEncoding.EncodeToString(mac.Sum(nil)) != parts[2] {
		t.Error("bad signature")
	}
	raw, _ := base64.RawURLEncoding.DecodeString(parts[1])
	var claims struct {
		Mercure struct {
			Publish   []string       `json:"publish"`
			Subscribe []string       `json:"subscribe"`
			Payload   map[string]any `json:"payload"`
		} `json:"mercure"`
		Exp int64  `json:"exp"`
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(raw, &claims); err != nil {
		t.Fatal(err)
	}
	if len(claims.Mercure.Publish) != 1 || claims.Mercure.Publish[0] != "https://example.com/a" ||
		claims.Mercure.Subscribe == nil || len(claims.Mercure.Subscribe) != 0 || claims.Mercure.Payload["user"] != "u1" ||
		claims.Sub != "me" {
		t.Errorf("claims %s", raw)
	}
	if d := time.Until(time.Unix(claims.Exp, 0)); d < 50*time.Second || d > 70*time.Second {
		t.Errorf("expires in %v", d)
	}
	for script, want := range map[string]string{
		`mercure.create_token("k", "https://a")`:                                   "publish or subscribe",
		`mercure.create_token("", "https://a", publish=["*"])`:                     "key is empty",
		`mercure.create_token("k", "https://a", publish=["*"], algorithm="RS256")`: "HS256, HS384 or HS512",
	} {
		if _, err := run(t, plain, script); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v, want %q", script, err, want)
		}
	}
	out, err = run(t, plain, `len(mercure.create_token("k", subscribe=["*"], expires=0, algorithm="HS512", draft07=True).split("."))`)
	if err != nil || out != "3" {
		t.Errorf("HS512: %s %v", out, err)
	}
}

// Mercure 1.0 access tokens and matchers, as hubs in Mercure 1.0 mode take them.
func TestMercure1AccessTokens(t *testing.T) {
	h := newHub(t, func(c *gossiper.Config) { c.TrustedIssuers = []string{"https://localhost"} })
	out, err := run(t, plain, `
KEY = "`+testKey+`"
URL = "`+h.url+`"
def token(**grants):
    return mercure.create_token(KEY, URL, **grants)
hub = mercure.Client(URL, token=token(publish=["*"], subscribe=["*"]))
books = hub.subscribe(match_urlpattern="https://example.com/books/:id")
exact = mercure.Client(URL, token=token(subscribe=["https://example.com/inbox/a"])).subscribe(match="https://example.com/inbox/a")
hub.publish("https://example.com/books/1", "b1")
hub.publish("https://example.com/inbox/a", "private", private=True)
wrong = mercure.Client(URL, token=mercure.create_token(KEY, URL, publish=["*"], issuer="https://other"))
try:
    wrong.publish("https://example.com/books/2")
    refused = False
except Exception as e:
    refused = "401" in str(e)
[books.receive(timeout=10).data, exact.receive(timeout=10).data, refused]
`)
	if err != nil || out != `['b1', 'private', True]` {
		t.Fatalf("%s %v", out, err)
	}
	for script, want := range map[string]string{
		`mercure.create_token("k", publish=["*"])`:                                        "needs the hub's URL as its audience",
		`mercure.create_token("k", "https://a", publish=["*"], expires=0)`:                "must expire",
		`mercure.create_token("k", "https://a", publish=["*"], match_type="x")`:           "match_type",
		`mercure.create_token("k", "https://a", publish=["*"], issuer="")`:                "issuer must not be empty",
		`mercure.create_token("k", "https://a", publish=["*"], audience="https://b")`:     "both by position and by name",
		`mercure.create_token("k", "https://a", publish=["*"], draft07=True)`:             "has no audience",
		`mercure.create_token("k", publish=["*"], draft07=True, issuer="https://i")`:      "for Mercure 1.0 tokens, not draft-07",
		`mercure.create_token("k", publish=["*"], draft07=True, match_type="urlpattern")`: "for Mercure 1.0 tokens, not draft-07",
	} {
		if _, err := run(t, plain, script); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v, want %q", script, err, want)
		}
	}
	out, err = run(t, plain, `mercure.create_token("k", publish=["*"], subscribe=["https://example.com/:id"], payload={"u": 1}, issuer="https://i", audience="https://a", match_type="urlpattern", client_id="c1")`)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(out, ".")
	head, _ := base64.RawURLEncoding.DecodeString(parts[0])
	raw, _ := base64.RawURLEncoding.DecodeString(parts[1])
	var claims map[string]any
	if err := json.Unmarshal(raw, &claims); err != nil {
		t.Fatal(err)
	}
	details, _ := json.Marshal(claims["authorization_details"])
	if !strings.Contains(string(head), `"typ":"at+jwt"`) || claims["iss"] != "https://i" || claims["aud"] != "https://a" ||
		claims["client_id"] != "c1" || claims["sub"] != "c1" || claims["jti"] == "" || claims["exp"] == nil ||
		string(details) != `[{"actions":["publish"],"topics":[{"match":"*","match_type":"urlpattern"}],"type":"https://mercure.rocks/authorization-detail"},{"actions":["subscribe"],"payload":{"u":1},"topics":[{"match":"https://example.com/:id","match_type":"urlpattern"}],"type":"https://mercure.rocks/authorization-detail"}]` {
		t.Errorf("header %s, claims %s", head, raw)
	}
}

// Topics given to subscribe() by position are Mercure 1.0 exact matchers;
// topics= are draft-07 selectors.
func TestSubscribeParameters(t *testing.T) {
	h := newHub(t, func(c *gossiper.Config) { c.Protocol = gossiper.ProtocolBoth }) // topic= too
	for script, want := range map[string]string{
		`hub.subscribe("https://example.com/a", "https://example.com/b")`:         "match=https%3A%2F%2Fexample.com%2Fa&match=https%3A%2F%2Fexample.com%2Fb",
		`hub.subscribe(["https://example.com/a"], match="https://example.com/b")`: "match=https%3A%2F%2Fexample.com%2Fa&match=https%3A%2F%2Fexample.com%2Fb",
		`hub.subscribe(match_urlpattern="https://example.com/:id")`:               "match_urlpattern=https%3A%2F%2Fexample.com%2F%3Aid",
		`hub.subscribe(topics="https://example.com/{id}")`:                        "topic=https%3A%2F%2Fexample.com%2F%7Bid%7D",
	} {
		if _, err := run(t, plain, prelude(h, "")+script); err != nil {
			t.Fatalf("%s: %v", script, err)
		}
		if got, _ := h.query.Load().(string); got != want {
			t.Errorf("%s: query %q, want %q", script, got, want)
		}
	}
}

// Draft-07, opted into, against a hub that takes only draft-07. (A hub's
// default, as the reference hub's, is Mercure 1.0 only.)
func TestDraft07Hub(t *testing.T) {
	h := newHub(t, func(c *gossiper.Config) { c.Protocol = gossiper.ProtocolLegacy })
	out, err := run(t, plain, `
KEY = "`+testKey+`"
hub = mercure.Client("`+h.url+`", token=mercure.create_token(KEY, publish=["*"], subscribe=["*"], draft07=True))
sub = hub.subscribe(topics="https://example.com/books/{id}")
hub.publish("https://example.com/books/1", "dune")
modern = mercure.Client("`+h.url+`", token=mercure.create_token(KEY, "`+h.url+`", publish=["*"]))
try:
    modern.publish("https://example.com/books/2")
    refused = False
except Exception as e:
    refused = "401" in str(e)
try:
    hub.subscribe("https://example.com/books/1")
    match_refused = False
except Exception as e:
    match_refused = "400" in str(e)
[sub.receive(timeout=10).data, refused, match_refused]
`)
	if err != nil || out != `['dune', True, True]` {
		t.Fatalf("%s %v", out, err)
	}
}

func TestNetworkPolicy(t *testing.T) {
	h := newHub(t, nil)
	blocked := func(p *scriptling.Scriptling) { Register(p, &netsecurity.Config{AllowIPLiterals: true}) }
	_, err := run(t, blocked, `mercure.Client("`+h.url+`")`)
	if err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Errorf("policy without loopback: %v", err)
	}
	allowed := func(p *scriptling.Scriptling) {
		Register(p, &netsecurity.Config{AllowLoopback: true, AllowIPLiterals: true})
	}
	out, err := run(t, allowed, prelude(h, "")+`
sub = hub.subscribe("https://example.com/p")
hub.publish("https://example.com/p", "ok")
sub.receive(timeout=10).data`)
	if err != nil || out != `ok` {
		t.Errorf("policy allowing it: %s %v", out, err)
	}
}

func TestCABundleAllowedPaths(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(outside, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(dir, "ca.pem")
	if err := os.WriteFile(inside, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	confined := func(p *scriptling.Scriptling) {
		RegisterConfigured(p, fssecurity.Config{AllowedPaths: []string{dir}}, nil)
	}
	if _, err := run(t, confined, `mercure.Client("https://hub.example/.well-known/mercure", verify="`+outside+`")`); err == nil || !strings.Contains(err.Error(), "outside allowed") {
		t.Errorf("CA bundle outside the allowed paths: %v", err)
	}
	if _, err := run(t, confined, `mercure.Client("https://hub.example/.well-known/mercure", verify="`+inside+`")`); err == nil || !strings.Contains(err.Error(), "no PEM certificates") {
		t.Errorf("CA bundle without certificates: %v", err)
	}
}

func TestTLSHub(t *testing.T) {
	cfg := gossiper.Config{PublisherKey: auth.KeyConfig{Key: testKey, Algorithm: "HS256"},
		TrustedIssuers: []string{"https://localhost"}}
	g, err := gossiper.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewTLSServer(g)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = g.Close(ctx)
		srv.Close()
	})
	url := srv.URL + "/.well-known/mercure"
	if _, err := run(t, plain, `mercure.Client("`+url+`", token=mercure.create_token("`+testKey+`", "`+url+`", publish=["*"])).publish("t")`); err == nil || !strings.Contains(err.Error(), "certificate") {
		t.Errorf("unverified certificate: %v", err)
	}
	out, err := run(t, plain, `
c = mercure.Client("`+url+`", token=mercure.create_token("`+testKey+`", "`+url+`", publish=["*"], subscribe=["*"]), verify=False, stream=True)
sub = c.subscribe("t")
c.publish("t", "secure")
sub.receive(timeout=10).data`)
	if err != nil || out != `secure` {
		t.Errorf("verify=False: %s %v", out, err)
	}
}

// A runtime reset closes the clients scripts left open.
func TestResetClosesClients(t *testing.T) {
	h := newHub(t, nil)
	if _, err := run(t, plain, prelude(h, "")+`sub = hub.subscribe("t")`); err != nil {
		t.Fatal(err)
	}
	openClients.Lock()
	n := len(openClients.m)
	openClients.Unlock()
	if n == 0 {
		t.Fatal("no open client")
	}
	extlibs.ResetRuntime()
	openClients.Lock()
	n = len(openClients.m)
	openClients.Unlock()
	if n != 0 {
		t.Errorf("%d clients open after a reset", n)
	}
}

// A script stopped while it waits for an event stops at once.
func TestReceiveStopsWithTheRun(t *testing.T) {
	h := newHub(t, nil)
	p := scriptling.New()
	stdlib.RegisterAll(p)
	Register(p)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := p.EvalWithContext(ctx, "import scriptling.mercure as mercure\n"+prelude(h, "")+`
sub = hub.subscribe("t")
for ev in sub:
    pass
`)
	if err == nil || time.Since(start) > 5*time.Second {
		t.Errorf("after %v: %v", time.Since(start), err)
	}
}

// A minted token is reused until half its life is gone, then replaced.
func TestTokenSourceRenews(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	src := newMintingSource(&tokenSpec{key: []byte(testKey), algorithm: "HS256", issuer: "https://localhost",
		audience: "https://hub", clientID: "c", matchType: "exact", publish: []string{"*"}, ttl: time.Hour})
	src.now = func() time.Time { return now }
	first, err := src.get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(29 * time.Minute)
	if again, _ := src.get(context.Background()); again != first {
		t.Error("renewed before half its life")
	}
	now = now.Add(2 * time.Minute)
	renewed, _ := src.get(context.Background())
	if renewed == first {
		t.Error("not renewed after half its life")
	}
	raw, _ := base64.RawURLEncoding.DecodeString(strings.Split(renewed, ".")[1])
	var claims struct{ Exp, Iat int64 }
	if err := json.Unmarshal(raw, &claims); err != nil || claims.Exp-claims.Iat != 3600 || claims.Iat != now.Unix() {
		t.Errorf("claims %s %v", raw, err)
	}
	if err := src.set("x"); err == nil {
		t.Error("set_token on a minting client")
	}
	draft := newMintingSource(&tokenSpec{key: []byte(testKey), algorithm: "HS256", draft07: true, publish: []string{"*"}})
	a, _ := draft.get(context.Background())
	draft.now = func() time.Time { return time.Now().Add(1000 * time.Hour) }
	if b, _ := draft.get(context.Background()); a != b {
		t.Error("a token without expiry was renewed")
	}
}

// With key=, a client keeps publishing and receiving across many token
// lifetimes, over a stream and with POSTs: the hub ends each stream as its
// token expires, and the client reopens it with a fresh one, losing nothing.
func TestKeyRenewsAcrossExpiry(t *testing.T) {
	minTokenTTL = time.Second
	t.Cleanup(func() { minTokenTTL = 10 * time.Second })
	for _, mode := range []string{"False", "True"} {
		t.Run("stream="+mode, func(t *testing.T) {
			h := newHub(t, nil)
			out, err := run(t, plain, `
import time
hub = mercure.Client("`+h.url+`", key="`+testKey+`", token_ttl=2, stream=`+mode+`)
sub = hub.subscribe("https://example.com/r", buffer=10000)
sent = 0
start = time.time()
while time.time() - start < 7:
    hub.publish_many([{"topic": "https://example.com/r", "data": str(sent + i)} for i in range(20)])
    sent += 20
    time.sleep(0.05)
got = []
while len(got) < sent:
    ev = sub.receive(timeout=10)
    if ev is None:
        break
    got.append(ev.data)
[sent > 0, got == [str(i) for i in range(sent)], len(got) == sent]
`)
			if err != nil || out != `[True, True, True]` {
				t.Fatalf("%s %v", out, err)
			}
			// 7s of 2s tokens: the hub ended the subscription (and the publish stream) as each expired.
			if h.gets.Load() < 3 || (mode == "True" && h.streams.Load() < 3) {
				t.Errorf("%d subscriptions and %d publish streams opened: the tokens did not expire on the way",
					h.gets.Load(), h.streams.Load())
			}
			t.Logf("%d subscriptions, %d publish streams, %d POSTs", h.gets.Load(), h.streams.Load(), h.forms.Load())
		})
	}
}

// A token given with token= is presented as it is: once it expires the hub
// refuses it, and the error says what to do; set_token() replaces it.
func TestStaticTokenExpiryAndSetToken(t *testing.T) {
	h := newHub(t, nil)
	out, err := run(t, plain, `
import time
URL = "`+h.url+`"
KEY = "`+testKey+`"
short = mercure.Client(URL, token=mercure.create_token(KEY, URL, publish=["*"], subscribe=["*"], expires=2))
sub = short.subscribe("https://example.com/s")
time.sleep(3)
try:
    short.publish("https://example.com/s", "late")
    pub_err = ""
except Exception as e:
    pub_err = str(e)
try:
    while True:
        if sub.receive(timeout=5) is None:
            break
    sub_err = ""
except Exception as e:
    sub_err = str(e)

renewed = mercure.Client(URL, token=mercure.create_token(KEY, URL, publish=["*"], subscribe=["*"], expires=2))
sub2 = renewed.subscribe("https://example.com/s")
renewed.set_token(mercure.create_token(KEY, URL, publish=["*"], subscribe=["*"], expires=60))
time.sleep(3)
renewed.publish("https://example.com/s", "after expiry")
[("401" in pub_err and "set_token" in pub_err), ("401" in sub_err and "key=" in sub_err), sub2.receive(timeout=10).data]
`)
	if err != nil || out != `[True, True, 'after expiry']` {
		t.Fatalf("%s %v", out, err)
	}
}

func TestKeyArguments(t *testing.T) {
	for script, want := range map[string]string{
		`mercure.Client("https://h.example/.well-known/mercure", key="k", token="t")`:     "not both",
		`mercure.Client("https://h.example/.well-known/mercure", publish=["*"])`:          "give it key=",
		`mercure.Client("https://h.example/.well-known/mercure", token_ttl=60)`:           "give it key=",
		`mercure.Client("https://h.example/.well-known/mercure", key="")`:                 "must not be empty",
		`mercure.Client("https://h.example/.well-known/mercure", key="k", token_ttl=5)`:   "at least 10s",
		`mercure.Client("https://h.example/.well-known/mercure", key="k", token_ttl=0)`:   "must expire",
		`mercure.Client("https://h.example/.well-known/mercure", token="t").set_token`:    "",
		`mercure.Client("https://h.example/.well-known/mercure", key="k").set_token("t")`: "makes its own tokens",
	} {
		_, err := run(t, plain, script)
		if want == "" {
			if err != nil {
				t.Errorf("%s: %v", script, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v, want %q", script, err, want)
		}
	}
	// draft-07 tokens without expiry, from the key.
	h := newHub(t, func(c *gossiper.Config) { c.Protocol = gossiper.ProtocolBoth })
	out, err := run(t, plain, `
hub = mercure.Client("`+h.url+`", key="`+testKey+`", draft07=True, token_ttl=0)
sub = hub.subscribe(topics="https://example.com/{id}")
hub.publish("https://example.com/1", "d7")
sub.receive(timeout=10).data`)
	if err != nil || out != "d7" {
		t.Errorf("draft-07 from the key: %s %v", out, err)
	}
}
