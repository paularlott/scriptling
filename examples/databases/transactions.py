#!/usr/bin/env scriptling
"""Transactions: begin, commit and rollback on the relational plugins.

The same surface runs on scriptling.sqlite and scriptling.sql (MySQL,
MariaDB, PostgreSQL): conn.begin() returns a Transaction whose query and
execute run inside it, and commit() keeps the work while rollback()
discards it. tx.get_orm() binds the ORM to the transaction too.

Runs unchanged against any of the three servers from the README by setting
SCRIPTLING_TX_BACKEND to "sqlite" (default), "mariadb", "mysql" or
"postgres"; the server examples assume the addresses and credentials the
README's containers use.
"""

import os

backend = os.environ.get("SCRIPTLING_TX_BACKEND", "sqlite")

if backend == "sqlite":
    import scriptling.sqlite as db

    conn = db.connect()
else:
    import scriptling.sql as db

    dsns = {
        "mariadb": "mariadb://root:scriptling@localhost:13306/scriptling",
        "mysql": "mysql://root:scriptling@localhost:13307/scriptling",
        "postgres": "postgres://postgres:scriptling@localhost:15432/scriptling",
    }
    conn = db.connect(dsns[backend])

orm = conn.get_orm()
orm.drop_table("accounts")
(orm.create_table("accounts")
 .column("id", "integer", primary_key=True, autoincrement=True)
 .column("name", "text", nullable=False)
 .column("balance", "integer", nullable=False, default=0)
 .execute())
orm.insert("accounts", {"name": "ada", "balance": 100})
orm.insert("accounts", {"name": "grace", "balance": 100})


def balance(name):
    return orm.select("accounts").where("name", "=", name).one()["balance"]


# A transfer is two updates that must land together: a failure between them
# would leave money created or destroyed. Both succeed, so commit keeps them.
tx = conn.begin()
tx.execute("update accounts set balance = balance - 25 where name = ?", "ada")
tx.execute("update accounts set balance = balance + 25 where name = ?", "grace")
tx.commit()
print("after transfer:", balance("ada"), balance("grace"))  # 75 125

# The same transfer with a failure in the middle: the missing target takes
# no money (rows_affected is 0), so the transfer is rejected and rollback
# restores the first update — no money moves at all.
tx = conn.begin()
tx.execute("update accounts set balance = balance - 25 where name = ?", "ada")
try:
    upd = tx.execute("update accounts set balance = balance + 25 where name = ?", "nobody")
    if upd.rows_affected != 1:
        raise ValueError("transfer target missing")
except Exception:
    pass
tx.rollback()
print("after rollback:", balance("ada"), balance("grace"))  # 75 125

# The ORM joins a transaction through tx.get_orm(): its inserts, updates
# and queries run inside it.
tx = conn.begin()
torm = tx.get_orm()
torm.insert("accounts", {"name": "linus", "balance": 0})
torm.update("accounts", {"balance": 50}).where("name", "=", "linus").execute()
tx.commit()
print("after orm commit:", balance("linus"))  # 50

tx = conn.begin()
tx.get_orm().update("accounts", {"balance": 999}).where("name", "=", "linus").execute()
tx.rollback()
print("after orm rollback:", balance("linus"))  # 50

orm.drop_table("accounts")
conn.close()
