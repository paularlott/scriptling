// Package relational provides the shared Connection and Transaction classes
// used by the database plugins (sqlite, sql). Both plugins expose the same
// surface — connect(...) -> Connection with query/execute/begin/close — so
// scripts written against one backend work unchanged against the other.
package relational

import (
	"context"
	"database/sql"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/object"
)

// Cursor streams rows from one query without materialising the result.
// A single consumer drains it: next() returns each row as a dict, then None
// forever once exhausted. close() releases the underlying rows; __del__
// makes abandoned cursors release them too.
type Cursor struct {
	rows        *sql.Rows
	columns     []string
	receptacles []any
	raw         []any
	done        bool
	// onRelease, when set, runs exactly once when the rows are released —
	// the connection-held bookkeeping for single-connection databases.
	onRelease func()
	mu        sync.Mutex
}

// CursorClass builds the Cursor class handed out by query_iter.
func CursorClass() *object.ClassBuilder {
	cb := object.NewClassBuilder("Cursor")
	cb.Constructor(func(kwargs object.Kwargs) (*Cursor, error) {
		return nil, fmt.Errorf("cursors come from conn.query_iter(...) or orm ... .iterate()")
	})
	cb.MethodWithHelp("next", func(self *Cursor) object.Object {
		self.mu.Lock()
		defer self.mu.Unlock()
		if self.done {
			return &object.Null{}
		}
		if !self.rows.Next() {
			// Next()==false is either a clean EOF or the driver dying
			// mid-stream; without the Err() check a connection drop would
			// look like a normal, short result set.
			if err := self.rows.Err(); err != nil {
				self.finish()
				return &object.Error{Message: fmt.Sprintf("cursor read: %v", err)}
			}
			self.finish()
			return &object.Null{}
		}
		if err := self.rows.Scan(self.receptacles...); err != nil {
			self.finish()
			return &object.Error{Message: fmt.Sprintf("cursor scan: %v", err)}
		}
		row := make(map[string]any, len(self.columns))
		for i, column := range self.columns {
			row[column] = convertValue(self.raw[i])
		}
		return conversion.FromGo(row)
	}, `next() -> dict | null

The next row as a dict keyed by column name, or null when the cursor
is exhausted (and stays null afterwards).`)
	cb.MethodWithHelp("close", func(self *Cursor) error {
		self.mu.Lock()
		defer self.mu.Unlock()
		self.finish()
		return nil
	}, "close() - Release the cursor's rows early; safe to call repeatedly.")
	cb.Method("__del__", func(self *Cursor) {
		self.release()
	})
	return cb
}

// release closes the rows from __del__ and the Go finalizer. Idempotent
// through the done flag; safe from the finalizer goroutine.
func (c *Cursor) release() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.finish()
}

// finish closes the rows, marks the cursor done and fires the release
// callback. Callers hold mu.
func (c *Cursor) finish() {
	if !c.done {
		c.done = true
		_ = c.rows.Close()
		if c.onRelease != nil {
			c.onRelease()
		}
	}
}

// newCursor wraps an open *sql.Rows in a Cursor instance.
func newCursor(rows *sql.Rows) (*Cursor, error) {
	columns, err := rows.Columns()
	if err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("columns: %w", err)
	}
	raw := make([]any, len(columns))
	receptacles := make([]any, len(columns))
	for i := range raw {
		receptacles[i] = &raw[i]
	}
	return &Cursor{
		rows:        rows,
		columns:     columns,
		receptacles: receptacles,
		raw:         raw,
	}, nil
}

// scanRows drains an already-executed statement into row dicts.
func scanRows(rows *sql.Rows) ([]any, error) {
	cursor, err := newCursor(rows)
	if err != nil {
		return nil, err
	}
	defer cursor.finish()
	results := make([]any, 0)
	for cursor.rows.Next() {
		if err := cursor.rows.Scan(cursor.receptacles...); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		row := make(map[string]any, len(cursor.columns))
		for i, column := range cursor.columns {
			row[column] = convertValue(cursor.raw[i])
		}
		results = append(results, row)
	}
	if err := cursor.rows.Err(); err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	return results, nil
}

