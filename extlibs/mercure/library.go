// Package mercure is the scriptling.mercure library: a client for Mercure hubs
// (https://mercure.rocks). A script publishes updates to topics and subscribes
// to them over Server-Sent Events, against any Mercure hub: the reference hub,
// FrankenPHP's built-in one, or a gossiper cluster.
//
// It speaks Mercure 1.0 by default: create_token makes access tokens bound to
// the hub, and subscribe takes exact topics and URL patterns. Draft-07 (the
// mercure claim and URI templates) is there for hubs that take only that:
// create_token(draft07=True) and subscribe(topics=...).
//
// It is built on gossiper's Go client. By default each publish is a POST of its
// own, as Mercure's HTTP protocol has it, which every hub takes. With
// stream=True a client publishes to a gossiper hub over one long streaming
// request instead, many times faster for a script that publishes a lot.
// Subscriptions reconnect by themselves and resume from their last event ID.
package mercure

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/paularlott/gossiper/client"
	"github.com/paularlott/scriptling/conversion"
	scriptlingerrors "github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
	"github.com/paularlott/scriptling/object"
)

const (
	LibraryName = "scriptling.mercure"
	LibraryDesc = "Mercure hub client: publish updates and subscribe to topics over Server-Sent Events"

	// defaultIssuer is the iss of Mercure 1.0 tokens when none is given: the
	// issuer the official hub's image and gossiper trust by default.
	defaultIssuer = "https://localhost"

	defaultTimeout        = 30 * time.Second
	defaultReceiveTimeout = 30.0
	// closeWait bounds how long a closed client waits for its subscriptions
	// to let go of their connections before it closes the idle ones.
	closeWait = 5 * time.Second
)

// minTokenTTL is the shortest token_ttl a client takes (a variable for tests).
var minTokenTTL = 10 * time.Second

var (
	library     *object.Library
	libraryOnce sync.Once

	// openClients are the clients not yet closed, closed for the script on a
	// runtime reset.
	openClients = struct {
		sync.Mutex
		m     map[*hubClient]struct{}
		armed bool // closeAll is registered with the runtime's cleanup
	}{m: map[*hubClient]struct{}{}}
)

// armCleanup registers closeAll to run on the next runtime reset, unless it
// is already. A reset forgets its cleanup functions once it has run them, so
// it is registered again with the next client.
func armCleanup() {
	openClients.Lock()
	defer openClients.Unlock()
	if openClients.armed {
		return
	}
	openClients.armed = true
	extlibs.RegisterCleanup(func() {
		openClients.Lock()
		openClients.armed = false
		openClients.Unlock()
		closeAll()
	})
}

// Register registers scriptling.mercure, under the network policy cfg when
// one is given. A CA bundle a script names with verify="path" may be any file:
// use RegisterConfigured to confine it.
func Register(registrar interface{ RegisterLibrary(*object.Library) }, cfg ...*netsecurity.Config) {
	var policy *netsecurity.Config
	if len(cfg) > 0 {
		policy = cfg[0]
	}
	RegisterConfigured(registrar, fssecurity.Config{}, policy)
}

// RegisterConfigured registers scriptling.mercure with a filesystem
// allow-path policy and a network policy, as the CLI's --allowed-paths and
// --network-policy give them: every connection to a hub passes the network
// policy, and a CA bundle named with verify="path" must lie within the allowed
// paths. A zero fssecurity.Config and a nil policy register it unrestricted.
//
// Clients live until the script closes them, or until a runtime reset; give
// WithLifetime to close them when an environment is done with as well.
func RegisterConfigured(registrar interface{ RegisterLibrary(*object.Library) }, paths fssecurity.Config, policy *netsecurity.Config, opts ...Option) {
	paths.AllowedPaths = fssecurity.NormalizeAllowedPaths(paths.AllowedPaths)
	t := &transports{paths: paths}
	for _, o := range opts {
		o(t)
	}
	if paths.AllowedPaths == nil && policy == nil && t.lifetime == nil {
		libraryOnce.Do(func() { library = buildLibrary(&transports{}) })
		registrar.RegisterLibrary(library)
		return
	}
	if policy != nil {
		guard, gerr := netsecurity.NewGuard(policy)
		if gerr != nil {
			guard = netsecurity.FailClosed(gerr)
		}
		t.guard = guard
	}
	registrar.RegisterLibrary(buildLibrary(t))
}

// closeAll closes every client still open.
func closeAll() {
	openClients.Lock()
	clients := make([]*hubClient, 0, len(openClients.m))
	for h := range openClients.m {
		clients = append(clients, h)
	}
	openClients.Unlock()
	for _, h := range clients {
		h.close()
	}
}

// ── Transport ───────────────────────────────────────────────────────────────

// transports builds a client's HTTP transport under the registration's
// network policy and allowed paths.
type transports struct {
	guard    *netsecurity.Guard
	paths    fssecurity.Config
	lifetime func() context.Context
}

// Option configures the library.
type Option func(*transports)

// WithLifetime bounds the clients scripts make through this registration:
// lifetime is called as each client is made, and the client and its
// subscriptions are closed when the context it returns ends. It is for an
// embedder that throws an environment away, or lends it to another user,
// after a script: a subscription the script left open goes with it. A client
// made after that context has ended fails.
func WithLifetime(lifetime func() context.Context) Option {
	return func(t *transports) { t.lifetime = lifetime }
}

