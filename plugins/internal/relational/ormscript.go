package relational

import (
	"fmt"
	"strings"
)

// The ORM's user-facing surface is scriptling script, not Go: it executes on
// the host in both plugin modes (compiled-in and external), so chained
// builder calls cost no JSON-RPC round trips and one implementation serves
// every backend — the connection's dialect() drives quoting, placeholder
// numbering and case-insensitive matching.
//
// The script below uses § where a backtick is meant; scriptKitSource
// substitutes the real character so the source can live in a Go raw string.

const scriptKitRaw = `
def _orm_check_ident(name):
    allowed = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_"
    if name == "":
        raise ValueError("empty identifier")
    for ch in name:
        if ch not in allowed:
            raise ValueError("unsafe identifier: " + name)
    return name


def _orm_quote(name, ctx):
    _orm_check_ident(name)
    if ctx["backtick"]:
        return "§" + name + "§"
    return '"' + name + '"'


def _orm_mark(n, ctx):
    if ctx["numbered"]:
        return "$" + str(n)
    return "?"


def _orm_renumber(fragment, ctx):
    # Rewrites ? placeholders in a user-supplied fragment into the dialect's
    # markers, continuing the numbering ctx already holds. Left alone:
    # single-quoted literals (with '' escapes), double-quoted identifiers,
    # -- and /* */ comments, and the postgres jsonb operators ?| and ?&. A
    # bare ? is always a placeholder on this path, so the bare jsonb ?
    # operator is not expressible here — jsonb_exists(data, 'k') is the
    # function form of it.
    if not ctx["numbered"]:
        return fragment
    out = ""
    in_string = False
    in_escape_string = False
    in_ident = False
    i = 0
    n = len(fragment)
    while i < n:
        ch = fragment[i]
        if in_string:
            out = out + ch
            if in_escape_string and ch == "\\":
                # E'...' strings: a backslash escapes the next character,
                # including a quote.
                if i + 1 < n:
                    out = out + fragment[i + 1]
                    i = i + 2
                    continue
            if ch == "'":
                if i + 1 < n and fragment[i + 1] == "'":
                    out = out + "'"
                    i = i + 2
                    continue
                in_string = False
                in_escape_string = False
            i = i + 1
            continue
        if in_ident:
            out = out + ch
            if ch == '"':
                in_ident = False
            i = i + 1
            continue
        if ch == "'":
            in_string = True
            # E'...' (escape string): the preceding character is the E.
            if i > 0 and (fragment[i - 1] == "E" or fragment[i - 1] == "e"):
                in_escape_string = True
            out = out + ch
            i = i + 1
            continue
        if ch == '"':
            in_ident = True
            out = out + ch
            i = i + 1
            continue
        if ch == "-" and i + 1 < n and fragment[i + 1] == "-":
            j = i
            while j < n and fragment[j] != "\n":
                j = j + 1
            out = out + fragment[i:j]
            i = j
            continue
        if ch == "/" and i + 1 < n and fragment[i + 1] == "*":
            # Block comments nest in postgres.
            j = i + 2
            depth = 1
            while j < n and depth > 0:
                if fragment[j] == "/" and j + 1 < n and fragment[j + 1] == "*":
                    depth = depth + 1
                    j = j + 2
                    continue
                if fragment[j] == "*" and j + 1 < n and fragment[j + 1] == "/":
                    depth = depth - 1
                    j = j + 2
                    continue
                j = j + 1
            out = out + fragment[i:j]
            i = j
            continue
        if ch == "$":
            # Dollar-quoted string ($$...$$ or $tag$...$tag$): a $ followed
            # by an optional tag (letters/underscore first) and another $.
            # A digit after $ (a $1 placeholder) is not an opener.
            j = i + 1
            while j < n and ((fragment[j] >= "a" and fragment[j] <= "z") or (fragment[j] >= "A" and fragment[j] <= "Z") or (fragment[j] >= "0" and fragment[j] <= "9") or fragment[j] == "_"):
                j = j + 1
            if j < n and fragment[j] == "$" and (j == i + 1 or not (fragment[i + 1] >= "0" and fragment[i + 1] <= "9")):
                delim = fragment[i:j + 1]
                k = fragment.find(delim, j + 1)
                if k == -1:
                    k = n
                else:
                    k = k + len(delim)
                out = out + fragment[i:k]
                i = k
                continue
            out = out + ch
            i = i + 1
            continue
        if ch == "?" and i + 1 < n and (fragment[i + 1] == "|" or fragment[i + 1] == "&"):
            out = out + ch + fragment[i + 1]
            i = i + 2
            continue
        if ch == "?" and len(out) > 0 and out[len(out) - 1] == "@":
            # the jsonb path operator @? takes its question mark with it
            out = out + ch
            i = i + 1
            continue
        if ch == "?":
            ctx["n"] = ctx["n"] + 1
            out = out + _orm_mark(ctx["n"], ctx)
            i = i + 1
            continue
        out = out + ch
        i = i + 1
    return out


def _orm_check_type(col_type):
    allowed = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_ (),"
    if col_type == "":
        raise ValueError("empty column type")
    for ch in col_type:
        if ch not in allowed:
            raise ValueError("unsafe column type: " + col_type)
    return col_type


def _orm_render_literal(value):
    if value == None:
        return "NULL"
    # isinstance first: 0.0 == False and 1 == True under loose equality
    if isinstance(value, bool):
        if value:
            return "TRUE"
        return "FALSE"
    if isinstance(value, str):
        return "'" + value.replace("'", "''") + "'"
    if isinstance(value, int):
        return str(value)
    if isinstance(value, float):
        # inf and nan are expressible as scriptling floats but have no SQL
        # literal; render them as a clear error instead of invalid DDL.
        if value != value or value == float("inf") or value == float("-inf"):
            raise ValueError("unsupported default value: " + str(value))
        return str(value)
    raise ValueError("unsupported default value: " + str(value))


class _orm_TableBuilder:
    def __init__(self, kit, table):
        self.kit = kit
        self.table = table
        self.columns = []
        self.want_if_not_exists = False

    def column(self, name, col_type, primary_key=False, autoincrement=False, nullable=True, unique=False, default=None):
        self.columns.append([name, col_type, primary_key, autoincrement, nullable, unique, default])
        return self

    def if_not_exists(self):
        self.want_if_not_exists = True
        return self

    def _render_column(self, c, ctx):
        name = _orm_quote(c[0], ctx)
        col_type = _orm_check_type(str(c[1]))
        if c[3]:
            # auto-incrementing primary key: SERIAL on postgres, explicit
            # syntax elsewhere
            if ctx["numbered"]:
                # SERIAL is a type on postgres, not a modifier
                return name + " SERIAL PRIMARY KEY"
            if ctx["backtick"]:
                return name + " " + col_type + " PRIMARY KEY AUTO_INCREMENT"
            return name + " " + col_type + " PRIMARY KEY AUTOINCREMENT"
        out = name + " " + col_type
        if not c[4]:
            out = out + " NOT NULL"
        if c[5]:
            out = out + " UNIQUE"
        if c[2]:
            out = out + " PRIMARY KEY"
        if c[6] != None:
            out = out + " DEFAULT " + _orm_render_literal(c[6])
        return out

    def execute(self):
        if len(self.columns) == 0:
            raise ValueError("create_table needs at least one column")
        ctx = self.kit._ctx()
        defs = []
        for c in self.columns:
            defs.append(self._render_column(c, ctx))
        sql = "CREATE TABLE "
        if self.want_if_not_exists:
            sql = sql + "IF NOT EXISTS "
        sql = sql + _orm_quote(self.table, ctx) + " (" + ", ".join(defs) + ")"
        return self.kit.conn.execute(sql)


class _orm_Criterion:
    def __init__(self, op, column, params):
        self.op = op
        self.column = column
        self.params = params

    def _sql(self, ctx):
        q = _orm_quote(self.column, ctx)
        if self.op == "is-null":
            return [q + " IS NULL", []]
        if self.op == "is-not-null":
            return [q + " IS NOT NULL", []]
        marks = []
        for p in self.params:
            ctx["n"] = ctx["n"] + 1
            marks.append(_orm_mark(ctx["n"], ctx))
        if self.op == "in" or self.op == "not in":
            return [q + " " + self.op + " (" + ", ".join(marks) + ")", self.params]
        if self.op == "ilike":
            return [q + " " + ctx["ilike"] + " " + marks[0], self.params]
        return [q + " " + self.op + " " + marks[0], self.params]


class _orm_Group:
    def __init__(self, sep, criteria):
        self.sep = sep
        self.criteria = criteria

    def _sql(self, ctx):
        parts = []
        params = []
        for c in self.criteria:
            pair = c._sql(ctx)
            parts.append(pair[0])
            params = params + pair[1]
        return ["(" + self.sep.join(parts) + ")", params]


def _orm_criterion_from_args(args):
    # where() accepts (column, op, value) or one criterion object; every
    # query builder shares this reading.
    if len(args) == 3:
        ops = {"=": "=", "!=": "!=", "<>": "<>", "<": "<", "<=": "<=", ">": ">", ">=": ">=", "like": "like"}
        op = str(args[1]).lower()
        if op not in ops:
            raise ValueError("unsupported operator: " + str(args[1]))
        return _orm_Criterion(ops[op], args[0], [args[2]])
    if len(args) != 1:
        raise ValueError("where takes (column, op, value) or a criterion from orm.eq()/orm.any_of()/...")
    return args[0]


def _orm_where_clause(ctx, criteria, raws):
    # Renders the collected criteria and raw fragments into
    # [" WHERE ...", params]; empty criteria give ["", params]. The ctx
    # counter continues from wherever the caller left it, so SET marks on an
    # update number before its where marks on numbered dialects.
    parts = []
    params = []
    for c in criteria:
        pair = c._sql(ctx)
        parts.append(pair[0])
        params = params + pair[1]
    for raw in raws:
        parts.append(_orm_renumber(raw[0], ctx))
        params = params + raw[1]
    if len(parts) == 0:
        return ["", params]
    return [" WHERE " + " AND ".join(parts), params]


class _orm_Query:
    def __init__(self, kit, table, columns):
        self.kit = kit
        self.table = table
        self.columns = columns
        self.criteria = []
        self.raws = []
        self.order = []
        self.limit_n = 0
        self.offset_n = 0

    def where(self, *args):
        self.criteria.append(_orm_criterion_from_args(list(args)))
        return self

    def where_sql(self, fragment, *params):
        # Escape hatch for what the builder cannot express; ? placeholders
        # are renumbered on numbered dialects.
        self.raws.append([fragment, list(params)])
        return self

    def order_by(self, column, desc=False):
        self.order.append([column, desc])
        return self

    def limit(self, n):
        self.limit_n = n
        return self

    def offset(self, n):
        self.offset_n = n
        return self

    def _assemble(self, what):
        ctx = self.kit._ctx()
        sql = "SELECT " + what + " FROM " + _orm_quote(self.table, ctx)
        pair = _orm_where_clause(ctx, self.criteria, self.raws)
        sql = sql + pair[0]
        if len(self.order) > 0:
            bits = []
            for o in self.order:
                bit = _orm_quote(o[0], ctx)
                if o[1]:
                    bit = bit + " DESC"
                bits.append(bit)
            sql = sql + " ORDER BY " + ", ".join(bits)
        if self.limit_n > 0:
            sql = sql + " LIMIT " + str(self.limit_n)
        if self.offset_n > 0:
            sql = sql + " OFFSET " + str(self.offset_n)
        return [sql, pair[1]]

    def fetch(self):
        if len(self.columns) == 0:
            what = "*"
        else:
            ctx = self.kit._ctx()
            cols = []
            for c in self.columns:
                cols.append(_orm_quote(c, ctx))
            what = ", ".join(cols)
        pair = self._assemble(what)
        return self.kit.conn.query(pair[0], *pair[1])

    def iterate(self):
        # Same assembly as fetch(), but rows stream one at a time. In
        # compiled-in builds the cursor reads rows lazily from the driver;
        # in external-plugin builds each next() is one call over the wire,
        # still constant memory.
        if len(self.columns) == 0:
            what = "*"
        else:
            ctx = self.kit._ctx()
            cols = []
            for c in self.columns:
                cols.append(_orm_quote(c, ctx))
            what = ", ".join(cols)
        pair = self._assemble(what)
        return _orm_RowIter(self.kit.conn.query_iter(pair[0], *pair[1]))

    def one(self):
        rows = self.fetch()
        if len(rows) == 0:
            return None
        return rows[0]

    def count(self):
        pair = self._assemble("count(*) AS cnt")
        rows = self.kit.conn.query(pair[0], *pair[1])
        # count(*) is an integer on every supported driver; the cast keeps
        # the contract if a driver ever hands back its count as a string.
        return int(rows[0]["cnt"])


class _orm_UpdateQuery:
    # orm.update(table, values) builder: where chains like select's, execute
    # refuses to run without one.
    def __init__(self, kit, table, values):
        self.kit = kit
        self.table = table
        self.values = values
        self.criteria = []
        self.raws = []

    def where(self, *args):
        self.criteria.append(_orm_criterion_from_args(list(args)))
        return self

    def where_sql(self, fragment, *params):
        # Escape hatch for what the builder cannot express; ? placeholders
        # are renumbered on numbered dialects.
        self.raws.append([fragment, list(params)])
        return self

    def execute(self):
        cols = list(self.values.keys())
        cols.sort()
        if len(cols) == 0:
            raise ValueError("update needs at least one column")
        ctx = self.kit._ctx()
        sets = []
        vals = []
        for c in cols:
            ctx["n"] = ctx["n"] + 1
            sets.append(_orm_quote(c, ctx) + " = " + self.kit._mark(ctx["n"]))
            vals.append(self.values[c])
        pair = _orm_where_clause(ctx, self.criteria, self.raws)
        if pair[0] == "":
            raise ValueError("update requires a where clause (update every row is not supported)")
        sql = "UPDATE " + _orm_quote(self.table, ctx) + " SET " + ", ".join(sets) + pair[0]
        return self.kit.conn.execute(sql, *(vals + pair[1]))


class _orm_DeleteQuery:
    # orm.delete(table) builder: where chains like select's, execute refuses
    # to run without one.
    def __init__(self, kit, table):
        self.kit = kit
        self.table = table
        self.criteria = []
        self.raws = []

    def where(self, *args):
        self.criteria.append(_orm_criterion_from_args(list(args)))
        return self

    def where_sql(self, fragment, *params):
        # Escape hatch for what the builder cannot express; ? placeholders
        # are renumbered on numbered dialects.
        self.raws.append([fragment, list(params)])
        return self

    def execute(self):
        ctx = self.kit._ctx()
        pair = _orm_where_clause(ctx, self.criteria, self.raws)
        if pair[0] == "":
            raise ValueError("delete requires a where clause (delete every row is not supported)")
        sql = "DELETE FROM " + _orm_quote(self.table, ctx) + pair[0]
        return self.kit.conn.execute(sql, *pair[1])


class _orm_RowIter:
    # Wraps a Cursor (Go side: next() -> dict|None) in the iteration
    # protocol so "for row in q.iterate():" streams row by row instead of
    # materialising the result.
    def __init__(self, cursor):
        self.cursor = cursor

    def __iter__(self):
        return self

    def __next__(self):
        row = self.cursor.next()
        if row == None:
            raise StopIteration()
        return row

    def close(self):
        return self.cursor.close()


class _orm_Model:
    def __init__(self, kit, factory, table, pk, columns):
        self.kit = kit
        self.factory = factory
        self.table = table
        self.pk = pk
        self.columns = columns
        # The column list for writes is the table's own columns, read from
        # the schema once per kit and cached there (every gateway to the same
        # table shares one lookup), unless columns= restricts it.
        self._write_cols = None

    def _write_columns(self):
        if self._write_cols != None:
            return self._write_cols
        if len(self.columns) > 0:
            self._write_cols = self.columns
            return self._write_cols
        cached = self.kit._schema_columns.get(self.table, None)
        if cached == None:
            if self.kit.columns_sql == "":
                # SQLite's PRAGMA cannot bind the table name: quote it.
                ctx = self.kit._ctx()
                rows = self.kit.conn.query("PRAGMA table_info(" + _orm_quote(self.table, ctx) + ")")
            else:
                # Renumber the ? here: the kit's connection may be the raw
                # plugin connection (compiled-in), which does not rewrite
                # placeholders the way the script wrapper does.
                ctx = self.kit._ctx()
                rows = self.kit.conn.query(_orm_renumber(self.kit.columns_sql, ctx), self.table)
            cached = []
            for r in rows:
                cached.append(r["name"])
            if len(cached) == 0:
                raise ValueError("table " + self.table + " has no columns (does it exist?)")
            self.kit._schema_columns[self.table] = cached
        self._write_cols = cached
        return self._write_cols

    def select(self, *columns):
        return _orm_Query(self.kit, self.table, list(columns))

    def count(self):
        return self.kit.select(self.table).count()

    def get(self, pk_value):
        rows = self.kit.select(self.table).where(self.pk, "=", pk_value).fetch()
        if len(rows) == 0:
            return None
        return self.factory(**rows[0])

    def insert(self, obj, columns=None):
        # Without columns, None means unset: skip it, so the primary key
        # auto-assigns (writing a NULL id violates the not-null key on
        # postgres) and columns take their schema defaults rather than being
        # overwritten with NULL. With columns, the list is explicit: None is
        # written as NULL (the way to override a schema default on insert),
        # except a None primary key, which always auto-assigns.
        explicit = columns != None
        cols = columns if explicit else self._write_columns()
        values = {}
        for c in cols:
            v = getattr(obj, c)
            if v == None and (not explicit or c == self.pk):
                continue
            values[c] = v
        return self.kit.insert(self.table, values, pk=self.pk)

    def save(self, obj, columns=None):
        # Every managed non-PK column by default (None clears the field), or
        # exactly the listed columns when columns is given — the per-call
        # subset, no second gateway needed.
        cols = columns if columns != None else self._write_columns()
        pk_value = getattr(obj, self.pk)
        values = {}
        for c in cols:
            if c != self.pk:
                values[c] = getattr(obj, c)
        return self.kit.update(self.table, values).where(self.pk, "=", pk_value).execute()

    def delete(self, target):
        # Accepts a primary key or an instance carrying one.
        pk_value = getattr(target, self.pk, None)
        if pk_value == None:
            pk_value = target
        return self.kit.delete(self.table).where(self.pk, "=", pk_value).execute()


class _orm_Kit:
    def __init__(self, conn, numbered, backtick, ilike, tables_sql, columns_sql):
        # The dialect constants are baked in by the plugin that hands the
        # kit out, so nothing here asks the connection for anything beyond
        # query/execute.
        self.conn = conn
        self.numbered = numbered
        self.backtick = backtick
        self.ilike = ilike
        self.tables_sql = tables_sql
        self.columns_sql = columns_sql
        self._schema_columns = {}

    def _ctx(self):
        return {"n": 0, "backtick": self.backtick, "numbered": self.numbered, "ilike": self.ilike}

    def _mark(self, n):
        if self.numbered:
            return "$" + str(n)
        return "?"

    # criteria

    def eq(self, column, value):
        return _orm_Criterion("=", column, [value])

    def ne(self, column, value):
        return _orm_Criterion("!=", column, [value])

    def lt(self, column, value):
        return _orm_Criterion("<", column, [value])

    def le(self, column, value):
        return _orm_Criterion("<=", column, [value])

    def gt(self, column, value):
        return _orm_Criterion(">", column, [value])

    def ge(self, column, value):
        return _orm_Criterion(">=", column, [value])

    def like(self, column, pattern):
        return _orm_Criterion("like", column, [pattern])

    def ilike(self, column, pattern):
        return _orm_Criterion("ilike", column, [pattern])

    def one_of(self, column, values):
        return _orm_Criterion("in", column, list(values))

    def not_one_of(self, column, values):
        return _orm_Criterion("not in", column, list(values))

    def is_null(self, column):
        return _orm_Criterion("is-null", column, [])

    def not_null(self, column):
        return _orm_Criterion("is-not-null", column, [])

    def any_of(self, *criteria):
        return _orm_Group(" OR ", list(criteria))

    def all_of(self, *criteria):
        return _orm_Group(" AND ", list(criteria))

    # queries

    def select(self, table, *columns):
        return _orm_Query(self, table, list(columns))

    def update(self, table, values):
        return _orm_UpdateQuery(self, table, values)

    def delete(self, table):
        return _orm_DeleteQuery(self, table)

    # tables

    def create_table(self, table):
        return _orm_TableBuilder(self, table)

    def drop_table(self, table):
        ctx = self._ctx()
        return self.conn.execute("DROP TABLE IF EXISTS " + _orm_quote(table, ctx))

    def insert(self, table, values, pk="id"):
        # On postgres there is no last-insert-id; RETURNING recovers one
        # through the primary key so the result looks like every other
        # backend's. An empty values dict is an all-defaults insert.
        cols = list(values.keys())
        cols.sort()
        ctx = self._ctx()
        if len(cols) == 0:
            if self.backtick:
                # MySQL has no DEFAULT VALUES form.
                return self.conn.execute("INSERT INTO " + _orm_quote(table, ctx) + " () VALUES ()")
            sql = "INSERT INTO " + _orm_quote(table, ctx) + " DEFAULT VALUES"
            if self.numbered:
                sql = sql + " RETURNING " + _orm_quote(pk, ctx)
                rows = self.conn.query(sql)
                if len(rows) != 1:
                    raise ValueError("insert returning gave " + str(len(rows)) + " rows")
                return {"last_insert_id": rows[0][pk], "rows_affected": 1}
            return self.conn.execute(sql)
        quoted = []
        marks = []
        params = []
        for c in cols:
            quoted.append(_orm_quote(c, ctx))
            ctx["n"] = ctx["n"] + 1
            marks.append(self._mark(ctx["n"]))
            params.append(values[c])
        sql = "INSERT INTO " + _orm_quote(table, ctx) + " (" + ", ".join(quoted) + ") VALUES (" + ", ".join(marks) + ")"
        if self.numbered:
            sql = sql + " RETURNING " + _orm_quote(pk, ctx)
            rows = self.conn.query(sql, *params)
            if len(rows) != 1:
                raise ValueError("insert returning gave " + str(len(rows)) + " rows")
            return {"last_insert_id": rows[0][pk], "rows_affected": 1}
        return self.conn.execute(sql, *params)

    def tables(self):
        rows = self.conn.query(self.tables_sql)
        names = []
        for r in rows:
            names.append(r["name"])
        return names

    # models

    def table(self, factory, table, pk="id", columns=None):
        if columns == None:
            columns = []
        return _orm_Model(self, factory, table, pk, list(columns))
`

