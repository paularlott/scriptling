// Package relational provides the shared Connection class used by the
// database plugins (sqlite, sql). Both plugins expose the same surface —
// connect(...) -> Connection with query/execute/close — so scripts written
// against one backend work unchanged against the other.
package relational

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
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
	mu          sync.Mutex
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
		self.mu.Lock()
		defer self.mu.Unlock()
		self.finish()
	})
	return cb
}

// finish closes the rows and marks the cursor done. Callers hold mu.
func (c *Cursor) finish() {
	if !c.done {
		c.done = true
		_ = c.rows.Close()
	}
}

// Conn is the typed receiver holding the live database handle. It is nothing
// but the driver: query/execute/close. Everything backend-specific above
// that — placeholder translation, quoting, the ORM — lives in the
// script-side kit the plugin's Connection wrapper hands out.
type Conn struct {
	DB *sql.DB
}

// ConnectionClass builds the Connection class. constructor is the
// plugin-specific typed constructor (e.g. sqlite's opens a file, sql's parses
// a DSN); it must return (*Conn, error). Scripts construct the class
// directly — Connection(path) — or through each plugin's connect() helper.
func ConnectionClass(constructor interface{}) *object.ClassBuilder {
	cursorClass := CursorClass().Build()
	cb := object.NewClassBuilder("Connection")
	cb.Constructor(constructor)
	cb.MethodWithHelp("query", func(self *Conn, ctx context.Context, query string, params ...any) ([]any, error) {
		return self.query(ctx, query, params)
	}, `query(sql, *params) -> list[dict]

Execute a SELECT-style statement and return every row as a dict keyed by
column name. Values are ints, floats, bools, strings, or null.`)
	cb.MethodWithHelp("query_iter", func(self *Conn, ctx context.Context, query string, params ...any) (object.Object, error) {
		rows, err := self.DB.QueryContext(ctx, query, params...)
		if err != nil {
			return nil, fmt.Errorf("query_iter: %w", err)
		}
		columns, err := rows.Columns()
		if err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("query_iter columns: %w", err)
		}
		raw := make([]any, len(columns))
		receptacles := make([]any, len(columns))
		for i := range raw {
			receptacles[i] = &raw[i]
		}
		return object.NewReceiverInstance(cursorClass, "Cursor", &Cursor{
			rows:        rows,
			columns:     columns,
			receptacles: receptacles,
			raw:         raw,
		}), nil
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

func (c *Conn) query(ctx context.Context, query string, params []any) ([]any, error) {
	rows, err := c.DB.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("query columns: %w", err)
	}
	raw := make([]any, len(columns))
	receptacles := make([]any, len(columns))
	for i := range raw {
		receptacles[i] = &raw[i]
	}

	results := make([]any, 0)
	for rows.Next() {
		if err := rows.Scan(receptacles...); err != nil {
			return nil, fmt.Errorf("query scan: %w", err)
		}
		row := make(map[string]any, len(columns))
		for i, column := range columns {
			row[column] = convertValue(raw[i])
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	return results, nil
}

func (c *Conn) execute(ctx context.Context, query string, params []any) (map[string]any, error) {
	result, err := c.DB.ExecContext(ctx, query, params...)
	if err != nil {
		return nil, fmt.Errorf("execute: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	// Postgres has no last-insert-id; drivers return an error rather than 0.
	lastInsertID, err := result.LastInsertId()
	if err != nil {
		lastInsertID = 0
	}
	return map[string]any{
		"last_insert_id": lastInsertID,
		"rows_affected":  rowsAffected,
	}, nil
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
