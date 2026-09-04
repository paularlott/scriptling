package sqlite

import (
	"strconv"
	"testing"
)

// churnHeap drives a few Go collection cycles from script so the receiver
// finalizers (transaction rollback, cursor release) get a chance to run.
// Each round allocates a few MB; a handful of rounds is normally enough.
const churnRounds = 100

// TestInProcessAbandonedTransactionReleased pins the compiled-in release
// path: a transaction abandoned without commit or rollback (here: lost to an
// exception nobody rolls back) is rolled back automatically once the runtime
// collects it, and the connection becomes usable again. Receiver instances
// born inside Go methods never pass through the evaluator's constructor
// path, so their cleanup rides on a finalizer installed directly on the
// typed receiver.
func TestInProcessAbandonedTransactionReleased(t *testing.T) {
	result, err := evalInProcess(t, nil, `
import scriptling.sqlite as sqlite

conn = sqlite.connect()
conn.execute("create table t (v text)")
conn.execute("insert into t (v) values (?)", "kept")

def leaky():
    tx = conn.begin()
    tx.execute("insert into t (v) values (?)", "leaked")
    raise ValueError("nobody rolls back")

try:
    leaky()
except Exception:
    pass

freed = -1
for i in range(`+strconv.Itoa(churnRounds)+`):
    try:
        conn.query("select 1")
        freed = i
        break
    except Exception:
        junk = []
        for j in range(20000):
            junk.append(["x" * 64, j, {"k": j}])
        junk = None
if freed < 0:
    return "connection never recovered"
rows = conn.query("select v from t")
conn.close()
if len(rows) != 1 or rows[0]["v"] != "kept":
    return "abandoned tx not rolled back: " + str(rows)
return "ok"
`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "ok" {
		t.Fatalf("script result: %s", result.Inspect())
	}
}

// TestInProcessAbandonedCursorReleased pins the same release path for
// cursors: an abandoned query_iter cursor releases its rows once collected,
// freeing the single pooled connection of a private in-memory database.
func TestInProcessAbandonedCursorReleased(t *testing.T) {
	result, err := evalInProcess(t, nil, `
import scriptling.sqlite as sqlite

conn = sqlite.connect()
conn.execute("create table t (v text)")
conn.execute("insert into t (v) values (?)", "x")

def leaky():
    cur = conn.query_iter("select v from t")
    cur.next()      # one row read, cursor left open
    raise ValueError("cursor abandoned")

try:
    leaky()
except Exception:
    pass

freed = -1
for i in range(`+strconv.Itoa(churnRounds)+`):
    try:
        conn.query("select 1")
        freed = i
        break
    except Exception:
        junk = []
        for j in range(20000):
            junk.append(["x" * 64, j, {"k": j}])
        junk = None
if freed < 0:
    return "connection never recovered"
conn.close()
return "ok"
`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "ok" {
		t.Fatalf("script result: %s", result.Inspect())
	}
}

// TestInProcessOpenCursorHoldsConnection pins the fail-fast guard for open
// cursors on a single-connection database: without it, a connection-level
// call would block forever on the exhausted pool instead of reporting why.
func TestInProcessOpenCursorHoldsConnection(t *testing.T) {
	result, err := evalInProcess(t, nil, `
import scriptling.sqlite as sqlite

conn = sqlite.connect()
conn.execute("create table t (v text)")
conn.execute("insert into t (v) values (?)", "x")

cur = conn.query_iter("select v from t")
try:
    conn.query("select 1")
    return "conn.query with open cursor worked"
except Exception as e:
    if "held by an open cursor" not in str(e):
        return "cursor busy error shape: " + str(e)
try:
    conn.begin()
    return "begin with open cursor worked"
except Exception as e:
    if "held by an open cursor" not in str(e):
        return "begin busy error shape: " + str(e)
cur.close()

rows = conn.query("select v from t")
conn.close()
if len(rows) != 1:
    return "conn still busy after cursor close"
return "ok"
`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "ok" {
		t.Fatalf("script result: %s", result.Inspect())
	}
}

// TestCursorExhaustionReleasesConnection pins that a fully drained cursor
// releases the connection immediately (no close() needed): next() returning
// None finishes the rows.
func TestCursorExhaustionReleasesConnection(t *testing.T) {
	result, err := evalInProcess(t, nil, `
import scriptling.sqlite as sqlite

conn = sqlite.connect()
conn.execute("create table t (v text)")
conn.execute("insert into t (v) values (?)", "x")

cur = conn.query_iter("select v from t")
while cur.next() != None:
    pass

rows = conn.query("select v from t")
conn.close()
if len(rows) != 1:
    return "conn busy after drained cursor"
return "ok"
`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "ok" {
		t.Fatalf("script result: %s", result.Inspect())
	}
}
