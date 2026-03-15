package memory

import (
	"context"
	"time"

	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/object"
)

const MemoryLibraryName = "scriptling.ai.memory"

// Register registers the scriptling.ai.memory library.
func Register(registrar interface{ RegisterLibrary(*object.Library) }) {
	registrar.RegisterLibrary(buildLibrary())
}

func buildLibrary() *object.Library {
	builder := object.NewLibraryBuilder(MemoryLibraryName,
		"Long-term memory store for AI agents. Pass a kv store object to memory.new() to create a memory store.")

	builder.FunctionWithHelp("new", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		if objErr := errors.MinArgs(args, 1); objErr != nil {
			return objErr
		}

		// Extract the underlying snapshotkv.DB via the package-level registry
		db := extlibs.KVStoreDB(args[0])
		if db == nil {
			return errors.NewError("memory.new: argument must be a kv store object (e.g. kv.default or kv.open(...))")
		}

		idleHours := kwargs.MustGetFloat("idle_timeout", 24)
		if len(args) > 1 {
			if v, err := args[1].CoerceFloat(); err == nil {
				idleHours = v
			}
		}

		var idleTimeout time.Duration
		if idleHours > 0 {
			idleTimeout = time.Duration(float64(time.Hour) * idleHours)
		}

		store := New(db, idleTimeout)
		return newMemoryObject(store)
	}, `new(kv_store, idle_timeout=24) - Create a memory store backed by a kv store

Parameters:
  kv_store: A kv store object (e.g. kv.default or kv.open(...))
  idle_timeout (float, optional): Hours before unaccessed memories are compacted (default: 24, 0 = disabled)

Returns:
  Memory store object with remember, recall, forget, list, count, compact, close methods

Example:
  import scriptling.runtime.kv as kv
  import scriptling.ai.memory as memory

  mem = memory.new(kv.default)
  mem.remember("User's name is Alice", type="fact", importance=0.9)

  # With a dedicated persistent store
  db = kv.open("/data/agent.db")
  mem = memory.new(db, idle_timeout=48)`)

	return builder.Build()
}