// transport is an HTTP transport for the hub at u: TLS verified, unverified
// (verify=False) or against a CA bundle (verify="path"), and HTTP/2 without
// TLS when h2c is asked for an http:// hub. Under a network policy the hub's
// URL is checked once (the client follows no redirects, so it asks nothing
// else) and every connection is dialled through the policy.
func (t *transports) transport(ctx context.Context, u *url.URL, verify object.Object, h2c bool, timeout time.Duration) (*http.Transport, object.Object) {
	if t.guard != nil {
		if err := t.guard.CheckURL(u); err != nil {
			return nil, &object.Error{Message: fmt.Sprintf("mercure: %v", err)}
		}
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	switch v := verify.(type) {
	case nil, *object.Null:
	case *object.Boolean:
		if on, _ := v.AsBool(); !on {
			tlsConfig.InsecureSkipVerify = true //nolint:gosec // asked for with verify=False
		}
	case *object.String:
		path, _ := v.AsString()
		roots, errObj := t.caBundle(ctx, path)
		if errObj != nil {
			return nil, errObj
		}
		tlsConfig.RootCAs = roots
	default:
		return nil, scriptlingerrors.NewTypeError("BOOLEAN or STRING", verify.Type().String())
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if t.guard != nil {
		tr.DialContext = t.guard.DialContext
		tr.Proxy = nil // a proxy would tunnel past the policy
	}
	tr.TLSClientConfig = tlsConfig
	tr.MaxIdleConnsPerHost = 64
	tr.ResponseHeaderTimeout = timeout
	// A connection gone without a word (half-open) is noticed: HTTP/2 pings it once quiet.
	tr.HTTP2 = &http.HTTP2Config{SendPingTimeout: 15 * time.Second, PingTimeout: 15 * time.Second}
	tr.Protocols = new(http.Protocols)
	if h2c && u.Scheme == "http" {
		tr.Protocols.SetUnencryptedHTTP2(true)
	} else {
		tr.Protocols.SetHTTP1(true)
		tr.Protocols.SetHTTP2(true)
	}
	return tr, nil
}

// caBundle reads the PEM CA certificates at path, through the allowed paths
// and the interpreter's limits.
func (t *transports) caBundle(ctx context.Context, path string) (*x509.CertPool, object.Object) {
	if path == "" {
		return nil, scriptlingerrors.NewValueError("verify: the CA bundle path is empty")
	}
	if t.paths.CheckPath(path) != nil {
		return nil, scriptlingerrors.NewPermissionError("access denied: CA bundle '%s' is outside allowed directories", path)
	}
	var pem []byte
	var limitErr object.Object
	var err error
	object.RunBlocking(ctx, func() { pem, limitErr, err = object.ReadFileLimited(path) })
	if limitErr != nil {
		return nil, limitErr
	}
	if err != nil {
		return nil, &object.Error{Message: fmt.Sprintf("verify: cannot read CA bundle: %v", err)}
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pem) {
		return nil, scriptlingerrors.NewValueError("verify: no PEM certificates in CA bundle '%s'", path)
	}
	return roots, nil
}

func noRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

// ── Library ─────────────────────────────────────────────────────────────────

func buildLibrary(t *transports) *object.Library {
	b := object.NewLibraryBuilder(LibraryName, LibraryDesc)

	b.FunctionWithHelp("Client", func(ctx context.Context, kwargs object.Kwargs, hubURL string) object.Object {
		return newClient(ctx, t, kwargs, hubURL)
	}, `Client(url, **kwargs) - A client of a Mercure hub, to publish updates and subscribe to topics

Parameters:
  url (str): The hub's URL, e.g. "https://example.com/.well-known/mercure"
  key (str, optional): The hub's JWT key. The client makes its own tokens with it and renews them before
    they expire, so long-running subscriptions and publish streams never stop for an expired token. The
    token's other arguments are create_token()'s: publish, subscribe (default: both ["*"]), payload,
    issuer, audience (default: url), match_type, subject, client_id, algorithm, draft07
  token_ttl (float, optional): With key=, the lifetime of each token in seconds. Default: 3600; tokens are
    renewed at half their life
  token (str, optional): A token made elsewhere, presented as it is; replace it with set_token() before it
    expires. Not with key=
  stream (bool or str, optional): How to publish. False (default): a POST per update, as Mercure's HTTP
    protocol does, which every hub takes. True: one long streaming publish to a gossiper hub, many times
    faster for many updates. "auto": stream to a gossiper hub, a POST per update to any other
  verify (bool or str, optional): Verify the hub's TLS certificate (default: True); False accepts any
    certificate (development only); a path names a PEM bundle of CA certificates to trust
  timeout (float, optional): Seconds to wait for the hub to answer a publish, or to accept a
    subscription. Default: 30
  headers (dict, optional): Headers added to every request
  host (str, optional): The host name to ask for, when it is not the URL's (a node reached by address)
  h2c (bool, optional): Speak HTTP/2 without TLS to an http:// gossiper hub, so the publish stream and
    every subscription share one connection. Default: False
  max_event_size (int, optional): Largest event a subscription takes, in bytes. Default: 8 MiB

Returns:
  Client

Example:
  hub = mercure.Client("https://example.com/.well-known/mercure", key=os.getenv("MERCURE_JWT_KEY"))
  sub = hub.subscribe(match_urlpattern="https://example.com/books/:id")
  hub.publish("https://example.com/books/1", {"title": "Dune"})
  for event in sub:
      print(event.id, event.json())`)

	b.FunctionWithHelp("create_token", func(kwargs object.Kwargs, key string, rest ...string) object.Object {
		return createToken(kwargs, key, rest)
	}, `create_token(key, audience, **kwargs) - A Mercure JWT, signed with a hub's HMAC key

By default a Mercure 1.0 access token (RFC 9068, with authorization_details) for the hub at audience, which
current hubs take: the official hub, gossiper. draft07=True makes a draft-07 token (the "mercure" claim) for
hubs that take only that.

Parameters:
  key (str): The hub's publisher or subscriber JWT key (keep it out of the script: read it with the
    secret library or os.getenv)
  audience (str): The hub's URL, its resource identifier (the aud claim). Not for draft07=True
  publish (str or list, optional): Topics the token may publish to, e.g. ["*"]
  subscribe (str or list, optional): Topics the token may receive private updates of
  payload (dict, optional): Shared with other subscribers in subscription events
  expires (float, optional): Seconds until the token expires. Default: 3600 (0, none, only for draft07=True)
  issuer (str, optional): The iss claim, an issuer the hub trusts. Default: "https://localhost", the
    issuer the official hub's image and gossiper trust unless configured otherwise
  match_type (str, optional): How publish and subscribe match topics: "exact" (default) or "urlpattern"
  subject (str, optional): The sub claim. Default: the client ID (none for draft07=True)
  client_id (str, optional): The client_id claim. Default: "scriptling"
  algorithm (str, optional): "HS256" (default), "HS384" or "HS512"
  draft07 (bool, optional): Make a draft-07 token instead: publish and subscribe are draft-07 selectors
    (topics, URI templates or "*"), and there is no audience or issuer. Default: False

Returns:
  str: the token

Example:
  URL = "https://hub.example.com/.well-known/mercure"
  jwt = mercure.create_token(os.getenv("MERCURE_JWT_KEY"), URL, publish=["*"], subscribe=["*"])`)

	return b.Build()
}

// ── Client ──────────────────────────────────────────────────────────────────

// hubClient is a Client: gossiper's client, and the transport it was given.
type hubClient struct {
	c      *client.Client
	tokens *tokenSource
	url    string
	tr     *http.Transport
	once   sync.Once
	mu     sync.Mutex
	subs   map[*subscription]struct{}
	ended  bool

	stopLifetime func() bool // forgets WithLifetime's close; nil without one
}

func newClient(ctx context.Context, t *transports, kwargs object.Kwargs, hubURL string) object.Object {
	u, err := url.Parse(hubURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return scriptlingerrors.NewValueError("mercure: url must be an http or https URL")
	}
	tokens, errObj := clientTokens(kwargs, hubURL)
	if errObj != nil {
		return errObj
	}
	host, errObj := optionalString(kwargs, "host")
	if errObj != nil {
		return errObj
	}
	mode, errObj := streamArg(kwargs.Get("stream"))
	if errObj != nil {
		return errObj
	}
	timeoutSecs, errObj := kwargs.GetFloat("timeout", defaultTimeout.Seconds())
	if errObj != nil {
		return errObj
	}
	if timeoutSecs <= 0 {
		return scriptlingerrors.NewValueError("timeout must be positive")
	}
	timeout := time.Duration(timeoutSecs * float64(time.Second))
	h2c, errObj := kwargs.GetBool("h2c", false)
	if errObj != nil {
		return errObj
	}
	maxEvent, errObj := kwargs.GetInt("max_event_size", 0)
	if errObj != nil {
		return errObj
	}
	if maxEvent < 0 {
		return scriptlingerrors.NewValueError("max_event_size must not be negative")
	}
	header, errObj := headersArg(kwargs.Get("headers"))
	if errObj != nil {
		return errObj
	}
	tr, errObj := t.transport(ctx, u, kwargs.Get("verify"), h2c, timeout)
	if errObj != nil {
		return errObj
	}
	c, err := client.New(client.Options{
		URL:          hubURL,
		Host:         host,
		TokenFunc:    tokens.get,
		Transport:    mode,
		HTTPClient:   &http.Client{Timeout: timeout, Transport: tr, CheckRedirect: noRedirect},
		Header:       header,
		MaxEventSize: int(maxEvent),
	})
	if err != nil {
		tr.CloseIdleConnections()
		return errObject(err)
	}
	h := &hubClient{c: c, url: hubURL, tr: tr, tokens: tokens, subs: map[*subscription]struct{}{}}
	if t.lifetime != nil {
		life := t.lifetime()
		if life.Err() != nil {
			_ = c.Close()
			tr.CloseIdleConnections()
			return &object.Error{Message: "mercure: this environment has ended"}
		}
		h.stopLifetime = context.AfterFunc(life, h.close)
	}
	armCleanup()
	openClients.Lock()
	openClients.m[h] = struct{}{}
	openClients.Unlock()
	return newInstance(getClientClass(), "MercureClient", h)
}

// clientTokens is a client's token source: minted from key= and renewed, or
// the token= given (None: anonymous).
func clientTokens(kwargs object.Kwargs, hubURL string) (*tokenSource, object.Object) {
	token, errObj := optionalString(kwargs, "token")
	if errObj != nil {
		return nil, errObj
	}
	key, errObj := optionalString(kwargs, "key")
	if errObj != nil {
		return nil, errObj
	}
	if key == "" {
		if kwargs.Has("key") {
			return nil, scriptlingerrors.NewValueError("key must not be empty")
		}
		for _, k := range append([]string{"token_ttl"}, tokenArgs...) {
			if kwargs.Has(k) {
				return nil, scriptlingerrors.NewValueError("%s is for a client that makes its own tokens: give it key=", k)
			}
		}
		return newStaticSource(token), nil
	}
	if token != "" {
		return nil, scriptlingerrors.NewValueError("give token= or key=, not both")
	}
	audience, errObj := optionalString(kwargs, "audience")
	if errObj != nil {
		return nil, errObj
	}
	draft07, _ := kwargs.GetBool("draft07", false)
	if audience == "" && !draft07 {
		audience = hubURL // the hub's resource identifier: its URL, unless it was given another
	}
	spec, errObj := parseTokenSpec(kwargs, key, audience, "token_ttl", true)
	if errObj != nil {
		return nil, prefixError(errObj, "Client: ")
	}
	if spec.ttl > 0 && spec.ttl < minTokenTTL {
		return nil, scriptlingerrors.NewValueError("token_ttl must be at least %s: tokens are renewed at half their life", minTokenTTL)
	}
	return newMintingSource(spec), nil
}

// close closes the client and its subscriptions, then its connections once
// the subscriptions let go of them.
func (h *hubClient) close() {
	h.once.Do(func() {
		if h.stopLifetime != nil {
			h.stopLifetime()
		}
		openClients.Lock()
		delete(openClients.m, h)
		openClients.Unlock()
		h.mu.Lock()
		h.ended = true
		subs := make([]*subscription, 0, len(h.subs))
		for s := range h.subs {
			subs = append(subs, s)
		}
		h.mu.Unlock()
		_ = h.c.Close()
		go func() {
			deadline := time.After(closeWait)
			for _, s := range subs {
				s.drain(deadline)
			}
			h.tr.CloseIdleConnections()
		}()
	})
}

func (h *hubClient) closed() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.ended
}

// streamArg reads stream=: False/None (a POST per update), True (a stream) or "auto".
func streamArg(v object.Object) (client.Transport, object.Object) {
	switch x := v.(type) {
	case nil, *object.Null:
		return client.HTTP, nil
	case *object.Boolean:
		if on, _ := x.AsBool(); on {
			return client.Stream, nil
		}
		return client.HTTP, nil
	case *object.String:
		s, _ := x.AsString()
		switch strings.ToLower(s) {
		case "auto":
			return client.Auto, nil
		case "stream":
			return client.Stream, nil
		case "http", "":
			return client.HTTP, nil
		}
		return 0, scriptlingerrors.NewValueError(`stream must be True, False or "auto", not %q`, s)
	}
	return 0, scriptlingerrors.NewTypeError("BOOLEAN or STRING", v.Type().String())
}

var (
	clientClass     *object.Class
	clientClassOnce sync.Once
)

func getClientClass() *object.Class {
	clientClassOnce.Do(func() { clientClass = buildClientClass() })
	return clientClass
}

func buildClientClass() *object.Class {
	cb := object.NewClassBuilder("Client")

	cb.Property("url", func(self *object.Instance) object.Object {
		h, errObj := impl[*hubClient](self)
		if errObj != nil {
			return errObj
		}
		return object.NewString(h.url)
	})
	cb.Property("transport", func(self *object.Instance) object.Object {
		h, errObj := impl[*hubClient](self)
		if errObj != nil {
			return errObj
		}
		return object.NewString(h.c.Transport().String())
	})
	cb.Property("closed", func(self *object.Instance) object.Object {
		h, errObj := impl[*hubClient](self)
		if errObj != nil {
			return errObj
		}
		return object.NewBoolean(h.closed())
	})

	cb.MethodWithHelp("publish", func(self *object.Instance, ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		h, errObj := impl[*hubClient](self)
		if errObj != nil {
			return errObj
		}
		if len(args) < 1 || len(args) > 2 {
			return scriptlingerrors.NewArgumentError(len(args), 2)
		}
		fields := map[string]object.Object{"topic": args[0]}
		if len(args) == 2 {
			if kwargs.Has("data") {
				return scriptlingerrors.NewValueError("data given both by position and by name")
			}
			fields["data"] = args[1]
		}
		for _, k := range kwargs.Keys() {
			fields[k] = kwargs.Get(k)
		}
		u, errObj := updateFrom(fields)
		if errObj != nil {
			return errObj
		}
		var id string
		var err error
		object.RunBlocking(ctx, func() { id, err = h.c.Publish(ctx, u) })
		if err != nil {
			return publishError(ctx, h.tokens.explain(err))
		}
		return object.NewString(id)
	}, `publish(topic, data=None, **kwargs) - Publish an update and return its event ID once the hub has it

Parameters:
  topic (str or list): The update's topic, or its canonical topic then alternates
  data (str or any, optional): The update's content. A str is sent as is; any other value (a dict, a
    list, a number) is sent as JSON. Default: empty
  id (str, optional): The event ID. Default: the hub makes one
  type (str, optional): The SSE event type
  private (bool, optional): Deliver only to subscribers allowed the topic. Default: False
  retry (float, optional): The reconnection time subscribers are given, in seconds
  replicas (int, optional): Copies a gossiper hub keeps, on as many nodes, before it answers

Returns:
  str: the event ID

Example:
  event_id = hub.publish("https://example.com/books/1", {"title": "Dune"}, type="book")`)

	cb.MethodWithHelp("publish_many", func(self *object.Instance, ctx context.Context, updates object.Object) object.Object {
		h, errObj := impl[*hubClient](self)
		if errObj != nil {
			return errObj
		}
		list, ok := updates.(*object.List)
		if !ok {
			if tup, ok := updates.(*object.Tuple); ok {
				list = &object.List{Elements: tup.Elements}
			} else {
				return scriptlingerrors.NewTypeError("LIST", updates.Type().String())
			}
		}
		us := make([]client.Update, len(list.Elements))
		for i, e := range list.Elements {
			d, ok := e.(*object.Dict)
			if !ok {
				return scriptlingerrors.NewValueError("publish_many: update %d must be a dict", i)
			}
			fields := map[string]object.Object{}
			for _, pair := range d.OrderedPairs() {
				k, err := pair.Key.AsString()
				if err != nil {
					return scriptlingerrors.NewValueError("publish_many: update %d: keys must be strings", i)
				}
				fields[k] = pair.Value
			}
			u, errObj := updateFrom(fields)
			if errObj != nil {
				return prefixError(errObj, fmt.Sprintf("publish_many: update %d: ", i))
			}
			us[i] = u
		}
		ids, failed, err := h.publishMany(ctx, us)
		if err != nil {
			errObj := publishError(ctx, h.tokens.explain(err))
			if failed >= 0 && ctx.Err() == nil {
				return prefixError(errObj, fmt.Sprintf("publish_many: update %d: ", failed))
			}
			return errObj
		}
		out := make([]object.Object, len(ids))
		for i, id := range ids {
			out[i] = object.NewString(id)
		}
		return &object.List{Elements: out}
	}, `publish_many(updates) - Publish several updates and return their event IDs, in order

Over a stream (stream=True) the updates are pipelined: sent without waiting for each answer, in order.
With a POST per update they are published one after another.

Parameters:
  updates (list): Dicts with the arguments of publish(): topic, data, id, type, private, retry, replicas

Returns:
  list: the event IDs. If one fails, the error names it; the updates before it were published

Example:
  ids = hub.publish_many([{"topic": "https://example.com/n", "data": str(i)} for i in range(1000)])`)

	cb.MethodWithHelp("subscribe", func(self *object.Instance, ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		h, errObj := impl[*hubClient](self)
		if errObj != nil {
			return errObj
		}
		// Topics by position are Mercure 1.0 exact matchers, as match= is;
		// topics= are draft-07 selectors (URI templates), for hubs that take
		// only those.
		var spec client.Subscribe
		for _, a := range args {
			topics, errObj := stringOrList(a, "topic")
			if errObj != nil {
				return errObj
			}
			spec.Match = append(spec.Match, topics...)
		}
		match, errObj := stringOrList(kwargs.Get("match"), "match")
		if errObj != nil {
			return errObj
		}
		spec.Match = append(spec.Match, match...)
		if spec.MatchURLPattern, errObj = stringOrList(kwargs.Get("match_urlpattern"), "match_urlpattern"); errObj != nil {
			return errObj
		}
		if spec.Topics, errObj = stringOrList(kwargs.Get("topics"), "topics"); errObj != nil {
			return errObj
		}
		if spec.LastEventID, errObj = optionalString(kwargs, "last_event_id"); errObj != nil {
			return errObj
		}
		buffer, errObj := kwargs.GetInt("buffer", 1024)
		if errObj != nil {
			return errObj
		}
		if buffer <= 0 {
			return scriptlingerrors.NewValueError("buffer must be positive")
		}
		spec.Buffer = int(buffer)
		if len(spec.Topics) == 0 && len(spec.Match) == 0 && len(spec.MatchURLPattern) == 0 {
			return scriptlingerrors.NewValueError("subscribe: give at least one topic")
		}
		s, errObj := h.subscribe(ctx, spec)
		if errObj != nil {
			return errObj
		}
		return newInstance(getSubscriptionClass(), "MercureSubscription", s)
	}, `subscribe(*topics, **kwargs) - Subscribe to topics; returns once the hub accepted the subscription

The subscription reconnects by itself, resuming from its last event ID, until it is closed.

Parameters:
  topics (str or list): Topics to subscribe to, exactly (Mercure 1.0 match), by position; "*" for every topic
  match (str or list, optional): More exact topics, as by position
  match_urlpattern (str or list, optional): URL patterns (Mercure 1.0), e.g. "https://example.com/books/:id"
  topics= (str or list, optional): Draft-07 topic selectors, for hubs that take only draft-07: topics, URI
    templates (e.g. "https://example.com/books/{id}") or "*"
  last_event_id (str, optional): Resume after that event ("earliest" for the hub's whole history)
  buffer (int, optional): Events held for the script before the stream waits. Default: 1024

Returns:
  Subscription

Example:
  sub = hub.subscribe(match_urlpattern="https://example.com/books/:id")
  event = sub.receive(timeout=10)`)

	cb.MethodWithHelp("set_token", func(self *object.Instance, token string) object.Object {
		h, errObj := impl[*hubClient](self)
		if errObj != nil {
			return errObj
		}
		if err := h.tokens.set(token); err != nil {
			return scriptlingerrors.NewValueError("%s", err.Error())
		}
		return &object.Null{}
	}, `set_token(token) - Replace the token of a client made with token=

The next request, and every stream the client opens or reopens from then on, presents it: a subscription
that the hub ends as its token expires reconnects with the new one. A client made with key= renews its own
tokens and has no use for it.

Parameters:
  token (str): The new token

Example:
  issued.refresh()                 # issued: a scriptling.oauth Token
  hub.set_token(issued.access_token)`)

	cb.MethodWithHelp("close", func(self *object.Instance) object.Object {
		h, errObj := impl[*hubClient](self)
		if errObj != nil {
			return errObj
		}
		h.close()
		return &object.Null{}
	}, `close() - Close the client, its subscriptions and its publish stream`)

	cb.MethodWithHelp("__enter__", func(self *object.Instance) object.Object {
		return self
	}, `__enter__() - Use the client in a with block; it is closed at its end`)

	cb.MethodWithHelp("__exit__", func(self *object.Instance, args ...object.Object) object.Object {
		h, errObj := impl[*hubClient](self)
		if errObj != nil {
			return errObj
		}
		h.close()
		return object.NewBoolean(false)
	}, `__exit__(*args) - Close the client at the end of a with block`)

	return cb.Build()
}

// publishMany publishes us: pipelined over a stream, one after another with a
// POST per update. failed is the index of the update that failed, -1 for none.
func (h *hubClient) publishMany(ctx context.Context, us []client.Update) (ids []string, failed int, err error) {
	ids = make([]string, len(us))
	failed = -1
	object.RunBlocking(ctx, func() {
		start := 0
		if h.c.Transport() == client.Auto && len(us) > 0 {
			// Whether the hub takes a stream is known once it has been asked:
			// the first update asks, and the rest go the way it found.
			id, perr := h.c.Publish(ctx, us[0])
			if perr != nil {
				failed, err = 0, perr
				return
			}
			ids[0], start = id, 1
		}
		if h.c.Transport() != client.Stream {
			for i := start; i < len(us); i++ {
				u := us[i]
				id, perr := h.c.Publish(ctx, u)
				if perr != nil {
					failed, err = i, perr
					return
				}
				ids[i] = id
			}
			return
		}
		var mu sync.Mutex
		errs := make([]error, len(us))
		var wg sync.WaitGroup
		for i := start; i < len(us); i++ {
			u := us[i]
			wg.Add(1)
			perr := h.c.PublishAsync(ctx, u, func(id string, e error) {
				mu.Lock()
				ids[i], errs[i] = id, e
				mu.Unlock()
				wg.Done()
			})
			if perr != nil {
				wg.Done()
				mu.Lock()
				errs[i] = perr
				mu.Unlock()
				break
			}
		}
		wg.Wait()
		for i, e := range errs {
			if e != nil {
				failed, err = i, e
				return
			}
		}
	})
	return ids, failed, err
}

// publishError is err as a script error: the run's own stop when its context
// ended, else the hub's answer or what failed.
func publishError(ctx context.Context, err error) object.Object {
	if ctx.Err() != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return scriptlingerrors.NewTimeoutError()
		}
		return scriptlingerrors.NewCancelledError()
	}
	return errObject(err)
}

