#!/usr/bin/env scriptling
"""PostgreSQL example: the relational API shared with plugin.sqlite.

? placeholders are translated to $n automatically, so queries written for
sqlite/mysql work unchanged; explicit $n also works. last_insert_id is 0 on
postgres (it has no such concept) — use `returning` instead, shown below.
"""

import os
import scriptling.sql as sql

DSN = os.getenv("SCRIPTLING_POSTGRES_DSN", "postgres://postgres:scriptling@127.0.0.1:15432/scriptling")

print("=== PostgreSQL ===\n")

conn = sql.connect(DSN)
print("server:", conn.query("select version() as v")[0]["v"])

conn.execute("drop table if exists people")
conn.execute("""
    create table people (
        id serial primary key,
        name text not null,
        score real
    )
""")

# ? works (translated to $1, $2) and so does explicit $n
result = conn.execute("insert into people (name, score) values (?, ?)", "ada", 9.5)
result = conn.execute("insert into people (name, score) values ($1, $2)", "grace", 8.0)

result = conn.execute("update people set score = ? where name = ?", 8.5, "grace")
print("rows updated:", result.rows_affected)

# The postgres way to get the generated id: returning + query
rows = conn.query("insert into people (name, score) values (?, ?) returning id", "linus", 7.0)
print("returned id:", rows[0]["id"])

rows = conn.query("select id, name, score from people order by id")
for row in rows:
    print(f"  {row['id']}: {row['name']} scored {row['score']}")

rows = conn.query("select name from people where score > ?", 9.0)
print("high scorers:", [row["name"] for row in rows])

conn.execute("drop table people")
conn.close()
