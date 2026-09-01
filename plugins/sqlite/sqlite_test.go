package sqlite

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/plugin"
	"github.com/paularlott/scriptling/plugins/internal/plugintest"
)

// evalInProcess registers the compiled-in form of the plugin and evaluates
// script — the path the default scriptling build takes.
func evalInProcess(t *testing.T, policy *plugin.Policy, script string) (object.Object, error) {
	t.Helper()
	p := scriptling.New()
	RegisterInProcess(p, policy)
	return p.Eval(script)
}

const crudScript = `
import scriptling.sqlite as sqlite

conn = sqlite.connect()
result = conn.execute("create table people (id integer primary key autoincrement, name text, score real, active int)")
if result.rows_affected != 0:
    return "create affected rows?"

ins = conn.execute("insert into people (name, score, active) values (?, ?, ?)", "ada", 9.5, 1)
if ins.last_insert_id != 1:
    return "bad insert id: " + str(ins.last_insert_id)
ins2 = conn.execute("insert into people (name, score, active) values (?, ?, ?)", "grace", 8.0, 0)
if ins2.last_insert_id != 2:
    return "bad second insert id: " + str(ins2.last_insert_id)

upd = conn.execute("update people set active = ? where name = ?", 1, "grace")
if upd.rows_affected != 1:
    return "bad update count: " + str(upd.rows_affected)

rows = conn.query("select id, name, score, active from people order by id")
if len(rows) != 2:
    return "expected 2 rows, got " + str(len(rows))
if rows[0]["name"] != "ada" or rows[0]["score"] != 9.5 or rows[0]["active"] != 1:
    return "row 0 wrong: " + str(rows[0])
if rows[1]["name"] != "grace":
    return "row 1 wrong: " + str(rows[1])

one = conn.query("select name from people where name = ?", "ada")
if len(one) != 1 or one[0]["name"] != "ada":
    return "parameterised query wrong: " + str(one)

none = conn.query("select name from people where name = ?", "nobody")
if len(none) != 0:
    return "expected no rows"

conn.close()
return "ok"
`

func TestInProcessCRUD(t *testing.T) {
	result, err := evalInProcess(t, nil, crudScript)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "ok" {
		t.Fatalf("script result: %s", result.Inspect())
	}
}

