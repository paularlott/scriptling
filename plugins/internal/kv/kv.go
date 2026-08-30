// Package kv provides the shared key/value client class used by the valkey
// and badgerdb plugins. Both plugins expose the identical surface — the only
// difference is the factory: valkey.connect(url) reaches a server over the
// network, badgerdb.open(path) opens an embedded store — so scripts can move
// between a distributed cache and local storage without changes.
package kv

import (
	"context"
	"fmt"
	"path"
	"time"

	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/plugins/internal/kwarg"
)

// TTLNoExpiry is what ttl() reports for a key that exists but never expires
// (redis semantics: -1). A missing key reports null.
const TTLNoExpiry = int64(-1)

// Store is the backend contract both plugins implement.
type Store interface {
	Get(ctx context.Context, key string) (string, bool, error)
	Set(ctx context.Context, key, value string, ttlSeconds int64) error
	Delete(ctx context.Context, keys []string) (int64, error)
	Exists(ctx context.Context, keys []string) (int64, error)
	Expire(ctx context.Context, key string, ttlSeconds int64) (bool, error)
	Persist(ctx context.Context, key string) (bool, error)
	TTL(ctx context.Context, key string) (int64, bool, error)
	Incr(ctx context.Context, key string, amount int64) (int64, error)
	Keys(ctx context.Context, pattern string) ([]string, error)
	MGet(ctx context.Context, keys []string) ([]*string, error)
	MSet(ctx context.Context, mapping map[string]string, ttlSeconds int64) error
	SetNX(ctx context.Context, key, value string, ttlSeconds int64) (bool, error)
	HashSet(ctx context.Context, key, field, value string) (int64, error)
	HashGet(ctx context.Context, key, field string) (*string, error)
	HashDelete(ctx context.Context, key string, fields []string) (int64, error)
	HashAll(ctx context.Context, key string) (map[string]string, error)
	HashSize(ctx context.Context, key string) (int64, error)
	Ping(ctx context.Context) error
	Close() error
}

// Client is the typed receiver wrapped by the class.
type Client struct {
	Store Store
}

