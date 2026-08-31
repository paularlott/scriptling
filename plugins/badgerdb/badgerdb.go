// Package badgerdb is the BadgerDB embedded key-value plugin. Scripts import
// it as scriptling.badger:
//
//	import scriptling.badger as badger
//	db = badgerdb.open("/var/data/state")
//	db.set("greeting", "hello", ttl_seconds=60)
//	print(db.get("greeting"))
//	db.close()
//
// The API mirrors the valkey plugin exactly, so scripts move between a
// shared cache and local storage unchanged. The directory must fall inside
// the host's allowed paths. The same library serves external plugin mode
// (plugins/badgerdb/cmd) and compiled-in registration (build tag plugin_badgerdb).
//
// Badger allows a single process to open a database; opening the same
// directory twice fails on its lock file, by design.
package badgerdb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	badgerdb "github.com/dgraph-io/badger/v4"

	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/plugin"
	"github.com/paularlott/scriptling/plugins/internal/kv"
)

// Description is the plugin metadata description.
const Description = "BadgerDB embedded key-value store"

// OpenSource is the scriptling-source wrapper for open() in external plugin
// mode, where a Go function cannot return an instance over the wire: it
// constructs the Client class through the plugin object protocol.
const OpenSource = `def open(path):
    return Client(path)
`

const openHelp = `open(path) -> Client

Open (creating if needed) a BadgerDB database directory and return a Client
with the same surface as the valkey plugin. The path must be inside the
host's allowed paths. Badger allows one process to hold a database open at
a time.`

// Build returns the scriptling.badger library. policy is read at call time so an
// external plugin sees the policy its handshake delivered.
func Build(policy plugin.PolicySource) *object.Library {
	clientClass := kv.ClientClass(func(kwargs object.Kwargs, path string) (*kv.Client, error) {
		return open(policy, path)
	}, nil).Build()

	functions := map[string]*object.Builtin{
		"open": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) != 1 {
					return &object.Error{Message: fmt.Sprintf("open takes exactly 1 argument (path), got %d", len(args))}
				}
				path, err := args[0].AsString()
				if err != nil {
					return err
				}
				client, openErr := open(policy, path)
				if openErr != nil {
					return &object.Error{Message: openErr.Error()}
				}
				return object.NewReceiverInstance(clientClass, "Client", client)
			},
			HelpText: openHelp,
		},
	}
	constants := map[string]object.Object{"Client": clientClass}
	return object.NewLibrary(plugin.NormalizeLibraryName("scriptling.badgerdb"), functions, constants, Description)
}

func open(policy plugin.PolicySource, path string) (*kv.Client, error) {
	if path == "" {
		return nil, fmt.Errorf("badger open requires a directory path")
	}
	if !policy.Policy().PathAllowed(path) {
		return nil, fmt.Errorf("path %q is not in the allowed paths", path)
	}
	db, err := badgerdb.Open(badgerdb.DefaultOptions(path).WithLogger(nil))
	if err != nil {
		return nil, fmt.Errorf("open badger: %w", err)
	}
	return &kv.Client{Store: &store{db: db}}, nil
}

// store adapts badger to the shared kv.Store surface. Transactions and
// value-log plumbing stay implementation details; scripts see flat
// get/set/delete operations.
type store struct {
	db *badgerdb.DB
	// Close is not idempotent in badger, and the class __del__ finalizer may
	// run after an explicit close(): close exactly once.
	closeOnce sync.Once
}

func (s *store) Get(ctx context.Context, key string) (string, bool, error) {
	var value []byte
	err := s.db.View(func(txn *badgerdb.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		value, err = item.ValueCopy(nil)
		return err
	})
	if errors.Is(err, badgerdb.ErrKeyNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get %s: %w", key, err)
	}
	if isHashValue(value) {
		return "", false, fmt.Errorf("get %s: %w", key, errWrongType)
	}
	return string(value), true, nil
}