// errObject is err as a script error: a ValueError for an update the hub
// refused as invalid (400), else an error any except Exception catches.
func errObject(err error) object.Object {
	var e *client.Error
	if errors.As(err, &e) && e.Status == http.StatusBadRequest {
		return scriptlingerrors.NewValueError("%s", err.Error())
	}
	return &object.Error{Message: err.Error()}
}

func prefixError(errObj object.Object, prefix string) object.Object {
	switch e := errObj.(type) {
	case *object.Error:
		c := *e
		c.Message = prefix + c.Message
		return &c
	case *object.Exception:
		c := *e
		c.Message = prefix + c.Message
		return &c
	}
	return errObj
}

// updateFrom builds an update from publish()'s arguments.
func updateFrom(fields map[string]object.Object) (client.Update, object.Object) {
	var u client.Update
	var errObj object.Object
	for k, v := range fields {
		switch k {
		case "topic", "topics":
			var topics []string
			if topics, errObj = stringOrList(v, k); errObj != nil {
				return u, errObj
			}
			u.Topics = append(u.Topics, topics...)
		case "data":
			if u.Data, errObj = dataArg(v); errObj != nil {
				return u, errObj
			}
		case "id", "type":
			s := ""
			switch x := v.(type) {
			case *object.Null:
			case *object.String:
				s, _ = x.AsString()
			default:
				return u, scriptlingerrors.NewTypeError("STRING", v.Type().String())
			}
			if k == "id" {
				u.ID = s
			} else {
				u.Type = s
			}
		case "private":
			if _, null := v.(*object.Null); !null {
				b, ok := v.(*object.Boolean)
				if !ok {
					return u, scriptlingerrors.NewTypeError("BOOLEAN", v.Type().String())
				}
				u.Private, _ = b.AsBool()
			}
		case "retry":
			if _, null := v.(*object.Null); !null {
				secs, err := v.AsFloat()
				if err != nil {
					return u, scriptlingerrors.NewTypeError("FLOAT", v.Type().String())
				}
				if secs < 0 {
					return u, scriptlingerrors.NewValueError("retry must not be negative")
				}
				u.Retry = time.Duration(secs * float64(time.Second))
			}
		case "replicas":
			if _, null := v.(*object.Null); !null {
				n, err := v.AsInt()
				if err != nil {
					return u, scriptlingerrors.NewTypeError("INT", v.Type().String())
				}
				if n < 0 {
					return u, scriptlingerrors.NewValueError("replicas must not be negative")
				}
				u.Replicas = int(n)
			}
		default:
			return u, scriptlingerrors.NewValueError("unexpected argument %q", k)
		}
	}
	if len(u.Topics) == 0 {
		return u, scriptlingerrors.NewValueError("an update needs a topic")
	}
	return u, nil
}

