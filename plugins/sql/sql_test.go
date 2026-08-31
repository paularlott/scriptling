package sql

import (
	"net/url"
	"os"
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
	RegisterInProcess(p, policy)
	return p.Eval(script)
}

func TestSchemeValidation(t *testing.T) {
	for _, tt := range []struct {
		dsn string
		msg string
	}{
		// url.Parse reads "localhost:5432/db" as scheme "localhost" — either
		// error wording is fine as long as a bare host is rejected.
		{dsn: `sql.connect("localhost:5432/db")`, msg: "dsn scheme"},
		{dsn: `sql.connect("mongodb://localhost/db")`, msg: "unsupported dsn scheme"},
	} {
		_, err := evalInProcess(t, nil, `
import scriptling.sql as sql
conn = `+tt.dsn+`
`)
		if err == nil {
			t.Fatalf("expected error for %s", tt.dsn)
		}
		if !strings.Contains(err.Error(), tt.msg) {
			t.Fatalf("expected %q error, got: %v", tt.msg, err)
		}
	}
}

// TestNetworkPolicyGuardedDial proves both drivers dial through the policy
// guard: with loopback denied, connect() fails before any server contact —
// no database server needed.
func TestNetworkPolicyGuardedDial(t *testing.T) {
	denied := &plugin.Policy{Network: &plugin.NetworkPolicy{}}
	for _, dsn := range []string{
		"postgres://user:pass@localhost:5432/app",
		"postgresql://user:pass@127.0.0.1:5432/app",
		"mysql://user:pass@localhost:3306/app",
		"mariadb://user:pass@127.0.0.1:3306/app",
	} {
		_, err := evalInProcess(t, denied, `
import scriptling.sql as sql
conn = sql.connect("`+dsn+`")
`)
		if err == nil {
			t.Fatalf("expected policy to deny %s", dsn)
		}
		if !strings.Contains(err.Error(), "not allowed") {
			t.Fatalf("expected policy denial for %s, got: %v", dsn, err)
		}
	}
}

func TestMySQLConfigFromURL(t *testing.T) {
	parsed, err := url.Parse("mysql://user:secret@db.example.com:3307/shop?charset=utf8mb4&parseTime=true")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	cfg, err := mysqlConfigFromURL(parsed)
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	if cfg.Net != "tcp" || cfg.Addr != "db.example.com:3307" {
		t.Fatalf("addr wrong: %s %s", cfg.Net, cfg.Addr)
	}
	if cfg.User != "user" || cfg.Passwd != "secret" {
		t.Fatalf("credentials wrong: %s/%s", cfg.User, cfg.Passwd)
	}
	if cfg.DBName != "shop" {
		t.Fatalf("dbname wrong: %s", cfg.DBName)
	}
	if cfg.Params["charset"] != "utf8mb4" || cfg.Params["parseTime"] != "true" {
		t.Fatalf("params wrong: %#v", cfg.Params)
	}

	// Defaults: no port, no credentials, no db.
	bare, _ := url.Parse("mysql://db.local")
	cfg, _ = mysqlConfigFromURL(bare)
	if cfg.Addr != "db.local:3306" || cfg.User != "" || cfg.DBName != "" {
		t.Fatalf("defaults wrong: %#v", cfg)
	}
}

// TestMySQLConfigDriverOptionsUnreachableFromURL pins the security-relevant
// property of the query-param mapping: URL query parameters become session
// variables (cfg.Params, sent as SET on connect), never driver options. tls
// is the single deliberate exception (its three registered names only, see
// TestMySQLConfigTLSOption). The go-sql-driver fields that would matter for
// a hostile-server attack — AllowAllFiles (LOAD DATA LOCAL INFILE file
// reads) among them — are only settable by ParseDSN from a DSN string, so a
// URL like ?allowAllFiles=true produces `SET allowAllFiles = true` (an
// unknown system variable the server rejects) and leaves the option struct
// untouched.
func TestMySQLConfigDriverOptionsUnreachableFromURL(t *testing.T) {
	parsed, err := url.Parse("mysql://user:pass@evil.example.com/db?allowAllFiles=true&interpolateParams=true")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	cfg, err := mysqlConfigFromURL(parsed)
	if err != nil {
		t.Fatalf("config: %v", err)
	}

	if cfg.AllowAllFiles {
		t.Fatal("AllowAllFiles must not be reachable from a URL query parameter")
	}
	if cfg.InterpolateParams {
		t.Fatal("InterpolateParams must not be reachable from a URL query parameter")
	}
	// They do arrive as session variables, where an unknown one simply fails
	// the connect.
	if cfg.Params["allowAllFiles"] != "true" {
		t.Fatalf("expected the param mapped to cfg.Params, got %#v", cfg.Params)
	}
}

