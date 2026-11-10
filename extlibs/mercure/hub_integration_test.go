package mercure

import (
	"os"
	"strings"
	"testing"
)

// TestExternalHubs runs against real hubs, when SCRIPTLING_MERCURE_HUBS names
// them: space-separated hub URLs, each optionally prefixed with "draft07=" to
// use draft-07 tokens and topic selectors with it. They all verify
// tokens with SCRIPTLING_MERCURE_KEY (HS256) and trust the issuer
// https://localhost for Mercure 1.0 tokens, as the official hub's image and
// gossiper do by default. For example:
//
//	docker run --rm -p 13300:80 -e SERVER_NAME=':80' \
//	  -e MERCURE_PUBLISHER_JWT_KEY="$KEY" -e MERCURE_SUBSCRIBER_JWT_KEY="$KEY" dunglas/mercure
//	gossiper serve --listen :13301 --http-only --publisher-key "$KEY"
//	gossiper serve --listen :13302 --http-only --publisher-key "$KEY" --protocol both
//	SCRIPTLING_MERCURE_KEY="$KEY" SCRIPTLING_MERCURE_HUBS="http://localhost:13300/.well-known/mercure \
//	  http://localhost:13301/.well-known/mercure draft07=http://localhost:13302/.well-known/mercure" \
//	  go test -run TestExternalHubs -v ./extlibs/mercure/
func TestExternalHubs(t *testing.T) {
	hubs := strings.Fields(os.Getenv("SCRIPTLING_MERCURE_HUBS"))
	key := os.Getenv("SCRIPTLING_MERCURE_KEY")
	if len(hubs) == 0 || key == "" {
		t.Skip("set SCRIPTLING_MERCURE_HUBS and SCRIPTLING_MERCURE_KEY to test against real hubs")
	}
	for _, hub := range hubs {
		draft07 := strings.HasPrefix(hub, "draft07=")
		hub = strings.TrimPrefix(hub, "draft07=")
		t.Run(hub, func(t *testing.T) {
			tokenArgs, books, inbox := `, URL`,
				`match_urlpattern="https://example.com/books/:id"`, `"https://example.com/inbox/a"`
			if draft07 {
				tokenArgs, books, inbox = `, draft07=True`, `topics="https://example.com/books/{id}"`, `topics="https://example.com/inbox/a"`
			}
			out, err := run(t, plain, `
URL = "`+hub+`"
KEY = "`+key+`"
def token(**grants):
    return mercure.create_token(KEY`+tokenArgs+`, **grants)
results = []
for mode in [False, "auto"]:
    with mercure.Client(URL, token=token(publish=["*"], subscribe=["*"]), stream=mode) as hub:
        sub = hub.subscribe(`+books+`)
        first = hub.publish("https://example.com/books/1", {"title": "Dune"}, type="book", retry=2)
        ids = hub.publish_many([{"topic": "https://example.com/books/" + str(i), "data": str(i)} for i in range(200)])
        ev = sub.receive(timeout=10)
        got = [sub.receive(timeout=10).data for _ in range(200)]
        reader = mercure.Client(URL, token=token(subscribe=["https://example.com/inbox/a"]))
        anon_hub = mercure.Client(URL, token=token(publish=["https://example.com/nothing"]))
        priv = reader.subscribe(`+inbox+`)
        hub.publish("https://example.com/inbox/a", "secret", private=True)
        try:
            anon_hub.publish("https://example.com/books/9", "x")
            refused = False
        except Exception as e:
            refused = "403" in str(e) or "401" in str(e)
        results.append([ev.id == first, ev.type, ev.retry, ev.json()["title"], len(set(ids)),
                        got == [str(i) for i in range(200)], priv.receive(timeout=10).data, refused])
        reader.close()
        anon_hub.close()
results
`)
			if err != nil {
				t.Fatal(err)
			}
			want := `[True, 'book', 2.0, 'Dune', 200, True, 'secret', True]`
			if out != "["+want+", "+want+"]" {
				t.Errorf("got  %s\nwant [%s, %s]", out, want, want)
			}
		})
	}
}

// TestExternalHubsTokenRenewal checks, against the Mercure 1.0 hubs of
// SCRIPTLING_MERCURE_HUBS (the draft07= ones are skipped), that a client made
// with key= keeps publishing and receiving across several token lifetimes.
// It runs about 25s a hub, so it also needs SCRIPTLING_MERCURE_RENEWAL=1.
func TestExternalHubsTokenRenewal(t *testing.T) {
	hubs := strings.Fields(os.Getenv("SCRIPTLING_MERCURE_HUBS"))
	key := os.Getenv("SCRIPTLING_MERCURE_KEY")
	if len(hubs) == 0 || key == "" || os.Getenv("SCRIPTLING_MERCURE_RENEWAL") == "" {
		t.Skip("set SCRIPTLING_MERCURE_HUBS, SCRIPTLING_MERCURE_KEY and SCRIPTLING_MERCURE_RENEWAL=1")
	}
	for _, hub := range hubs {
		if strings.HasPrefix(hub, "draft07=") {
			continue
		}
		t.Run(hub, func(t *testing.T) {
			t.Parallel()
			out, err := run(t, plain, `
import time
hub = mercure.Client("`+hub+`", key="`+key+`", token_ttl=10, stream="auto")
sub = hub.subscribe("https://example.com/renewal")
sent = 0
start = time.time()
while time.time() - start < 25:
    hub.publish("https://example.com/renewal", str(sent))
    sent += 1
    time.sleep(0.1)
got = []
while len(got) < sent:
    ev = sub.receive(timeout=10)
    if ev is None:
        break
    got.append(ev.data)
[sent > 100, got == [str(i) for i in range(sent)]]
`)
			if err != nil || out != "[True, True]" {
				t.Fatalf("%s %v", out, err)
			}
		})
	}
}
