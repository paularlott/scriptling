package extlibs

import (
	"context"
	"path/filepath"
	"time"

	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// kvEntry represents a stored value with optional TTL
type kvEntry struct {
	value     interface{}
	expiresAt time.Time
}

// isExpired checks if an entry has expired
func (e *kvEntry) isExpired() bool {
	if e.expiresAt.IsZero() {
		return false
	}
	return time.Now().After(e.expiresAt)
}

// deepCopy creates a deep copy of basic types
func deepCopy(v interface{}) interface{} {
	if v == nil {
		return nil
	}

	switch val := v.(type) {
	case string, int, int64, float64, bool:
		return val
	case []interface{}:
		result := make([]interface{}, len(val))
		for i, item := range val {
			result[i] = deepCopy(item)
		}
		return result
	case map[string]interface{}:
		result := make(map[string]interface{})
		for k, v := range val {
			result[k] = deepCopy(v)
		}
		return result
	default:
		return val
	}
}

// convertObjectToKVValue converts a scriptling Object to a storable basic type
func convertObjectToKVValue(obj object.Object) (interface{}, *object.Error) {
	return conversion.ToGoWithError(obj)
}

// convertKVValueToObject converts a storable basic type back to a scriptling Object
func convertKVValueToObject(v interface{}) object.Object {
	return conversion.FromGo(v)
}

var KVSubLibrary = object.NewLibrary("kv", map[string]*object.Builtin{
	"set": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 2); err != nil {
				return err
			}

			key, err := args[0].AsString()
			if err != nil {
				return err
			}

			value, convErr := convertObjectToKVValue(args[1])
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

			entry := &kvEntry{value: value}
			if ttl > 0 {
				entry.expiresAt = time.Now().Add(time.Duration(ttl) * time.Second)
			}

			RuntimeState.Lock()
			RuntimeState.KVData[key] = entry
			RuntimeState.Unlock()

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
			if err := errors.MinArgs(args, 1); err != nil {
				return err
			}

			key, err := args[0].AsString()
			if err != nil {
				return err
			}

			var defaultValue object.Object = &object.Null{}
			if d := kwargs.Get("default"); d != nil {
				defaultValue = d
			} else if len(args) > 1 {
				defaultValue = args[1]
			}

			RuntimeState.RLock()
			entry, exists := RuntimeState.KVData[key]
			RuntimeState.RUnlock()

			if !exists || entry.isExpired() {
				return defaultValue
			}

			return convertKVValueToObject(deepCopy(entry.value))
		},
		HelpText: `get(key, default=None) - Retrieve a value by key

Parameters:
  key (string): The key to retrieve
  default: Value to return if key doesn't exist (default: None)

Returns:
  The stored value (deep copy), or the default if not found

Example:
  value = runtime.kv.get("api_key")
  count = runtime.kv.get("counter", default=0)`,
	},

	"delete": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.MinArgs(args, 1); err != nil {
				return err
			}

			key, err := args[0].AsString()
			if err != nil {
				return err
			}

			RuntimeState.Lock()
			delete(RuntimeState.KVData, key)
			RuntimeState.Unlock()

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
			if err := errors.MinArgs(args, 1); err != nil {
				return err
			}

			key, err := args[0].AsString()
			if err != nil {
				return err
			}

			RuntimeState.RLock()
			entry, exists := RuntimeState.KVData[key]
			RuntimeState.RUnlock()

			if !exists || entry.isExpired() {
				return &object.Boolean{Value: false}
			}
			return &object.Boolean{Value: true}
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
			if err := errors.MinArgs(args, 1); err != nil {
				return err
			}

			key, err := args[0].AsString()
			if err != nil {
				return err
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

			RuntimeState.Lock()
			defer RuntimeState.Unlock()

			entry, exists := RuntimeState.KVData[key]
			if !exists || entry.isExpired() {
				RuntimeState.KVData[key] = &kvEntry{value: amount}
				return object.NewInteger(amount)
			}

			currentVal, ok := entry.value.(int64)
			if !ok {
				return errors.NewError("kv.incr: value is not an integer")
			}

			newVal := currentVal + amount
			entry.value = newVal

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
			if err := errors.MinArgs(args, 1); err != nil {
				return err
			}

			key, err := args[0].AsString()
			if err != nil {
				return err
			}

			RuntimeState.RLock()
			entry, exists := RuntimeState.KVData[key]
			RuntimeState.RUnlock()

			if !exists || entry.isExpired() {
				return object.NewInteger(-2)
			}

			if entry.expiresAt.IsZero() {
				return object.NewInteger(-1)
			}

			remaining := time.Until(entry.expiresAt).Seconds()
			return object.NewInteger(int64(remaining))
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

			RuntimeState.RLock()
			defer RuntimeState.RUnlock()

			var keys []object.Object
			for key, entry := range RuntimeState.KVData {
				if entry.isExpired() {
					continue
				}

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
			RuntimeState.Lock()
			RuntimeState.KVData = make(map[string]*kvEntry)
			RuntimeState.Unlock()

			return &object.Null{}
		},
		HelpText: `clear() - Remove all keys from the store

Warning: This operation cannot be undone.

Example:
  runtime.kv.clear()`,
	},
}, nil, "Thread-safe key-value store for sharing state across requests.\n\nNote: The KV store is in-memory with no size limits. Keys without a TTL persist\nindefinitely. Use TTLs and periodic cleanup to avoid unbounded memory growth.\nExpired entries are cleaned up automatically every 60 seconds.")

// kvCleanupCancel cancels the KV cleanup goroutine
var kvCleanupCancel context.CancelFunc

// startKVCleanup starts the background cleanup goroutine for expired KV entries.
// It cancels any previously running cleanup goroutine first.
func startKVCleanup() {
	if kvCleanupCancel != nil {
		kvCleanupCancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	kvCleanupCancel = cancel

	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				RuntimeState.Lock()
				for key, entry := range RuntimeState.KVData {
					if entry.isExpired() {
						delete(RuntimeState.KVData, key)
					}
				}
				RuntimeState.Unlock()
			}
		}
	}()
}

// StopKVCleanup stops the background KV cleanup goroutine.
func StopKVCleanup() {
	if kvCleanupCancel != nil {
		kvCleanupCancel()
		kvCleanupCancel = nil
	}
}

func init() {
	startKVCleanup()
}