func TestLibraryShape(t *testing.T) {
	lib := Build(&plugin.StaticPolicy{})
	if lib.Name() != "scriptling._sql" {
		t.Fatalf("native twin library name: %s", lib.Name())
	}
	if lib.Functions()["connect"] == nil {
		t.Fatal("connect function missing")
	}
}

// TestExternalSchemeValidation runs scheme dispatch through the wire path.
func TestExternalSchemeValidation(t *testing.T) {
	bin := plugintest.BuildPlugin(t, "./cmd")
	result, err := plugintest.External(t, bin, nil, `
import scriptling.sql as sql
try:
    conn = sql.connect("localhost/db")
    return "should have failed"
except:
    return "caught"
`)
	if err != nil {
		t.Fatalf("external eval: %v", err)
	}
	if result.Inspect() != "caught" {
		t.Fatalf("script result: %s", result.Inspect())
	}
}

// TestIntegrationPostgres runs the CRUD surface against a real server when
// SCRIPTLING_TEST_POSTGRES_DSN (postgres://user:pass@host:port/db) is set.
func TestIntegrationPostgres(t *testing.T) {
	dsn := os.Getenv("SCRIPTLING_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("SCRIPTLING_TEST_POSTGRES_DSN not set")
	}
	result, err := evalInProcess(t, &plugin.Policy{Network: &plugin.NetworkPolicy{AllowLoopback: true, AllowPrivateIPs: true}}, `
import scriptling.sql as sql
conn = sql.connect("`+dsn+`")
conn.execute("drop table if exists scriptling_people")
conn.execute("create table scriptling_people (id serial primary key, name text)")
ins = conn.execute("insert into scriptling_people (name) values (?)", "ada")
rows = conn.query("select name from scriptling_people where name = ?", "ada")
conn.execute("drop table scriptling_people")
conn.close()
if len(rows) != 1 or rows[0]["name"] != "ada":
    return "rows wrong: " + str(rows)
return "ok"
`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "ok" {
		t.Fatalf("script result: %s", result.Inspect())
	}
}

// TestIntegrationMySQL runs the CRUD surface against a real MySQL or MariaDB
// server when SCRIPTLING_TEST_MYSQL_DSN (mysql://user:pass@host:port/db) is set.
func TestIntegrationMySQL(t *testing.T) {
	dsn := os.Getenv("SCRIPTLING_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("SCRIPTLING_TEST_MYSQL_DSN not set")
	}
	result, err := evalInProcess(t, &plugin.Policy{Network: &plugin.NetworkPolicy{AllowLoopback: true, AllowPrivateIPs: true}}, `
import scriptling.sql as sql
conn = sql.connect("`+dsn+`")
conn.execute("drop table if exists scriptling_people")
conn.execute("create table scriptling_people (id integer primary key auto_increment, name text)")
ins = conn.execute("insert into scriptling_people (name) values (?)", "ada")
if ins.last_insert_id != 1:
    return "bad insert id: " + str(ins.last_insert_id)
rows = conn.query("select name from scriptling_people where name = ?", "ada")
conn.execute("drop table scriptling_people")
conn.close()
if len(rows) != 1 or rows[0]["name"] != "ada":
    return "rows wrong: " + str(rows)
return "ok"
`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "ok" {
		t.Fatalf("script result: %s", result.Inspect())
	}
}

// ormIntegrationScript exercises the ORM surface with backend-appropriate
// DDL; lastid differs (postgres has none), so the check is rows-based.
const ormIntegrationScript = `
conn = sql.connect("DSN")
orm = conn.get_orm()
orm.drop_table("scriptling_people")
(orm.create_table("scriptling_people")
 .column("id", "integer", primary_key=True, autoincrement=True)
 .column("name", "text", nullable=False)
 .column("score", "real", default=0.0)
 .if_not_exists()
 .execute())
orm.insert("scriptling_people", {"name": "ada", "score": 9.5})
orm.insert("scriptling_people", {"name": "grace", "score": 8.0})
orm.insert("scriptling_people", {"name": "linus", "score": 7.0})
if orm.select("scriptling_people").count() != 3:
    return "count all: " + str(orm.select("scriptling_people").count())
if orm.select("scriptling_people").where_sql("score >= ?", 8.0).count() != 2:
    return "count where"
# where_sql binds params like every other value: a hostile string stays
# data, never SQL. Spliced raw, this WHERE would be always-true.
if orm.select("scriptling_people").where_sql("name = ?", "x' OR '1'='1").count() != 0:
    return "where_sql leaked a param into SQL"
# mixed builder criteria and where_sql keep placeholder order straight on
# numbered dialects (postgres) and ? dialects alike
if orm.select("scriptling_people").where("score", ">", 6.5).where_sql("name != ?", "linus").count() != 2:
    return "where_sql mixed"
# a ? inside a quoted literal is a literal: both renumber passes must skip it
if orm.select("scriptling_people").where_sql("name != 'o''brien?' and score > ?", 0.0).count() != 3:
    return "where_sql literal question mark"
rows = (orm.select("scriptling_people", "name")
        .where("score", ">=", 8.0)
        .order_by("score", desc=True)
        .fetch())
if len(rows) != 2 or rows[0]["name"] != "ada":
    return "select: " + str(rows)
rows = orm.select("scriptling_people").order_by("id").limit(2).fetch()
if len(rows) != 2:
    return "limit: " + str(len(rows))
iterated = []
for row in orm.select("scriptling_people", "name").where("score", ">=", 7.0).order_by("id").iterate():
    iterated.append(row["name"])
if len(iterated) != 3 or iterated[0] != "ada":
    return "iterate: " + str(iterated)
grouped = (orm.select("scriptling_people", "name")
           .where(orm.any_of(orm.eq("name", "ada"), orm.eq("name", "grace")))
           .where(orm.any_of(orm.eq("score", 9.5), orm.ge("score", 8.0)))
           .fetch())
if len(grouped) != 2:
    return "grouped: " + str(grouped)
upd = orm.update("scriptling_people", {"score": 9.9}).where("name", "=", "ada").execute()
if upd.rows_affected != 1:
    return "update: " + str(upd)
upd = (orm.update("scriptling_people", {"score": 8.5})
       .where(orm.one_of("name", ["grace", "nobody"]))
       .execute())
if upd.rows_affected != 1:
    return "update criteria: " + str(upd)
tables = orm.tables()
if "scriptling_people" not in tables:
    return "tables: " + str(tables)
dele = orm.delete("scriptling_people").where("score", "<", 8.5).execute()
if dele.rows_affected != 1:
    return "delete: " + str(dele)
try:
    orm.delete("scriptling_people").execute()
    return "blanket delete allowed"
except:
    pass

# models: no columns= means the gateway reads the table's columns from the
# schema (information_schema here), once per kit, cached
def make_person(id=None, name=None, score=None, active=None):
    return {"id": id, "name": name, "score": score, "active": active}

people = orm.table(make_person, "scriptling_people", pk="id")
p = people.get(1)
if p == None or p.name != "ada" or p.score < 9.85 or p.score > 9.95:
    return "model get: " + str(p)
p.score = 9.0
people.save(p)
row = orm.select("scriptling_people").where("id", "=", 1).one()
if row["score"] != 9.0:
    return "model save: " + str(row)
people.insert(make_person(name="kurt", score=6.0, active=0))
if people.count() != 3:
    return "model insert: " + str(people.count())
# per-call columns: save writes exactly what the call lists, a listed None
# clears, and an insert list is explicit (None inserts as NULL)
k = people.get(4)
k.name = "kurtz"
k.score = 7.5
people.save(k, columns=["score"])
row = orm.select("scriptling_people").where("id", "=", 4).one()
if row["name"] != "kurt" or row["score"] != 7.5:
    return "per-call save: " + str(row)
k.score = None
people.save(k, columns=["score"])
row = orm.select("scriptling_people").where("id", "=", 4).one()
if row["score"] != None:
    return "per-call save None: " + str(row)
people.insert(make_person(name="nullscore", score=None), columns=["name", "score"])
row = orm.select("scriptling_people").where("name", "=", "nullscore").one()
if row == None or row["score"] != None:
    return "explicit None insert: " + str(row)
people.delete(people.get(2))
if people.count() != 3:
    return "model delete"
orm.drop_table("scriptling_people")
conn.close()
return "ok"
`

func runORMIntegration(t *testing.T, dsn string) {
	t.Helper()
	script := ormIntegrationScript
	script = strings.ReplaceAll(script, "DSN", dsn)
	result, err := evalInProcess(t, &plugin.Policy{Network: &plugin.NetworkPolicy{AllowLoopback: true, AllowPrivateIPs: true}}, "import scriptling.sql as sql\n"+script)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "ok" {
		t.Fatalf("script result: %s", result.Inspect())
	}
}

func TestIntegrationPostgresORM(t *testing.T) {
	dsn := os.Getenv("SCRIPTLING_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("SCRIPTLING_TEST_POSTGRES_DSN not set")
	}
	runORMIntegration(t, dsn)
}

func TestIntegrationMariaDBORM(t *testing.T) {
	dsn := os.Getenv("SCRIPTLING_TEST_MARIADB_DSN")
	if dsn == "" {
		t.Skip("SCRIPTLING_TEST_MARIADB_DSN not set")
	}
	runORMIntegration(t, dsn)
}

func TestIntegrationMySQLOrm(t *testing.T) {
	dsn := os.Getenv("SCRIPTLING_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("SCRIPTLING_TEST_MYSQL_DSN not set")
	}
	runORMIntegration(t, dsn)
}

// TestMySQLConfigTLSOption pins the one driver option reachable from a URL:
// tls accepts exactly the driver's three registered names, and anything else
// is refused rather than becoming a session variable.
func TestMySQLConfigTLSOption(t *testing.T) {
	for _, value := range []string{"true", "false", "skip-verify"} {
		parsed, err := url.Parse("mysql://db.local/db?tls=" + value)
		if err != nil {
			t.Fatal(err)
		}
		cfg, cfgErr := mysqlConfigFromURL(parsed)
		if cfgErr != nil || cfg.TLSConfig != value {
			t.Fatalf("tls=%s: cfg %v err %v", value, cfg, cfgErr)
		}
		if _, ok := cfg.Params["tls"]; ok {
			t.Fatalf("tls=%s leaked into session params", value)
		}
	}
	parsed, err := url.Parse("mysql://db.local/db?tls=custom")
	if err != nil {
		t.Fatal(err)
	}
	if _, cfgErr := mysqlConfigFromURL(parsed); cfgErr == nil {
		t.Fatal("expected an unknown tls name to be refused")
	}
}

// TestIntegrationMySQLOrmExternalIterate proves the external-mode multi-driver
// wrapper carries query_iter: iterate() used to raise because the wrapper
// only proxied query and execute. Runs against live MySQL when the DSN env is
// set, through a real external plugin binary.
func TestIntegrationMySQLOrmExternalIterate(t *testing.T) {
	dsn := os.Getenv("SCRIPTLING_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("SCRIPTLING_TEST_MYSQL_DSN not set")
	}
	bin := plugintest.BuildPlugin(t, "./cmd")
	script := "import scriptling.sql as sql\n" + `
conn = sql.connect("` + dsn + `")
orm = conn.get_orm()
orm.drop_table("scriptling_ext_iter")
(orm.create_table("scriptling_ext_iter")
 .column("id", "integer", primary_key=True, autoincrement=True)
 .column("v", "text")
 .execute())
orm.insert("scriptling_ext_iter", {"v": "a"})
orm.insert("scriptling_ext_iter", {"v": "b"})
total = 0
for row in orm.select("scriptling_ext_iter").order_by("id").iterate():
    total = total + 1
orm.drop_table("scriptling_ext_iter")
conn.close()
return total
`
	result, err := plugintest.External(t, bin,
		&plugin.Policy{Network: &plugin.NetworkPolicy{AllowLoopback: true, AllowPrivateIPs: true}}, script)
	if err != nil {
		t.Fatalf("external eval: %v", err)
	}
	if result.Inspect() != "2" {
		t.Fatalf("iterated %s rows through the external plugin, want 2", result.Inspect())
	}
}

// TestIntegrationPostgresJsonbOperators pins placeholder renumbering against
// postgres: the jsonb operators ?|, ?& and @?, question marks in comments,
// in quoted literals, and inside dollar-quoted strings ($$...$$ and
// $tag$...$tag$) are not placeholders and must survive renumbering intact.
// The bare jsonb ? operator is inherently ambiguous with a placeholder, so
// jsonb_exists() is the spelling that works.
func TestIntegrationPostgresJsonbOperators(t *testing.T) {
	dsn := os.Getenv("SCRIPTLING_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("SCRIPTLING_TEST_POSTGRES_DSN not set")
	}
	script := strings.ReplaceAll(`
conn = sql.connect("DSN")
rows = conn.query('select (\'{"k": 1}\'::jsonb ?| array[\'k\']) as a, (\'{"k": 2}\'::jsonb ?& array[\'k\']) as b, jsonb_exists(\'{"k": 3}\'::jsonb, \'k\') as c, -- comment with ?
$func$ a ? b $func$ as d, (\'{"k": 4}\'::jsonb @? \'$.k\') as e, $$?$$ as f, /* outer ? /* nested ? */ still ? */ E\'\\?\' as g from (values (1)) as t(x)')
conn.close()
return [rows[0]["a"], rows[0]["b"], rows[0]["c"], rows[0]["d"] == " a ? b ", rows[0]["e"], rows[0]["f"], rows[0]["g"]]
`, "DSN", dsn)
	result, err := evalInProcess(t, &plugin.Policy{Network: &plugin.NetworkPolicy{AllowLoopback: true, AllowPrivateIPs: true}}, "import scriptling.sql as sql\n"+script)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "[True, True, True, True, True, ?, ?]" {
		t.Fatalf("jsonb operators broken: %s", result.Inspect())
	}
}

// TestIntegrationPostgresUppercaseScheme pins case-insensitive scheme
// handling: Postgres:// used to reach pgx unnormalized (and the script-side
// dialect check fall through to the MySQL kit).
func TestIntegrationPostgresUppercaseScheme(t *testing.T) {
	dsn := os.Getenv("SCRIPTLING_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("SCRIPTLING_TEST_POSTGRES_DSN not set")
	}
	script := strings.ReplaceAll(`
conn = sql.connect("DSN")
orm = conn.get_orm()
orm.drop_table("scriptling_case")
(orm.create_table("scriptling_case").column("id", "integer", primary_key=True, autoincrement=True).execute())
orm.insert("scriptling_case", {})
rows = orm.select("scriptling_case").where("id", "=", 1).fetch()
orm.drop_table("scriptling_case")
conn.close()
return len(rows)
`, "DSN", "Postgres"+strings.TrimPrefix(dsn, "postgres"))
	result, err := evalInProcess(t, &plugin.Policy{Network: &plugin.NetworkPolicy{AllowLoopback: true, AllowPrivateIPs: true}}, "import scriptling.sql as sql\n"+script)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "1" {
		t.Fatalf("uppercase-scheme connection broken: %s", result.Inspect())
	}
}