func (s *store) Set(ctx context.Context, key, value string, ttlSeconds int64) error {
	entry := badgerdb.NewEntry([]byte(key), []byte(value))
	if ttlSeconds > 0 {
		entry = entry.WithTTL(time.Duration(ttlSeconds) * time.Second)
	}
	if err := s.db.Update(func(txn *badgerdb.Txn) error {
		// SET replaces whatever the key held, hash included (Redis
		// semantics); only reads and counter operations are type-checked.
		return txn.SetEntry(entry)
	}); err != nil {
		return fmt.Errorf("set %s: %w", key, err)
	}
	return nil
}

func (s *store) Delete(ctx context.Context, keys []string) (int64, error) {
	var removed int64
	err := s.db.Update(func(txn *badgerdb.Txn) error {
		for _, key := range keys {
			if _, err := txn.Get([]byte(key)); errors.Is(err, badgerdb.ErrKeyNotFound) {
				continue
			} else if err != nil {
				return err
			}
			if err := txn.Delete([]byte(key)); err != nil {
				return err
			}
			removed++
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("delete: %w", err)
	}
	return removed, nil
}

func (s *store) Exists(ctx context.Context, keys []string) (int64, error) {
	var count int64
	err := s.db.View(func(txn *badgerdb.Txn) error {
		for _, key := range keys {
			if _, err := txn.Get([]byte(key)); err == nil {
				count++
			} else if !errors.Is(err, badgerdb.ErrKeyNotFound) {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("exists: %w", err)
	}
	return count, nil
}

func (s *store) Expire(ctx context.Context, key string, ttlSeconds int64) (bool, error) {
	found := false
	err := s.db.Update(func(txn *badgerdb.Txn) error {
		item, err := txn.Get([]byte(key))
		if errors.Is(err, badgerdb.ErrKeyNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		value, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		entry := badgerdb.NewEntry([]byte(key), value)
		if ttlSeconds > 0 {
			entry = entry.WithTTL(time.Duration(ttlSeconds) * time.Second)
		}
		if err := txn.SetEntry(entry); err != nil {
			return err
		}
		found = true
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("expire %s: %w", key, err)
	}
	return found, nil
}

func (s *store) TTL(ctx context.Context, key string) (int64, bool, error) {
	var remaining int64
	found := false
	err := s.db.View(func(txn *badgerdb.Txn) error {
		item, err := txn.Get([]byte(key))
		if errors.Is(err, badgerdb.ErrKeyNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		found = true
		if expiresAt := item.ExpiresAt(); expiresAt != 0 {
			remaining = kv.TTLSeconds(time.Until(time.Unix(int64(expiresAt), 0)))
		} else {
			remaining = kv.TTLNoExpiry
		}
		return nil
	})
	if err != nil {
		return 0, false, fmt.Errorf("ttl %s: %w", key, err)
	}
	return remaining, found, nil
}

func (s *store) Incr(ctx context.Context, key string, amount int64) (int64, error) {
	var value int64
	err := s.db.Update(func(txn *badgerdb.Txn) error {
		item, err := txn.Get([]byte(key))
		switch {
		case errors.Is(err, badgerdb.ErrKeyNotFound):
			value = amount
		case err != nil:
			return err
		default:
			raw, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			if isHashValue(raw) {
				return errWrongType
			}
			current, convErr := strconv.ParseInt(string(raw), 10, 64)
			if convErr != nil {
				return fmt.Errorf("incr %s: value is not an integer", key)
			}
			value = current + amount
			// Redis keeps a key's TTL across INCRBY; preserve it here too.
			if expiresAt := item.ExpiresAt(); expiresAt != 0 {
				remaining := time.Until(time.Unix(int64(expiresAt), 0))
				if remaining > 0 {
					return txn.SetEntry(badgerdb.NewEntry([]byte(key), []byte(strconv.FormatInt(value, 10))).WithTTL(remaining))
				}
			}
		}
		return txn.SetEntry(badgerdb.NewEntry([]byte(key), []byte(strconv.FormatInt(value, 10))))
	})
	if err != nil {
		return 0, err
	}
	return value, nil
}

func (s *store) Keys(ctx context.Context, pattern string) ([]string, error) {
	prefix := kv.GlobPrefix(pattern)
	var keys []string
	err := s.db.View(func(txn *badgerdb.Txn) error {
		opts := badgerdb.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Prefix = []byte(prefix)
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Rewind(); it.ValidForPrefix(opts.Prefix); it.Next() {
			key := string(it.Item().KeyCopy(nil))
			if kv.MatchGlob(pattern, key) {
				keys = append(keys, key)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("keys %s: %w", pattern, err)
	}
	if keys == nil {
		keys = []string{}
	}
	return keys, nil
}

func (s *store) Ping(ctx context.Context) error {
	// An embedded store is always reachable once open; verify with a cheap
	// empty iteration.
	return s.db.View(func(txn *badgerdb.Txn) error { return nil })
}

func (s *store) Persist(ctx context.Context, key string) (bool, error) {
	found := false
	err := s.db.Update(func(txn *badgerdb.Txn) error {
		item, err := txn.Get([]byte(key))
		if errors.Is(err, badgerdb.ErrKeyNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		found = true
		if expiresAt := item.ExpiresAt(); expiresAt == 0 {
			return nil // already persistent; no rewrite needed
		}
		value, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		return txn.SetEntry(badgerdb.NewEntry([]byte(key), value))
	})
	if err != nil {
		return false, fmt.Errorf("persist %s: %w", key, err)
	}
	return found, nil
}

func (s *store) MGet(ctx context.Context, keys []string) ([]*string, error) {
	values := make([]*string, len(keys))
	err := s.db.View(func(txn *badgerdb.Txn) error {
		for i, key := range keys {
			item, err := txn.Get([]byte(key))
			if errors.Is(err, badgerdb.ErrKeyNotFound) {
				continue
			}
			if err != nil {
				return err
			}
			raw, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			if isHashValue(raw) {
				return errWrongType
			}
			value := string(raw)
			values[i] = &value
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("mget: %w", err)
	}
	return values, nil
}

func (s *store) MSet(ctx context.Context, mapping map[string]string, ttlSeconds int64) error {
	if len(mapping) == 0 {
		return nil
	}
	err := s.db.Update(func(txn *badgerdb.Txn) error {
		// MSET replaces like SET does, whatever the key held.
		for key, value := range mapping {
			entry := badgerdb.NewEntry([]byte(key), []byte(value))
			if ttlSeconds > 0 {
				entry = entry.WithTTL(time.Duration(ttlSeconds) * time.Second)
			}
			if err := txn.SetEntry(entry); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("mset: %w", err)
	}
	return nil
}

func (s *store) SetNX(ctx context.Context, key, value string, ttlSeconds int64) (bool, error) {
	stored := false
	err := s.db.Update(func(txn *badgerdb.Txn) error {
		if _, err := txn.Get([]byte(key)); err == nil {
			return nil // exists (any kind of value); the write is refused
		} else if !errors.Is(err, badgerdb.ErrKeyNotFound) {
			return err
		}
		entry := badgerdb.NewEntry([]byte(key), []byte(value))
		if ttlSeconds > 0 {
			entry = entry.WithTTL(time.Duration(ttlSeconds) * time.Second)
		}
		if err := txn.SetEntry(entry); err != nil {
			return err
		}
		stored = true
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("set_if_absent %s: %w", key, err)
	}
	return stored, nil
}

// errWrongType is the Redis-style refusal a hash command gets when the key
// holds a plain value (and vice versa), so neither side can silently destroy
// the other's data.
var errWrongType = errors.New("WRONGTYPE: key holds a different kind of value")

// hashValueTag prefixes the stored bytes of every hash, distinguishing a
// stored hash from a plain string that happens to hold JSON. A plain string
// beginning with these bytes would be misread; NUL-prefixed strings are
// pathological enough to accept that residual corner.
const hashValueTag = "\x00hash:"

func isHashValue(raw []byte) bool {
	return bytes.HasPrefix(raw, []byte(hashValueTag))
}

// A hash occupies one badger key holding its fields as a tagged JSON object,
// so the whole key lives or dies together exactly as it does on valkey:
// keys(), exists, delete, expire, ttl and persist see the hash as one key
// with one expiry covering every field, and the key vanishes with its last
// field. Field writes rewrite the object inside one transaction, so same-key
// races conflict-detect rather than interleave, and a remaining expiry
// survives the rewrite like INCR's does.
func hashLoad(txn *badgerdb.Txn, key string) (map[string]string, uint64, error) {
	item, err := txn.Get([]byte(key))
	if errors.Is(err, badgerdb.ErrKeyNotFound) {
		return map[string]string{}, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	raw, err := item.ValueCopy(nil)
	if err != nil {
		return nil, 0, err
	}
	if !isHashValue(raw) {
		return nil, 0, errWrongType
	}
	hash := map[string]string{}
	if json.Unmarshal(raw[len(hashValueTag):], &hash) != nil {
		return nil, 0, fmt.Errorf("hash %s: stored value is corrupt", key)
	}
	return hash, item.ExpiresAt(), nil
}

func hashSave(txn *badgerdb.Txn, key string, hash map[string]string, expiresAt uint64) error {
	if len(hash) == 0 {
		return txn.Delete([]byte(key))
	}
	encoded, err := json.Marshal(hash)
	if err != nil {
		return err
	}
	entry := badgerdb.NewEntry([]byte(key), append([]byte(hashValueTag), encoded...))
	if expiresAt != 0 {
		if remaining := time.Until(time.Unix(int64(expiresAt), 0)); remaining > 0 {
			entry = entry.WithTTL(remaining)
		}
	}
	return txn.SetEntry(entry)
}

func (s *store) HashSet(ctx context.Context, key, field, value string) (int64, error) {
	var added int64
	err := s.db.Update(func(txn *badgerdb.Txn) error {
		hash, expiresAt, err := hashLoad(txn, key)
		if err != nil {
			return err
		}
		if _, exists := hash[field]; !exists {
			added = 1
		}
		hash[field] = value
		return hashSave(txn, key, hash, expiresAt)
	})
	if err != nil {
		return 0, fmt.Errorf("hash_set %s: %w", key, err)
	}
	return added, nil
}

func (s *store) HashGet(ctx context.Context, key, field string) (*string, error) {
	var value *string
	err := s.db.View(func(txn *badgerdb.Txn) error {
		hash, _, err := hashLoad(txn, key)
		if err != nil {
			return err
		}
		if v, ok := hash[field]; ok {
			value = &v
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("hash_get %s: %w", key, err)
	}
	return value, nil
}

func (s *store) HashDelete(ctx context.Context, key string, fields []string) (int64, error) {
	var removed int64
	err := s.db.Update(func(txn *badgerdb.Txn) error {
		hash, expiresAt, err := hashLoad(txn, key)
		if err != nil {
			return err
		}
		for _, field := range fields {
			if _, ok := hash[field]; ok {
				delete(hash, field)
				removed++
			}
		}
		return hashSave(txn, key, hash, expiresAt)
	})
	if err != nil {
		return 0, fmt.Errorf("hash_delete %s: %w", key, err)
	}
	return removed, nil
}

func (s *store) HashAll(ctx context.Context, key string) (map[string]string, error) {
	var hash map[string]string
	err := s.db.View(func(txn *badgerdb.Txn) error {
		loaded, _, err := hashLoad(txn, key)
		if err != nil {
			return err
		}
		hash = loaded
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("hash_all %s: %w", key, err)
	}
	return hash, nil
}

func (s *store) HashSize(ctx context.Context, key string) (int64, error) {
	var size int64
	err := s.db.View(func(txn *badgerdb.Txn) error {
		hash, _, err := hashLoad(txn, key)
		if err != nil {
			return err
		}
		size = int64(len(hash))
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("hash_size %s: %w", key, err)
	}
	return size, nil
}

func (s *store) Close() error {
	var err error
	s.closeOnce.Do(func() { err = s.db.Close() })
	return err
}
