#!/usr/bin/env scriptling
"""MySQL example: the relational API shared with plugin.sqlite.

Point DSN at any MySQL server; MariaDB speaks the same protocol, so
sql_mariadb.py works against MySQL and this script works against MariaDB.
See README.md for a ready-to-run container.
"""

import os
import scriptling.sql as sql

DSN = os.getenv("SCRIPTLING_MYSQL_DSN", "mysql://root:scriptling@127.0.0.1:13307/scriptling")

print("=== MySQL ===\n")

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

# The ORM: dict-shaped rows without hand-written SQL
orm = conn.get_orm()
orm.insert("people", {"name": "linus", "score": 7.0})
high = orm.select("people", columns=["name"], where="score >= ?", params=[8.0], order_by="score desc")
print("orm high scorers:", [row["name"] for row in high])
print("orm tables:", orm.tables())

conn.execute("drop table people")
conn.close()