// ScriptKitSource is the shared ORM script, backticks restored.
var ScriptKitSource = strings.ReplaceAll(scriptKitRaw, "§", "`")

// ScriptEntry is one function or class the plugin registers as script
// source. External mode registers each entry so the host's generated module
// carries them; compiled-in mode concatenates them into one module.
type ScriptEntry struct {
	Name   string
	Source string
	Class  bool
}

// ScriptKitEntries decomposes the kit into per-name registrations. Each
// source already carries its full def/class header, so the entries emit
// verbatim into a generated module (external mode) or concatenate
// (compiled-in mode).
func ScriptKitEntries() []ScriptEntry {
	return []ScriptEntry{
		{Name: "_orm_check_ident", Source: sourceOf(ScriptKitSource, "def _orm_check_ident")},
		{Name: "_orm_quote", Source: sourceOf(ScriptKitSource, "def _orm_quote")},
		{Name: "_orm_mark", Source: sourceOf(ScriptKitSource, "def _orm_mark")},
		{Name: "_orm_renumber", Source: sourceOf(ScriptKitSource, "def _orm_renumber")},
		{Name: "_orm_check_type", Source: sourceOf(ScriptKitSource, "def _orm_check_type")},
		{Name: "_orm_render_literal", Source: sourceOf(ScriptKitSource, "def _orm_render_literal")},
		{Name: "_orm_criterion_from_args", Source: sourceOf(ScriptKitSource, "def _orm_criterion_from_args")},
		{Name: "_orm_where_clause", Source: sourceOf(ScriptKitSource, "def _orm_where_clause")},
		{Name: "_orm_TableBuilder", Source: sourceOf(ScriptKitSource, "class _orm_TableBuilder"), Class: true},
		{Name: "_orm_Criterion", Source: sourceOf(ScriptKitSource, "class _orm_Criterion"), Class: true},
		{Name: "_orm_Group", Source: sourceOf(ScriptKitSource, "class _orm_Group"), Class: true},
		{Name: "_orm_Query", Source: sourceOf(ScriptKitSource, "class _orm_Query"), Class: true},
		{Name: "_orm_UpdateQuery", Source: sourceOf(ScriptKitSource, "class _orm_UpdateQuery"), Class: true},
		{Name: "_orm_DeleteQuery", Source: sourceOf(ScriptKitSource, "class _orm_DeleteQuery"), Class: true},
		{Name: "_orm_RowIter", Source: sourceOf(ScriptKitSource, "class _orm_RowIter"), Class: true},
		{Name: "_orm_Model", Source: sourceOf(ScriptKitSource, "class _orm_Model"), Class: true},
		{Name: "_orm_Kit", Source: sourceOf(ScriptKitSource, "class _orm_Kit"), Class: true},
	}
}

