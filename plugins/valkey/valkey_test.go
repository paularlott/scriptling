package valkey

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/valkey-io/valkey-go"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/plugin"
	"github.com/paularlott/scriptling/plugins/internal/plugintest"
	"github.com/paularlott/scriptling/stdlib"
)

func evalInProcess(t *testing.T, policy *plugin.Policy, script string) (object.Object, error) {
	t.Helper()
	p := scriptling.New()
	stdlib.RegisterAll(p)
	p.RegisterLibrary(Build(&plugin.StaticPolicy{P: policy}))
	return p.Eval(script)
}

// kvScript is the shared KV surface; the badger plugin runs the same script
// so the two APIs stay mirrored.
const kvScript = `
client = valkey.connect("__URL__")
client.ping()

client.set("user:1", "ada")
client.set("user:2", "grace")
client.set("temp:x", "gone soon", ttl_seconds=60)

if client.get("user:1") != "ada":
    return "get lost value"
if client.get("missing") != None:
    return "missing key should be null"

if client.exists("user:1", "user:2", "missing") != 2:
    return "exists count wrong"

if client.incr("hits") != 1:
    return "incr from empty should be 1"
if client.incr("hits", amount=5) != 6:
    return "incr by amount wrong"
if client.decr("hits", amount=2) != 4:
    return "decr wrong"

if client.ttl("user:1") != -1:
    return "key without expiry should report -1"
tempTTL = client.ttl("temp:x")
if tempTTL == None or tempTTL <= 0 or tempTTL > 60:
    return "ttl on expiring key wrong: " + str(tempTTL)
if client.ttl("missing") != None:
    return "ttl on missing key should be null"

keys = client.keys("user:*")
if len(keys) != 2:
    return "keys pattern wrong: " + str(keys)

if client.expire("user:1", 120) != True:
    return "expire should succeed on existing key"
if client.expire("missing", 120) != False:
    return "expire should fail on missing key"

# persist: drop the expiry
if client.persist("temp:x") != True:
    return "persist should succeed on existing key"
if client.ttl("temp:x") != -1:
    return "persist should clear the ttl"
if client.persist("missing") != False:
    return "persist should report missing key"

# batch round trips: mset / mget
client.mset({"a:1": "x", "a:2": "y"})
values = client.mget("a:1", "a:2", "a:missing")
if values != ["x", "y", None]:
    return "mget: " + str(values)
if client.get("a:1") != "x":
    return "mset did not store"
client.mset({"b:1": "x", "b:2": "y"}, ttl_seconds=120)
bttl = client.ttl("b:1")
if bttl == None or bttl <= 0 or bttl > 120:
    return "mset ttl_seconds: " + str(bttl)
client.delete("a:1", "a:2", "b:1", "b:2")

# set_if_absent: the take-once primitive
if client.set_if_absent("lock:1", "owner") != True:
    return "set_if_absent should store into empty key"
if client.set_if_absent("lock:1", "other") != False:
    return "set_if_absent should refuse existing key"
if client.get("lock:1") != "owner":
    return "set_if_absent lost the original value"
if client.set_if_absent("lock:2", "owner", ttl_seconds=120) != True:
    return "set_if_absent with ttl"
lttl = client.ttl("lock:2")
if lttl == None or lttl <= 0 or lttl > 120:
    return "set_if_absent ttl: " + str(lttl)
client.delete("lock:1", "lock:2")

# hashes: field-value pairs under one key
if client.hash_set("session:1", "user", "ada") != 1:
    return "hash_set new field should be 1"
if client.hash_set("session:1", "user", "ada") != 0:
    return "hash_set overwrite should be 0"
if client.hash_set("session:1", "role", "admin") != 1:
    return "hash_set second field"
if client.hash_get("session:1", "user") != "ada":
    return "hash_get"
if client.hash_get("session:1", "nobody") != None:
    return "hash_get missing field"
if client.hash_get("session:9", "user") != None:
    return "hash_get missing key"
if client.hash_size("session:1") != 2:
    return "hash_size: " + str(client.hash_size("session:1"))
if client.hash_size("session:9") != 0:
    return "hash_size missing key"
hash = client.hash_all("session:1")
if hash != {"user": "ada", "role": "admin"}:
    return "hash_all: " + str(hash)
if client.hash_all("session:9") != {}:
    return "hash_all missing key"
if client.hash_delete("session:1", "nobody", "role") != 1:
    return "hash_delete count"
if client.hash_all("session:1") != {"user": "ada"}:
    return "hash after delete"
# expiry covers the whole hash and survives field writes
client.expire("session:1", 300)
client.hash_set("session:1", "role", "dev")
sttl = client.ttl("session:1")
if sttl == None or sttl <= 0 or sttl > 300:
    return "hash ttl lost across hash_set: " + str(sttl)
client.persist("session:1")
# the key disappears with its last field
client.hash_delete("session:1", "user", "role")
if client.exists("session:1") != 0:
    return "hash key should vanish with its last field"

removed = client.delete("user:1", "user:2", "missing")
if removed != 2:
    return "delete count wrong: " + str(removed)

client.close()
return "ok"
`

