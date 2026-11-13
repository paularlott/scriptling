// Package kv provides the scriptling.runtime.kv library: a thread-safe
// key-value store (the process-wide default store plus named stores opened
// with kv.open), backed by snapshotkv. It is independent of the rest of the
// runtime family, so an application can register it without background tasks
// or the HTTP server.
package kv

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/paularlott/logger"
	logslog "github.com/paularlott/logger/slog"
	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/libutil"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/snapshotkv"
)

const kvMemoryPrefix = ":memory:"

// kvLogger reports KV snapshot save failures. It defaults to a console
// logger on stderr so disk-write errors are never silent; InitStore
// replaces it with the host application's logger.
var kvLogger = newKVDefaultLogger()

func newKVDefaultLogger() logger.Logger {
	return logslog.New(logslog.Config{
		Level:  "error",
		Format: "console",
		Writer: os.Stderr,
	})
}

// kvSaveErrorHook returns an OnSaveError callback that logs failed snapshot
// writes for the store at path. Memory-only stores never save, so the hook
// is a no-op for them.
func kvSaveErrorHook(path string) func(error) {
	if path == "" {
		return func(error) {}
	}
	return func(err error) {
		if kvLogger != nil {
			kvLogger.Error("kv store: snapshot write to disk failed, data may not be persisted", "path", path, "error", err)
		}
	}
}

// kvRegistry is the global store registry, keyed by path/name.
var kvRegistry = struct {
	sync.Mutex
	stores map[string]*snapshotkv.DB
}{
	stores: make(map[string]*snapshotkv.DB),
}

// LibraryName is the name scripts import the library by.
const LibraryName = "scriptling.runtime.kv"

// defaultStore is the system-wide default store exposed as kv.default.
var defaultStore struct {
	sync.RWMutex
	db *snapshotkv.DB
}

// cleanupOnce registers the store cleanup with libutil.RegisterCleanup once:
// registered cleanups persist across resets, so registering per library build
// would grow the list with every interpreter.
var cleanupOnce sync.Once

// registerCleanup arranges for runtime.ResetRuntime (which runs
// libutil.RunCleanups) to close the default store and every named store.
func registerCleanup() {
	cleanupOnce.Do(func() {
		libutil.RegisterCleanup(func() {
			CloseStore()
			closeKVRegistry()
		})
	})
}

// InitStore initializes the system-wide default KV store.
// If path is empty, the store operates in memory-only mode.
// A non-nil log receives snapshot save-failure reports; nil keeps the
// current logger (a stderr console logger by default).
// Call it before Register: kv.default is bound when the library is built.
func InitStore(path string, log logger.Logger) error {
	registerCleanup()
	if log != nil {
		kvLogger = log
	}

	cfg := &snapshotkv.Config{
		TTLCleanupInterval: time.Minute,
		OnSaveError:        kvSaveErrorHook(path),
	}

	db, err := snapshotkv.Open(path, cfg)
	if err != nil {
		return err
	}

	defaultStore.Lock()
	old := defaultStore.db
	defaultStore.db = db
	defaultStore.Unlock()
	if old != nil {
		old.Close()
	}

	return nil
}

// CloseStore closes the system-wide default KV store.
func CloseStore() {
	defaultStore.Lock()
	db := defaultStore.db
	defaultStore.db = nil
	defaultStore.Unlock()
	if db != nil {
		db.Close()
	}
}

// DefaultDB returns the system-wide default store opened by InitStore, or nil
// when none is open.
func DefaultDB() *snapshotkv.DB {
	defaultStore.RLock()
	defer defaultStore.RUnlock()
	return defaultStore.db
}

// closeKVRegistry closes all stores in the registry and clears it.
func closeKVRegistry() {
	kvRegistry.Lock()
	defer kvRegistry.Unlock()
	for _, db := range kvRegistry.stores {
		db.Close()
	}
	kvRegistry.stores = make(map[string]*snapshotkv.DB)
}