// sourceOf extracts one top-level def/class block (up to the next top-level
// def/class or EOF) from the kit source.
func sourceOf(source, marker string) string {
	start := strings.Index(source, marker)
	if start < 0 {
		panic("ormscript: marker not found: " + marker)
	}
	rest := source[start:]
	end := len(rest)
	for _, next := range []string{"\ndef ", "\nclass "} {
		// find the next top-level marker after the first line
		if idx := strings.Index(rest[len(marker):], next); idx >= 0 {
			if candidate := len(marker) + idx; candidate < end {
				end = candidate
			}
		}
	}
	return strings.TrimSpace(rest[:end]) + "\n"
}

// DialectSpec is the baked-in dialect a plugin's ORM carries: placeholder
// numbering, identifier quoting, the case-insensitive LIKE word, and the
// catalogue query behind tables(). With this, the native side of a plugin
// is nothing but the driver — query/execute/close.
type DialectSpec struct {
	Numbered  bool
	Backtick  bool
	ILike     string
	TablesSQL string
	// ColumnsSQL reads a table's column names (as "name", in table order)
	// with the table name bound as one parameter. Empty means the dialect
	// cannot parameterize it and uses SQLite's PRAGMA table_info instead.
	ColumnsSQL string
}

// SQLiteSpec, MySQLSpec and PostgresSpec are the known dialects; a plugin
// serving one driver bakes its spec in, and the sql plugin picks per
// connection from the DSN scheme.
var (
	SQLiteSpec = DialectSpec{
		ILike:     "like",
		TablesSQL: "select name from sqlite_master where type = 'table' and name not like 'sqlite_%' order by name",
	}
	MySQLSpec = DialectSpec{
		Backtick:   true,
		ILike:      "like",
		TablesSQL:  "select table_name as name from information_schema.tables where table_schema = database() and table_type = 'BASE TABLE' order by table_name",
		ColumnsSQL: "select column_name as name from information_schema.columns where table_schema = database() and table_name = ? order by ordinal_position",
	}
	PostgresSpec = DialectSpec{
		Numbered:   true,
		ILike:      "ilike",
		TablesSQL:  "select table_name as name from information_schema.tables where table_schema = current_schema() and table_type = 'BASE TABLE' order by table_name",
		ColumnsSQL: "select column_name as name from information_schema.columns where table_schema = current_schema() and table_name = ? order by ordinal_position",
	}
)

