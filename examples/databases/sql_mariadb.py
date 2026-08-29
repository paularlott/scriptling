#!/usr/bin/env scriptling
"""MariaDB / MySQL example: the relational API shared with plugin.sqlite.

Point DSN at any MariaDB or MySQL server (both speak the MySQL wire
protocol). See README.md for a ready-to-run container.
"""

import os
import scriptling.sql as sql

DSN = os.getenv("SCRIPTLING_MARIADB_DSN", "mariadb://root:scriptling@127.0.0.1:13306/scriptling")

print("=== MariaDB / MySQL ===\n")

conn = sql.connect(DSN)
print("server:", conn.query("select version() as v")[0]["v"])

conn.execute("drop table if exists people")
conn.execute("""
    create table people (
        id integer primary key auto_increment,
        name varchar(100) not null,
        score decimal(4,1)
    )
""")

result = conn.execute("insert into people (name, score) values (?, ?)", "ada", 9.5)
print("inserted id:", result.last_insert_id)
result = conn.execute("insert into people (name, score) values (?, ?)", "grace", 8.0)
print("inserted id:", result.last_insert_id)

result = conn.execute("update people set score = ? where name = ?", 8.5, "grace")
print("rows updated:", result.rows_affected)

rows = conn.query("select id, name, score from people order by id")
for row in rows:
    print(f"  {row['id']}: {row['name']} scored {row['score']}")

rows = conn.query("select name from people where score > ?", 9.0)
print("high scorers:", [row["name"] for row in rows])

conn.execute("drop table people")
conn.close()