func TestInProcessClassConstructor(t *testing.T) {
	result, err := evalInProcess(t, nil, `
import scriptling.sqlite as sqlite
conn = sqlite.Connection(":memory:")
conn.execute("create table t (v text)")
conn.execute("insert into t (v) values (?)", "x")
rows = conn.query("select v from t")
conn.close()
return rows[0]["v"]
`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "x" {
		t.Fatalf("script result: %s", result.Inspect())
	}
}

func TestInProcessFileWithinAllowedPaths(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "app.db")

	result, err := evalInProcess(t, &plugin.Policy{AllowedPaths: []string{dir}}, `
import scriptling.sqlite as sqlite
conn = sqlite.connect("`+dbPath+`")
conn.execute("create table t (v text)")
conn.execute("insert into t (v) values (?)", "persisted")
conn.close()

conn2 = sqlite.connect("`+dbPath+`")
rows = conn2.query("select v from t")
conn2.close()
return rows[0]["v"]
`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "persisted" {
		t.Fatalf("file database did not persist: %s", result.Inspect())
	}
}

func TestInProcessPathPolicyDenied(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(dir, "outside", "app.db")

	_, err := evalInProcess(t, &plugin.Policy{AllowedPaths: []string{filepath.Join(dir, "allowed")}}, `
import scriptling.sqlite as sqlite
conn = sqlite.connect("`+outside+`")
`)
	if err == nil {
		t.Fatal("expected path policy to deny the connection")
	}
	if !strings.Contains(err.Error(), "allowed paths") {
		t.Fatalf("expected allowed-paths error, got: %v", err)
	}
}

// TestInProcessFileURIDSN covers file: URI DSNs: the database path named in
// the URI is what the allowed-paths policy judges, so a legitimate URI to an
// allowed location works and one pointing outside is refused.
func TestInProcessFileURIDSN(t *testing.T) {
	dir := t.TempDir()
	inside := filepath.Join(dir, "uri.db")
	outside := filepath.Join(dir, "outside", "uri.db")

	result, err := evalInProcess(t, &plugin.Policy{AllowedPaths: []string{dir}}, `
import scriptling.sqlite as sqlite
conn = sqlite.connect("file:`+inside+`")
conn.execute("create table t (v text)")
conn.execute("insert into t (v) values (?)", "uri")
conn.close()
conn2 = sqlite.connect("file:`+inside+`?cache=shared")
rows = conn2.query("select v from t")
conn2.close()
return rows[0]["v"]
`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "uri" {
		t.Fatalf("file: uri database did not persist: %s", result.Inspect())
	}

	_, err = evalInProcess(t, &plugin.Policy{AllowedPaths: []string{dir}}, `
import scriptling.sqlite as sqlite
conn = sqlite.connect("file:`+outside+`")
`)
	if err == nil {
		t.Fatal("expected path policy to deny the file: uri outside the allowed paths")
	}
	if !strings.Contains(err.Error(), "allowed paths") {
		t.Fatalf("expected allowed-paths error, got: %v", err)
	}
}

// TestInProcessMemoryURIClassification pins how the in-memory spellings are
// classified against a deny-all path policy. :memory:, file::memory:, and a
// file: URI carrying mode=memory all store the database in memory (the URI
// path is only a name), so there is no file for the policy to guard; a
// file-backed URI with no mode=memory stays a path the policy judges.
func TestInProcessMemoryURIClassification(t *testing.T) {
	denyAll := &plugin.Policy{AllowedPaths: []string{}}

	for _, uri := range []string{
		":memory:",
		"file::memory:",
		"file::memory:?cache=shared",
		"file:any-name?mode=memory",
		"file:/etc/passwd?mode=memory",
		"file:app.db?mode=memory&cache=shared",
	} {
		_, err := evalInProcess(t, denyAll, `
import scriptling.sqlite as sqlite
conn = sqlite.connect("`+uri+`")
conn.close()
`)
		if err != nil {
			t.Errorf("connect(%q) should be allowed as in-memory, got: %v", uri, err)
		}
	}

	for _, uri := range []string{
		"app.db",
		"file:app.db",
		"file:/etc/passwd",
		"file:app.db?cache=shared",
	} {
		_, err := evalInProcess(t, denyAll, `
import scriptling.sqlite as sqlite
conn = sqlite.connect("`+uri+`")
`)
		if err == nil {
			t.Errorf("connect(%q) should be denied by the path policy", uri)
		}
	}
}

// TestInProcessSharedMemoryURIAcrossConnections proves cache=shared memory
// databases really are shared: two connections to the same URI see each
// other's writes, which only works when pooling is left enabled for them.
func TestInProcessSharedMemoryURIAcrossConnections(t *testing.T) {
	result, err := evalInProcess(t, &plugin.Policy{AllowedPaths: []string{}}, `
import scriptling.sqlite as sqlite

writer = sqlite.connect("file:sharedmem?mode=memory&cache=shared")
writer.execute("create table t (v text)")
writer.execute("insert into t (v) values (?)", "seen")

reader = sqlite.connect("file:sharedmem?mode=memory&cache=shared")
rows = reader.query("select v from t")
writer.close()
reader.close()
return rows[0]["v"]
`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "seen" {
		t.Fatalf("shared memory database was not shared across connections: %s", result.Inspect())
	}
}

// TestInProcessPrivateMemoryURIPersistsWithinConnection proves a mode=memory
// URI without cache=shared behaves like :memory:: writes stay visible to
// later statements on the connection (single pooled connection), and nothing
// named after the URI path is created on disk.
func TestInProcessPrivateMemoryURIPersistsWithinConnection(t *testing.T) {
	// The URI path is relative, so a bug that opened a file would create it
	// in the working directory; chdir into a scratch dir to make that
	// observable and keep the repository clean.
	dir := t.TempDir()
	t.Chdir(dir)

	result, err := evalInProcess(t, &plugin.Policy{AllowedPaths: []string{}}, `
import scriptling.sqlite as sqlite

conn = sqlite.connect("file:private-db?mode=memory")
conn.execute("create table t (v text)")
conn.execute("insert into t (v) values (?)", "kept")
rows = conn.query("select v from t")
conn.close()
return rows[0]["v"]
`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "kept" {
		t.Fatalf("private memory database lost its writes: %s", result.Inspect())
	}

	if _, err := os.Stat(filepath.Join(dir, "private-db")); !os.IsNotExist(err) {
		t.Fatalf("mode=memory created a file named after the URI path: %v", err)
	}
}

func TestInProcessNullAndBoolRoundTrip(t *testing.T) {
	result, err := evalInProcess(t, nil, `
import scriptling.sqlite as sqlite
conn = sqlite.connect()
conn.execute("create table t (a text, b text)")
conn.execute("insert into t values (?, ?)", None, "set")
rows = conn.query("select a, b from t")
conn.close()
if rows[0]["a"] != None:
    return "null lost: " + str(rows[0])
return "ok"
`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "ok" {
		t.Fatalf("script result: %s", result.Inspect())
	}
}

func TestExternalCRUD(t *testing.T) {
	bin := plugintest.BuildPlugin(t, "./cmd")
	result, err := plugintest.External(t, bin, nil, crudScript)
	if err != nil {
		t.Fatalf("external eval: %v", err)
	}
	if result.Inspect() != "ok" {
		t.Fatalf("script result: %s", result.Inspect())
	}
}

func TestExternalPathPolicyDeliveredByHandshake(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(dir, "outside", "app.db")

	bin := plugintest.BuildPlugin(t, "./cmd")
	_, err := plugintest.External(t, bin, &plugin.Policy{AllowedPaths: []string{filepath.Join(dir, "allowed")}}, `
import scriptling.sqlite as sqlite
conn = sqlite.connect("`+outside+`")
`)
	if err == nil {
		t.Fatal("expected handshake-delivered policy to deny the connection")
	}
	if !strings.Contains(err.Error(), "allowed paths") {
		t.Fatalf("expected allowed-paths error, got: %v", err)
	}
}

func TestExternalFileDatabase(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "app.db")
	if err := os.WriteFile(dbPath, nil, 0o600); err != nil {
		t.Fatalf("seed db file: %v", err)
	}

	bin := plugintest.BuildPlugin(t, "./cmd")
	result, err := plugintest.External(t, bin, &plugin.Policy{AllowedPaths: []string{dir}}, `
import scriptling.sqlite as sqlite
conn = sqlite.connect("`+dbPath+`")
conn.execute("create table t (v text)")
conn.execute("insert into t (v) values (?)", "over-the-wire")
rows = conn.query("select v from t")
conn.close()
return rows[0]["v"]
`)
	if err != nil {
		t.Fatalf("external eval: %v", err)
	}
	if result.Inspect() != "over-the-wire" {
		t.Fatalf("script result: %s", result.Inspect())
	}
}

func TestLibraryShape(t *testing.T) {
	lib := Build(&plugin.StaticPolicy{})
	if lib.Name() != "scriptling._sqlite" {
		t.Fatalf("native twin library name: %s", lib.Name())
	}
	if lib.Functions()["connect"] == nil {
		t.Fatal("connect function missing")
	}
	if _, ok := lib.Constants()["Connection"]; !ok {
		t.Fatal("Connection class missing from constants")
	}
}
