// Package sql is the network relational database plugin covering MySQL,
// MariaDB and PostgreSQL. Scripts import it as scriptling.sql:
//
//	import scriptling.sql as sql
//	conn = sql.connect("postgres://user:pass@host:5432/app")
//	conn.execute("create table t (id serial primary key, name text)")
//	conn.execute("insert into t (name) values (?)", "ada")
//	rows = conn.query("select * from t where name = ?", "ada")
//	conn.close()
//
// The DSN scheme picks the driver: postgres:// or postgresql:// for
// PostgreSQL, mysql:// or mariadb:// for MySQL and MariaDB (both speak the
// MySQL protocol). The query/execute/close surface is identical to the
// sqlite plugin; ? placeholders are translated to $n on PostgreSQL, which
// also accepts explicit $n. The server address must pass the host's network
// policy. The same library serves external plugin mode (plugins/sql/cmd)
// and compiled-in registration (build tag plugin_sql).
package sql

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/paularlott/scriptling/extlibs/netsecurity"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/plugin"
	"github.com/paularlott/scriptling/plugins/internal/relational"
)

// Description is the plugin metadata description.
const Description = "MySQL, MariaDB and PostgreSQL client"

// ConnectSource is the scriptling-source wrapper for connect() in external
// plugin mode, where a Go function cannot return an instance over the wire:
// it constructs the Connection class through the plugin object protocol.
const ConnectSource = `def connect(dsn):
    return Connection(dsn)
`

// ScriptModule is the user-facing scriptling.sql module for compiled-in
// registration: the Connection wrapper (with get_orm) plus the shared ORM
// kit, importing the native twin.
func ScriptModule() string {
	return relational.ScriptModuleSource("scriptling._sql", false, relational.MySQLSpec)
}

// RegisterInProcess registers both halves for embedders and tests.
func RegisterInProcess(registrar interface {
	RegisterLibrary(*object.Library)
	RegisterScriptLibrary(name string, script string) error
}, policy *plugin.Policy) {
	registrar.RegisterLibrary(Build(&plugin.StaticPolicy{P: policy}))
	if err := registrar.RegisterScriptLibrary("scriptling.sql", ScriptModule()); err != nil {
		panic("register scriptling.sql script module: " + err.Error())
	}
}

const connectHelp = `connect(dsn) -> Connection

Connect to a relational database and return a Connection with the same
query/execute/close surface as the sqlite plugin. The DSN scheme selects
the driver: postgres:// (or postgresql://) for PostgreSQL, mysql:// or
mariadb:// for MySQL and MariaDB. ? placeholders become $n on PostgreSQL.
The server address must pass the host's network policy.`

// Build returns the scriptling.sql library. policy is read at call time so an
// external plugin sees the policy its handshake delivered.
func Build(policy plugin.PolicySource) *object.Library {
	connectionClass := relational.ConnectionClass(func(kwargs object.Kwargs, dsn string) (*relational.Conn, error) {
		return connect(policy, dsn)
	}).Build()

	functions := map[string]*object.Builtin{
		"connect": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) != 1 {
					return &object.Error{Message: fmt.Sprintf("connect takes exactly 1 argument (dsn), got %d", len(args))}
				}
				dsn, err := args[0].AsString()
				if err != nil {
					return err
				}
				conn, connErr := connect(policy, dsn)
				if connErr != nil {
					return &object.Error{Message: connErr.Error()}
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
	// scriptling.sql module is script source (ScriptModule) wrapping this.
	return object.NewLibrary("scriptling._sql", functions, constants, Description)
}

func connect(policy plugin.PolicySource, dsn string) (*relational.Conn, error) {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return nil, fmt.Errorf("invalid dsn: %w", err)
	}
	guard, err := policy.Policy().Guard()
	if err != nil {
		return nil, err
	}

	switch strings.ToLower(parsed.Scheme) {
	case "postgres", "postgresql":
		return openPostgres(dsn, guard)
	case "mysql", "mariadb":
		return openMySQL(parsed, guard)
	case "":
		return nil, fmt.Errorf("dsn requires a scheme (postgres://, mysql:// or mariadb://)")
	default:
		return nil, fmt.Errorf("unsupported dsn scheme %q (use postgres://, mysql:// or mariadb://)", parsed.Scheme)
	}
}

// pgConfigCache maps a guard-free DSN to its registered connector name.
// stdlib.RegisterConnConfig appends to a global registry with no removal,
// so long-lived hosts opening many connections would grow it forever;
// caching one entry per distinct DSN bounds the common no-policy case.
// Guarded configs are never cached: the dialer closes over one specific
// guard, and hosts may connect with the same DSN under different policies.
var pgConfigCache sync.Map // dsn string -> registered name (string)

func openPostgres(dsn string, guard *netsecurity.Guard) (*relational.Conn, error) {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse postgres dsn: %w", err)
	}
	if guard != nil {
		cfg.DialFunc = guard.DialContext
	}
	// RegisterConnConfig mints a private name backed by cfg; database/sql
	// opens through it, keeping the guarded dialer in place.
	var registered string
	if guard == nil {
		if cached, ok := pgConfigCache.Load(dsn); ok {
			registered = cached.(string)
		} else {
			registered = stdlib.RegisterConnConfig(cfg)
			pgConfigCache.Store(dsn, registered)
		}
	} else {
		registered = stdlib.RegisterConnConfig(cfg)
	}
	db, err := sql.Open("pgx", registered)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	return &relational.Conn{DB: db}, nil
}

func openMySQL(parsed *url.URL, guard *netsecurity.Guard) (*relational.Conn, error) {
	cfg := mysqlConfigFromURL(parsed)
	if guard != nil {
		cfg.DialFunc = guard.DialContext
	}
	// Open through a connector, not FormatDSN: the DSN string cannot carry
	// the guarded dialer, and silently dropping it would bypass policy.
	connector, err := mysql.NewConnector(cfg)
	if err != nil {
		return nil, fmt.Errorf("connect mysql: %w", err)
	}
	db := sql.OpenDB(connector)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect mysql: %w", err)
	}
	return &relational.Conn{DB: db}, nil
}

// mysqlConfigFromURL converts a mysql:// or mariadb:// URL into the driver's
// Config. Query parameters (charset, tls, etc.) pass through as DSN params.
func mysqlConfigFromURL(parsed *url.URL) *mysql.Config {
	cfg := mysql.NewConfig()
	cfg.Net = "tcp"
	host := parsed.Hostname()
	if host == "" {
		host = "localhost"
	}
	port := parsed.Port()
	if port == "" {
		port = "3306"
	}
	cfg.Addr = net.JoinHostPort(host, port)
	cfg.User = parsed.User.Username()
	cfg.Passwd, _ = parsed.User.Password()
	cfg.DBName = strings.TrimPrefix(parsed.Path, "/")
	if len(parsed.RawQuery) > 0 {
		if cfg.Params == nil {
			cfg.Params = make(map[string]string)
		}
		for key, values := range parsed.Query() {
			if len(values) > 0 {
				cfg.Params[key] = values[0]
			}
		}
	}
	return cfg
}