// openRegisteredStore opens or reuses a store from the registry.
func openRegisteredStore(name string) (*snapshotkv.DB, error) {
	kvRegistry.Lock()
	defer kvRegistry.Unlock()

	if db, ok := kvRegistry.stores[name]; ok {
		return db, nil
	}

	var fsPath string
	if !strings.HasPrefix(name, kvMemoryPrefix) {
		fsPath = name
	}

	cfg := &snapshotkv.Config{
		TTLCleanupInterval: time.Minute,
		OnSaveError:        kvSaveErrorHook(fsPath),
	}
	db, err := snapshotkv.Open(fsPath, cfg)
	if err != nil {
		return nil, err
	}

	kvRegistry.stores[name] = db
	return db, nil
}

// closeRegisteredStore immediately closes and removes a store from the
// registry. The error from the final flush is returned (it has already been
// logged via the store's OnSaveError hook).
func closeRegisteredStore(name string) error {
	kvRegistry.Lock()
	defer kvRegistry.Unlock()

	db, ok := kvRegistry.stores[name]
	if !ok {
		return nil
	}
	err := db.Close()
	delete(kvRegistry.stores, name)
	return err
}

// kvDBRegistry maps a kv store Builtin pointer to its underlying DB so that
// other packages (e.g. scriptling.ai.memory) can retrieve it via StoreDB.
var kvDBRegistry = struct {
	sync.RWMutex
	m map[*object.Builtin]*snapshotkv.DB
}{
	m: make(map[*object.Builtin]*snapshotkv.DB),
}

// StoreDB returns the underlying snapshotkv.DB for a kv store object,
// or nil if the object is not a kv store.
func StoreDB(store object.Object) *snapshotkv.DB {
	b, ok := store.(*object.Builtin)
	if !ok {
		return nil
	}
	kvDBRegistry.RLock()
	defer kvDBRegistry.RUnlock()
	return kvDBRegistry.m[b]
}

