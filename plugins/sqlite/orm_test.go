package sqlite

import (
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/plugins/internal/plugintest"
)

// ormScript exercises the full script-side ORM surface: the quick insert
// form, the query builders for select/update/delete (including grouped
// criteria), where_sql, and models. The sql plugin runs the same script
// against live MariaDB and PostgreSQL in its env-gated integration tests,
// proving the dialect handling.
const ormScript = `
conn = sqlite.connect()

orm = conn.get_orm()
orm.drop_table("people")
(orm.create_table("people")
 .column("id", "integer", primary_key=True, autoincrement=True)
 .column("name", "text", nullable=False)
 .column("score", "real", default=0.0)
 .column("active", "integer", default=1)
 .if_not_exists()
 .execute())

# quick insert form
ins = orm.insert("people", {"name": "ada", "score": 9.5, "active": 1})
if ins.last_insert_id != 1:
    return "insert id: " + str(ins.last_insert_id)
orm.insert("people", {"name": "grace", "score": 8.0, "active": 1})
orm.insert("people", {"name": "linus", "score": 7.0, "active": 0})

if orm.select("people").count() != 3:
    return "count all: " + str(orm.select("people").count())
if orm.select("people").where_sql("score >= ?", 8.0).count() != 2:
    return "count where"

# builder: flat conditions
rows = orm.select("people", "name", "score").where("score", ">=", 8.0).order_by("score", desc=True).fetch()
if len(rows) != 2 or rows[0]["name"] != "ada":
    return "builder select: " + str(rows)

# builder: (a OR b) AND (a OR c)
rows = (orm.select("people", "name")
        .where(orm.any_of(orm.eq("name", "ada"), orm.eq("name", "grace")))
        .where(orm.any_of(orm.eq("active", 1), orm.ge("score", 9.0)))
        .fetch())
if len(rows) != 2:
    return "grouped criteria: " + str(rows)

# builder: all_of nesting, one_of, not_one_of
rows = orm.select("people").where(orm.all_of(
        orm.one_of("name", ["ada", "grace", "nobody"]),
        orm.not_one_of("name", ["grace"]),
    )).fetch()
if len(rows) != 1 or rows[0]["name"] != "ada":
    return "one_of/not_one_of: " + str(rows)

# builder: limit/one/count
rows = orm.select("people").order_by("id").limit(2).fetch()
if len(rows) != 2:
    return "limit: " + str(len(rows))
one = orm.select("people").where("name", "=", "linus").one()
if one == None or one["score"] != 7.0:
    return "one: " + str(one)
if orm.select("people").where("active", "=", 1).count() != 2:
    return "builder count"

# iterate: row-by-row without materialising
total = 0
names = []
for row in orm.select("people", "name").where("score", ">=", 7.0).order_by("id").iterate():
    names.append(row["name"])
    total = total + 1
if total != 3 or names[0] != "ada":
    return "iterate: " + str(names)
# partial consumption + explicit close
it = orm.select("people").order_by("id").iterate()
first = it.__next__()
it.close()
if first["name"] != "ada":
    return "iterate partial"
if orm.select("people").count() != 3:
    return "iterate changed data?!"

# where_sql escape hatch
rows = orm.select("people").where_sql("score > ? and score < ?", 6.5, 9.0).fetch()
if len(rows) != 2:
    return "where_sql: " + str(rows)

# where_sql binds params like every other value: a hostile string stays
# data. Spliced raw, this WHERE would be always-true and match all 3 rows.
needle = "x' OR '1'='1"
if orm.select("people").where_sql("name = ?", needle).count() != 0:
    return "where_sql leaked a param into SQL"
# mixed builder criteria and where_sql keep placeholder order straight
# (the sql plugin's live runs prove this on numbered dialects too)
if orm.select("people").where("active", "=", 1).where_sql("score > ?", 6.5).count() != 2:
    return "where_sql mixed: " + str(orm.select("people").where("active", "=", 1).where_sql("score > ?", 6.5).fetch())
# a ? inside a quoted literal is a literal: every renumber pass must skip it
if orm.select("people").where_sql("name != 'o''brien?' and score > ?", 0.0).count() != 3:
    return "where_sql literal question mark"

# count() is an integer, whatever the driver hands back
if not isinstance(orm.select("people").count(), int):
    return "count type"

# update/delete refuse blanket writes
try:
    orm.update("people", {"score": 0.0}).execute()
    return "blanket update allowed"
except:
    pass
try:
    orm.delete("people").execute()
    return "blanket delete allowed"
except:
    pass

# update/delete builders: criteria, groups and the where_sql escape hatch
upd = orm.update("people", {"score": 9.9}).where("name", "=", "ada").execute()
if upd.rows_affected != 1:
    return "update: " + str(upd)
upd = (orm.update("people", {"active": 1})
       .where(orm.any_of(orm.eq("name", "ada"), orm.eq("name", "grace")))
       .execute())
if upd.rows_affected != 2:
    return "update groups: " + str(upd)
upd = orm.update("people", {"score": 8.5}).where_sql("name = ?", "grace").execute()
if upd.rows_affected != 1:
    return "update where_sql: " + str(upd)
dele = orm.delete("people").where("name", "=", "linus").execute()
if dele.rows_affected != 1:
    return "delete: " + str(dele)
if orm.select("people").count() != 2:
    return "after delete: " + str(orm.select("people").count())
dele = orm.delete("people").where(orm.one_of("name", ["ada", "nobody"])).execute()
if dele.rows_affected != 1:
    return "delete criteria: " + str(dele)
if orm.select("people").count() != 1:
    return "after criteria delete"

if "people" not in orm.tables():
    return "tables: " + str(orm.tables())

# non-finite float defaults have no SQL literal: refuse them at render time
# rather than emitting invalid DDL
for bad in [float("inf"), float("-inf"), float("nan")]:
    try:
        (orm.create_table("bad_defaults").column("x", "real", default=bad).execute())
        return "non-finite default accepted: " + str(bad)
    except:
        pass

# models: a factory function builds instances from row dicts. Without
# columns= the gateway writes every column the table has, read from the
# schema once per kit and cached
def make_person(id=None, name=None, score=None, active=None):
    return {"id": id, "name": name, "score": score, "active": active}

people = orm.table(make_person, "people", pk="id")
p = people.get(2)
if p == None or p.name != "grace" or p.score != 8.5:
    return "model get: " + str(p)
p.score = 8.8
people.save(p)
if orm.select("people").where("score", ">=", 8.8).count() != 1:
    return "model save"
people.insert(make_person(name="kurt", score=6.0, active=0))
if orm.select("people").count() != 2:
    return "model insert"
people.delete(people.get(2))
if orm.select("people").count() != 1:
    return "model delete"
if people.count() != 1:
    return "model count"
rows = people.select("name").where("active", "=", 0).fetch()
if len(rows) != 1:
    return "model select: " + str(rows)

# columns= restricts what insert/save touch: renaming kurt must leave his
# score alone even though the object carries a stale value
named = orm.table(make_person, "people", pk="id", columns=["id", "name"])
k = named.get(4)
if k == None or k.name != "kurt":
    return "restricted get: " + str(k)
k.score = 0.0
k.name = "kurtz"
named.save(k)
row = orm.select("people").where("id", "=", 4).one()
if row["name"] != "kurtz" or row["score"] != 6.0:
    return "restricted save leaked: " + str(row)

orm.drop_table("people")
conn.close()
return "ok"
`

func TestInProcessORM(t *testing.T) {
	p := scriptling.New()
	RegisterInProcess(p, nil)
	result, err := p.Eval("import scriptling.sqlite as sqlite\n" + ormScript)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "ok" {
		t.Fatalf("script result: %s", result.Inspect())
	}
}

// TestExternalORM proves the script-side ORM works over the wire: the whole
// kit executes host-side, only query/execute calls round-trip.
func TestExternalORM(t *testing.T) {
	bin := plugintest.BuildPlugin(t, "./cmd")
	result, err := plugintest.External(t, bin, nil, "import scriptling.sqlite as sqlite\n"+ormScript)
	if err != nil {
		t.Fatalf("external eval: %v", err)
	}
	if result.Inspect() != "ok" {
		t.Fatalf("script result: %s", result.Inspect())
	}
}
