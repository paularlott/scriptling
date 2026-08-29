#!/usr/bin/env scriptling
"""ORM example: conn.get_orm() — queries, builders and models.

Covers the kwargs forms, the chained query builder (including grouped
criteria), the where_sql escape hatch, and model gateways. Runs against
SQLite out of the box; point DSN at MariaDB/MySQL/PostgreSQL to run the
identical script against a server (see README.md for containers).
"""

import os
import scriptling.sqlite as sqlite

DSN = os.getenv("SCRIPTLING_ORM_SQLITE_PATH", "/tmp/scriptling-example-orm.db")

print("=== ORM over SQLite ===\n")

conn = sqlite.connect(DSN)
orm = conn.get_orm()

# table builder: the same calls render AUTOINCREMENT on SQLite,
# AUTO_INCREMENT on MySQL/MariaDB and SERIAL on PostgreSQL
(orm.create_table("people")
 .column("id", "integer", primary_key=True, autoincrement=True)
 .column("name", "text", nullable=False)
 .column("score", "real", default=0.0)
 .column("active", "integer", default=1)
 .if_not_exists()
 .execute())

# kwargs forms: dicts in, parameters bound — nothing interpolated
ins = orm.insert("people", {"name": "ada", "score": 9.5})
print("inserted id:", ins.last_insert_id)
orm.insert("people", {"name": "grace", "score": 8.0})
orm.insert("people", {"name": "linus", "score": 7.0, "active": 0})

print("everyone:", orm.count("people", ""))
print("high scorers:", orm.count("people", "score >= ?", 8.0))

# builder: flat conditions AND together, terminal fetch() runs one query
rows = (orm.select("people", "name", "score")
        .where("score", ">=", 8.0)
        .order_by("score", desc=True)
        .fetch())
for row in rows:
    print(f"  {row['name']} scored {row['score']}")

# builder: grouped criteria — (a OR b) AND (a OR c)
rows = (orm.select("people", "name")
        .where(orm.any_of(orm.eq("name", "ada"), orm.eq("name", "grace")))
        .where(orm.any_of(orm.eq("active", 1), orm.ge("score", 9.0)))
        .fetch())
print("grouped:", [row["name"] for row in rows])

# builder: collections, one(), count(), limit
rows = orm.select("people").where(orm.one_of("name", ["ada", "grace"])).fetch()
print("one_of:", [row["name"] for row in rows])
one = orm.select("people").where("name", "=", "linus").one()
print("one:", one["name"])
print("builder count:", orm.select("people").where("active", "=", 1).count())
print("limited:", len(orm.select("people").order_by("id").limit(2).fetch()))

# iterate: stream row by row instead of materialising the result
total = 0
for row in orm.select("people", "name").where("score", ">=", 8.0).iterate():
    total = total + 1
print("iterated high scorers:", total)

# where_sql: escape hatch for anything the builder cannot express
rows = orm.select("people").where_sql("score > ? and score < ?", 6.5, 9.0).fetch()
print("where_sql:", [row["name"] for row in rows])

# update and delete refuse to run without a where clause
upd = orm.update("people", {"score": 9.9}, "name = ?", "ada")
print("updated rows:", upd.rows_affected)
dele = orm.delete("people", "active = ?", 0)
print("deleted rows:", dele.rows_affected)

print("tables:", orm.tables())

# models: a factory function turns row dicts into your objects; the gateway
# carries the table, primary key and column list.
def make_person(id=None, name=None, score=None, active=None):
    return {"id": id, "name": name, "score": score, "active": active}

people = orm.table(make_person, "people", pk="id",
                   columns=["id", "name", "score", "active"])

p = people.get(1)
print("model get:", p["name"], p["score"])
p["score"] = 8.8
people.save(p)
print("model save:", orm.count("people", "score >= ?", 8.8))

people.insert(make_person(name="kurt", score=6.0, active=0))
print("model insert:", people.count())
people.delete(people.get(1))
print("model delete:", people.count())
rows = people.select("name").where("active", "=", 0).fetch()
print("model select:", [row["name"] for row in rows])

orm.drop_table("people")
conn.close()