// newKVStoreObject returns a Builtin object with kv methods bound to db.
// If registryName is non-empty, close() will decrement the registry ref count.
// If registryName is empty (system default), close() is a no-op.
func newKVStoreObject(db *snapshotkv.DB, registryName string) *object.Builtin {
	// A nil DB (the default store before InitStore, or after
	// runtime.ResetRuntime/CloseStore) must surface as a catchable error on first
	// use rather than dereferencing nil and crashing the host.
	notOpen := func() object.Object {
		return errors.NewError("kv store is not open; open one with scriptling.runtime.kv.open()")
	}
	obj := &object.Builtin{
		Attributes: map[string]object.Object{
			"set": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if db == nil {
						return notOpen()
					}
					if objErr := errors.MinArgs(args, 2); objErr != nil {
						return objErr
					}
					key, objErr := args[0].AsString()
					if objErr != nil {
						return objErr
					}
					value, convErr := conversion.ToGoWithError(args[1])
					if convErr != nil {
						return convErr
					}
					var ttl int64
					if t := kwargs.Get("ttl"); t != nil {
						if ttlVal, e := t.AsInt(); e == nil {
							ttl = ttlVal
						}
					} else if len(args) > 2 {
						if ttlVal, e := args[2].AsInt(); e == nil {
							ttl = ttlVal
						}
					}
					var ttlDuration time.Duration
					if ttl > 0 {
						ttlDuration = time.Duration(ttl) * time.Second
					}
					if goErr := db.SetEx(key, value, ttlDuration); goErr != nil {
						return errors.NewError("kv.set: %v", goErr)
					}
					return &object.Null{}
				},
				HelpText: `set(key, value, ttl=0) - Store a value with optional TTL in seconds`,
			},

			"get": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if db == nil {
						return notOpen()
					}
					if objErr := errors.MinArgs(args, 1); objErr != nil {
						return objErr
					}
					key, objErr := args[0].AsString()
					if objErr != nil {
						return objErr
					}
					var defaultValue object.Object = &object.Null{}
					if d := kwargs.Get("default"); d != nil {
						defaultValue = d
					} else if len(args) > 1 {
						defaultValue = args[1]
					}
					value, goErr := db.Get(key)
					if goErr != nil {
						return defaultValue
					}
					return conversion.FromGo(value)
				},
				HelpText: `get(key, default=None) - Retrieve a value by key`,
			},

			"delete": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if db == nil {
						return notOpen()
					}
					if objErr := errors.MinArgs(args, 1); objErr != nil {
						return objErr
					}
					key, objErr := args[0].AsString()
					if objErr != nil {
						return objErr
					}
					db.Delete(key)
					return &object.Null{}
				},
				HelpText: `delete(key) - Remove a key from the store`,
			},

			"exists": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if db == nil {
						return notOpen()
					}
					if objErr := errors.MinArgs(args, 1); objErr != nil {
						return objErr
					}
					key, objErr := args[0].AsString()
					if objErr != nil {
						return objErr
					}
					return object.NewBoolean(db.Exists(key))
				},
				HelpText: `exists(key) - Check if a key exists and is not expired`,
			},

			"ttl": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if db == nil {
						return notOpen()
					}
					if objErr := errors.MinArgs(args, 1); objErr != nil {
						return objErr
					}
					key, objErr := args[0].AsString()
					if objErr != nil {
						return objErr
					}
					if !db.Exists(key) {
						return object.NewInteger(-2)
					}
					remaining := db.TTL(key)
					if remaining < 0 {
						return object.NewInteger(-1)
					}
					return object.NewInteger(int64(remaining.Seconds()))
				},
				HelpText: `ttl(key) - Get remaining TTL in seconds; -1 if no expiration, -2 if key doesn't exist`,
			},

			"keys": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if db == nil {
						return notOpen()
					}
					pattern := "*"
					if p := kwargs.Get("pattern"); p != nil {
						if pat, e := p.AsString(); e == nil {
							pattern = pat
						}
					} else if len(args) > 0 {
						if pat, e := args[0].AsString(); e == nil {
							pattern = pat
						}
					}
					allKeys := db.FindKeysByPrefix("")
					var keys []object.Object
					for _, key := range allKeys {
						if pattern == "*" {
							keys = append(keys, object.NewString(key))
						} else {
							matched, _ := filepath.Match(pattern, key)
							if matched {
								keys = append(keys, object.NewString(key))
							}
						}
					}
					return &object.List{Elements: keys}
				},
				HelpText: `keys(pattern="*") - Get all keys matching a glob pattern`,
			},

			"clear": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if db == nil {
						return notOpen()
					}
					for _, key := range db.FindKeysByPrefix("") {
						db.Delete(key)
					}
					return &object.Null{}
				},
				HelpText: `clear() - Remove all keys from the store`,
			},

			"incr": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if db == nil {
						return notOpen()
					}
					if objErr := errors.MinArgs(args, 1); objErr != nil {
						return objErr
					}
					key, objErr := args[0].AsString()
					if objErr != nil {
						return objErr
					}
					delta := int64(1)
					if len(args) > 1 {
						if d, e := args[1].AsInt(); e == nil {
							delta = d
						}
					}
					newVal, goErr := db.Update(key, func(old any) any {
						var current int64
						switch v := old.(type) {
						case int64:
							current = v
						case float64:
							current = int64(v)
						}
						return current + delta
					})
					if goErr != nil {
						return errors.NewError("kv.incr: %v", goErr)
					}
					n, ok := newVal.(int64)
					if !ok {
						return errors.NewError("kv.incr: unexpected stored type %T", newVal)
					}
					return object.NewInteger(n)
				},
				HelpText: `incr(key, delta=1) - Atomically increment an integer value, returns new value`,
			},

			"close": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if db == nil {
						return notOpen()
					}
					if registryName != "" {
						// Flush failures are already logged via OnSaveError;
						// surface them to the script as well.
						if err := closeRegisteredStore(registryName); err != nil {
							return errors.NewError("kv.close: %v", err)
						}
					}
					// no-op for the default system store
					return &object.Null{}
				},
				HelpText: `close() - Close this store, flushing pending writes to disk. No-op on the default store. Returns an error if the flush fails.`,
			},
		},
		HelpText: "KV store object — call .get(), .set(), .incr(), .delete(), .exists(), .ttl(), .keys(), .clear(), .close()",
	}
	kvDBRegistry.Lock()
	kvDBRegistry.m[obj] = db
	kvDBRegistry.Unlock()
	return obj
}