// ClientClass builds the shared class. constructor is the plugin-specific
// typed constructor (valkey's dials a server, badger's opens a directory);
// it must return (*Client, error). Scripts construct the class directly or
// through each plugin's connect()/open() helper. extras, when non-nil,
// registers backend-specific methods beyond the mirrored core (valkey's
// sets and queues); backends without those types pass nil and the mirror
// stays exact.
func ClientClass(constructor interface{}, extras func(*object.ClassBuilder)) *object.ClassBuilder {
	cb := object.NewClassBuilder("Client")
	cb.Constructor(constructor)
	if extras != nil {
		extras(cb)
	}
	cb.MethodWithHelp("get", func(self *Client, ctx context.Context, key string) (any, error) {
		value, exists, err := self.Store.Get(ctx, key)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, nil
		}
		return value, nil
	}, `get(key) -> str | null

Return the value stored at key, or null when the key does not exist.`)
	cb.MethodWithHelp("set", func(self *Client, ctx context.Context, kwargs object.Kwargs, key string, value string) error {
		ttlSeconds, errObj := kwargs.GetInt("ttl_seconds", 0)
		if errObj != nil {
			return kwarg.Err(errObj)
		}
		return self.Store.Set(ctx, key, value, ttlSeconds)
	}, `set(key, value, ttl_seconds=0) - Store a string value.

A ttl_seconds of 0 (the default) stores the key without expiry.`)
	cb.MethodWithHelp("set_if_absent", func(self *Client, ctx context.Context, kwargs object.Kwargs, key, value string) (bool, error) {
		ttlSeconds, errObj := kwargs.GetInt("ttl_seconds", 0)
		if errObj != nil {
			return false, kwarg.Err(errObj)
		}
		return self.Store.SetNX(ctx, key, value, ttlSeconds)
	}, `set_if_absent(key, value, ttl_seconds=0) -> bool

Store a value only when the key does not exist, returning whether it was
stored. The take-once primitive behind locks and once-only actions.`)
	cb.MethodWithHelp("delete", func(self *Client, ctx context.Context, keys ...string) (int64, error) {
		return self.Store.Delete(ctx, keys)
	}, "delete(*keys) -> int - Delete keys, returning how many existed.")
	cb.MethodWithHelp("exists", func(self *Client, ctx context.Context, keys ...string) (int64, error) {
		return self.Store.Exists(ctx, keys)
	}, "exists(*keys) -> int - Return how many of the keys exist.")
	cb.MethodWithHelp("expire", func(self *Client, ctx context.Context, key string, ttlSeconds int64) (bool, error) {
		return self.Store.Expire(ctx, key, ttlSeconds)
	}, "expire(key, ttl_seconds) -> bool - Set a key's time to live. False when the key is missing.")
	cb.MethodWithHelp("ttl", func(self *Client, ctx context.Context, key string) (any, error) {
		remaining, exists, err := self.Store.TTL(ctx, key)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, nil
		}
		return remaining, nil
	}, `ttl(key) -> int | null

Remaining seconds before the key expires. null when the key is missing,
-1 when the key exists but has expiry.`)
	cb.MethodWithHelp("persist", func(self *Client, ctx context.Context, key string) (bool, error) {
		return self.Store.Persist(ctx, key)
	}, "persist(key) -> bool - Remove a key's expiry so it lives forever. False when the key is missing.")
	cb.MethodWithHelp("mget", func(self *Client, ctx context.Context, keys ...string) ([]any, error) {
		values, err := self.Store.MGet(ctx, keys)
		if err != nil {
			return nil, err
		}
		out := make([]any, len(values))
		for i, value := range values {
			if value != nil {
				out[i] = *value
			}
		}
		return out, nil
	}, "mget(*keys) -> list - Values for the keys in one call, in order; null where a key is missing.")
	cb.MethodWithHelp("mset", func(self *Client, ctx context.Context, kwargs object.Kwargs, mapping map[string]string) error {
		ttlSeconds, errObj := kwargs.GetInt("ttl_seconds", 0)
		if errObj != nil {
			return kwarg.Err(errObj)
		}
		return self.Store.MSet(ctx, mapping, ttlSeconds)
	}, "mset(mapping, ttl_seconds=0) - Store every entry of the dict in one call.")
	cb.MethodWithHelp("incr", func(self *Client, ctx context.Context, kwargs object.Kwargs, key string) (int64, error) {
		amount, errObj := kwargs.GetInt("amount", 1)
		if errObj != nil {
			return 0, kwarg.Err(errObj)
		}
		return self.Store.Incr(ctx, key, amount)
	}, "incr(key, amount=1) -> int - Add amount to the integer stored at key, returning the new value.")
	cb.MethodWithHelp("decr", func(self *Client, ctx context.Context, kwargs object.Kwargs, key string) (int64, error) {
		amount, errObj := kwargs.GetInt("amount", 1)
		if errObj != nil {
			return 0, kwarg.Err(errObj)
		}
		return self.Store.Incr(ctx, key, -amount)
	}, "decr(key, amount=1) -> int - Subtract amount from the integer stored at key.")
	cb.MethodWithHelp("hash_set", func(self *Client, ctx context.Context, key, field, value string) (int64, error) {
		return self.Store.HashSet(ctx, key, field, value)
	}, "hash_set(key, field, value) -> int - Set one field, returning 1 when the field was new, 0 when it overwrote.")
	cb.MethodWithHelp("hash_get", func(self *Client, ctx context.Context, key, field string) (any, error) {
		value, err := self.Store.HashGet(ctx, key, field)
		if err != nil {
			return nil, err
		}
		if value == nil {
			return nil, nil
		}
		return *value, nil
	}, "hash_get(key, field) -> str | null - The field's value, or null when the key or field is missing.")
	cb.MethodWithHelp("hash_delete", func(self *Client, ctx context.Context, key string, fields ...string) (int64, error) {
		return self.Store.HashDelete(ctx, key, fields)
	}, `hash_delete(key, *fields) -> int - Delete fields, returning how many existed.

The hash key disappears with its last field, expiry included.`)
	cb.MethodWithHelp("hash_all", func(self *Client, ctx context.Context, key string) (map[string]any, error) {
		hash, err := self.Store.HashAll(ctx, key)
		if err != nil {
			return nil, err
		}
		out := make(map[string]any, len(hash))
		for field, value := range hash {
			out[field] = value
		}
		return out, nil
	}, "hash_all(key) -> dict - Every field and value; an empty dict when the key is missing.")
	cb.MethodWithHelp("hash_size", func(self *Client, ctx context.Context, key string) (int64, error) {
		return self.Store.HashSize(ctx, key)
	}, "hash_size(key) -> int - How many fields the hash holds. 0 when the key is missing.")
	cb.MethodWithHelp("keys", func(self *Client, ctx context.Context, pattern string) ([]any, error) {
		matched, err := self.Store.Keys(ctx, pattern)
		if err != nil {
			return nil, err
		}
		keys := make([]any, 0, len(matched))
		for _, key := range matched {
			keys = append(keys, key)
		}
		return keys, nil
	}, `keys(pattern) -> list[str]

Keys matching a glob pattern (* and ?). Matches every key when the pattern
is "*".`)
	cb.MethodWithHelp("ping", func(self *Client, ctx context.Context) error {
		return self.Store.Ping(ctx)
	}, "ping() - Check the store is reachable, raising on failure.")
	cb.MethodWithHelp("close", func(self *Client) error {
		return self.Store.Close()
	}, "close() - Close the client and release its resources.")
	// Safety net mirroring relational.Connection: a handler or background
	// task that opens a client per run and goes away without close()
	// releases the handle when the instance is collected. For badgerdb that
	// matters doubly: the store holds an exclusive lock on its directory,
	// and a leaked handle blocks every later open() of that path. Store
	// Close is idempotent, so finalizer plus explicit close is safe.
	cb.Method("__del__", func(self *Client) {
		_ = self.Store.Close()
	})
	return cb
}

// MatchGlob reports whether key matches a redis-style glob pattern using the
// host's path.Match syntax (*, ?, [class]) — the same wildcard language for
// both backends.
func MatchGlob(pattern, key string) bool {
	matched, err := path.Match(pattern, key)
	return err == nil && matched
}

// GlobPrefix returns the literal prefix of a glob pattern (everything before
// the first wildcard character); backends use it to bound iteration.
func GlobPrefix(pattern string) string {
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*', '?', '[':
			return pattern[:i]
		case '\\':
			i++ // escaped wildcard — skip the next byte
		}
	}
	return pattern
}

// FormatTTLError normalises backend TTL probing for error messages.
func FormatTTLError(op string, err error) error {
	return fmt.Errorf("%s: %w", op, err)
}

// TTLSeconds converts a remaining-time duration to whole seconds, rounding
// up so a key that expires "now" still reports 1s rather than vanishing
// mid-report.
func TTLSeconds(remaining time.Duration) int64 {
	if remaining < 0 {
		return 0
	}
	return int64((remaining + time.Second - 1) / time.Second)
}
