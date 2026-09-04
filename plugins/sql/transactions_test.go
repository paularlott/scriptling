package sql

import (
	"os"
	"strings"
	"testing"

	"github.com/paularlott/scriptling/plugin"
	"github.com/paularlott/scriptling/plugins/internal/plugintest"
)

// transactionIntegrationScript exercises the transaction surface against a
// live server. DDL comes from the dialect-aware ORM builder so one script
// serves postgres and mysql/mariadb; ? placeholders exercise the renumbering
// path on postgres. The isolation probe (a connection-level read while the
// transaction is open) is standard READ COMMITTED / REPEATABLE READ
// behaviour: the uncommitted row is invisible outside the transaction.
const transactionIntegrationScript = `
conn = sql.connect("DSN")
orm = conn.get_orm()
orm.drop_table("scriptling_tx")
(orm.create_table("scriptling_tx")
 .column("id", "integer", primary_key=True, autoincrement=True)
 .column("name", "text", nullable=False)
 .execute())
conn.execute("insert into scriptling_tx (name) values (?)", "committed")

tx = conn.begin()
ins = tx.execute("insert into scriptling_tx (name) values (?)", "pending")
if ins.rows_affected != 1:
    return "tx insert: " + str(ins)
inside = tx.query("select name from scriptling_tx order by id")
if len(inside) != 2 or inside[1]["name"] != "pending":
    return "inside tx: " + str(inside)
outside = conn.query("select name from scriptling_tx")
if len(outside) != 1:
    return "uncommitted row visible outside tx: " + str(outside)
tx.rollback()

if orm.select("scriptling_tx").count() != 1:
    return "rollback leaked"
try:
    tx.query("select 1")
    return "query after rollback worked"
except Exception as e:
    if "already committed or rolled back" not in str(e):
        return "done error shape: " + str(e)

tx = conn.begin()
tx.execute("insert into scriptling_tx (name) values (?)", "kept")
cur = tx.query_iter("select name from scriptling_tx order by id")
seen = []
row = cur.next()
while row != None:
    seen.append(row["name"])
    row = cur.next()
cur.close()
if len(seen) != 2 or seen[1] != "kept":
    return "tx query_iter: " + str(seen)
tx.commit()

if orm.select("scriptling_tx").count() != 2:
    return "commit lost"
try:
    tx.commit()
    return "double commit worked"
except Exception as e:
    if "already committed or rolled back" not in str(e):
        return "double commit error shape: " + str(e)

tx = conn.begin()
torm = tx.get_orm()
torm.insert("scriptling_tx", {"name": "orm-rollback"})
tx.rollback()
tx = conn.begin()
torm = tx.get_orm()
torm.insert("scriptling_tx", {"name": "orm-commit"})
tx.commit()

names = [r["name"] for r in orm.select("scriptling_tx").order_by("id").fetch()]
if names != ["committed", "kept", "orm-commit"]:
    return "orm in tx: " + str(names)
orm.drop_table("scriptling_tx")
conn.close()
return "ok"
`

func runTransactionIntegration(t *testing.T, dsn string) {
	t.Helper()
	script := "import scriptling.sql as sql\n" + strings.ReplaceAll(transactionIntegrationScript, "DSN", dsn)
	result, err := evalInProcess(t, &plugin.Policy{Network: &plugin.NetworkPolicy{AllowLoopback: true, AllowPrivateIPs: true}}, script)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "ok" {
		t.Fatalf("script result: %s", result.Inspect())
	}
}

func TestIntegrationPostgresTransactions(t *testing.T) {
	dsn := os.Getenv("SCRIPTLING_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("SCRIPTLING_TEST_POSTGRES_DSN not set")
	}
	runTransactionIntegration(t, dsn)
}

func TestIntegrationMySQLTransactions(t *testing.T) {
	dsn := os.Getenv("SCRIPTLING_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("SCRIPTLING_TEST_MYSQL_DSN not set")
	}
	runTransactionIntegration(t, dsn)
}

func TestIntegrationMariaDBTransactions(t *testing.T) {
	dsn := os.Getenv("SCRIPTLING_TEST_MARIADB_DSN")
	if dsn == "" {
		t.Skip("SCRIPTLING_TEST_MARIADB_DSN not set")
	}
	runTransactionIntegration(t, dsn)
}

// TestIntegrationTransactionsExternal drives begin/commit/rollback through
// the wire path: the plugin process holds the transaction and the host-side
// multi-driver wrappers (including postgres placeholder renumbering) proxy
// every call.
func TestIntegrationTransactionsExternal(t *testing.T) {
	dsn := firstSetEnv(t, "SCRIPTLING_TEST_POSTGRES_DSN", "SCRIPTLING_TEST_MYSQL_DSN", "SCRIPTLING_TEST_MARIADB_DSN")
	if dsn == "" {
		t.Skip("no database DSN env var set")
	}
	bin := plugintest.BuildPlugin(t, "./cmd")
	script := "import scriptling.sql as sql\n" + strings.ReplaceAll(`
conn = sql.connect("DSN")
conn.execute("drop table if exists scriptling_tx_ext")
(conn.get_orm().create_table("scriptling_tx_ext")
 .column("id", "integer", primary_key=True, autoincrement=True)
 .column("name", "text", nullable=False)
 .execute())

tx = conn.begin()
tx.execute("insert into scriptling_tx_ext (name) values (?)", "kept")
rows = tx.query("select name from scriptling_tx_ext where name = ?", "kept")
if len(rows) != 1:
    return "inside tx: " + str(rows)
tx.commit()

tx = conn.begin()
tx.execute("insert into scriptling_tx_ext (name) values (?)", "discarded")
tx.rollback()

rows = conn.query("select name from scriptling_tx_ext")
conn.execute("drop table scriptling_tx_ext")
conn.close()
if len(rows) != 1 or rows[0]["name"] != "kept":
    return "after both: " + str(rows)
return "ok"
`, "DSN", dsn)
	result, err := plugintest.External(t, bin,
		&plugin.Policy{Network: &plugin.NetworkPolicy{AllowLoopback: true, AllowPrivateIPs: true}}, script)
	if err != nil {
		t.Fatalf("external eval: %v", err)
	}
	if result.Inspect() != "ok" {
		t.Fatalf("script result: %s", result.Inspect())
	}
}

func firstSetEnv(t *testing.T, names ...string) string {
	t.Helper()
	for _, name := range names {
		if value := os.Getenv(name); value != "" {
			return value
		}
	}
	return ""
}