// newKVOpenBuiltin returns a kv.open builtin restricted to the given fssecurity.Config.
// In-memory stores (":memory:...") are always allowed regardless of the config.
func newKVOpenBuiltin(cfg fssecurity.Config) *object.Builtin {
	return &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if objErr := errors.MinArgs(args, 1); objErr != nil {
				return objErr
			}
			name, objErr := args[0].AsString()
			if objErr != nil {
				return objErr
			}
			if name == "" {
				return errors.NewError("kv.open: store name must not be empty; use \":memory:name\" for in-memory stores")
			}
			// Only validate filesystem paths — in-memory stores are always allowed.
			if !strings.HasPrefix(name, kvMemoryPrefix) {
				resolved, err := resolveKVPath(name)
				if err != nil || !cfg.IsPathAllowed(resolved) {
					return errors.NewPermissionError("kv.open: access denied: path '%s' is outside allowed directories", name)
				}
				// Open, and share, the store by the path that was checked: a
				// symlink swapped after the check cannot redirect it, and
				// aliases of one directory share one store.
				name = resolved
			}
			db, err := openRegisteredStore(name)
			if err != nil {
				return errors.NewError("kv.open: %v", err)
			}
			return newKVStoreObject(db, name)
		},
		HelpText: `open(name) - Open or reuse a named KV store

Parameters:
  name (string): Store name. Use ":memory:name" for in-memory stores,
                 or a filesystem path for persistent stores.

Returns:
  KV store object with get, set, delete, exists, ttl, keys, clear, close methods.

Example:
  import scriptling.runtime.kv as kv

  mem = kv.open(":memory:session")
  mem.set("user", "alice")
  mem.close()

  db = kv.open("/data/agent.db")
  db.set("fact", "the sky is blue")
  db.close()`,
	}
}

// resolveKVPath makes a store path absolute and resolves symlinks through its
// longest existing prefix (the store directory may not exist yet).
func resolveKVPath(name string) (string, error) {
	abs, err := filepath.Abs(name)
	if err != nil {
		return "", err
	}
	return fssecurity.ResolveExistingPrefix(abs), nil
}

// Register registers the scriptling.runtime.kv library. allowedPaths
// restricts where kv.open may create or open a persistent store, as it does
// for the filesystem libraries: nil leaves stores unrestricted, an empty
// slice denies every filesystem path. In-memory stores (":memory:name") are
// always allowed. A path outside the allowed paths is refused with a
// PermissionError that a script cannot catch.
//
// When scriptling.runtime is also registered with the same registrar the
// library is reachable as an attribute of it (`rt.kv`) too.
// Call InitStore first if scripts should have a kv.default store.
func Register(registrar object.LibraryRegistrar, allowedPaths []string) {
	libutil.RegisterSubLibrary(registrar, newLibrary(allowedPaths))
}

// newLibrary builds the kv library restricted to allowedPaths (nil means
// unrestricted, empty denies every filesystem path).
func newLibrary(allowedPaths []string) *object.Library {
	registerCleanup()
	cfg := fssecurity.Config{AllowedPaths: fssecurity.NormalizeAllowedPaths(allowedPaths)}
	return object.NewLibrary(LibraryName,
		map[string]*object.Builtin{
			"open": newKVOpenBuiltin(cfg),
		},
		map[string]object.Object{
			"default": newKVStoreObject(DefaultDB(), ""),
		},
		"Thread-safe key-value store. Use kv.default for the system store or kv.open() for named stores.",
	)
}
