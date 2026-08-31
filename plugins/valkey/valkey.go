// Package valkey is the valkey/redis key-value plugin. Scripts import it as
// scriptling.valkey:
//
//	import scriptling.valkey as valkey
//	client = valkey.connect("valkey://localhost:6379")
//	client.set("greeting", "hello", ttl_seconds=60)
//	print(client.get("greeting"))
//	client.close()
//
// URLs accept the schemes valkey://, redis:// and tcp:// (plaintext) and
// valkeys://, rediss:// (TLS), with optional user:pass@ and a /db path, and
// take one address or a comma-separated seed list for clusters and sentinels.
// Every address must pass the host's network policy. The API is mirrored
// exactly by the badgerdb plugin, so scripts can switch between a shared cache
// and local storage unchanged. The same library serves external plugin mode
// (plugins/valkey/cmd) and compiled-in registration (build tag plugin_valkey).
package valkey

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/valkey-io/valkey-go"

	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/plugin"
	"github.com/paularlott/scriptling/plugins/internal/kv"
	"github.com/paularlott/scriptling/plugins/internal/kwarg"
)

// Description is the plugin metadata description.
const Description = "Valkey and Redis key-value client"

// ConnectSource is the scriptling-source wrapper for connect() in external
// plugin mode, where a Go function cannot return an instance over the wire:
// it constructs the Client class through the plugin object protocol.
const ConnectSource = `def connect(url="valkey://localhost:6379", mode="single", master_set="mymaster"):
    return Client(url, mode=mode, master_set=master_set)
`

const connectHelp = `connect(url="valkey://localhost:6379", mode="single", master_set="mymaster") -> Client

Connect to Valkey or Redis and return a Client. Accepted schemes:
valkey://, redis://, tcp:// (plaintext) and valkeys://, rediss:// (TLS).
Optional user:pass@ credentials and a /db number are honoured. The url takes
one address or a comma-separated seed list (cluster, sentinel).

mode picks the client shape: "single" (the default) talks straight to the
one server named in the url; "cluster" treats the addresses as cluster
seeds and follows the topology; "sentinel" treats them as sentinels and
follows the master named by master_set (default "mymaster"); "auto" asks
the server and builds a cluster client when it answers like one, falling
back to a single connection otherwise. Every dial, including the nodes
topology discovery returns, must pass the host's network policy.`

// Build returns the scriptling.valkey library. policy is read at call time so an
// external plugin sees the policy its handshake delivered.
func Build(policy plugin.PolicySource) *object.Library {
	clientClass := kv.ClientClass(func(kwargs object.Kwargs, url string) (*kv.Client, error) {
		mode, errObj := kwargs.GetString("mode", "single")
		if errObj != nil {
			return nil, kwarg.Err(errObj)
		}
		masterSet, errObj := kwargs.GetString("master_set", "mymaster")
		if errObj != nil {
			return nil, kwarg.Err(errObj)
		}
		return connect(policy, url, mode, masterSet)
	}, registerValkeyExtras).Build()

	functions := map[string]*object.Builtin{
		"connect": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) > 1 {
					return &object.Error{Message: fmt.Sprintf("connect takes at most 1 positional argument (url), got %d", len(args))}
				}
				serverURL := "valkey://localhost:6379"
				if len(args) == 1 {
					value, err := args[0].AsString()
					if err != nil {
						return err
					}
					serverURL = value
				}
				mode, errObj := kwargs.GetString("mode", "single")
				if errObj != nil {
					return errObj
				}
				masterSet, errObj := kwargs.GetString("master_set", "mymaster")
				if errObj != nil {
					return errObj
				}
				if err := validateMode(mode); err != nil {
					return &object.Error{Message: err.Error()}
				}
				client, err := connect(policy, serverURL, mode, masterSet)
				if err != nil {
					return &object.Error{Message: err.Error()}
				}
				return object.NewReceiverInstance(clientClass, "Client", client)
			},
			HelpText: connectHelp,
		},
	}
	constants := map[string]object.Object{"Client": clientClass}
	return object.NewLibrary(plugin.NormalizeLibraryName("scriptling.valkey"), functions, constants, Description)
}

