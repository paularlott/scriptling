package sqlite

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/paularlott/scriptling/plugin"
	"github.com/paularlott/scriptling/plugins/internal/plugintest"
)

// TestInProcessTransactionCommit pins the happy path: statements through the
// transaction are invisible-or-visible as a unit, and commit makes them
// permanent for the connection.
func TestInProcessTransactionCommit(t *testing.T) {
	result, err := evalInProcess(t, nil, `
import scriptling.sqlite as sqlite

conn = sqlite.connect()
conn.execute("create table people (id integer primary key autoincrement, name text)")

tx = conn.begin()
ins = tx.execute("insert into people (name) values (?)", "ada")
if ins.last_insert_id != 1 or ins.rows_affected != 1:
    return "bad insert result: " + str(ins)
rows = tx.query("select name from people")
if len(rows) != 1 or rows[0]["name"] != "ada":
    return "inside tx: " + str(rows)
tx.commit()

rows = conn.query("select name from people")
if len(rows) != 1 or rows[0]["name"] != "ada":
    return "after commit: " + str(rows)
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

// TestInProcessTransactionRollback pins the discard path: rollback leaves
// the database exactly as it was, including the autoincrement counter.
func TestInProcessTransactionRollback(t *testing.T) {
	result, err := evalInProcess(t, nil, `
import scriptling.sqlite as sqlite

conn = sqlite.connect()
conn.execute("create table people (id integer primary key autoincrement, name text)")
conn.execute("insert into people (name) values (?)", "ada")

tx = conn.begin()
tx.execute("insert into people (name) values (?)", "grace")
tx.execute("update people set name = 'removed' where name = 'ada'")
rows = tx.query("select name from people order by id")
if len(rows) != 2 or rows[0]["name"] != "removed" or rows[1]["name"] != "grace":
    return "inside tx: " + str(rows)
tx.rollback()

rows = conn.query("select name from people")
if len(rows) != 1 or rows[0]["name"] != "ada":
    return "after rollback: " + str(rows)
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

// TestInProcessTransactionDoneErrors pins that every operation on a finished
// transaction reports the same clear shape, whichever way it ended.
func TestInProcessTransactionDoneErrors(t *testing.T) {
	for _, tt := range []struct {
		ending string
		want   int
	}{
		{ending: "commit", want: 1},
		{ending: "rollback", want: 0},
	} {
		result, err := evalInProcess(t, nil, `
import scriptling.sqlite as sqlite

conn = sqlite.connect()
conn.execute("create table t (v text)")
tx = conn.begin()
tx.execute("insert into t (v) values (?)", "x")
tx.`+tt.ending+`()

try:
    tx.query("select 1")
    return "query after `+tt.ending+` worked"
except Exception as e:
    if "already committed or rolled back" not in str(e):
        return "query error shape: " + str(e)
try:
    tx.execute("insert into t (v) values (?)", "y")
    return "execute after `+tt.ending+` worked"
except Exception as e:
    if "already committed or rolled back" not in str(e):
        return "execute error shape: " + str(e)
try:
    tx.commit()
    return "double commit worked"
except Exception as e:
    if "already committed or rolled back" not in str(e):
        return "commit error shape: " + str(e)
try:
    tx.rollback()
    return "rollback after `+tt.ending+` worked"
except Exception as e:
    if "already committed or rolled back" not in str(e):
        return "rollback error shape: " + str(e)
rows = conn.query("select v from t")
if len(rows) != `+strconv.Itoa(tt.want)+`:
    return "rows after `+tt.ending+`: " + str(rows)
conn.close()
return "ok"
`)
		if err != nil {
			t.Fatalf("eval (%s): %v", tt.ending, err)
		}
		if result.Inspect() != "ok" {
			t.Fatalf("script result (%s): %s", tt.ending, result.Inspect())
		}
	}
}

// TestInProcessTransactionHeldConnection pins the single-connection guard:
// a private in-memory database runs on one pooled connection, so while the
// transaction holds it the connection's own calls fail fast instead of
// blocking forever on the exhausted pool — and work again once it ends.
func TestInProcessTransactionHeldConnection(t *testing.T) {
	for _, ending := range []string{"commit", "rollback"} {
		result, err := evalInProcess(t, nil, `
import scriptling.sqlite as sqlite

conn = sqlite.connect()
conn.execute("create table t (v text)")

tx = conn.begin()
try:
    conn.query("select 1")
    return "conn.query during tx worked"
except Exception as e:
    if "held by an open transaction" not in str(e):
        return "query busy error shape: " + str(e)
try:
    conn.execute("insert into t (v) values (?)", "x")
    return "conn.execute during tx worked"
except Exception as e:
    if "held by an open transaction" not in str(e):
        return "execute busy error shape: " + str(e)
try:
    conn.begin()
    return "nested begin worked"
except Exception as e:
    if "held by an open transaction" not in str(e):
        return "begin busy error shape: " + str(e)
tx.`+ending+`()

rows = conn.query("select 1")
if len(rows) != 1:
    return "conn still busy after " + "`+ending+`"
conn.close()
return "ok"
`)
		if err != nil {
			t.Fatalf("eval (%s): %v", ending, err)
		}
		if result.Inspect() != "ok" {
			t.Fatalf("script result (%s): %s", ending, result.Inspect())
		}
	}
}

// TestInProcessTransactionFileParallelCalls pins the pooled case: a file
// database serves the transaction and the connection from separate pooled
// connections, so connection-level reads run while the transaction is open
// and see the pre-transaction (committed) view until commit lands.
func TestInProcessTransactionFileParallelCalls(t *testing.T) {
	dir := t.TempDir()
	result, err := evalInProcess(t, &plugin.Policy{AllowedPaths: []string{dir}}, `
import scriptling.sqlite as sqlite

conn = sqlite.connect("`+filepath.Join(dir, "app.db")+`")
conn.execute("create table t (v text)")
conn.execute("insert into t (v) values (?)", "committed")

tx = conn.begin()
tx.execute("insert into t (v) values (?)", "pending")
outside = conn.query("select v from t order by v")
if len(outside) != 1 or outside[0]["v"] != "committed":
    return "conn read during tx: " + str(outside)
tx.commit()

outside = conn.query("select v from t order by v")
if len(outside) != 2:
    return "after commit: " + str(outside)
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

// TestInProcessTransactionQueryIter streams a transaction's rows and checks
// the streamed statement participates in the rollback.
func TestInProcessTransactionQueryIter(t *testing.T) {
	result, err := evalInProcess(t, nil, `
import scriptling.sqlite as sqlite

conn = sqlite.connect()
conn.execute("create table t (v text)")

tx = conn.begin()
tx.execute("insert into t (v) values (?)", "a")
tx.execute("insert into t (v) values (?)", "b")
seen = []
cur = tx.query_iter("select v from t order by v")
row = cur.next()
while row != None:
    seen.append(row["v"])
    row = cur.next()
cur.close()
if seen != ["a", "b"]:
    return "streamed: " + str(seen)
tx.rollback()

rows = conn.query("select v from t")
if len(rows) != 0:
    return "rollback leaked: " + str(rows)
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

// TestInProcessTransactionORM runs the ORM inside a transaction: the kit
// from tx.get_orm() drives every statement through the transaction, so
// commit keeps its writes and rollback discards them.
func TestInProcessTransactionORM(t *testing.T) {
	result, err := evalInProcess(t, nil, `
import scriptling.sqlite as sqlite

conn = sqlite.connect()
orm = conn.get_orm()
(orm.create_table("people")
 .column("id", "integer", primary_key=True, autoincrement=True)
 .column("name", "text")
 .execute())

tx = conn.begin()
torm = tx.get_orm()
ins = torm.insert("people", {"name": "ada"})
if ins.last_insert_id != 1:
    return "tx insert: " + str(ins)
if torm.select("people").count() != 1:
    return "tx count"
tx.rollback()

if orm.select("people").count() != 0:
    return "rollback leaked through the orm"

tx = conn.begin()
torm = tx.get_orm()
torm.insert("people", {"name": "grace"})
tx.commit()

if orm.select("people").count() != 1:
    return "commit lost through the orm"
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

// TestTransactionConstructorRefused pins that the native Transaction class
// is not constructible: transactions only come from conn.begin(). (The
// module-level Transaction name is the script wrapper begin() hands out;
// the native twin is what the plugin protocol's object construction hits.)
func TestTransactionConstructorRefused(t *testing.T) {
	_, err := evalInProcess(t, nil, `
import scriptling._sqlite as n
tx = n.Transaction()
`)
	if err == nil {
		t.Fatal("expected constructor to refuse")
	}
	if !strings.Contains(err.Error(), "conn.begin()") {
		t.Fatalf("error shape: %v", err)
	}
}

// TestExternalTransactions drives begin/commit/rollback through the wire
// path: the plugin process holds the transaction, the host-side wrappers
// proxy every call, and the semantics match compiled-in mode.
func TestExternalTransactions(t *testing.T) {
	bin := plugintest.BuildPlugin(t, "./cmd")
	result, err := plugintest.External(t, bin, nil, `
import scriptling.sqlite as sqlite

conn = sqlite.connect()
conn.execute("create table people (id integer primary key autoincrement, name text)")

tx = conn.begin()
tx.execute("insert into people (name) values (?)", "ada")
rows = tx.query("select name from people")
if len(rows) != 1 or rows[0]["name"] != "ada":
    return "inside tx: " + str(rows)
tx.commit()

tx = conn.begin()
tx.execute("insert into people (name) values (?)", "grace")
tx.rollback()

rows = conn.query("select name from people")
if len(rows) != 1 or rows[0]["name"] != "ada":
    return "after both: " + str(rows)
conn.close()
return "ok"
`)
	if err != nil {
		t.Fatalf("external eval: %v", err)
	}
	if result.Inspect() != "ok" {
		t.Fatalf("script result: %s", result.Inspect())
	}
}
