// Package sql is the network relational database plugin covering MySQL,
// MariaDB and PostgreSQL. Scripts import it as scriptling.sql:
//
//	import scriptling.sql as sql
//	conn = sql.connect("postgres://user:pass@host:5432/app")
//	conn.execute("create table t (id serial primary key, name text)")
//	conn.execute("insert into t (name) values (?)", "ada")
//	rows = conn.query("select * from t where name = ?", "ada")
//	tx = conn.begin()
//	tx.execute("insert into t (name) values (?)", "grace")
//	tx.commit()
//	conn.close()
//
// The DSN scheme picks the driver: postgres:// or postgresql:// for
// PostgreSQL, mysql:// or mariadb:// for MySQL and MariaDB (both speak the
// MySQL protocol). The query/execute/begin/close surface is identical to the
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
query/execute/begin/close surface as the sqlite plugin. The DSN scheme
selects the driver: postgres:// (or postgresql://) for PostgreSQL, mysql://
or mariadb:// for MySQL and MariaDB. ? placeholders become $n on PostgreSQL.
The server address must pass the host's network policy.`

// Build returns the scriptling.sql library. policy is read at call time so an
// external plugin sees the policy its handshake delivered.
func Build(policy plugin.PolicySource) *object.Library {
	connectionClass := relational.ConnectionClass(func(ctx context.Context, kwargs object.Kwargs, dsn string) (*relational.Conn, error) {
		return connect(ctx, policy, dsn)
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
				conn, connErr := connect(ctx, policy, dsn)
				if connErr != nil {
					return &object.Error{Message: connErr.Error()}
				}
				return object.NewReceiverInstance(connectionClass, "Connection", conn)
			},
			HelpText: connectHelp,
		},
	}
	constants := map[string]object.Object{
		"Connection":  connectionClass,
		"Cursor":      relational.CursorClass().Build(),
		"Transaction": relational.TransactionClass().Build(),
	}
	// The library registers under the twin name: the user-facing
	// scriptling.sql module is script source (ScriptModule) wrapping this.
	return object.NewLibrary("scriptling._sql", functions, constants, Description)
}

func connect(ctx context.Context, policy plugin.PolicySource, dsn string) (*relational.Conn, error) {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return nil, fmt.Errorf("invalid dsn: %w", err)
	}
	guard, err := policy.Policy().Guard()
	if err != nil {
		return nil, err
	}

	// url.Parse lowercases the scheme; hand the drivers the re-serialized
	// form so a Postgres:// DSN reaches pgx with the scheme it expects.
	dsn = parsed.String()
	switch strings.ToLower(parsed.Scheme) {
	case "postgres", "postgresql":
		return openPostgres(ctx, dsn, guard)
	case "mysql", "mariadb":
		return openMySQL(ctx, parsed, guard)
	case "":
		return nil, fmt.Errorf("dsn requires a scheme (postgres://, mysql:// or mariadb://)")
	default:
		return nil, fmt.Errorf("unsupported dsn scheme %q (use postgres://, mysql:// or mariadb://)", parsed.Scheme)
	}
}

func openPostgres(ctx context.Context, dsn string, guard *netsecurity.Guard) (*relational.Conn, error) {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse postgres dsn: %w", err)
	}
	if guard != nil {
		cfg.DialFunc = guard.DialContext
	}
	// OpenDB takes the config directly: RegisterConnConfig would mint an
	// entry in a process-global registry that has no unregister, leaking one
	// name per connect attempt (including failed pings).
	db := stdlib.OpenDB(*cfg)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	return &relational.Conn{DB: db}, nil
}

func openMySQL(ctx context.Context, parsed *url.URL, guard *netsecurity.Guard) (*relational.Conn, error) {
	cfg, cfgErr := mysqlConfigFromURL(parsed)
	if cfgErr != nil {
		return nil, cfgErr
	}
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
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect mysql: %w", err)
	}
	return &relational.Conn{DB: db}, nil
}

// mysqlConfigFromURL converts a mysql:// or mariadb:// URL into the driver's
// Config. Query parameters pass through as session variables (cfg.Params,
// sent as SET on connect) — charset, sql_mode, time_zone and friends. tls is
// the one driver option reachable from the URL (true, false, skip-verify);
// the others (allowAllFiles, timeouts) are DSN-string settings and stay
// deliberately unreachable from a URL query.
func mysqlConfigFromURL(parsed *url.URL) (*mysql.Config, error) {
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
			if len(values) == 0 {
				continue
			}
			// tls is the one driver option worth reaching from a URL; the
			// driver's registered names are exactly these three. Everything
			// else stays a session variable.
			if key == "tls" {
				switch value := values[0]; value {
				case "true", "false", "skip-verify":
					cfg.TLSConfig = value
				default:
					return nil, fmt.Errorf("invalid mysql tls option %q (true, false or skip-verify)", value)
				}
				continue
			}
			cfg.Params[key] = values[0]
		}
	}
	return cfg, nil
}