// execInfo shapes the dict every execute() returns. Postgres has no
// last-insert-id; drivers return an error rather than 0 there.
func execInfo(result sql.Result) map[string]any {
	rowsAffected, _ := result.RowsAffected()
	lastInsertID, err := result.LastInsertId()
	if err != nil {
		lastInsertID = 0
	}
	return map[string]any{
		"last_insert_id": lastInsertID,
		"rows_affected":  rowsAffected,
	}
}

// errTxDone is the script-facing shape of using a finished transaction.
const errTxDone = "transaction is already committed or rolled back"

// Conn is the typed receiver holding the live database handle. It is nothing
// but the driver: query/execute/begin/close. Everything backend-specific above
// that — placeholder translation, quoting, the ORM — lives in the script-side
// kit the plugin's Connection wrapper hands out.
type Conn struct {
	DB *sql.DB
	// SingleConn marks a database served by exactly one pooled connection (a
	// private in-memory SQLite). While a transaction or an open cursor holds
	// that connection the pool is exhausted, so connection-level calls would
	// block forever on it; they fail fast with a clear error instead. A
	// pooled database can serve several at once, so it leaves the flag unset.
	SingleConn    bool
	activeTxs     atomic.Int32
	activeCursors atomic.Int32
}

// busy reports why a connection-level call cannot run right now, or nil.
func (c *Conn) busy() error {
	if !c.SingleConn {
		return nil
	}
	if c.activeTxs.Load() > 0 {
		return fmt.Errorf("connection is held by an open transaction; use the transaction's methods until commit() or rollback() (an abandoned transaction is rolled back automatically once collected)")
	}
	if c.activeCursors.Load() > 0 {
		return fmt.Errorf("connection is held by an open cursor; drain it or call close() before further connection calls (an abandoned cursor releases its rows automatically once collected)")
	}
	return nil
}

