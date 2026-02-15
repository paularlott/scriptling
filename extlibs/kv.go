package extlibs

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync"
	"time"

	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

const KVLibraryName = "scriptling.kv"

// kvEntry represents a stored value with optional TTL
type kvEntry struct {
	value     interface{}
	expiresAt time.Time // zero time means no expiration
}

// kvStore is the global key-value store
var kvStore = struct {
	sync.RWMutex
	data map[string]*kvEntry
}{
	data: make(map[string]*kvEntry),
}

// deepCopy creates a deep copy of basic types (string, int, float, bool, list, dict)
// This ensures thread safety - callers can modify returned values without affecting the store
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
// Uses the conversion package for consistency
func convertObjectToKVValue(obj object.Object) (interface{}, *object.Error) {
	return conversion.ToGoWithError(obj)
}

// convertKVValueToObject converts a storable basic type back to a scriptling Object
// Uses the conversion package for consistency
func convertKVValueToObject(v interface{}) object.Object {
	return conversion.FromGo(v)
}

// isExpired checks if an entry has expired
func (e *kvEntry) isExpired() bool {
	if e.expiresAt.IsZero() {
		return false
	}
	return time.Now().After(e.expiresAt)
}

func RegisterKVLibrary(registrar interface{ RegisterLibrary(*object.Library) }) {
	registrar.RegisterLibrary(KVLibrary)
}

var KVLibrary = object.NewLibrary(KVLibraryName, map[string]*object.Builtin{
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

			// Check for TTL in kwargs or third argument
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

			kvStore.Lock()
			kvStore.data[key] = entry
			kvStore.Unlock()

			return &object.Null{}
		},
		HelpText: `set(key, value, ttl=0) - Store a value with optional TTL in seconds

Parameters:
  key (string): The key to store the value under
  value: The value to store (string, int, float, bool, list, dict)
  ttl (int, optional): Time-to-live in seconds. 0 means no expiration.

Example:
  scriptling.kv.set("api_key", "secret123")
  scriptling.kv.set("session:abc", {"user": "bob"}, ttl=3600)`,
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

			// Check for default value
			var defaultValue object.Object = &object.Null{}
			if d := kwargs.Get("default"); d != nil {
				defaultValue = d
			} else if len(args) > 1 {
				defaultValue = args[1]
			}

			kvStore.RLock()
			entry, exists := kvStore.data[key]
			kvStore.RUnlock()

			if !exists || entry.isExpired() {
				return defaultValue
			}

			// Return a deep copy to ensure thread safety
			return convertKVValueToObject(deepCopy(entry.value))
		},
		HelpText: `get(key, default=None) - Retrieve a value by key

Parameters:
  key (string): The key to retrieve
  default: Value to return if key doesn't exist (default: None)

Returns:
  The stored value (deep copy), or the default if not found

Example:
  value = scriptling.kv.get("api_key")
  count = scriptling.kv.get("counter", default=0)`,
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

			kvStore.Lock()
			delete(kvStore.data, key)
			kvStore.Unlock()

			return &object.Null{}
		},
		HelpText: `delete(key) - Remove a key from the store

Parameters:
  key (string): The key to delete

Example:
  scriptling.kv.delete("session:abc")`,
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

			kvStore.RLock()
			entry, exists := kvStore.data[key]
			kvStore.RUnlock()

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
  if scriptling.kv.exists("config"):
      config = scriptling.kv.get("config")`,
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

			// Get increment amount
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

			kvStore.Lock()
			defer kvStore.Unlock()

			entry, exists := kvStore.data[key]
			if !exists || entry.isExpired() {
				// Key doesn't exist, initialize with amount
				kvStore.data[key] = &kvEntry{value: amount}
				return object.NewInteger(amount)
			}

			// Check existing value is an integer
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
  scriptling.kv.set("counter", 0)
  scriptling.kv.incr("counter")      # returns 1
  scriptling.kv.incr("counter", 5)   # returns 6`,
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

			kvStore.RLock()
			entry, exists := kvStore.data[key]
			kvStore.RUnlock()

			if !exists || entry.isExpired() {
				return object.NewInteger(-2) // Key does not exist
			}

			if entry.expiresAt.IsZero() {
				return object.NewInteger(-1) // No expiration
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
  scriptling.kv.set("session", "data", ttl=3600)
  remaining = scriptling.kv.ttl("session")  # e.g., 3599`,
	},

	"keys": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// Get optional pattern
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

			kvStore.RLock()
			defer kvStore.RUnlock()

			var keys []object.Object
			for key, entry := range kvStore.data {
				// Skip expired entries
				if entry.isExpired() {
					continue
				}

				// Match pattern
				if pattern == "*" {
					keys = append(keys, &object.String{Value: key})
				} else {
					// Simple glob matching
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
  all_keys = scriptling.kv.keys()
  user_keys = scriptling.kv.keys("user:*")
  session_keys = scriptling.kv.keys("session:*")`,
	},

	"clear": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			kvStore.Lock()
			kvStore.data = make(map[string]*kvEntry)
			kvStore.Unlock()

			return &object.Null{}
		},
		HelpText: `clear() - Remove all keys from the store

Warning: This operation cannot be undone.

Example:
  scriptling.kv.clear()`,
	},
}, nil, "Thread-safe key-value store for sharing state across requests")

// ClearExpired removes all expired entries from the store
// This can be called periodically for cleanup
func ClearExpired() {
	kvStore.Lock()
	defer kvStore.Unlock()

	for key, entry := range kvStore.data {
		if entry.isExpired() {
			delete(kvStore.data, key)
		}
	}
}

// ExportStore exports the current store state as JSON
// Useful for debugging or persistence
func ExportStore() (string, error) {
	kvStore.RLock()
	defer kvStore.RUnlock()

	export := make(map[string]interface{})
	for key, entry := range kvStore.data {
		if !entry.isExpired() {
			export[key] = map[string]interface{}{
				"value":     entry.value,
				"expiresAt": entry.expiresAt,
			}
		}
	}

	data, err := json.Marshal(export)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ImportStore imports store state from JSON
// This replaces the current store
func ImportStore(jsonData string) error {
	var export map[string]json.RawMessage
	if err := json.Unmarshal([]byte(jsonData), &export); err != nil {
		return err
	}

	kvStore.Lock()
	defer kvStore.Unlock()

	kvStore.data = make(map[string]*kvEntry)

	for key, rawValue := range export {
		var entryData struct {
			Value     interface{} `json:"value"`
			ExpiresAt time.Time   `json:"expiresAt"`
		}
		if err := json.Unmarshal(rawValue, &entryData); err != nil {
			continue // Skip invalid entries
		}
		kvStore.data[key] = &kvEntry{
			value:     entryData.Value,
			expiresAt: entryData.ExpiresAt,
		}
	}

	return nil
}

// init registers cleanup goroutine
func init() {
	// Periodic cleanup of expired entries every minute
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			ClearExpired()
		}
	}()
}