// newMemoryObject wraps a Store as a Scriptling Builtin object.
func newMemoryObject(store *Store) *object.Builtin {
	return &object.Builtin{
		Attributes: map[string]object.Object{

			"remember": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if objErr := errors.MinArgs(args, 1); objErr != nil {
						return objErr
					}
					content, objErr := args[0].AsString()
					if objErr != nil {
						return objErr
					}

					memType := kwargs.MustGetString("type", TypeNote)
					importance := kwargs.MustGetFloat("importance", 0.5)

					if len(args) > 1 {
						if v, err := args[1].AsString(); err == nil {
							memType = v
						}
					}
					if len(args) > 2 {
						if v, err := args[2].CoerceFloat(); err == nil {
							importance = v
						}
					}

					m, err := store.Remember(content, memType, importance)
					if err != nil {
						return errors.NewError("memory.remember: %v", err)
					}
					return memoryToDict(m)
				},
				HelpText: `remember(content, type="note", importance=0.5) - Store a memory

Parameters:
  content (str): What to remember
  type (str, optional): "fact", "preference", "event", or "note" (default: "note")
  importance (float, optional): 0.0-1.0; memories >= 0.8 are exempt from compaction (default: 0.5)

Returns:
  dict: The stored memory with id, content, type, importance, created_at, accessed_at`,
			},

			"recall": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					query := kwargs.MustGetString("query", "")
					limit := int(kwargs.MustGetInt("limit", 10))
					typeFilter := kwargs.MustGetString("type", "")

					if len(args) > 0 {
						if v, err := args[0].AsString(); err == nil {
							query = v
						}
					}
					if len(args) > 1 {
						if v, err := args[1].AsInt(); err == nil {
							limit = int(v)
						}
					}

					memories := store.Recall(query, limit, typeFilter)
					elems := make([]object.Object, 0, len(memories))
					for _, m := range memories {
						elems = append(elems, memoryToDict(m))
					}
					return &object.List{Elements: elems}
				},
				HelpText: `recall(query="", limit=10, type="") - Search memories by keyword

Parameters:
  query (str, optional): Keyword search query against memory content; empty returns memories ranked by recency/importance
  limit (int, optional): Maximum results to return (default: 10)
  type (str, optional): Filter by type: "fact", "preference", "event", "note"

Returns:
  list: Matching memory dicts ranked by relevance`,
			},

			"forget": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if objErr := errors.MinArgs(args, 1); objErr != nil {
						return objErr
					}
					id, objErr := args[0].AsString()
					if objErr != nil {
						return objErr
					}
					return &object.Boolean{Value: store.Forget(id)}
				},
				HelpText: `forget(id) - Remove a memory by ID

Parameters:
  id (str): Memory ID returned by remember()

Returns:
  bool: True if a memory was removed`,
			},

			"list": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					typeFilter := kwargs.MustGetString("type", "")
					limit := int(kwargs.MustGetInt("limit", 50))

					if len(args) > 0 {
						if v, err := args[0].AsString(); err == nil {
							typeFilter = v
						}
					}
					if len(args) > 1 {
						if v, err := args[1].AsInt(); err == nil {
							limit = int(v)
						}
					}

					memories := store.List(typeFilter, limit)
					elems := make([]object.Object, 0, len(memories))
					for _, m := range memories {
						elems = append(elems, memoryToDict(m))
					}
					return &object.List{Elements: elems}
				},
				HelpText: `list(type="", limit=50) - List stored memories

Parameters:
  type (str, optional): Filter by type: "fact", "preference", "event", "note"
  limit (int, optional): Maximum results (default: 50)

Returns:
  list: Memory dicts`,
			},

			"count": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					return object.NewInteger(int64(store.Count()))
				},
				HelpText: `count() - Return the total number of stored memories`,
			},

			"compact": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					idleHours := kwargs.MustGetFloat("idle_timeout", 24)
					exemptThreshold := kwargs.MustGetFloat("exempt_threshold", 0.8)

					if len(args) > 0 {
						if v, err := args[0].CoerceFloat(); err == nil {
							idleHours = v
						}
					}
					if len(args) > 1 {
						if v, err := args[1].CoerceFloat(); err == nil {
							exemptThreshold = v
						}
					}

					removed := store.Compact(time.Duration(float64(time.Hour)*idleHours), exemptThreshold)
					return object.NewInteger(int64(removed))
				},
				HelpText: `compact(idle_timeout=24, exempt_threshold=0.8) - Manually trigger compaction

Parameters:
  idle_timeout (float, optional): Remove memories not accessed in this many hours (default: 24)
  exempt_threshold (float, optional): Memories with importance >= this are kept (default: 0.8)

Returns:
  int: Number of memories removed`,
			},

			"close": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					store.Close()
					return &object.Null{}
				},
				HelpText: `close() - Stop the background compaction goroutine`,
			},
		},
		HelpText: "Memory store object — call .remember(), .recall(), .forget(), .list(), .count(), .compact(), .close()",
	}
}

// memoryToDict converts a Memory to a Scriptling dict.
func memoryToDict(m *Memory) *object.Dict {
	d := &object.Dict{Pairs: make(map[string]object.DictPair)}
	d.SetByString("id", &object.String{Value: m.ID})
	d.SetByString("content", &object.String{Value: m.Content})
	d.SetByString("type", &object.String{Value: m.Type})
	d.SetByString("importance", &object.Float{Value: m.Importance})
	d.SetByString("created_at", conversion.FromGo(m.CreatedAt.Format(time.RFC3339)))
	d.SetByString("accessed_at", conversion.FromGo(m.AccessedAt.Format(time.RFC3339)))
	return d
}
