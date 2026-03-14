package extlibs

import (
	"context"
	"path/filepath"
	"time"

	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/snapshotkv"
)

// InitKVStore initializes the KV store with the given storage path.
// If path is empty, the store operates in memory-only mode.
func InitKVStore(path string) error {
	// Close existing store if any
	if RuntimeState.KVDB != nil {
		RuntimeState.KVDB.Close()
	}

	// Configure with short TTL cleanup interval for background cleanup
	cfg := &snapshotkv.Config{
		TTLCleanupInterval: time.Minute, // Periodic cleanup
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

// CloseKVStore closes the KV store
func CloseKVStore() {
	if RuntimeState.KVDB != nil {
		RuntimeState.KVDB.Close()
		RuntimeState.KVDB = nil
	}
}

var KVSubLibrary = object.NewLibrary("kv", map[string]*object.Builtin{
	"set": {
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

			var ttl int64 = 0
			if t := kwargs.Get("ttl"); t != nil {
				if ttlVal, e := t.AsInt(); e == nil {
					ttl = ttlVal
				}
			} else if len(args) > 2 {
				if ttlVal, e := args[2].AsInt(); e == nil {
					ttl = ttlVal
				}
			}

			db := RuntimeState.KVDB
			if db == nil {
				return errors.NewError("kv.set: KV store not initialized")
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
		HelpText: `set(key, value, ttl=0) - Store a value with optional TTL in seconds

Parameters:
  key (string): The key to store the value under
  value: The value to store (string, int, float, bool, list, dict)
  ttl (int, optional): Time-to-live in seconds. 0 means no expiration.

Example:
  runtime.kv.set("api_key", "secret123")
  runtime.kv.set("session:abc", {"user": "bob"}, ttl=3600)`,
	},

	"get": {
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

			db := RuntimeState.KVDB
			if db == nil {
				return errors.NewError("kv.get: KV store not initialized")
			}

			value, goErr := db.Get(key)
			if goErr != nil {
				return defaultValue
			}

			return conversion.FromGo(value)
		},
		HelpText: `get(key, default=None) - Retrieve a value by key

Parameters:
  key (string): The key to retrieve
  default: Value to return if key doesn't exist (default: None)

Returns:
  The stored value, or the default if not found

Example:
  value = runtime.kv.get("api_key")
  count = runtime.kv.get("counter", default=0)`,
	},

	"delete": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if objErr := errors.MinArgs(args, 1); objErr != nil {
				return objErr
			}

			key, objErr := args[0].AsString()
			if objErr != nil {
				return objErr
			}

			db := RuntimeState.KVDB
			if db == nil {
				return errors.NewError("kv.delete: KV store not initialized")
			}

			db.Delete(key)
			return &object.Null{}
		},
		HelpText: `delete(key) - Remove a key from the store

Parameters:
  key (string): The key to delete

Example:
  runtime.kv.delete("session:abc")`,
	},

	"exists": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if objErr := errors.MinArgs(args, 1); objErr != nil {
				return objErr
			}

			key, objErr := args[0].AsString()
			if objErr != nil {
				return objErr
			}

			db := RuntimeState.KVDB
			if db == nil {
				return errors.NewError("kv.exists: KV store not initialized")
			}

			return &object.Boolean{Value: db.Exists(key)}
		},
		HelpText: `exists(key) - Check if a key exists and is not expired

Parameters:
  key (string): The key to check

Returns:
  bool: True if key exists and is not expired

Example:
  if runtime.kv.exists("config"):
      config = runtime.kv.get("config")`,
	},

	"incr": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if objErr := errors.MinArgs(args, 1); objErr != nil {
				return objErr
			}

			key, objErr := args[0].AsString()
			if objErr != nil {
				return objErr
			}

			var amount int64 = 1
			if a := kwargs.Get("amount"); a != nil {
				if amt, e := a.AsInt(); e == nil {
					amount = amt
				}
			} else if len(args) > 1 {
				if amt, e := args[1].AsInt(); e == nil {
					amount = amt
				}
			}

			// Lock for atomic read-modify-write (snapshotkv is thread-safe but
			// we need to ensure the Get and Set happen atomically for incr)
			RuntimeState.Lock()
			defer RuntimeState.Unlock()

			db := RuntimeState.KVDB
			if db == nil {
				return errors.NewError("kv.incr: KV store not initialized")
			}

			currentVal, goErr := db.Get(key)
			if goErr != nil {
				// Key doesn't exist, create it
				db.Set(key, amount)
				return object.NewInteger(amount)
			}

			// Handle different integer types from deserialization
			var intVal int64
			switch v := currentVal.(type) {
			case int64:
				intVal = v
			case int:
				intVal = int64(v)
			case float64:
				intVal = int64(v)
			default:
				return errors.NewError("kv.incr: value is not an integer")
			}

			newVal := intVal + amount
			db.Set(key, newVal)

			return object.NewInteger(newVal)
		},
		HelpText: `incr(key, amount=1) - Atomically increment an integer value

Parameters:
  key (string): The key to increment
  amount (int, optional): Amount to increment by (default: 1)

Returns:
  int: The new value after incrementing

Example:
  runtime.kv.set("counter", 0)
  runtime.kv.incr("counter")      # returns 1
  runtime.kv.incr("counter", 5)   # returns 6`,
	},

	"ttl": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if objErr := errors.MinArgs(args, 1); objErr != nil {
				return objErr
			}

			key, objErr := args[0].AsString()
			if objErr != nil {
				return objErr
			}

			db := RuntimeState.KVDB
			if db == nil {
				return errors.NewError("kv.ttl: KV store not initialized")
			}

			// Check if key exists first
			if !db.Exists(key) {
				return object.NewInteger(-2) // Key doesn't exist
			}

			// Get TTL from snapshotkv
			remaining := db.TTL(key)
			if remaining < 0 {
				return object.NewInteger(-1) // No expiration
			}

			return object.NewInteger(int64(remaining.Seconds()))
		},
		HelpText: `ttl(key) - Get remaining time-to-live for a key

Parameters:
  key (string): The key to check

Returns:
  int: Remaining TTL in seconds, -1 if no expiration, -2 if key doesn't exist

Example:
  runtime.kv.set("session", "data", ttl=3600)
  remaining = runtime.kv.ttl("session")  # e.g., 3599`,
	},

	"keys": {
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

			db := RuntimeState.KVDB
			if db == nil {
				return errors.NewError("kv.keys: KV store not initialized")
			}

			// Get all keys (empty prefix matches all)
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
		HelpText: `keys(pattern="*") - Get all keys matching a glob pattern

Parameters:
  pattern (string, optional): Glob pattern to match keys (default: "*")

Returns:
  list: List of matching keys

Example:
  all_keys = runtime.kv.keys()
  user_keys = runtime.kv.keys("user:*")
  session_keys = runtime.kv.keys("session:*")`,
	},

	"clear": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			db := RuntimeState.KVDB
			if db == nil {
				return errors.NewError("kv.clear: KV store not initialized")
			}

			// Get all keys and delete them (snapshotkv is thread-safe)
			allKeys := db.FindKeysByPrefix("")
			for _, key := range allKeys {
				db.Delete(key)
			}

			return &object.Null{}
		},
		HelpText: `clear() - Remove all keys from the store

Warning: This operation cannot be undone.

Example:
  runtime.kv.clear()`,
	},
}, nil, "Thread-safe key-value store for sharing state across requests.\n\nNote: By default the KV store is in-memory. To persist data, configure a storage\npath when starting the server. Keys without a TTL persist indefinitely.\nUse TTLs and periodic cleanup to avoid unbounded storage growth.")