func (d DialectSpec) literals() string {
	ilike := d.ILike
	if ilike == "" {
		ilike = "like"
	}
	return fmt.Sprintf("%s, %s, %q, %q, %q", scriptBool(d.Numbered), scriptBool(d.Backtick), ilike, d.TablesSQL, d.ColumnsSQL)
}

func scriptBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

// ConnectionScriptSource returns the Connection class a single-driver
// plugin registers in external mode: a script-defined wrapper whose methods
// proxy to the plugin object, with get_orm() returning the host-side kit
// carrying the plugin's baked dialect.
func ConnectionScriptSource(pluginName string, spec DialectSpec) string {
	return `class Connection:
    def __init__(self, *args, **kwargs):
        self._plugin_remote = scriptling.plugin._new_object("` + pluginName + `", "Connection", *args, **kwargs)

    def query(self, *args, **kwargs):
        return scriptling.plugin.call_method(self._plugin_remote, "query", *args, **kwargs)

    def query_iter(self, *args, **kwargs):
        return scriptling.plugin.call_method(self._plugin_remote, "query_iter", *args, **kwargs)

    def execute(self, *args, **kwargs):
        return scriptling.plugin.call_method(self._plugin_remote, "execute", *args, **kwargs)

    def close(self):
        return scriptling.plugin.call_method(self._plugin_remote, "close")

    def get_orm(self):
        return _orm_Kit(self, ` + spec.literals() + `)
`
}

