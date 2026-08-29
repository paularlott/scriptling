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
	cfg := mysqlConfigFromURL(parsed)
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
	cfg = mysqlConfigFromURL(bare)
	if cfg.Addr != "db.local:3306" || cfg.User != "" || cfg.DBName != "" {
		t.Fatalf("defaults wrong: %#v", cfg)
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
if orm.count("scriptling_people", "") != 3:
    return "count all: " + str(orm.count("scriptling_people", ""))
if orm.count("scriptling_people", "score >= ?", 8.0) != 2:
    return "count where: " + str(orm.count("scriptling_people", "score >= ?", 8.0))
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
upd = orm.update("scriptling_people", {"score": 9.9}, "name = ?", "ada")
if upd.rows_affected != 1:
    return "update: " + str(upd)
tables = orm.tables()
if "scriptling_people" not in tables:
    return "tables: " + str(tables)
dele = orm.delete("scriptling_people", "score < ?", 8.0)
if dele.rows_affected != 1:
    return "delete: " + str(dele)
try:
    orm.delete("scriptling_people", "")
    return "blanket delete allowed"
except:
    pass
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