// dataArg is an update's data: a str as it is, None empty, anything else as JSON.
func dataArg(v object.Object) (string, object.Object) {
	switch x := v.(type) {
	case nil, *object.Null:
		return "", nil
	case *object.String:
		s, _ := x.AsString()
		return s, nil
	}
	g, errObj := conversion.ToGoLimited(v)
	if errObj != nil {
		return "", errObj
	}
	b, err := json.Marshal(g)
	if err != nil {
		return "", scriptlingerrors.NewValueError("data: cannot encode as JSON: %s", err.Error())
	}
	return string(b), nil
}

// ── Subscription ────────────────────────────────────────────────────────────

type subscription struct {
	h *hubClient
	s *client.Subscription
}

func (h *hubClient) subscribe(ctx context.Context, spec client.Subscribe) (*subscription, object.Object) {
	if h.closed() {
		return nil, &object.Error{Message: client.ErrClosed.Error()}
	}
	var s *client.Subscription
	var err error
	object.RunBlocking(ctx, func() { s, err = h.c.Subscribe(ctx, spec) })
	if err != nil {
		return nil, publishError(ctx, h.tokens.explain(err))
	}
	sub := &subscription{h: h, s: s}
	h.mu.Lock()
	h.subs[sub] = struct{}{}
	h.mu.Unlock()
	return sub, nil
}