func validateMode(mode string) error {
	switch mode {
	case "auto", "single", "cluster", "sentinel":
		return nil
	default:
		return fmt.Errorf("connect mode must be auto, single, cluster or sentinel, got %q", mode)
	}
}

func connect(policy plugin.PolicySource, serverURL, mode, masterSet string) (*kv.Client, error) {
	if err := validateMode(mode); err != nil {
		return nil, err
	}
	opts, err := parseURL(serverURL)
	if err != nil {
		return nil, err
	}
	client, err := dial(policy, opts, opts.db, mode, masterSet)
	if err != nil {
		return nil, err
	}
	st := &store{client: client, opts: opts, db: opts.db, policySource: policy, mode: mode, masterSet: masterSet}
	st.pools = map[int]valkey.Client{opts.db: client}
	return &kv.Client{Store: st}, nil
}

// dial builds a client for one database index. mode shapes it: the auto and
// cluster paths let the client discover and follow the topology (auto falls
// back to a plain connection when the server answers that it is not a
// cluster), single pins one server, sentinel follows the master named by
// masterSet. RESP2 with client-side caching off keeps the plugin compatible
// across valkey and redis builds without costing it anything (it does no
// response caching); the guarded dialer applies the host network policy to
// every dial, including nodes the topology discovery returns.
func dial(policy plugin.PolicySource, opts serverOptions, db int, mode, masterSet string) (valkey.Client, error) {
	if mode == "cluster" && db != 0 {
		return nil, fmt.Errorf("cluster mode has a single database (0)")
	}
	if mode == "sentinel" {
		if masterSet == "" {
			return nil, fmt.Errorf("sentinel mode requires a master_set")
		}
		if db != 0 {
			return nil, fmt.Errorf("sentinel mode addresses database 0 of the master (the client cannot replay a select across failover)")
		}
	}
	option := valkey.ClientOption{
		InitAddress: opts.addresses,
		Username:    opts.username,
		Password:    opts.password,
		SelectDB:    db,
		// RESP2 with client-side caching off: skips the CLIENT TRACKING
		// handshake, which older redis and valkey builds reject, without
		// costing this plugin anything (it does no response caching).
		AlwaysRESP2:  true,
		DisableCache: true,
	}
	switch mode {
	case "single":
		option.ForceSingleClient = true
	case "sentinel":
		option.Sentinel = valkey.SentinelOption{MasterSet: masterSet}
	}
	if opts.tls {
		// ServerName is left empty: the guarded dialer derives it from each
		// address it dials, so seed lists validate every host's certificate
		// against its own name.
		option.TLSConfig = &tls.Config{}
	}
	if guard, guardErr := policy.Policy().Guard(); guardErr != nil {
		return nil, guardErr
	} else if guard != nil {
		// With a custom DialCtxFn the client delegates the whole connection
		// (TLS included) to us, so policy enforcement covers the dial, the
		// cluster and sentinel nodes discovered later, and we wrap TLS
		// ourselves for valkeys:// URLs.
		option.DialCtxFn = func(ctx context.Context, dst string, _ *net.Dialer, tlsConfig *tls.Config) (net.Conn, error) {
			conn, dialErr := guard.DialContext(ctx, "tcp", dst)
			if dialErr != nil {
				return nil, dialErr
			}
			if tlsConfig == nil {
				return conn, nil
			}
			if tlsConfig.ServerName == "" {
				host, _, _ := net.SplitHostPort(dst)
				wrapped := tlsConfig.Clone()
				wrapped.ServerName = host
				tlsConfig = wrapped
			}
			tlsConn := tls.Client(conn, tlsConfig)
			if handshakeErr := tlsConn.HandshakeContext(ctx); handshakeErr != nil {
				_ = conn.Close()
				return nil, handshakeErr
			}
			return tlsConn, nil
		}
	}

	client, err := valkey.NewClient(option)
	if err != nil {
		return nil, fmt.Errorf("connect valkey: %w", err)
	}
	if err := client.Do(context.Background(), client.B().Ping().Build()).Error(); err != nil {
		client.Close()
		return nil, fmt.Errorf("connect valkey: %w", err)
	}
	return client, nil
}

