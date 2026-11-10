package mercure

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/paularlott/gossiper/client"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/stdlib"
)

// The benchmarks compare scriptling.mercure with gossiper's Go client doing
// the same against the same in-process hub: what the scriptling layer costs
// per publish and per event received.

func benchScript(b *testing.B, h *testHub, setup, body string) {
	b.Helper()
	p := scriptling.New()
	stdlib.RegisterAll(p)
	Register(p)
	if _, err := p.Eval("import scriptling.mercure as mercure\n" + setup); err != nil {
		b.Fatal(err)
	}
	p.SetVar("N", int64(b.N))
	b.ResetTimer()
	if _, err := p.EvalWithContext(context.Background(), body); err != nil {
		b.Fatal(err)
	}
	b.StopTimer()
}

func benchClient(b *testing.B, h *testHub, tr client.Transport) *client.Client {
	b.Helper()
	tok := benchToken(b, h)
	c, err := client.New(client.Options{URL: h.direct, Token: tok, Transport: tr})
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = c.Close() })
	return c
}

func benchToken(b *testing.B, h *testHub) string {
	b.Helper()
	spec := &tokenSpec{key: []byte(testKey), algorithm: "HS256", issuer: defaultIssuer, audience: h.direct,
		clientID: "bench", matchType: "exact", publish: []string{"*"}, subscribe: []string{"*"}, ttl: 3600e9}
	tok, err := newMintingSource(spec).get(context.Background())
	if err != nil {
		b.Fatal(err)
	}
	return tok
}

func clientSetup(h *testHub, args string) string {
	return fmt.Sprintf("hub = mercure.Client(%q, key=%q%s)\n", h.direct, testKey, args)
}

// One publish at a time, each a POST answered before the next.
func BenchmarkPublishPOST(b *testing.B) {
	b.Run("go", func(b *testing.B) {
		c := benchClient(b, newHub(b), client.HTTP)
		b.ResetTimer()
		for range b.N {
			if _, err := c.Publish(context.Background(), client.Update{Topics: []string{"https://example.com/b"}, Data: "x"}); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("scriptling", func(b *testing.B) {
		h := newHub(b)
		benchScript(b, h, clientSetup(h, ""), `
for i in range(N):
    hub.publish("https://example.com/b", "x")
`)
	})
}

// One publish at a time over a stream, each answered before the next.
func BenchmarkPublishStreamOneByOne(b *testing.B) {
	b.Run("go", func(b *testing.B) {
		c := benchClient(b, newHub(b), client.Stream)
		b.ResetTimer()
		for range b.N {
			if _, err := c.Publish(context.Background(), client.Update{Topics: []string{"https://example.com/b"}, Data: "x"}); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("scriptling", func(b *testing.B) {
		h := newHub(b)
		benchScript(b, h, clientSetup(h, ", stream=True"), `
for i in range(N):
    hub.publish("https://example.com/b", "x")
`)
	})
}

// Many publishes pipelined over a stream.
func BenchmarkPublishStreamPipelined(b *testing.B) {
	b.Run("go", func(b *testing.B) {
		c := benchClient(b, newHub(b), client.Stream)
		b.ResetTimer()
		var wg sync.WaitGroup
		wg.Add(b.N)
		for range b.N {
			if err := c.PublishAsync(context.Background(), client.Update{Topics: []string{"https://example.com/b"}, Data: "x"},
				func(string, error) { wg.Done() }); err != nil {
				b.Fatal(err)
			}
		}
		wg.Wait()
	})
	b.Run("scriptling", func(b *testing.B) {
		h := newHub(b)
		benchScript(b, h, clientSetup(h, ", stream=True"), `
hub.publish_many([{"topic": "https://example.com/b", "data": "x"} for i in range(N)])
`)
	})
}

// Receiving events already delivered: what each receive() costs. Events are
// published in batches (outside the timer) the subscription's buffer holds,
// and every one is checked to arrive.
func BenchmarkReceive(b *testing.B) {
	const batch = 1000
	fill := func(b *testing.B, c *client.Client) {
		var wg sync.WaitGroup
		wg.Add(batch)
		for range batch {
			_ = c.PublishAsync(context.Background(), client.Update{Topics: []string{"https://example.com/r"}, Data: `{"n": 1}`},
				func(string, error) { wg.Done() })
		}
		wg.Wait()
	}
	b.Run("go", func(b *testing.B) {
		h := newHub(b)
		pub := benchClient(b, h, client.Stream)
		sub, err := benchClient(b, h, client.HTTP).Subscribe(context.Background(),
			client.Subscribe{Match: []string{"https://example.com/r"}, Buffer: 2 * batch})
		if err != nil {
			b.Fatal(err)
		}
		b.ResetTimer()
		for done := 0; done < b.N; {
			b.StopTimer()
			fill(b, pub)
			b.StartTimer()
			for i := 0; i < batch && done < b.N; i++ {
				if _, ok := <-sub.Events(); !ok {
					b.Fatalf("subscription ended: %v", sub.Err())
				}
				done++
			}
			for len(sub.Events()) > 0 { // what this batch left, so the next starts empty
				<-sub.Events()
			}
		}
	})
	b.Run("scriptling", func(b *testing.B) {
		h := newHub(b)
		pub := benchClient(b, h, client.Stream)
		p := scriptling.New()
		stdlib.RegisterAll(p)
		Register(p)
		if _, err := p.Eval("import scriptling.mercure as mercure\n" + clientSetup(h, "") +
			fmt.Sprintf("sub = hub.subscribe(\"https://example.com/r\", buffer=%d)\n", 2*batch)); err != nil {
			b.Fatal(err)
		}
		b.ResetTimer()
		for done := 0; done < b.N; {
			b.StopTimer()
			fill(b, pub)
			n := min(batch, b.N-done)
			p.SetVar("N", int64(n))
			b.StartTimer()
			got, err := p.Eval(`
got = 0
for i in range(N):
    if sub.receive(timeout=10) is not None:
        got += 1
got
`)
			b.StopTimer()
			if err != nil || got.Inspect() != fmt.Sprint(n) {
				b.Fatalf("received %v of %d: %v", got, n, err)
			}
			if _, err := p.Eval("while sub.receive(timeout=0) is not None:\n    pass\n"); err != nil {
				b.Fatal(err)
			}
			b.StartTimer()
			done += n
		}
	})
}