func (s *subscription) close() {
	s.s.Close()
	s.h.mu.Lock()
	delete(s.h.subs, s)
	s.h.mu.Unlock()
}

// drain reads what is left of the events until the subscription has ended,
// or until deadline.
func (s *subscription) drain(deadline <-chan time.Time) {
	for {
		select {
		case _, ok := <-s.s.Events():
			if !ok {
				return
			}
		case <-deadline:
			return
		}
	}
}

// next waits up to timeout (forever when 0) for the next event. ok is false
// on a timeout, ended true once the subscription has ended.
func (s *subscription) next(ctx context.Context, timeout time.Duration) (ev client.Event, ok, ended bool) {
	select { // an event already there needs no wait
	case ev, open := <-s.s.Events():
		return ev, open, !open
	default:
	}
	object.RunBlocking(ctx, func() {
		var timer <-chan time.Time
		if timeout > 0 {
			t := time.NewTimer(timeout)
			defer t.Stop()
			timer = t.C
		}
		select {
		case e, open := <-s.s.Events():
			ev, ok, ended = e, open, !open
		case <-timer:
		case <-ctx.Done():
		}
	})
	return ev, ok, ended
}

// endError is why the subscription ended, as a script error: nil when it was
// closed.
func (s *subscription) endError() object.Object {
	if err := s.s.Err(); err != nil {
		return errObject(fmt.Errorf("subscription ended: %w", s.h.tokens.explain(err)))
	}
	return nil
}

