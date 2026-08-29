package sqlite

import (
	"os"
	"testing"

	"github.com/paularlott/scriptling"
	sqlplugin "github.com/paularlott/scriptling/plugins/sql"
)

// TestSimultaneousThreeBackends drives SQLite, MariaDB and PostgreSQL at the
// same time on one interpreter — raw queries and the ORM interleaved — and
// checks results never cross between connections. Env-gated on both DSNs.
func TestSimultaneousThreeBackends(t *testing.T) {
	mariaDSN := os.Getenv("SCRIPTLING_TEST_MARIADB_DSN")
	pgDSN := os.Getenv("SCRIPTLING_TEST_POSTGRES_DSN")
	if mariaDSN == "" || pgDSN == "" {
		t.Skip("SCRIPTLING_TEST_MARIADB_DSN and SCRIPTLING_TEST_POSTGRES_DSN not set")
	}

	p := scriptling.New()
	RegisterInProcess(p, nil)
	sqlplugin.RegisterInProcess(p, nil)

	script := `
import scriptling.sqlite as sqlite
import scriptling.sql as sql

s = sqlite.connect()
m = sql.connect("` + mariaDSN + `")
p = sql.connect("` + pgDSN + `")

conns = [["sqlite", s], ["mariadb", m], ["postgres", p]]

# table builder: the same call renders AUTOINCREMENT / AUTO_INCREMENT / SERIAL
for pair in conns:
    name = pair[0]
    conn = pair[1]
    orm = conn.get_orm()
    orm.drop_table("simul")
    r = (orm.create_table("simul")
         .column("id", "integer", primary_key=True, autoincrement=True)
         .column("name", "text", nullable=False, unique=True)
         .column("score", "real", default=0.0)
         .if_not_exists()
         .execute())
    if "simul" not in orm.tables():
        return name + ": create_table failed, tables=" + str(orm.tables())

# interleave ORM writes across all three, checking isolation as we go
for i in range(6):
    for pair in conns:
        name = pair[0]
        orm = pair[1].get_orm()
        ins = orm.insert("simul", {"name": name + "-" + str(i), "score": i * 1.5})
        if name != "postgres":
            if ins.last_insert_id != i + 1:
                return name + ": bad id sequence " + str(ins.last_insert_id)
        elif ins.rows_affected != 1:
            return name + ": insert failed"

for pair in conns:
    name = pair[0]
    orm = pair[1].get_orm()
    if orm.count("simul", "") != 6:
        return name + ": row leak, count=" + str(orm.count("simul", ""))
    rows = (orm.select("simul", "name")
            .where(orm.any_of(orm.eq("name", name + "-0"), orm.eq("name", name + "-5")))
            .order_by("id")
            .fetch())
    if len(rows) != 2 or rows[0]["name"] != name + "-0":
        return name + ": grouped select crossed, rows=" + str(rows)

# raw queries interleaved, including ? placeholders on postgres (the
# wrapper renumbers to $n)
for pair in conns:
    name = pair[0]
    conn = pair[1]
    rows = conn.query("select name from simul where score >= ? order by id limit 2", 6.0)
    if len(rows) != 2 or rows[0]["name"] != name + "-4":
        return name + ": raw query wrong, rows=" + str(rows)
    upd = conn.execute("update simul set score = ? where name = ?", 99.0, name + "-3")
    if upd.rows_affected != 1:
        return name + ": raw update wrong"

for pair in conns:
    conn = pair[1]
    orm = conn.get_orm()
    if orm.count("simul", "score >= ?", 99.0) != 1:
        return pair[0] + ": raw update effect wrong"
    orm.drop_table("simul")
    conn.close()

return "ok"
`
	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "ok" {
		t.Fatalf("script result: %s", result.Inspect())
	}
}
