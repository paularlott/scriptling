// Package sqlite is the sqlite database plugin. Scripts import it as
// scriptling.sqlite and get a relational API shared with the sql plugin:
//
//	import scriptling.sqlite as sqlite
//	conn = sqlite.connect("app.db")
//	conn.execute("create table t (id integer primary key, name text)")
//	conn.execute("insert into t (name) values (?)", "ada")
//	rows = conn.query("select * from t where name = ?", "ada")
//	conn.close()
//
// The database file must fall inside the host's allowed paths (":memory:"
// is always allowed). The same library serves two modes: an external plugin
// process (plugins/sqlite/cmd) and compiled-in registration (build tag
// plugin_sqlite) — Build is the single registration for both.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver

	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/plugin"
	"github.com/paularlott/scriptling/plugins/internal/kwarg"
	"github.com/paularlott/scriptling/plugins/internal/relational"
)

// Description is the plugin metadata description.
const Description = "SQLite embedded database (pure Go)"

// ConnectSource is the scriptling-source wrapper for connect() used in
// external plugin mode, where a Go function cannot return an instance over
// the wire: it constructs the Connection class through the plugin object
// protocol instead. In compiled-in mode the Go builtin below is used.
const ConnectSource = `def connect(path=":memory:", timeout_ms=5000):
    return Connection(path, timeout_ms=timeout_ms)
`

// ScriptModule is the user-facing scriptling.sqlite module for compiled-in
// registration: the Connection wrapper (with get_orm) plus the shared ORM
// kit, importing the native twin.
func ScriptModule() string {
	return relational.ScriptModuleSource("scriptling._sqlite", true, relational.SQLiteSpec)
}

// RegisterInProcess registers both halves for embedders and tests: the
// native twin library and the scriptling.sqlite script module.
func RegisterInProcess(registrar interface {
	RegisterLibrary(*object.Library)
	RegisterScriptLibrary(name string, script string) error
}, policy *plugin.Policy) {
	registrar.RegisterLibrary(Build(&plugin.StaticPolicy{P: policy}))
	if err := registrar.RegisterScriptLibrary("scriptling.sqlite", ScriptModule()); err != nil {
		panic("register scriptling.sqlite script module: " + err.Error())
	}
}

const connectHelp = `connect(path=":memory:", timeout_ms=5000) -> Connection

Open a SQLite database file and return a Connection. ":memory:" opens a
private in-memory database (always allowed, no file needed). The path must
be inside the host's allowed paths. timeout_ms bounds how long writers wait
for a lock before failing.`

// Build returns the scriptling.sqlite library. policy is read at call time so an
// external plugin sees the policy its handshake delivered (the handshake
// always precedes the first function call).
func Build(policy plugin.PolicySource) *object.Library {
	connectionClass := relational.ConnectionClass(func(kwargs object.Kwargs, path string) (*relational.Conn, error) {
		timeout, errObj := kwargs.GetInt("timeout_ms", defaultTimeoutMs)
		if errObj != nil {
			return nil, kwarg.Err(errObj)
		}
		return open(policy, path, timeout)
	}).Build()

	functions := map[string]*object.Builtin{
		"connect": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				path, errObj := connectArgs(kwargs, args)
				if errObj != nil {
					return errObj
				}
				timeout, errObj := kwargs.GetInt("timeout_ms", defaultTimeoutMs)
				if errObj != nil {
					return errObj
				}
				conn, err := open(policy, path, timeout)
				if err != nil {
					return &object.Error{Message: err.Error()}
				}
				return object.NewReceiverInstance(connectionClass, "Connection", conn)
			},
			HelpText: connectHelp,
		},
	}
	constants := map[string]object.Object{
		"Connection": connectionClass,
		"Cursor":     relational.CursorClass().Build(),
	}
	// The library registers under the twin name: the user-facing
	// scriptling.sqlite module is script source (ScriptModule) wrapping this.
	return object.NewLibrary("scriptling._sqlite", functions, constants, Description)
}

const defaultTimeoutMs = int64(5000)

// connectArgs resolves connect()'s optional path: absent means ":memory:",
// and a lone positional is the database path.
func connectArgs(kwargs object.Kwargs, args []object.Object) (string, object.Object) {
	if len(args) > 1 {
		return "", &object.Error{Message: fmt.Sprintf("connect takes at most 1 positional argument (path), got %d", len(args))}
	}
	path := ":memory:"
	if len(args) == 1 {
		value, err := args[0].AsString()
		if err != nil {
			return "", err
		}
		path = value
	}
	return path, nil
}

func open(policy plugin.PolicySource, path string, timeoutMilliseconds int64) (*relational.Conn, error) {
	if timeoutMilliseconds <= 0 {
		timeoutMilliseconds = defaultTimeoutMs
	}
	if path == "" {
		path = ":memory:"
	}

	isMemory := path == ":memory:" || strings.HasPrefix(path, "file::memory:")
	sharedCache := false
	// A file: URI names its database in the URI path (file:/tmp/app.db,
	// file:relative.db): the policy check applies to that path, not to the
	// URI string. Unparseable or non-local URIs fail closed.
	checkPath := path
	if strings.HasPrefix(path, "file:") {
		uri, uriErr := url.Parse(path)
		if uriErr != nil || uri.Host != "" {
			return nil, fmt.Errorf("invalid file: uri %q", path)
		}
		query := uri.Query()
		// SQLite's mode=memory stores the database in memory; the URI path is
		// only a name, so like :memory: there is no file for the policy to
		// guard. cache=shared shares one memory database across connections;
		// without it each pooled connection would see a private one.
		if !isMemory && strings.EqualFold(query.Get("mode"), "memory") {
			isMemory = true
		}
		sharedCache = strings.EqualFold(query.Get("cache"), "shared")
		if !isMemory {
			checkPath = uri.Path
			if checkPath == "" {
				checkPath = uri.Opaque
			}
			if checkPath == "" {
				return nil, fmt.Errorf("invalid file: uri %q", path)
			}
		}
	}
	if !isMemory && !policy.Policy().PathAllowed(checkPath) {
		return nil, fmt.Errorf("path %q is not in the allowed paths", checkPath)
	}

	// busy_timeout keeps writers from failing immediately when another
	// connection holds the lock; foreign_keys is what scripts expect on by
	// default.
	dsn := path
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	dsn = dsn + sep + fmt.Sprintf("_pragma=busy_timeout(%d)&_pragma=foreign_keys(1)", timeoutMilliseconds)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// An in-memory database lives inside a single pooled connection; pooling
	// several would give each its own private database. A shared-cache memory
	// database is the exception: cache=shared is exactly the mechanism that
	// lets connections share one, so pooling stays enabled.
	if isMemory && !sharedCache {
		db.SetMaxOpenConns(1)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	return &relational.Conn{DB: db}, nil
}
