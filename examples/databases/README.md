# Database Examples

Scripts for the four database plugins: `scriptling.sqlite`, `scriptling.sql`
(MySQL / MariaDB / PostgreSQL), `scriptling.valkey` and `scriptling.badgerdb`. The
relational plugins share one API, and the key/value plugins share another, so
there are really only two surfaces to learn.

| Script | Plugin | Needs |
|---|---|---|
| `sqlite.py` | `scriptling.sqlite` | nothing (embedded) |
| `sql_mariadb.py` | `scriptling.sql` | a MariaDB or MySQL server |
| `sql_mysql.py` | `scriptling.sql` | a MySQL or MariaDB server |
| `sql_postgres.py` | `scriptling.sql` | a PostgreSQL server |
| `valkey.py` | `scriptling.valkey` | a Valkey or Redis server |
| `badgerdb.py` | `scriptling.badgerdb` | nothing (embedded) |
| `orm.py` | `scriptling.sqlite` | nothing (SQLite); set `SCRIPTLING_ORM_SQLITE_PATH` to place the file |
| `transactions.py` | either relational plugin | nothing by default (SQLite); set `SCRIPTLING_TX_BACKEND` to `mariadb`, `mysql` or `postgres` for a server |

## Running the examples

The plugins are available in two modes — the scripts are identical either way:

The `orm.py` example covers `conn.get_orm()` — the dict-shaped table helper
shared by the relational plugins. The `transactions.py` example covers
`conn.begin()` with `commit()`/`rollback()` (and the ORM inside a
transaction) on the same shared surface.

```bash
# 1. A build with the plugins compiled in (task build, or any subset:
#    go build -tags plugin_sqlite -o scriptling ./scriptling-cli)
scriptling sqlite.py
scriptling badgerdb.py
scriptling sql_mysql.py

# 2. A slim build, with the plugin binaries on the plugin path
scriptling --plugin-dir bin sql_postgres.py
```

## Starting test databases

Any MariaDB/PostgreSQL/Valkey server works; for a quick local one the
`knot` images on Docker Hub are convenient:

```bash
docker run -d --name mariadb -p 13306:3306 \
  -e MARIADB_ROOT_PASSWORD=scriptling -e MARIADB_DATABASE=scriptling \
  paularlott/knot-mariadb:12.3

docker run -d --name mysql -p 13307:3306 \
  -e MYSQL_ROOT_PASSWORD=scriptling -e MYSQL_DATABASE=scriptling \
  paularlott/knot-mysql:9.7

docker run -d --name postgres -p 15432:5432 \
  -e POSTGRES_PASSWORD=scriptling -e POSTGRES_DB=scriptling \
  paularlott/knot-postgres:18

docker run -d --name valkey -p 16379:6379 \
  paularlott/knot-valkey:9.1 \
  valkey-server /etc/valkey/valkey-server.conf --bind 0.0.0.0 --protected-mode no
```

The images carry version tags (`12.3`, `9.7`, `18`, `9.1` and so on), not a
`latest`: pick the line you want.

The scripts default to those addresses and credentials; override with the
`SCRIPTLING_MARIADB_DSN`, `SCRIPTLING_POSTGRES_DSN` and
`SCRIPTLING_VALKEY_URL` environment variables, e.g.:

```bash
scriptling sql_postgres.py                       # defaults above
SCRIPTLING_POSTGRES_DSN='postgres://u:p@db.internal:5432/app' scriptling sql_postgres.py
```

`mysql://` and `mariadb://` DSNs are interchangeable — MySQL and MariaDB
speak the same protocol, so `sql_mariadb.py` works against either.