var (
	subscriptionClass     *object.Class
	subscriptionClassOnce sync.Once
)

func getSubscriptionClass() *object.Class {
	subscriptionClassOnce.Do(func() { subscriptionClass = buildSubscriptionClass() })
	return subscriptionClass
}

func buildSubscriptionClass() *object.Class {
	cb := object.NewClassBuilder("Subscription")

	cb.Property("last_event_id", func(self *object.Instance) object.Object {
		s, errObj := impl[*subscription](self)
		if errObj != nil {
			return errObj
		}
		return optionalStringObject(s.s.LastEventID())
	})

	cb.MethodWithHelp("receive", func(self *object.Instance, ctx context.Context, kwargs object.Kwargs) object.Object {
		s, errObj := impl[*subscription](self)
		if errObj != nil {
			return errObj
		}
		timeout := defaultReceiveTimeout
		if v := kwargs.Get("timeout"); v != nil {
			if _, null := v.(*object.Null); null {
				timeout = 0
			} else {
				f, err := v.AsFloat()
				if err != nil {
					return scriptlingerrors.NewTypeError("FLOAT", v.Type().String())
				}
				timeout = max(f, 0)
				if f <= 0 {
					timeout = -1 // no wait: only an event already there
				}
			}
		}
		var ev client.Event
		var ok, ended bool
		if timeout < 0 {
			select {
			case e, open := <-s.s.Events():
				ev, ok, ended = e, open, !open
			default:
			}
		} else {
			ev, ok, ended = s.next(ctx, time.Duration(timeout*float64(time.Second)))
		}
		if errObj := ctxError(ctx); errObj != nil {
			return errObj
		}
		if ended {
			if errObj := s.endError(); errObj != nil {
				return errObj
			}
			return &object.Null{}
		}
		if !ok {
			return &object.Null{}
		}
		return newEvent(ev)
	}, `receive(timeout=30) - The next event, or None when none comes within timeout seconds

Parameters:
  timeout (float, optional): Seconds to wait. Default: 30. None waits until an event comes or the
    subscription is closed; 0 takes only an event already received

Returns:
  Event, or None on a timeout or once the subscription is closed. Raises when the hub refused the
  subscription for good once reconnecting (its token expired, say)`)

	cb.MethodWithHelp("__iter__", func(self *object.Instance, ctx context.Context) object.Object {
		s, errObj := impl[*subscription](self)
		if errObj != nil {
			return errObj
		}
		return object.NewIterator(func() (object.Object, bool) {
			ev, ok, ended := s.next(ctx, 0)
			if errObj := ctxError(ctx); errObj != nil {
				return errObj, true
			}
			if ended {
				if errObj := s.endError(); errObj != nil {
					return errObj, true
				}
				return nil, false
			}
			if !ok {
				return nil, false
			}
			return newEvent(ev), true
		})
	}, `__iter__() - Iterate over the events as they come, until the subscription is closed`)

	cb.MethodWithHelp("close", func(self *object.Instance) object.Object {
		s, errObj := impl[*subscription](self)
		if errObj != nil {
			return errObj
		}
		s.close()
		return &object.Null{}
	}, `close() - End the subscription`)

	cb.MethodWithHelp("__enter__", func(self *object.Instance) object.Object {
		return self
	}, `__enter__() - Use the subscription in a with block; it is closed at its end`)

	cb.MethodWithHelp("__exit__", func(self *object.Instance, args ...object.Object) object.Object {
		s, errObj := impl[*subscription](self)
		if errObj != nil {
			return errObj
		}
		s.close()
		return object.NewBoolean(false)
	}, `__exit__(*args) - End the subscription at the end of a with block`)

	return cb.Build()
}

