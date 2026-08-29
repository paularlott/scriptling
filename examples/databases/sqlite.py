#!/usr/bin/env scriptling
"""SQLite example: the relational API shared with plugin.sql.

Covers connect (file, :memory:), execute with parameters, last_insert_id,
rows_affected, query with parameters, and close. No database server needed —
run with any build that has the sqlite plugin (compiled in via the
plugin_sqlite build tag, or scriptling-plugin-sqlite on the plugin path).
"""

import scriptling.sqlite as sqlite

print("=== SQLite ===\n")

# An on-disk database; sqlite.connect() with no arguments gives ":memory:".
conn = sqlite.connect("/tmp/scriptling-example.db")
conn.execute("drop table if exists people")
conn.execute("create table people (id integer primary key autoincrement, name text, score real)")

# execute() returns {"last_insert_id": ..., "rows_affected": ...}
result = conn.execute("insert into people (name, score) values (?, ?)", "ada", 9.5)
print("inserted id:", result.last_insert_id)
result = conn.execute("insert into people (name, score) values (?, ?)", "grace", 8.0)
print("inserted id:", result.last_insert_id)

result = conn.execute("update people set score = ? where name = ?", 8.5, "grace")
print("rows updated:", result.rows_affected)

# query() returns a list of dicts keyed by column name
rows = conn.query("select id, name, score from people order by id")
for row in rows:
    print(f"  {row['id']}: {row['name']} scored {row['score']}")

# Parameterised lookups
rows = conn.query("select name from people where score > ?", 9.0)
print("high scorers:", [row["name"] for row in rows])

conn.close()

# :memory: needs no file and is always allowed by the security policy
mem = sqlite.connect()
mem.execute("create table t (v text)")
mem.execute("insert into t (v) values (?)", "ephemeral")
print("\nin-memory:", mem.query("select v from t")[0]["v"])
mem.close()
