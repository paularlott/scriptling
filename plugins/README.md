# Database Plugins

Four first-party database plugins, each usable in two modes:

- **External plugin** — a standalone binary loaded with `--plugin` / `--plugin-dir`
  (or `SCRIPTLING_PLUGIN_DIR`). Scripts import `plugin.<name>`; calls cross a
  JSON-RPC boundary to the plugin process.
- **Compiled in** — build the CLI with the plugin's build tag and the identical
  library is registered natively (no subprocess, full speed). The default
  `scriptling` build compiles all four in.

Scripts are the same in both modes — always `import scriptling.sqlite` etc.

| Plugin | Import | API | Backend |
|---|---|---|---|
| sqlite | `scriptling.sqlite` | relational | SQLite via [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) (pure Go) |
| sql | `scriptling.sql` | relational | MySQL, MariaDB, PostgreSQL |
| valkey | `scriptling.valkey` | key/value | Valkey and Redis |
| badgerdb | `scriptling.badgerdb` | key/value | BadgerDB (embedded) |

## Relational API

Shared by `scriptling.sqlite` and `scriptling.sql`:

```python
import scriptling.sqlite as sqlite

conn = sqlite.connect("app.db")          # or sqlite.connect() for :memory:
conn.execute("create table people (id integer primary key autoincrement, name text)")
result = conn.execute("insert into people (name) values (?)", "ada")
print(result.last_insert_id, result.rows_affected)

rows = conn.query("select * from people where name = ?", "ada")
print(rows[0]["name"])                   # rows are dicts keyed by column name

conn.close()
```

- `connect(path, timeout_ms=5000)` — sqlite: a file path or `":memory:"`
  (always allowed, no policy needed).
- `connect(dsn)` — sql: the scheme picks the driver — `postgres://` /
  `postgresql://`, `mysql://`, `mariadb://` (MySQL and MariaDB share the MySQL
  protocol). `?` placeholders are translated to `$n` on PostgreSQL, which also
  accepts explicit `$n`. `last_insert_id` is 0 on postgres (it has no such
  concept; use `returning`).
- `Connection.query(sql, *params)` — returns a list of row dicts. Values are
  ints, floats, bools, strings or null.
- `Connection.execute(sql, *params)` — returns
  `{"last_insert_id": int, "rows_affected": int}`.
- `Connection.close()`.

The `Connection` class can also be constructed directly:
`sqlite.Connection(path, timeout_ms=5000)`.

## Key/Value API

`scriptling.valkey` and `scriptling.badgerdb` expose the identical surface, so scripts
move between a shared cache and local storage unchanged:

```python
import scriptling.valkey as valkey

client = valkey.connect("valkey://localhost:6379")
client.set("greeting", "hello", ttl_seconds=60)
print(client.get("greeting"))
print(client.ttl("greeting"))            # remaining seconds
print(client.keys("gr*"))
client.incr("hits")
client.close()
```

- `valkey.connect(url)` — schemes `valkey://`, `redis://`, `tcp://` (plaintext)
  and `valkeys://`, `rediss://` (TLS); optional `user:pass@` and a `/db` path.
  Single-node servers (the client does not do cluster routing).
- `badger.open(path)` — opens (creating if needed) a database directory.
  Badger allows one process to hold a database open at a time.
- `Client` methods (both plugins): `get(key)` → str|null, `set(key, value,
  ttl_seconds=0)`, `delete(*keys)` → count removed, `exists(*keys)` → count,
  `expire(key, ttl_seconds)` → bool, `ttl(key)` → seconds|null (-1 = no
  expiry), `incr(key, amount=1)`, `decr(key, amount=1)`, `keys(pattern)` →
  glob match, `ping()`, `close()`.

## Security policy

The host delivers its security context — `--allowed-paths` and the network
policy — to every plugin in the `scriptling.handshake` params, and these
first-party plugins enforce it on every operation:

- sqlite/badger: the database path must be inside the allowed paths.
- sql/valkey: connections dial through the same guard as the `requests`
  library — host allow/deny lists, category blocks (loopback, private,
  link-local), and DNS-rebinding-safe validated-IP dialing.

Plugins that predate the policy block simply ignore it (it is an additive,
optional handshake field), and third-party plugins opt in by advertising the
`policy` capability and enforcing what they receive.

## Building

```bash
task build-plugins              # plugin binaries for the current platform
task build-plugins-platforms    # all platforms
task build                      # CLI with all four compiled in (the default)
task build-slim                 # CLI without the plugins compiled in
task build-slim-platforms       # scriptling-slim for all platforms
```

Compile in a subset with build tags, e.g. a CLI with only valkey:

```bash
go build -tags plugin_valkey -o scriptling ./scriptling-cli
```

Tags: `plugin_sqlite`, `plugin_sql`, `plugin_valkey`, `plugin_badgerdb`.
Everything is pure Go — all six release platforms cross-compile without cgo.

Runnable examples live in [examples/databases](../examples/databases) —
including container commands for local MariaDB, MySQL, PostgreSQL and Valkey
test servers.

## Installing

- **scriptling** has all four plugins compiled in
  (`brew install paularlott/tap/scriptling`, or the release zip).
- **scriptling-slim** is the same CLI without them
  (`brew install paularlott/tap/scriptling-slim`, or the slim release zip).
- **Any build** loads the external plugin binaries explicitly: point
  `--plugin-dir` at their directory (or export `SCRIPTLING_PLUGIN_DIR`);
  nothing is auto-discovered, so what loads is exactly what you named. The
  Homebrew plugin formula installs them under
  `$(brew --prefix)/opt/scriptling-plugins/libexec/plugins`.