// ctxError is the run's own stop once its context ended, else nil.
func ctxError(ctx context.Context) object.Object {
	switch ctx.Err() {
	case nil:
		return nil
	case context.DeadlineExceeded:
		return scriptlingerrors.NewTimeoutError()
	}
	return scriptlingerrors.NewCancelledError()
}

// ── Event ───────────────────────────────────────────────────────────────────

var (
	eventClass     *object.Class
	eventClassOnce sync.Once
)

func getEventClass() *object.Class {
	eventClassOnce.Do(func() { eventClass = buildEventClass() })
	return eventClass
}

func newEvent(ev client.Event) object.Object {
	var retry object.Object = &object.Null{}
	if ev.Retry > 0 {
		retry = object.NewFloat(ev.Retry.Seconds())
	}
	return object.NewInstanceWithFields(getEventClass(), map[string]object.Object{
		"id":    object.NewString(ev.ID),
		"type":  optionalStringObject(ev.Type),
		"data":  object.NewString(ev.Data),
		"retry": retry,
	})
}

func buildEventClass() *object.Class {
	cb := object.NewClassBuilder("Event")

	cb.MethodWithHelp("json", func(self *object.Instance) object.Object {
		data, _ := self.Field("data").AsString()
		v, err := conversion.ParseJSON(data)
		if err != nil {
			return scriptlingerrors.NewValueError("event data is not JSON: %s", err.Error())
		}
		return v
	}, `json() - The event's data parsed as JSON

Returns:
  The decoded value (dict, list, str, int, float, bool or None)`)

	cb.MethodWithHelp("to_dict", func(self *object.Instance) object.Object {
		d := object.NewStringDict(nil)
		for _, k := range []string{"id", "type", "data", "retry"} {
			d.SetByString(k, self.Field(k))
		}
		return d
	}, `to_dict() - The event as a dict: id, type, data and retry`)

	cb.MethodWithHelp("__repr__", func(self *object.Instance) object.Object {
		id, _ := self.Field("id").AsString()
		data, _ := self.Field("data").AsString()
		if len(data) > 60 {
			data = data[:57] + "..."
		}
		return object.NewString(fmt.Sprintf("Event(id=%q, data=%q)", id, data))
	}, `__repr__() - The event, briefly`)

	return cb.Build()
}