// ConnectionClass builds the Connection class. constructor is the
// plugin-specific typed constructor (e.g. sqlite's opens a file, sql's parses
// a DSN); it must return (*Conn, error). Scripts construct the class
// directly — Connection(path) — or through each plugin's connect() helper.
func ConnectionClass(constructor interface{}) *object.ClassBuilder {
	cursorClass := CursorClass().Build()
	transactionClass := TransactionClass().Build()
	cb := object.NewClassBuilder("Connection")
	cb.Constructor(constructor)
	cb.MethodWithHelp("query", func(self *Conn, ctx context.Context, query string, params ...any) ([]any, error) {
		return self.query(ctx, query, params)
	}, `query(sql, *params) -> list[dict]

Execute a SELECT-style statement and return every row as a dict keyed by
column name. Values are ints, floats, bools, strings, or null.`)
	cb.MethodWithHelp("query_iter", func(self *Conn, ctx context.Context, query string, params ...any) (object.Object, error) {
		if err := self.busy(); err != nil {
			return nil, err
		}
		rows, err := self.DB.QueryContext(ctx, query, params...)
		if err != nil {
			return nil, fmt.Errorf("query_iter: %w", err)
		}
		cursor, err := newCursor(rows)
		if err != nil {
			return nil, fmt.Errorf("query_iter: %w", err)
		}
		cursor.onRelease = func() { self.activeCursors.Add(-1) }
		self.activeCursors.Add(1)
		// Receiver instances born inside a Go method never pass through the
		// evaluator's constructor path, so its __del__ finalizer wiring never
		// runs for them: install one directly on the typed receiver. An
		// abandoned cursor then releases its rows (and its pooled connection)
		// at the next collection cycle.
		runtime.SetFinalizer(cursor, func(c *Cursor) { c.release() })
		return object.NewReceiverInstance(cursorClass, "Cursor", cursor), nil
	}, `query_iter(sql, *params) -> Cursor

Run a SELECT-style statement and stream rows one at a time instead of
materialising the whole result. The cursor's next() returns each row as
a dict, then null at the end.`)
	cb.MethodWithHelp("execute", func(self *Conn, ctx context.Context, query string, params ...any) (map[string]any, error) {
		return self.execute(ctx, query, params)
	}, `execute(sql, *params) -> dict

Execute a statement that changes rows (INSERT/UPDATE/DELETE/DDL) and return
{"last_insert_id": int, "rows_affected": int}. last_insert_id is 0 on
backends without the concept (postgres).`)
	cb.MethodWithHelp("begin", func(self *Conn, ctx context.Context) (object.Object, error) {
		if err := self.busy(); err != nil {
			return nil, err
		}
		tx, err := self.DB.BeginTx(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("begin: %w", err)
		}
		self.activeTxs.Add(1)
		t := &Tx{conn: self, tx: tx}
		// Receiver instances born inside a Go method never pass through the
		// evaluator's constructor path, so its __del__ finalizer wiring never
		// runs for them: install one directly on the typed receiver, whose
		// cleanup is pure Go. Abandoned transactions then roll back (and
		// release their connection) at the next collection cycle instead of
		// pinning it for the rest of the process.
		runtime.SetFinalizer(t, func(t *Tx) { t.discard() })
		return object.NewReceiverInstance(transactionClass, "Transaction", t), nil
	}, `begin() -> Transaction

Start a transaction. Statements run through the transaction (its query,
query_iter and execute) are one atomic unit until commit() makes them
permanent or rollback() discards them. Rollback is the default: a
transaction that is abandoned without either call rolls back when it is
collected.`)
	cb.MethodWithHelp("close", func(self *Conn) error {
		return self.DB.Close()
	}, "close() - Close the connection and release the database handle.")
	// Safety net for environments that go away without calling close() —
	// a server handler or background task that opens a connection per run
	// releases the database when its instances are collected. Double closes
	// are harmless for database/sql drivers.
	cb.Method("__del__", func(self *Conn) {
		_ = self.DB.Close()
	})
	return cb
}

// Tx is the typed receiver holding one open transaction: the same
// query/execute surface as Conn, plus commit and rollback.
type Tx struct {
	conn *Conn
	tx   *sql.Tx
	mu   sync.Mutex
	done bool
}

