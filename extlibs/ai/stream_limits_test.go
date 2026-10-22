package ai

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	scriptlib "github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

// streamFormat writes one provider's native stream: the first chunk, then
// (once released) the rest, each chunk carrying piece.
type streamFormat struct {
	contentType string
	path        string // request path the provider streams from
	first       func(w http.ResponseWriter, piece string)
	chunk       func(w http.ResponseWriter, piece string)
	finish      func(w http.ResponseWriter)
}

var streamFormats = map[string]streamFormat{
	"ai.OPENAI": {
		contentType: "text/event-stream",
		path:        "/chat/completions",
		first: func(w http.ResponseWriter, piece string) {
			fmt.Fprintf(w, `data: {"id":"c","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"role":"assistant","content":"%s"}}]}`+"\n\n", piece)
		},
		chunk: func(w http.ResponseWriter, piece string) {
			fmt.Fprintf(w, `data: {"id":"c","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"%s"}}]}`+"\n\n", piece)
		},
		finish: func(w http.ResponseWriter) {
			fmt.Fprint(w, `data: {"id":"c","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`+"\n\ndata: [DONE]\n\n")
		},
	},
	"ai.OLLAMA": {
		contentType: "application/x-ndjson",
		path:        "/api/chat",
		first: func(w http.ResponseWriter, piece string) {
			fmt.Fprintf(w, `{"model":"m","message":{"role":"assistant","content":"%s"},"done":false}`+"\n", piece)
		},
		chunk: func(w http.ResponseWriter, piece string) {
			fmt.Fprintf(w, `{"model":"m","message":{"role":"assistant","content":"%s"},"done":false}`+"\n", piece)
		},
		finish: func(w http.ResponseWriter) {
			fmt.Fprint(w, `{"model":"m","message":{"role":"assistant","content":""},"done":true,"done_reason":"stop"}`+"\n")
		},
	},
	"ai.CLAUDE": {
		contentType: "text/event-stream",
		path:        "/messages",
		first: func(w http.ResponseWriter, piece string) {
			fmt.Fprint(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"m1\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"m\",\"content\":[],\"usage\":{\"input_tokens\":1,\"output_tokens\":0}}}\n\n")
			fmt.Fprint(w, "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n")
			fmt.Fprintf(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"%s\"}}\n\n", piece)
		},
		chunk: func(w http.ResponseWriter, piece string) {
			fmt.Fprintf(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"%s\"}}\n\n", piece)
		},
		finish: func(w http.ResponseWriter) {
			fmt.Fprint(w, "event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n")
			fmt.Fprint(w, "event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":2}}\n\n")
			fmt.Fprint(w, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
		},
	},
}

// Streaming chat completions keep working under the interpreter's limits,
// for every provider's stream format: chunks reach the script as they are
// sent (the response wrapper does not buffer), and a stream far longer than
// the value limit in total, sent in small chunks, is read to the end.
func TestStreamingUnderLimits(t *testing.T) {
	const chunks, size = 200, 16 * 1024 // 3.2 MB in total, 1 MB value limit
	piece := strings.Repeat("A", size)

	for provider, f := range streamFormats {
		t.Run(provider, func(t *testing.T) {
			released := make(chan struct{})
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasSuffix(r.URL.Path, f.path) {
					http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
					return
				}
				w.Header().Set("Content-Type", f.contentType)
				f.first(w, piece)
				w.(http.Flusher).Flush()
				// Hold the rest until the script has the first chunk: if
				// anything buffered the stream, the script never gets it.
				select {
				case <-released:
				case <-time.After(10 * time.Second):
					return
				}
				for i := 1; i < chunks; i++ {
					f.chunk(w, piece)
				}
				f.finish(w)
			}))
			defer srv.Close()

			p := scriptlib.New(scriptlib.WithMaxValueBytes(mib))
			Register(p)
			p.SetVar("url", srv.URL)
			p.RegisterFunc("release", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				close(released)
				return &object.Null{}
			})

			done := make(chan struct{})
			var result object.Object
			var err error
			go func() {
				defer close(done)
				result, err = p.Eval(`
import scriptling.ai as ai
c = ai.Client(url, provider=` + provider + `, api_key="k", max_retries=0)
s = c.completion_stream("m", "hi")
total = 0
first = True
while True:
    ch = s.next()
    if ch == None:
        break
    choices = ch.get("choices") or []
    if len(choices) > 0:
        delta = choices[0].get("delta") or {}
        content = delta.get("content")
        if content:
            total = total + len(content)
    if first:
        first = False
        release()
[total, s.err()]`)
			}()
			select {
			case <-done:
			case <-time.After(15 * time.Second):
				t.Fatal("the stream never delivered its first chunk: something buffered it")
			}
			if err != nil {
				t.Fatal(err)
			}
			if got, want := result.Inspect(), fmt.Sprintf("[%d, None]", chunks*size); got != want {
				t.Fatalf("got %s, want %s", got, want)
			}
		})
	}
}