type serverOptions struct {
	addresses []string // seed addresses; one for standalone, several for cluster and sentinel
	username  string
	password  string
	db        int
	tls       bool
}

// parseURL accepts one address or a comma-separated seed list
// ("valkey://node-a:7000,node-b:7000,node-c:7000"). Credentials and the /db
// path are shared by every address; schemes are all-or-nothing.
func parseURL(serverURL string) (serverOptions, error) {
	raw := serverURL
	if !strings.Contains(raw, "://") {
		raw = "valkey://" + raw
	}
	scheme, rest, ok := strings.Cut(raw, "://")
	if !ok {
		return serverOptions{}, fmt.Errorf("invalid valkey url: %q", serverURL)
	}

	var opts serverOptions
	switch strings.ToLower(scheme) {
	case "valkey", "redis", "tcp":
		opts.tls = false
	case "valkeys", "rediss", "tcps":
		opts.tls = true
	default:
		return serverOptions{}, fmt.Errorf("unsupported valkey url scheme %q (use valkey:// or valkeys://)", scheme)
	}

	seenDB := false
	for _, piece := range strings.Split(rest, ",") {
		piece = strings.TrimSpace(piece)
		if piece == "" {
			return serverOptions{}, fmt.Errorf("empty address in valkey url %q", serverURL)
		}
		if strings.Contains(piece, "://") {
			return serverOptions{}, fmt.Errorf("one scheme covers every address in valkey url %q", serverURL)
		}
		parsed, err := url.Parse(scheme + "://" + piece)
		if err != nil {
			return serverOptions{}, fmt.Errorf("invalid valkey url: %w", err)
		}
		host := parsed.Hostname()
		if host == "" {
			host = "localhost"
		}
		port := parsed.Port()
		if port == "" {
			port = "6379"
		}
		if parsed.User != nil {
			opts.username = parsed.User.Username()
			opts.password, _ = parsed.User.Password()
		}
		if db := strings.TrimPrefix(parsed.Path, "/"); db != "" && !seenDB {
			n, convErr := strconv.Atoi(db)
			if convErr != nil || n < 0 {
				return serverOptions{}, fmt.Errorf("invalid database %q in valkey url", db)
			}
			opts.db = n
			seenDB = true
		}
		opts.addresses = append(opts.addresses, net.JoinHostPort(host, port))
	}
	return opts, nil
}