// TransactionClass builds the Transaction class handed out by begin.
// Transactions come from conn.begin(); the constructor refuses direct use.
func TransactionClass() *object.ClassBuilder {
	cursorClass := CursorClass().Build()
	cb := object.NewClassBuilder("Transaction")
	cb.Constructor(func(kwargs object.Kwargs) (*Tx, error) {
		return nil, fmt.Errorf("transactions come from conn.begin()")
	})
	cb.MethodWithHelp("query", func(self *Tx, ctx context.Context, query string, params ...any) ([]any, error) {
		self.mu.Lock()
		defer self.mu.Unlock()
		if self.done {
			return nil, fmt.Errorf("query: %s", errTxDone)
		}
		rows, err := self.tx.QueryContext(ctx, query, params...)
		if err != nil {
			return nil, fmt.Errorf("query: %w", err)
		}
		return scanRows(rows)
	}, `query(sql, *params) -> list[dict]

Execute a SELECT-style statement inside the transaction and return every
row as a dict keyed by column name.`)
	cb.MethodWithHelp("query_iter", func(self *Tx, ctx context.Context, query string, params ...any) (object.Object, error) {
		self.mu.Lock()
		defer self.mu.Unlock()
		if self.done {
			return nil, fmt.Errorf("query_iter: %s", errTxDone)
		}
		rows, err := self.tx.QueryContext(ctx, query, params...)
		if err != nil {
			return nil, fmt.Errorf("query_iter: %w", err)
		}
		cursor, err := newCursor(rows)
		if err != nil {
			return nil, fmt.Errorf("query_iter: %w", err)
		}
		// Same receiver-finalizer as the connection's cursors: an abandoned
		// cursor releases its rows at the next collection cycle.
		runtime.SetFinalizer(cursor, func(c *Cursor) { c.release() })
		return object.NewReceiverInstance(cursorClass, "Cursor", cursor), nil
	}, `query_iter(sql, *params) -> Cursor

Run a SELECT-style statement inside the transaction and stream rows one at
a time. Drain or close the cursor before commit() or rollback().`)
	cb.MethodWithHelp("execute", func(self *Tx, ctx context.Context, query string, params ...any) (map[string]any, error) {
		self.mu.Lock()
		defer self.mu.Unlock()
		if self.done {
			return nil, fmt.Errorf("execute: %s", errTxDone)
		}
		result, err := self.tx.ExecContext(ctx, query, params...)
		if err != nil {
			return nil, fmt.Errorf("execute: %w", err)
		}
		return execInfo(result), nil
	}, `execute(sql, *params) -> dict

Execute a row-changing statement inside the transaction and return
{"last_insert_id": int, "rows_affected": int}. The change becomes permanent
at commit() and disappears at rollback().`)
	cb.MethodWithHelp("commit", func(self *Tx) error {
		self.mu.Lock()
		defer self.mu.Unlock()
		if self.done {
			return fmt.Errorf("commit: %s", errTxDone)
		}
		// A failed commit still ends the transaction (database/sql reports
		// ErrTxDone for anything after), so the connection is released either
		// way — the error tells the script what became of the data.
		err := self.tx.Commit()
		self.finish()
		if err != nil {
			return fmt.Errorf("commit: %w", err)
		}
		return nil
	}, "commit() - Make the transaction's changes permanent and end it.")
	cb.MethodWithHelp("rollback", func(self *Tx) error {
		self.mu.Lock()
		defer self.mu.Unlock()
		if self.done {
			return fmt.Errorf("rollback: %s", errTxDone)
		}
		err := self.tx.Rollback()
		self.finish()
		if err != nil {
			return fmt.Errorf("rollback: %w", err)
		}
		return nil
	}, "rollback() - Discard the transaction's changes and end it.")
	// Safety net mirroring Conn's: an abandoned transaction otherwise holds
	// its pooled connection forever. database/sql has no finaliser of its
	// own, so __del__ supplies the rollback. Idempotent with commit and
	// rollback through the done flag.
	cb.Method("__del__", func(self *Tx) {
		self.discard()
	})
	return cb
}

// discard rolls back an abandoned transaction. Idempotent with commit and
// rollback through the done flag; safe from __del__ and the Go finalizer
// goroutine.
func (t *Tx) discard() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.done {
		return
	}
	_ = t.tx.Rollback()
	t.finish()
}

// finish marks the transaction ended and releases its claim on the
// connection. Callers hold mu.
func (t *Tx) finish() {
	t.done = true
	t.conn.activeTxs.Add(-1)
}

func (c *Conn) query(ctx context.Context, query string, params []any) ([]any, error) {
	if err := c.busy(); err != nil {
		return nil, err
	}
	rows, err := c.DB.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	return scanRows(rows)
}

func (c *Conn) execute(ctx context.Context, query string, params []any) (map[string]any, error) {
	if err := c.busy(); err != nil {
		return nil, err
	}
	result, err := c.DB.ExecContext(ctx, query, params...)
	if err != nil {
		return nil, fmt.Errorf("execute: %w", err)
	}
	return execInfo(result), nil
}

// convertValue maps driver values onto script-visible ones. []byte (how
// drivers hand back text) becomes string; times become RFC3339 strings;
// anything exotic falls back to its string form.
func convertValue(value any) any {
	switch v := value.(type) {
	case nil:
		return nil
	case []byte:
		return string(v)
	case time.Time:
		return v.Format(time.RFC3339Nano)
	case int64, float64, bool, string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}