// loopbackPolicy allows 127.0.0.1 — miniredis only listens on loopback.
func loopbackPolicy() *plugin.Policy {
	return &plugin.Policy{Network: &plugin.NetworkPolicy{AllowLoopback: true}}
}

func TestInProcessKVSurface(t *testing.T) {
	server := miniredis.RunT(t)
	script := "import scriptling.valkey as valkey\n" + strings.ReplaceAll(kvScript, "__URL__", "valkey://"+server.Addr())

	result, err := evalInProcess(t, loopbackPolicy(), script)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "ok" {
		t.Fatalf("script result: %s", result.Inspect())
	}
}

func TestInProcessNetworkPolicyDeniesLoopback(t *testing.T) {
	server := miniredis.RunT(t)
	// Network policy enabled but loopback not allowed: the guarded dialer
	// must refuse the connection before any valkey traffic.
	_, err := evalInProcess(t, &plugin.Policy{Network: &plugin.NetworkPolicy{}}, `
import scriptling.valkey as valkey
client = valkey.connect("valkey://`+server.Addr()+`")
`)
	if err == nil {
		t.Fatal("expected network policy to deny the loopback connection")
	}
	if !strings.Contains(err.Error(), "connect valkey") {
		t.Fatalf("expected connect error, got: %v", err)
	}
}

func TestInProcessNetworkPolicyAllowHosts(t *testing.T) {
	server := miniredis.RunT(t)
	_, port, _ := splitHostPort(server.Addr())
	// AllowHosts trusts the named host; its resolved IPs bypass category
	// checks (that is the documented way to grant internal-service access).
	allowHost := &plugin.Policy{Network: &plugin.NetworkPolicy{AllowHosts: []string{"localhost"}}}
	script := `
import scriptling.valkey as valkey
client = valkey.connect("valkey://localhost:` + port + `")
client.set("k", "v")
value = client.get("k")
client.close()
return value
`
	result, err := evalInProcess(t, allowHost, script)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "v" {
		t.Fatalf("script result: %s", result.Inspect())
	}
}

func TestInProcessURLForms(t *testing.T) {
	plain := miniredis.RunT(t)
	authed := miniredis.RunT(t)
	authed.RequireAuth("secret")

	for _, tt := range []struct {
		name string
		url  string
		ok   bool
	}{
		{name: "redis scheme", url: "redis://" + plain.Addr(), ok: true},
		{name: "tcp scheme", url: "tcp://" + plain.Addr(), ok: true},
		{name: "bare host:port", url: plain.Addr(), ok: true},
		{name: "password", url: "valkey://:secret@" + authed.Addr(), ok: true},
		{name: "bad password", url: "valkey://:wrong@" + authed.Addr(), ok: false},
		{name: "db select", url: "valkey://" + plain.Addr() + "/1", ok: true},
		{name: "bad db", url: "valkey://" + plain.Addr() + "/notanumber", ok: false},
		{name: "bad scheme", url: "http://" + plain.Addr(), ok: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := evalInProcess(t, loopbackPolicy(), `
import scriptling.valkey as valkey
client = valkey.connect("`+tt.url+`")
client.ping()
client.close()
return "ok"
`)
			if tt.ok && err != nil {
				t.Fatalf("expected success, got: %v", err)
			}
			if !tt.ok && err == nil {
				t.Fatal("expected failure, got success")
			}
		})
	}
}