// registerValkeyExtras adds the valkey/redis-specific surface beyond the
// badgerdb-mirrored core: database selection, sets and queues. badgerdb has
// no native equivalents, so these exist only here.
func registerValkeyExtras(cb *object.ClassBuilder) {
	cb.MethodWithHelp("select", func(self *kv.Client, ctx context.Context, index int64) error {
		vs, ok := self.Store.(*store)
		if !ok {
			return fmt.Errorf("select requires a valkey client")
		}
		if err := vs.selectDB(index); err != nil {
			return fmt.Errorf("select %d: %w", index, err)
		}
		return nil
	}, `select(index) - Switch the connection to a different database.

Subsequent commands address the new database. Each database is dialed
once and its pool kept, so switching back to a database you already used
is instant.`)
	cb.MethodWithHelp("db", func(self *kv.Client) (int64, error) {
		vs, ok := self.Store.(*store)
		if !ok {
			return 0, fmt.Errorf("db requires a valkey client")
		}
		return int64(vs.db), nil
	}, "db() -> int - The database index this client currently addresses.")
	cb.MethodWithHelp("mode", func(self *kv.Client) (string, error) {
		vs, ok := self.Store.(*store)
		if !ok {
			return "", fmt.Errorf("mode requires a valkey client")
		}
		return string(vs.active().Mode()), nil
	}, `mode() -> str - How the client is talking to the server: "standalone", "cluster" or "sentinel".

With the default mode="auto" this reports what the connection resolved to.`)
	cb.MethodWithHelp("flushdb", func(self *kv.Client, ctx context.Context) error {
		vs, ok := self.Store.(*store)
		if !ok {
			return fmt.Errorf("flushdb requires a valkey client")
		}
		return vs.flushNodes(ctx, func(b valkey.Builder) valkey.Completed { return b.Flushdb().Build() })
	}, `flushdb() - Delete every key in the current database.

On a cluster the command reaches every node that accepts writes (replicas
need no flush; they mirror their master). Destructive: the data is gone.`)
	cb.MethodWithHelp("flushall", func(self *kv.Client, ctx context.Context) error {
		vs, ok := self.Store.(*store)
		if !ok {
			return fmt.Errorf("flushall requires a valkey client")
		}
		return vs.flushNodes(ctx, func(b valkey.Builder) valkey.Completed { return b.Flushall().Build() })
	}, `flushall() - Delete every key in every database on the server.

On a cluster the command reaches every node that accepts writes (replicas
need no flush; they mirror their master). More destructive than flushdb:
prefer it unless you truly mean every database.`)

	cb.MethodWithHelp("set_add", func(self *kv.Client, ctx context.Context, key string, members ...string) (int64, error) {
		c := self.Store.(*store).active()
		return c.Do(ctx, c.B().Sadd().Key(key).Member(members...).Build()).ToInt64()
	}, "set_add(key, *members) -> int - Add members to a set; returns how many were new.")
	cb.MethodWithHelp("set_remove", func(self *kv.Client, ctx context.Context, key string, members ...string) (int64, error) {
		c := self.Store.(*store).active()
		return c.Do(ctx, c.B().Srem().Key(key).Member(members...).Build()).ToInt64()
	}, "set_remove(key, *members) -> int - Remove members from a set; returns how many existed.")
	cb.MethodWithHelp("set_members", func(self *kv.Client, ctx context.Context, key string) ([]any, error) {
		c := self.Store.(*store).active()
		members, err := c.Do(ctx, c.B().Smembers().Key(key).Build()).AsStrSlice()
		if err != nil {
			return nil, fmt.Errorf("set_members %s: %w", key, err)
		}
		out := make([]any, 0, len(members))
		for _, m := range members {
			out = append(out, m)
		}
		return out, nil
	}, "set_members(key) -> list[str] - Every member of the set, unordered.")
	cb.MethodWithHelp("set_contains", func(self *kv.Client, ctx context.Context, key string, member string) (bool, error) {
		c := self.Store.(*store).active()
		reply, err := c.Do(ctx, c.B().Sismember().Key(key).Member(member).Build()).ToInt64()
		if err != nil {
			return false, fmt.Errorf("set_contains %s: %w", key, err)
		}
		return reply == 1, nil
	}, "set_contains(key, member) -> bool - Whether member is in the set.")
	cb.MethodWithHelp("set_size", func(self *kv.Client, ctx context.Context, key string) (int64, error) {
		c := self.Store.(*store).active()
		return c.Do(ctx, c.B().Scard().Key(key).Build()).ToInt64()
	}, "set_size(key) -> int - Number of members in the set.")

	cb.MethodWithHelp("queue_push", func(self *kv.Client, ctx context.Context, key string, values ...string) (int64, error) {
		c := self.Store.(*store).active()
		return c.Do(ctx, c.B().Rpush().Key(key).Element(values...).Build()).ToInt64()
	}, `queue_push(key, *values) -> int - Push values onto the queue's tail; returns the queue length.

FIFO with queue_pop: producers push right, consumers pop left.`)
	cb.MethodWithHelp("queue_pop", func(self *kv.Client, ctx context.Context, key string) (any, error) {
		c := self.Store.(*store).active()
		value, err := c.Do(ctx, c.B().Lpop().Key(key).Build()).ToString()
		if errors.Is(err, valkey.Nil) {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("queue_pop %s: %w", key, err)
		}
		return value, nil
	}, "queue_pop(key) -> str | null - Pop the value at the queue's head, or null when empty.")
	cb.MethodWithHelp("queue_peek", func(self *kv.Client, ctx context.Context, key string) (any, error) {
		c := self.Store.(*store).active()
		value, err := c.Do(ctx, c.B().Lindex().Key(key).Index(0).Build()).ToString()
		if errors.Is(err, valkey.Nil) {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("queue_peek %s: %w", key, err)
		}
		return value, nil
	}, "queue_peek(key) -> str | null - The value at the queue's head without removing it.")
	cb.MethodWithHelp("queue_wait", func(self *kv.Client, ctx context.Context, key string, timeout float64) (any, error) {
		if timeout < 0 {
			return nil, fmt.Errorf("queue_wait timeout must be >= 0")
		}
		c := self.Store.(*store).active()
		parts, err := c.Do(ctx, c.B().Blpop().Key(key).Timeout(timeout).Build()).AsStrSlice()
		if errors.Is(err, valkey.Nil) {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("queue_wait %s: %w", key, err)
		}
		// BLPOP answers [key, value]; the key echo confirms which queue fired.
		if len(parts) != 2 {
			return nil, fmt.Errorf("queue_wait %s: unexpected reply", key)
		}
		return parts[1], nil
	}, `queue_wait(key, timeout) -> str | null

Pop the value at the queue's head, waiting up to timeout seconds for one
to arrive (fractional seconds allowed). Returns the value, or null when
the timeout expires. A timeout of 0 behaves like queue_pop. There is no
infinite wait on purpose: a worker loop re-issues queue_wait, which also
keeps the script responsive to cancellation.`)
	cb.MethodWithHelp("queue_size", func(self *kv.Client, ctx context.Context, key string) (int64, error) {
		c := self.Store.(*store).active()
		return c.Do(ctx, c.B().Llen().Key(key).Build()).ToInt64()
	}, "queue_size(key) -> int - Number of values in the queue.")
	cb.MethodWithHelp("queue_range", func(self *kv.Client, ctx context.Context, kwargs object.Kwargs, key string) ([]any, error) {
		start, errObj := kwargs.GetInt("start", 0)
		if errObj != nil {
			return nil, kwarg.Err(errObj)
		}
		stop, errObj := kwargs.GetInt("stop", -1)
		if errObj != nil {
			return nil, kwarg.Err(errObj)
		}
		c := self.Store.(*store).active()
		values, err := c.Do(ctx, c.B().Lrange().Key(key).Start(start).Stop(stop).Build()).AsStrSlice()
		if err != nil {
			return nil, fmt.Errorf("queue_range %s: %w", key, err)
		}
		out := make([]any, 0, len(values))
		for _, v := range values {
			out = append(out, v)
		}
		return out, nil
	}, `queue_range(key, start=0, stop=-1) -> list[str]

Values from the queue in order, head first, without removing them.
Indices work like list slicing: 0 is the head, -1 the tail.`)
}