// ── Token ───────────────────────────────────────────────────────────────────

func createToken(kwargs object.Kwargs, key string, rest []string) object.Object {
	audience, errObj := optionalString(kwargs, "audience")
	if errObj != nil {
		return errObj
	}
	switch {
	case len(rest) > 1:
		return scriptlingerrors.NewArgumentError(len(rest)+1, 2)
	case len(rest) == 1 && kwargs.Has("audience"):
		return scriptlingerrors.NewValueError("create_token: audience given both by position and by name")
	case len(rest) == 1:
		audience = rest[0]
	}
	spec, errObj := parseTokenSpec(kwargs, key, audience, "expires", false)
	if errObj != nil {
		return errObj
	}
	tok, err := spec.mint(time.Now())
	if err != nil {
		return scriptlingerrors.NewValueError("create_token: %s", err.Error())
	}
	return object.NewString(tok)
}

// selectors reads a token's selectors: absent (nil), or a list (empty allowed).
func selectors(kwargs object.Kwargs, name string) ([]string, object.Object) {
	v := kwargs.Get(name)
	if v == nil {
		return nil, nil
	}
	if _, null := v.(*object.Null); null {
		return nil, nil
	}
	s, errObj := stringOrList(v, name)
	if errObj != nil {
		return nil, errObj
	}
	if s == nil {
		s = []string{}
	}
	return s, nil
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func newInstance(class *object.Class, typeName string, v any) *object.Instance {
	return object.NewInstanceWithFields(class, map[string]object.Object{
		"_impl": &object.ClientWrapper{TypeName: typeName, Client: v},
	})
}

func impl[T any](self *object.Instance) (T, object.Object) {
	var zero T
	w, ok := object.GetClientField(self, "_impl")
	if !ok || w.Client == nil {
		return zero, &object.Error{Message: "mercure: missing internal state"}
	}
	v, ok := w.Client.(T)
	if !ok {
		return zero, &object.Error{Message: "mercure: invalid internal state"}
	}
	return v, nil
}

// optionalString reads a string keyword argument that may be None (not given).
func optionalString(kwargs object.Kwargs, name string) (string, object.Object) {
	if v := kwargs.Get(name); v == nil {
		return "", nil
	} else if _, null := v.(*object.Null); null {
		return "", nil
	}
	return kwargs.GetString(name, "")
}

func optionalStringObject(s string) object.Object {
	if s == "" {
		return &object.Null{}
	}
	return object.NewString(s)
}

func stringOrList(v object.Object, name string) ([]string, object.Object) {
	switch x := v.(type) {
	case nil, *object.Null:
		return nil, nil
	case *object.String:
		s, _ := x.AsString()
		if s == "" {
			return nil, nil
		}
		return []string{s}, nil
	case *object.List, *object.Tuple:
		var elems []object.Object
		if l, ok := x.(*object.List); ok {
			elems = l.Elements
		} else {
			elems = x.(*object.Tuple).Elements
		}
		out := make([]string, 0, len(elems))
		for _, e := range elems {
			s, err := e.AsString()
			if err != nil {
				return nil, scriptlingerrors.NewValueError("%s: every item must be a string", name)
			}
			out = append(out, s)
		}
		return out, nil
	}
	return nil, scriptlingerrors.NewTypeError("STRING or LIST", v.Type().String())
}

func headersArg(v object.Object) (http.Header, object.Object) {
	switch v.(type) {
	case nil, *object.Null:
		return nil, nil
	}
	d, err := v.AsDict()
	if err != nil {
		return nil, scriptlingerrors.NewTypeError("DICT", v.Type().String())
	}
	h := http.Header{}
	for k, val := range d {
		s, err := val.AsString()
		if err != nil {
			return nil, scriptlingerrors.NewValueError("headers: the value of %q must be a string", k)
		}
		h.Set(k, s)
	}
	return h, nil
}