func TestExternalKVSurface(t *testing.T) {
	server := miniredis.RunT(t)
	script := "import scriptling.valkey as valkey\n" + strings.ReplaceAll(kvScript, "__URL__", "valkey://"+server.Addr())

	bin := plugintest.BuildPlugin(t, "./cmd")
	result, err := plugintest.External(t, bin, loopbackPolicy(), script)
	if err != nil {
		t.Fatalf("external eval: %v", err)
	}
	if result.Inspect() != "ok" {
		t.Fatalf("script result: %s", result.Inspect())
	}
}

func TestExternalNetworkPolicyDeliveredByHandshake(t *testing.T) {
	server := miniredis.RunT(t)
	bin := plugintest.BuildPlugin(t, "./cmd")
	// The policy travels in the handshake; the plugin's guarded dialer must
	// refuse loopback even though the plugin process could reach it.
	_, err := plugintest.External(t, bin, &plugin.Policy{Network: &plugin.NetworkPolicy{}}, `
import scriptling.valkey as valkey
client = valkey.connect("valkey://`+server.Addr()+`")
`)
	if err == nil {
		t.Fatal("expected handshake-delivered policy to deny the loopback connection")
	}
	if !strings.Contains(err.Error(), "connect valkey") {
		t.Fatalf("expected connect error, got: %v", err)
	}
}

// extrasScript covers the valkey-only surface: database selection, sets,
// queues. miniredis backs all of it.
const extrasScript = `
client = valkey.connect("__URL__")
client.ping()

# database selection
client.set("home", "db0")
client.select(2)
if client.db() != 2:
    return "db() after select: " + str(client.db())
client.set("away", "db2")
if client.get("home") != None:
    return "select did not switch database"
client.select(0)
if client.get("home") != "db0":
    return "select back lost data"
if client.get("away") != None:
    return "db0 sees db2 keys"

# switching back to a used database reuses its pool: data still there,
# and the round trip works without any re-dial
client.select(2)
if client.get("away") != "db2":
    return "cached pool lost db2 data"
client.select(0)
if client.db() != 0 or client.get("home") != "db0":
    return "cached pool back to db0 failed"

# sets
if client.set_add("crew", "ada", "grace", "ada") != 2:
    return "set_add count"
if client.set_size("crew") != 2:
    return "set_size: " + str(client.set_size("crew"))
if client.set_contains("crew", "ada") != True or client.set_contains("crew", "linus") != False:
    return "set_contains"
if client.set_add("crew", "linus") != 1:
    return "set_add new member"
members = client.set_members("crew")
if len(members) != 3 or "linus" not in members:
    return "set_members: " + str(members)
if client.set_remove("crew", "nobody", "ada") != 1:
    return "set_remove count"
if client.set_size("crew") != 2:
    return "set after remove"

# queues: FIFO
client.delete("jobs")
if client.queue_push("jobs", "a", "b", "c") != 3:
    return "queue_push length"
if client.queue_size("jobs") != 3:
    return "queue_size"
if client.queue_peek("jobs") != "a":
    return "queue_peek"
if client.queue_pop("jobs") != "a":
    return "queue_pop"
if client.queue_pop("jobs") != "b":
    return "queue_pop 2"
if client.queue_size("jobs") != 1:
    return "size after pops"
ranged = client.queue_range("jobs")
if ranged != ["c"]:
    return "queue_range: " + str(ranged)
ranged = client.queue_range("jobs", start=0, stop=-1)
if ranged != ["c"]:
    return "queue_range explicit"
client.delete("jobs")
if client.queue_pop("jobs") != None:
    return "queue_pop empty"
if client.queue_peek("jobs") != None:
    return "queue_peek empty"

# queue_wait: immediate when an item is present
client.queue_push("jobs", "now")
if client.queue_wait("jobs", 1) != "now":
    return "queue_wait immediate"

# queue_wait: None on timeout, and the wait actually elapses
start = time.time()
if client.queue_wait("jobs", 0.3) != None:
    return "queue_wait timeout should be None"
elapsed = time.time() - start
if elapsed < 0.25:
    return "queue_wait returned early: " + str(elapsed)

client.close()
return "ok"
`

