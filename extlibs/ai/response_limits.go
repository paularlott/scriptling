package ai

import (
	"bytes"
	stderrors "errors"
	"fmt"
	"io"
	"math"
	"mime"
	"net/http"
	"strings"

	"github.com/paularlott/mcp/pool"
	"github.com/paularlott/scriptling/object"
)

// tooLargeMarker starts the message of a responseTooLargeError. Providers
// sometimes format the read error with %v rather than wrapping it, so the
// message is how providerError recognises it when errors.As cannot.
const tooLargeMarker = "AI response too large"

// responseTooLargeError aborts a response body that would take the
// interpreter past its value limit or its memory limit.
type responseTooLargeError struct{ msg string }

func (e *responseTooLargeError) Error() string { return e.msg }

// limitsInForce reports whether the interpreter has a value or memory limit
// for response bodies to be held to.
func limitsInForce(limits *object.ResourceGuard) bool {
	max := limits.MaxValueBytes()
	return (max > 0 && max != math.MaxInt64) || limits.MemoryLimit() > 0
}

// limitedTransport holds every response body to the limits of the
// interpreter that created the client. The provider (mcp) reads bodies in Go,
// on its own goroutines, before anything becomes a script value, so without
// this a single response could take the interpreter far past its limits.
type limitedTransport struct {
	base   http.RoundTripper
	limits *object.ResourceGuard
}

func (t *limitedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil || resp.Body == nil {
		return resp, err
	}
	resp.Body = &limitedBody{ReadCloser: resp.Body, limits: t.limits, stream: isStreamingResponse(resp)}
	return resp, nil
}

// isStreamingResponse reports a server-sent event or NDJSON stream. A stream
// is consumed line by line as it arrives, so it is held to the limits per
// line, not in total: a long answer streamed in small chunks is legitimate.
func isStreamingResponse(resp *http.Response) bool {
	mt, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return false
	}
	return mt == "text/event-stream" || mt == "application/x-ndjson" || strings.HasSuffix(mt, "+json-seq")
}

// limitedBody counts what is read: the whole body, or for a stream the
// current line, against the value limit and the memory headroom.
type limitedBody struct {
	io.ReadCloser
	limits *object.ResourceGuard
	stream bool
	n      int64 // bytes in the body so far, or in the current stream line
}

func (b *limitedBody) Read(p []byte) (int, error) {
	k, err := b.ReadCloser.Read(p)
	if k == 0 {
		return k, err
	}
	if b.stream {
		if i := bytes.LastIndexByte(p[:k], '\n'); i >= 0 {
			b.n = int64(k - 1 - i)
		} else {
			b.n += int64(k)
		}
	} else {
		b.n += int64(k)
	}
	if lerr := b.check(); lerr != nil {
		return 0, lerr
	}
	return k, err
}

func (b *limitedBody) check() error {
	what := "AI response"
	if b.stream {
		what = "AI response line"
	}
	if max := b.limits.MaxValueBytes(); max > 0 && max != math.MaxInt64 && b.n > max {
		return &responseTooLargeError{fmt.Sprintf("%s: more than the %d byte limit", tooLargeMarker, max)}
	}
	// The body is held in Go until it becomes a value: charge it to the
	// interpreter's memory headroom, on whichever goroutine is reading.
	release := object.BindGuard(b.limits)
	errObj := object.CheckRoom(what, b.n)
	release()
	if errObj != nil {
		msg := errObj.Inspect()
		if e, ok := errObj.(*object.Error); ok {
			msg = e.Message
		}
		return &responseTooLargeError{fmt.Sprintf("%s: %s", tooLargeMarker, msg)}
	}
	return nil
}

// limitedClient returns client (or the shared pool client the providers
// use by default, when nil) with its transport wrapped to hold response
// bodies to limits, or client unchanged when the interpreter has no limits.
func limitedClient(client *http.Client, limits *object.ResourceGuard) *http.Client {
	if limits == nil || !limitsInForce(limits) {
		return client
	}
	if client == nil {
		client = pool.GetPool().GetHTTPClient()
	}
	c := *client
	base := c.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	c.Transport = &limitedTransport{base: base, limits: limits}
	return &c
}

// providerError turns a provider error into the script's error: a response
// over the interpreter's limits is a MemoryError, as from requests;
// anything else an ordinary error. The message starts with prefix once: some
// providers (the OpenAI client and those built on it) already start their
// errors with the same words.
func providerError(prefix string, err error) object.Object {
	msg := err.Error()
	if !strings.HasPrefix(msg, prefix+": ") {
		msg = prefix + ": " + msg
	}
	var tooLarge *responseTooLargeError
	if stderrors.As(err, &tooLarge) || strings.Contains(msg, tooLargeMarker) {
		return object.NewMemoryError("%s", msg)
	}
	return &object.Error{Message: msg}
}