// ConnectionScriptSourceMultiDriver is the external-mode wrapper for a
// plugin serving several drivers (the sql plugin): the dialect is picked
// from the DSN scheme at connect time, host-side, since the wrapper sees
// the DSN before the plugin does.
func ConnectionScriptSourceMultiDriver(pluginName string) string {
	return `class Connection:
    def __init__(self, *args, **kwargs):
        self._dsn = ""
        if len(args) > 0:
            self._dsn = str(args[0])
        elif "dsn" in kwargs:
            self._dsn = str(kwargs["dsn"])
        self._pg = self._dsn.lower().startswith("postgres")
        self._plugin_remote = scriptling.plugin._new_object("` + pluginName + `", "Connection", *args, **kwargs)

    def query(self, *args, **kwargs):
        if self._pg and len(args) > 0:
            args = [_orm_renumber(args[0], {"n": 0, "numbered": True})] + list(args[1:])
        return scriptling.plugin.call_method(self._plugin_remote, "query", *args, **kwargs)

    def query_iter(self, *args, **kwargs):
        # Same renumbering as query: iterate()'s SQL carries ? placeholders
        # that postgres cannot take unrewritten.
        if self._pg and len(args) > 0:
            args = [_orm_renumber(args[0], {"n": 0, "numbered": True})] + list(args[1:])
        return scriptling.plugin.call_method(self._plugin_remote, "query_iter", *args, **kwargs)

    def execute(self, *args, **kwargs):
        if self._pg and len(args) > 0:
            args = [_orm_renumber(args[0], {"n": 0, "numbered": True})] + list(args[1:])
        return scriptling.plugin.call_method(self._plugin_remote, "execute", *args, **kwargs)

    def close(self):
        return scriptling.plugin.call_method(self._plugin_remote, "close")

    def get_orm(self):
        if self._pg:
            return _orm_Kit(self, True, False, "ilike", ` + fmt.Sprintf("%q, %q", PostgresSpec.TablesSQL, PostgresSpec.ColumnsSQL) + `)
        return _orm_Kit(self, False, True, "like", ` + fmt.Sprintf("%q, %q", MySQLSpec.TablesSQL, MySQLSpec.ColumnsSQL) + `)
`
}