func TestInProcessExtras(t *testing.T) {
	server := miniredis.RunT(t)
	script := "import scriptling.valkey as valkey\nimport time\n" + strings.ReplaceAll(extrasScript, "__URL__", "valkey://"+server.Addr())
	result, err := evalInProcess(t, loopbackPolicy(), script)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "ok" {
		t.Fatalf("script result: %s", result.Inspect())
	}
}

func TestExternalExtras(t *testing.T) {
	server := miniredis.RunT(t)
	script := "import scriptling.valkey as valkey\nimport time\n" + strings.ReplaceAll(extrasScript, "__URL__", "valkey://"+server.Addr())
	bin := plugintest.BuildPlugin(t, "./cmd")
	result, err := plugintest.External(t, bin, loopbackPolicy(), script)
	if err != nil {
		t.Fatalf("external eval: %v", err)
	}
	if result.Inspect() != "ok" {
		t.Fatalf("script result: %s", result.Inspect())
	}
}

// TestQueueWaitBlocksAndReceives proves queue_wait truly blocks: an item
// pushed from outside the interpreter (a second client, mid-wait) arrives
// through the waiting call rather than a poll.
func TestQueueWaitBlocksAndReceives(t *testing.T) {
	server := miniredis.RunT(t)
	pusher, err := valkey.NewClient(valkey.ClientOption{
		InitAddress:       []string{server.Addr()},
		ForceSingleClient: true,
		DisableCache:      true,
		AlwaysRESP2:       true,
	})
	if err != nil {
		t.Fatalf("pusher client: %v", err)
	}
	defer pusher.Close()

	timer := time.AfterFunc(150*time.Millisecond, func() {
		_ = pusher.Do(context.Background(), pusher.B().Rpush().Key("late").Element("arrived").Build()).Error()
	})
	defer timer.Stop()

	p := scriptling.New()
	stdlib.RegisterAll(p)
	p.RegisterLibrary(Build(&plugin.StaticPolicy{P: loopbackPolicy()}))
	result, err := p.Eval(`import time
import scriptling.valkey as valkey
client = valkey.connect("valkey://` + server.Addr() + `")
start = time.time()
value = client.queue_wait("late", 5)
elapsed = time.time() - start
client.close()
if value != "arrived":
    return "wrong value: " + str(value)
if elapsed < 0.1:
    return "did not block: " + str(elapsed)
if elapsed > 3:
    return "blocked too long: " + str(elapsed)
return "ok"
`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "ok" {
		t.Fatalf("script result: %s", result.Inspect())
	}
}

func TestParseURLAddresses(t *testing.T) {
	for _, tt := range []struct {
		name      string
		url       string
		addresses []string
		username  string
		password  string
		db        int
		tls       bool
	}{
		{name: "single", url: "valkey://localhost:6379", addresses: []string{"localhost:6379"}},
		{name: "bare host:port", url: "127.0.0.1:6379", addresses: []string{"127.0.0.1:6379"}},
		{name: "default port", url: "valkey://redis.internal", addresses: []string{"redis.internal:6379"}},
		{name: "seed list", url: "valkey://node-a:7000,node-b:7000,node-c:7000", addresses: []string{"node-a:7000", "node-b:7000", "node-c:7000"}},
		{name: "bare seed list", url: "node-a:7000,node-b:7000", addresses: []string{"node-a:7000", "node-b:7000"}},
		{name: "seed list with spaces", url: "valkey://node-a:7000, node-b:7000", addresses: []string{"node-a:7000", "node-b:7000"}},
		{name: "credentials cover the list", url: "valkey://:secret@node-a:26379,node-b:26379", addresses: []string{"node-a:26379", "node-b:26379"}, password: "secret"},
		{name: "db on the list", url: "valkey://node-a:7000,node-b:7000/2", addresses: []string{"node-a:7000", "node-b:7000"}, db: 2},
		{name: "tls list", url: "valkeys://node-a:6379,node-b:6379", addresses: []string{"node-a:6379", "node-b:6379"}, tls: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := parseURL(tt.url)
			if err != nil {
				t.Fatalf("parseURL: %v", err)
			}
			if len(opts.addresses) != len(tt.addresses) {
				t.Fatalf("addresses: %v, want %v", opts.addresses, tt.addresses)
			}
			for i, addr := range tt.addresses {
				if opts.addresses[i] != addr {
					t.Fatalf("address[%d] = %s, want %s", i, opts.addresses[i], addr)
				}
			}
			if opts.username != tt.username || opts.password != tt.password || opts.db != tt.db || opts.tls != tt.tls {
				t.Fatalf("options: %+v", opts)
			}
		})
	}
}

