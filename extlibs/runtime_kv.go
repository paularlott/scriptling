package extlibs

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/snapshotkv"
)

const kvMemoryPrefix = ":memory:"

// kvRegistry is the global store registry, keyed by path/name.
var kvRegistry = struct {
	sync.Mutex
	stores map[string]*snapshotkv.DB
}{
	stores: make(map[string]*snapshotkv.DB),
}

// InitKVStore initializes the system-wide default KV store.
// If path is empty, the store operates in memory-only mode.
func InitKVStore(path string) error {
	if RuntimeState.KVDB != nil {
		RuntimeState.KVDB.Close()
	}

	cfg := &snapshotkv.Config{
		TTLCleanupInterval: time.Minute,
	}

	db, err := snapshotkv.Open(path, cfg)
	if err != nil {
		return err
	}

	RuntimeState.Lock()
	RuntimeState.KVDB = db
	RuntimeState.Unlock()

	return nil
}

// CloseKVStore closes the system-wide default KV store.
func CloseKVStore() {
	if RuntimeState.KVDB != nil {
		RuntimeState.KVDB.Close()
		RuntimeState.KVDB = nil
	}
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
	}
	db, err := snapshotkv.Open(fsPath, cfg)
	if err != nil {
		return nil, err
	}

	kvRegistry.stores[name] = db
	return db, nil
}

// closeRegisteredStore immediately closes and removes a store from the registry.
func closeRegisteredStore(name string) {
	kvRegistry.Lock()
	defer kvRegistry.Unlock()

	db, ok := kvRegistry.stores[name]
	if !ok {
		return
	}
	db.Close()
	delete(kvRegistry.stores, name)
}

// kvDBRegistry maps a kv store Builtin pointer to its underlying DB so that
// other packages (e.g. scriptling.ai.memory) can retrieve it via KVStoreDB.
var kvDBRegistry = struct {
	sync.RWMutex
	m map[*object.Builtin]*snapshotkv.DB
}{
	m: make(map[*object.Builtin]*snapshotkv.DB),
}

// KVStoreDB returns the underlying snapshotkv.DB for a kv store object,
// or nil if the object is not a kv store.
func KVStoreDB(store object.Object) *snapshotkv.DB {
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
	obj := &object.Builtin{
		Attributes: map[string]object.Object{
			"set": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
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
							keys = append(keys, &object.String{Value: key})
						} else {
							matched, _ := filepath.Match(pattern, key)
							if matched {
								keys = append(keys, &object.String{Value: key})
							}
						}
					}
					return &object.List{Elements: keys}
				},
				HelpText: `keys(pattern="*") - Get all keys matching a glob pattern`,
			},

			"clear": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					for _, key := range db.FindKeysByPrefix("") {
						db.Delete(key)
					}
					return &object.Null{}
				},
				HelpText: `clear() - Remove all keys from the store`,
			},

			"incr": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
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
					var current int64
					if val, err := db.Get(key); err == nil {
						switch v := val.(type) {
						case int64:
							current = v
						case float64:
							current = int64(v)
						}
					}
					newVal := current + delta
					if goErr := db.Set(key, newVal); goErr != nil {
						return errors.NewError("kv.incr: %v", goErr)
					}
					return object.NewInteger(newVal)
				},
				HelpText: `incr(key, delta=1) - Atomically increment an integer value, returns new value`,
			},

			"close": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if registryName != "" {
						closeRegisteredStore(registryName)
					}
					// no-op for the default system store
					return &object.Null{}
				},
				HelpText: `close() - Close this store immediately. No-op on the default store.`,
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
			if !strings.HasPrefix(name, kvMemoryPrefix) && !cfg.IsPathAllowed(name) {
				return errors.NewError("kv.open: access denied: path '%s' is outside allowed directories", name)
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

// kvOpenBuiltin is the unrestricted default (nil AllowedPaths = allow all).
var kvOpenBuiltin = newKVOpenBuiltin(fssecurity.Config{AllowedPaths: nil})

// NewKVSubLibrary builds the kv sub-library with no path restrictions.
// Must be called after InitKVStore so RuntimeState.KVDB is set.
func NewKVSubLibrary() *object.Library {
	return newKVSubLibraryWithConfig(fssecurity.Config{AllowedPaths: nil})
}

// NewKVSubLibraryWithSecurity builds the kv sub-library restricted to allowedPaths.
// In-memory stores are always permitted. If allowedPaths is nil, all paths are allowed.
// If allowedPaths is an empty slice, all filesystem paths are denied.
// Must be called after InitKVStore so RuntimeState.KVDB is set.
func NewKVSubLibraryWithSecurity(allowedPaths []string) *object.Library {
	var normalized []string
	if allowedPaths != nil {
		normalized = make([]string, 0, len(allowedPaths))
		for _, p := range allowedPaths {
			abs, err := filepath.Abs(p)
			if err != nil {
				continue
			}
			normalized = append(normalized, filepath.Clean(abs))
		}
	}
	return newKVSubLibraryWithConfig(fssecurity.Config{AllowedPaths: normalized})
}

func newKVSubLibraryWithConfig(cfg fssecurity.Config) *object.Library {
	RegisterCleanup(closeKVRegistry)
	return object.NewLibrary(RuntimeKVLibraryName,
		map[string]*object.Builtin{
			"open": newKVOpenBuiltin(cfg),
		},
		map[string]object.Object{
			"default": newKVStoreObject(RuntimeState.KVDB, ""),
		},
		"Thread-safe key-value store. Use kv.default for the system store or kv.open() for named stores.",
	)
}