// ScriptModuleSource returns the full user-facing module for compiled-in
// registration: the native twin carries the connection; this wrapper adds
// get_orm() and the shared kit. twinName is the native library name
// ("scriptling._sqlite"). singleDriver bakes the spec in; a multi-driver
// plugin (sql) sniffs the DSN scheme instead.
func ScriptModuleSource(twinName string, singleDriver bool, spec DialectSpec) string {
	getORM := `        return _orm_Kit(self._c, ` + spec.literals() + `)`
	queryShim := `        return self._c.query(*args, **kwargs)`
	queryIterShim := `        return self._c.query_iter(*args, **kwargs)`
	execShim := `        return self._c.execute(*args, **kwargs)`
	initCapture := ""
	if singleDriver == false {
		initCapture = `        self._dsn = ""
        if len(args) > 0:
            self._dsn = str(args[0])
        elif "dsn" in kwargs:
            self._dsn = str(kwargs["dsn"])
        self._pg = self._dsn.lower().startswith("postgres")
`
		getORM = `        if self._pg:
            return _orm_Kit(self._c, True, False, "ilike", ` + fmt.Sprintf("%q, %q", PostgresSpec.TablesSQL, PostgresSpec.ColumnsSQL) + `)
        return _orm_Kit(self._c, False, True, "like", ` + fmt.Sprintf("%q, %q", MySQLSpec.TablesSQL, MySQLSpec.ColumnsSQL) + `)`
		queryShim = `        if self._pg and len(args) > 0:
            args = [_orm_renumber(args[0], {"n": 0, "numbered": True})] + list(args[1:])
        return self._c.query(*args, **kwargs)`
		queryIterShim = `        if self._pg and len(args) > 0:
            args = [_orm_renumber(args[0], {"n": 0, "numbered": True})] + list(args[1:])
        return self._c.query_iter(*args, **kwargs)`
		execShim = `        if self._pg and len(args) > 0:
            args = [_orm_renumber(args[0], {"n": 0, "numbered": True})] + list(args[1:])
        return self._c.execute(*args, **kwargs)`
	}
	module := `import ` + twinName + ` as _n

class Connection:
    def __init__(self, *args, **kwargs):
` + initCapture + `        self._c = _n.Connection(*args, **kwargs)

    def query(self, *args, **kwargs):
` + queryShim + `
    def query_iter(self, *args, **kwargs):
` + queryIterShim + `
    def execute(self, *args, **kwargs):
` + execShim + `
    def close(self):
        return self._c.close()

    def get_orm(self):
` + getORM + `


def connect(path=":memory:", timeout_ms=5000):
    return Connection(path, timeout_ms=timeout_ms)

`
	return module + ScriptKitSource
}