func TestParseURLRejects(t *testing.T) {
	for _, tt := range []struct {
		name string
		url  string
	}{
		{name: "mixed schemes", url: "valkey://node-a:7000,rediss://node-b:7000"},
		{name: "trailing comma", url: "valkey://node-a:7000,"},
		{name: "bad db", url: "valkey://localhost:6379/nope"},
		{name: "bad scheme", url: "http://localhost:6379"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseURL(tt.url); err == nil {
				t.Fatal("expected parse error")
			}
		})
	}
}

func TestConnectModeValidation(t *testing.T) {
	server := miniredis.RunT(t)
	_, err := evalInProcess(t, loopbackPolicy(), `
import scriptling.valkey as valkey
client = valkey.connect("valkey://`+server.Addr()+`", mode="bogus")
`)
	if err == nil {
		t.Fatal("expected mode validation error")
	}
	if !strings.Contains(err.Error(), "mode must be") {
		t.Fatalf("expected mode error, got: %v", err)
	}
}

func TestInProcessModeAndFlush(t *testing.T) {
	server := miniredis.RunT(t)
	result, err := evalInProcess(t, loopbackPolicy(), `
import scriptling.valkey as valkey
client = valkey.connect("valkey://`+server.Addr()+`")
if client.mode() != "standalone":
    return "mode: " + client.mode()

client.set("a", "db0")
client.select(1)
client.set("b", "db1")
client.flushdb()
if client.keys("*") != []:
    return "flushdb cleared other databases: " + str(client.keys("*"))
client.select(0)
if client.get("a") != "db0":
    return "flushdb touched database 0"
client.flushall()
if client.keys("*") != []:
    return "flushall left keys"
client.close()
return "ok"
`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "ok" {
		t.Fatalf("script result: %s", result.Inspect())
	}
}

// TestErrorOnlyMethodFailureIsRaised covers the error plumbing for methods
// whose only return is an error (set, select, flushdb): a failure must raise
// the error, not surface it as a value conversion error. select(-1) fails
// inside the plugin, before any server round trip.
func TestErrorOnlyMethodFailureIsRaised(t *testing.T) {
	server := miniredis.RunT(t)
	script := `
import scriptling.valkey as valkey
client = valkey.connect("valkey://` + server.Addr() + `")
client.select(-1)
`
	if _, err := evalInProcess(t, loopbackPolicy(), script); err == nil || !strings.Contains(err.Error(), ">= 0") {
		t.Fatalf("in-process: expected '>= 0' error, got: %v", err)
	}

	bin := plugintest.BuildPlugin(t, "./cmd")
	_, err := plugintest.External(t, bin, loopbackPolicy(), script)
	if err == nil || !strings.Contains(err.Error(), ">= 0") {
		t.Fatalf("external: expected '>= 0' error, got: %v", err)
	}
}

// TestLiveCluster runs the whole surface against a real cluster when
// SCRIPTLING_VALKEY_CLUSTER holds its seed addresses (comma separated):
//
//	container run -d --name valkey-cluster hub.anaconda.ovh/library/knot-valkey:9.1.1 ...
func TestLiveCluster(t *testing.T) {
	seeds := os.Getenv("SCRIPTLING_VALKEY_CLUSTER")
	if seeds == "" {
		t.Skip("SCRIPTLING_VALKEY_CLUSTER not set")
	}
	result, err := evalInProcess(t, nil, `
import scriptling.valkey as valkey
client = valkey.connect("`+seeds+`", mode="cluster")
if client.mode() != "cluster":
    return "mode: " + client.mode()

client.set("greeting", "hello cluster")
if client.get("greeting") != "hello cluster":
    return "get lost value"
client.incr("hits")
client.mset({"h{a}": "1", "j{a}": "2"})     # same slot via hash tags
values = client.mget("h{a}", "j{a}")
if values != ["1", "2"]:
    return "mget: " + str(values)
# multi-key commands need same-slot keys on a cluster, so delete per key
removed = client.delete("greeting") + client.delete("hits") + client.delete("h{a}", "j{a}")
if removed != 4:
    return "delete: " + str(removed)
client.set("gone", "x")
client.flushdb()
if client.keys("*") != []:
    return "flushdb left keys on the cluster"
client.close()
return "ok"
`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "ok" {
		t.Fatalf("script result: %s", result.Inspect())
	}
	// keys() on a cluster reaches one node only, so prove the flush landed on
	// every master: dial each seed directly and check it holds nothing.
	for _, seed := range strings.Split(seeds, ",") {
		node, err := valkey.NewClient(valkey.ClientOption{
			InitAddress:       []string{strings.TrimSpace(seed)},
			ForceSingleClient: true,
			DisableCache:      true,
			AlwaysRESP2:       true,
		})
		if err != nil {
			t.Fatalf("node %s: %v", seed, err)
		}
		size, err := node.Do(context.Background(), node.B().Dbsize().Build()).ToInt64()
		node.Close()
		if err != nil {
			t.Fatalf("dbsize %s: %v", seed, err)
		}
		if size != 0 {
			t.Fatalf("node %s still holds %d keys after flushdb", seed, size)
		}
	}
}

// TestLiveSentinel runs against a real sentinel when
// SCRIPTLING_VALKEY_SENTINEL holds its address (the master set is mymaster).
func TestLiveSentinel(t *testing.T) {
	sentinel := os.Getenv("SCRIPTLING_VALKEY_SENTINEL")
	if sentinel == "" {
		t.Skip("SCRIPTLING_VALKEY_SENTINEL not set")
	}
	result, err := evalInProcess(t, nil, `
import scriptling.valkey as valkey
client = valkey.connect("`+sentinel+`", mode="sentinel")
if client.mode() != "sentinel":
    return "mode: " + client.mode()
client.set("greeting", "hello sentinel")
if client.get("greeting") != "hello sentinel":
    return "get lost value"
client.delete("greeting")
client.close()
return "ok"
`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "ok" {
		t.Fatalf("script result: %s", result.Inspect())
	}
}

func TestLibraryShape(t *testing.T) {
	lib := Build(&plugin.StaticPolicy{})
	if lib.Name() != "scriptling.valkey" {
		t.Fatalf("library name: %s", lib.Name())
	}
	if lib.Functions()["connect"] == nil {
		t.Fatal("connect function missing")
	}
}

func splitHostPort(addr string) (string, string, error) {
	parts := strings.LastIndex(addr, ":")
	if parts < 0 {
		return addr, "", nil
	}
	return addr[:parts], addr[parts+1:], nil
}