// store adapts the valkey client to the shared kv.Store surface. It keeps
// one connection pool per database index: the client multiplexes
// connections, so a protocol-level SELECT would race in-flight commands.
// select() swaps which pool commands address; the first switch to a
// database dials it, later switches reuse the cached pool.
type store struct {
	client       valkey.Client
	opts         serverOptions
	db           int
	pools        map[int]valkey.Client
	policySource plugin.PolicySource
	mode         string
	masterSet    string
	mu           sync.Mutex
}

// selectDB switches the database every subsequent command on this client
// addresses. Each database is dialed once and its pool cached, so
// select(0), select(2), select(0) dials twice in total, never per switch.
func (s *store) selectDB(index int64) error {
	if index < 0 {
		return fmt.Errorf("database index must be >= 0")
	}
	db := int(index)
	switch s.active().Mode() {
	case valkey.ClientModeCluster:
		if db != 0 {
			return fmt.Errorf("cluster mode has a single database (0)")
		}
	case valkey.ClientModeSentinel:
		if db != 0 {
			return fmt.Errorf("sentinel mode addresses database 0 of the master (the client cannot replay a select across failover)")
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if client, cached := s.pools[db]; cached {
		s.client = client
		s.db = db
		return nil
	}
	client, err := dial(s.policySource, s.opts, db, s.mode, s.masterSet)
	if err != nil {
		return err
	}
	s.pools[db] = client
	s.client = client
	s.db = db
	return nil
}

func (s *store) active() valkey.Client {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.client
}

func (s *store) Get(ctx context.Context, key string) (string, bool, error) {
	c := s.active()
	value, err := c.Do(ctx, c.B().Get().Key(key).Build()).ToString()
	if errors.Is(err, valkey.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get %s: %w", key, err)
	}
	return value, true, nil
}

func (s *store) Set(ctx context.Context, key, value string, ttlSeconds int64) error {
	var err error
	if ttlSeconds > 0 {
		c := s.active()
		err = c.Do(ctx, c.B().Set().Key(key).Value(value).ExSeconds(ttlSeconds).Build()).Error()
	} else {
		c := s.active()
		err = c.Do(ctx, c.B().Set().Key(key).Value(value).Build()).Error()
	}
	if err != nil {
		return fmt.Errorf("set %s: %w", key, err)
	}
	return nil
}

func (s *store) Delete(ctx context.Context, keys []string) (int64, error) {
	c := s.active()
	removed, err := c.Do(ctx, c.B().Del().Key(keys...).Build()).ToInt64()
	if err != nil {
		return 0, fmt.Errorf("delete: %w", err)
	}
	return removed, nil
}

func (s *store) Exists(ctx context.Context, keys []string) (int64, error) {
	c := s.active()
	count, err := c.Do(ctx, c.B().Exists().Key(keys...).Build()).ToInt64()
	if err != nil {
		return 0, fmt.Errorf("exists: %w", err)
	}
	return count, nil
}

func (s *store) Expire(ctx context.Context, key string, ttlSeconds int64) (bool, error) {
	// RESP2 answers EXPIRE with 0/1 rather than a bool.
	c := s.active()
	reply, err := c.Do(ctx, c.B().Expire().Key(key).Seconds(ttlSeconds).Build()).ToInt64()
	if err != nil {
		return false, fmt.Errorf("expire %s: %w", key, err)
	}
	return reply == 1, nil
}

func (s *store) TTL(ctx context.Context, key string) (int64, bool, error) {
	c := s.active()
	ttl, err := c.Do(ctx, c.B().Ttl().Key(key).Build()).ToInt64()
	if err != nil {
		return 0, false, fmt.Errorf("ttl %s: %w", key, err)
	}
	if ttl == -2 { // redis: key does not exist
		return 0, false, nil
	}
	return ttl, true, nil
}

func (s *store) Incr(ctx context.Context, key string, amount int64) (int64, error) {
	c := s.active()
	value, err := c.Do(ctx, c.B().Incrby().Key(key).Increment(amount).Build()).ToInt64()
	if err != nil {
		return 0, fmt.Errorf("incr %s: %w", key, err)
	}
	return value, nil
}

func (s *store) Keys(ctx context.Context, pattern string) ([]string, error) {
	c := s.active()
	keys, err := c.Do(ctx, c.B().Keys().Pattern(pattern).Build()).AsStrSlice()
	if err != nil {
		return nil, fmt.Errorf("keys %s: %w", pattern, err)
	}
	return keys, nil
}

func (s *store) Persist(ctx context.Context, key string) (bool, error) {
	c := s.active()
	reply, err := c.Do(ctx, c.B().Persist().Key(key).Build()).ToInt64()
	if err != nil {
		return false, fmt.Errorf("persist %s: %w", key, err)
	}
	return reply == 1, nil
}

func (s *store) MGet(ctx context.Context, keys []string) ([]*string, error) {
	if len(keys) == 0 {
		return []*string{}, nil
	}
	c := s.active()
	array, err := c.Do(ctx, c.B().Mget().Key(keys...).Build()).ToArray()
	if err != nil {
		return nil, fmt.Errorf("mget: %w", err)
	}
	values := make([]*string, len(array))
	for i, msg := range array {
		if msg.IsNil() {
			continue
		}
		value, err := msg.ToString()
		if err != nil {
			return nil, fmt.Errorf("mget: %w", err)
		}
		values[i] = &value
	}
	return values, nil
}

func (s *store) MSet(ctx context.Context, mapping map[string]string, ttlSeconds int64) error {
	if len(mapping) == 0 {
		return nil
	}
	// MSET cannot carry an expiry; with one, the EXPIREs ride behind it in a
	// single pipeline so the batch still costs one round trip.
	mset := s.active().B().Mset().KeyValue()
	for key, value := range mapping {
		mset = mset.KeyValue(key, value)
	}
	if ttlSeconds <= 0 {
		if err := s.active().Do(ctx, mset.Build()).Error(); err != nil {
			return fmt.Errorf("mset: %w", err)
		}
		return nil
	}
	cmds := []valkey.Completed{mset.Build()}
	for key := range mapping {
		cmds = append(cmds, s.active().B().Expire().Key(key).Seconds(ttlSeconds).Build())
	}
	for _, resp := range s.active().DoMulti(ctx, cmds...) {
		if err := resp.Error(); err != nil {
			return fmt.Errorf("mset: %w", err)
		}
	}
	return nil
}

func (s *store) SetNX(ctx context.Context, key, value string, ttlSeconds int64) (bool, error) {
	var cmd valkey.Completed
	if ttlSeconds > 0 {
		cmd = s.active().B().Set().Key(key).Value(value).Nx().ExSeconds(ttlSeconds).Build()
	} else {
		cmd = s.active().B().Set().Key(key).Value(value).Nx().Build()
	}
	reply := s.active().Do(ctx, cmd)
	if errors.Is(reply.Error(), valkey.Nil) {
		return false, nil // NX refused: the key exists
	}
	if err := reply.Error(); err != nil {
		return false, fmt.Errorf("set_if_absent %s: %w", key, err)
	}
	return true, nil
}

func (s *store) HashSet(ctx context.Context, key, field, value string) (int64, error) {
	c := s.active()
	added, err := c.Do(ctx, c.B().Hset().Key(key).FieldValue().FieldValue(field, value).Build()).ToInt64()
	if err != nil {
		return 0, fmt.Errorf("hash_set %s: %w", key, err)
	}
	return added, nil
}

func (s *store) HashGet(ctx context.Context, key, field string) (*string, error) {
	c := s.active()
	value, err := c.Do(ctx, c.B().Hget().Key(key).Field(field).Build()).ToString()
	if errors.Is(err, valkey.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("hash_get %s: %w", key, err)
	}
	return &value, nil
}

func (s *store) HashDelete(ctx context.Context, key string, fields []string) (int64, error) {
	c := s.active()
	removed, err := c.Do(ctx, c.B().Hdel().Key(key).Field(fields...).Build()).ToInt64()
	if err != nil {
		return 0, fmt.Errorf("hash_delete %s: %w", key, err)
	}
	return removed, nil
}

func (s *store) HashAll(ctx context.Context, key string) (map[string]string, error) {
	c := s.active()
	hash, err := c.Do(ctx, c.B().Hgetall().Key(key).Build()).AsStrMap()
	if err != nil {
		return nil, fmt.Errorf("hash_all %s: %w", key, err)
	}
	return hash, nil
}

func (s *store) HashSize(ctx context.Context, key string) (int64, error) {
	c := s.active()
	size, err := c.Do(ctx, c.B().Hlen().Key(key).Build()).ToInt64()
	if err != nil {
		return 0, fmt.Errorf("hash_size %s: %w", key, err)
	}
	return size, nil
}

func (s *store) Ping(ctx context.Context) error {
	c := s.active()
	if err := c.Do(ctx, c.B().Ping().Build()).Error(); err != nil {
		return fmt.Errorf("ping: %w", err)
	}
	return nil
}

// flushNodes sends a flush command to every node the client knows: one
// server for standalone and sentinel (the master), every master of the
// cluster for cluster mode. Replicas refuse writes with READONLY; that is
// not a failure, they have nothing to flush and mirror their master.
func (s *store) flushNodes(ctx context.Context, build func(valkey.Builder) valkey.Completed) error {
	var failures []string
	for addr, node := range s.active().Nodes() {
		if err := node.Do(ctx, build(node.B())).Error(); err != nil {
			if strings.Contains(strings.ToUpper(err.Error()), "READONLY") {
				continue
			}
			failures = append(failures, addr+": "+err.Error())
		}
	}
	if len(failures) > 0 {
		sort.Strings(failures)
		return fmt.Errorf("flush: %s", strings.Join(failures, "; "))
	}
	return nil
}

func (s *store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for db, client := range s.pools {
		client.Close()
		delete(s.pools, db)
	}
	return nil
}
