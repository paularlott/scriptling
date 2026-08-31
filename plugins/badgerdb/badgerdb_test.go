package badgerdb

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/plugin"
	"github.com/paularlott/scriptling/plugins/internal/plugintest"
)

func evalInProcess(t *testing.T, policy *plugin.Policy, script string) (object.Object, error) {
	t.Helper()
	p := scriptling.New()
	p.RegisterLibrary(Build(&plugin.StaticPolicy{P: policy}))
	return p.Eval(script)
}

// kvScript exercises the full KV surface; the valkey plugin runs the same
// script against miniredis to prove the two APIs stay mirrored.
const kvScript = `
client = __FACTORY__("__PATH__")
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

func TestInProcessKVSurface(t *testing.T) {
	dir := t.TempDir()
	script := strings.ReplaceAll(kvScript, "__FACTORY__", "badger.open")
	script = strings.ReplaceAll(script, "__PATH__", filepath.Join(dir, "kv"))
	script = "import scriptling.badgerdb as badger\n" + script

	result, err := evalInProcess(t, nil, script)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "ok" {
		t.Fatalf("script result: %s", result.Inspect())
	}
}

func TestInProcessTTLExpiry(t *testing.T) {
	dir := t.TempDir()
	result, err := evalInProcess(t, nil, `
import scriptling.badgerdb as badger
client = badger.open("`+filepath.Join(dir, "kv")+`")
client.set("ephemeral", "data", ttl_seconds=1)
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

func TestInProcessPathPolicyDenied(t *testing.T) {
	dir := t.TempDir()
	_, err := evalInProcess(t, &plugin.Policy{AllowedPaths: []string{filepath.Join(dir, "allowed")}}, `
import scriptling.badgerdb as badger
client = badger.open("`+filepath.Join(dir, "outside")+`")
`)
	if err == nil {
		t.Fatal("expected path policy to deny the open")
	}
	if !strings.Contains(err.Error(), "allowed paths") {
		t.Fatalf("expected allowed-paths error, got: %v", err)
	}
}

func TestInProcessIncrPreservesTTL(t *testing.T) {
	dir := t.TempDir()
	result, err := evalInProcess(t, nil, `
import scriptling.badgerdb as badger
client = badger.open("`+filepath.Join(dir, "kv")+`")
client.set("counter", "10", ttl_seconds=300)
client.incr("counter", amount=5)
ttl = client.ttl("counter")
if ttl == None or ttl <= 0 or ttl > 300:
    return "ttl lost across incr: " + str(ttl)
if client.get("counter") != "15":
    return "incr value wrong: " + client.get("counter")
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

func TestExternalKVSurface(t *testing.T) {
	dir := t.TempDir()
	script := strings.ReplaceAll(kvScript, "__FACTORY__", "badger.open")
	script = strings.ReplaceAll(script, "__PATH__", filepath.Join(dir, "kv"))
	script = "import scriptling.badgerdb as badger\n" + script

	bin := plugintest.BuildPlugin(t, "./cmd")
	result, err := plugintest.External(t, bin, &plugin.Policy{AllowedPaths: []string{dir}}, script)
	if err != nil {
		t.Fatalf("external eval: %v", err)
	}
	if result.Inspect() != "ok" {
		t.Fatalf("script result: %s", result.Inspect())
	}
}

func TestExternalPathPolicyDeliveredByHandshake(t *testing.T) {
	dir := t.TempDir()
	bin := plugintest.BuildPlugin(t, "./cmd")
	_, err := plugintest.External(t, bin, &plugin.Policy{AllowedPaths: []string{filepath.Join(dir, "allowed")}}, `
import scriptling.badgerdb as badger
client = badger.open("`+filepath.Join(dir, "outside")+`")
`)
	if err == nil {
		t.Fatal("expected handshake-delivered policy to deny the open")
	}
	if !strings.Contains(err.Error(), "allowed paths") {
		t.Fatalf("expected allowed-paths error, got: %v", err)
	}
}

func TestLibraryShape(t *testing.T) {
	lib := Build(&plugin.StaticPolicy{})
	if lib.Name() != "scriptling.badgerdb" {
		t.Fatalf("library name: %s", lib.Name())
	}
	if lib.Functions()["open"] == nil {
		t.Fatal("open function missing")
	}
}

// TestInProcessHashWrongType pins the Redis-style type separation: hash
// commands on plain-valued keys fail with WRONGTYPE instead of silently
// destroying the data, and reads/counters refuse hash keys — while writes
// (set, mset, set_if_absent) keep Redis semantics: they replace any value or
// report false, never erroring on type. Before type tags, hash_set overwrote
// a scalar, hash_delete on a scalar key could delete it while reporting zero,
// and get() on a hash returned raw JSON.
func TestInProcessHashWrongType(t *testing.T) {
	dir := t.TempDir()
	script := "import scriptling.badgerdb as badger\n" + `
client = badger.open("` + filepath.Join(dir, "kv") + `")

client.set("plain", "scalar value")
client.set("looks-like-hash", "{\"field\": \"not really a hash\"}")

for op in ["hash_set", "hash_get", "hash_size", "hash_all"]:
    try:
        if op == "hash_set":
            client.hash_set("plain", "f", "v")
        elif op == "hash_get":
            client.hash_get("plain", "f")
        elif op == "hash_size":
            client.hash_size("plain")
        else:
            client.hash_all("plain")
        return op + " accepted a scalar key"
    except:
        pass

# a plain string that merely looks like hash JSON is still a plain string
if client.get("looks-like-hash") != "{\"field\": \"not really a hash\"}":
    return "scalar json string was eaten"

# hash_delete on a scalar key must not delete it while reporting zero
try:
    removed = client.hash_delete("plain", "missing")
    return "hash_delete accepted a scalar key"
except:
    pass
if client.get("plain") != "scalar value":
    return "hash_delete destroyed a scalar"

# reads and counters refuse a hash key; writes follow Redis semantics
client.hash_set("realhash", "f", "v")
for op in ["get", "incr", "mget"]:
    try:
        if op == "get":
            client.get("realhash")
        elif op == "incr":
            client.incr("realhash")
        else:
            client.mget("realhash")
        return op + " accepted a hash key"
    except:
        pass
# SETNX reports false against any existing key, a hash included
if client.set_if_absent("realhash", "x") != False:
    return "set_if_absent should refuse an existing hash"
if client.hash_get("realhash", "f") != "v":
    return "set_if_absent must not touch the hash"
# SET and MSET replace whatever was there, a hash included
client.set("realhash", "now a string")
if client.get("realhash") != "now a string":
    return "set should replace a hash"
client.hash_set("realhash2", "f", "v")
client.mset({"realhash2": "also a string"})
if client.get("realhash2") != "also a string":
    return "mset should replace a hash"

client.close()
return "ok"
`
	result, err := evalInProcess(t, nil, script)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "ok" {
		t.Fatalf("script result: %s", result.Inspect())
	}
}
