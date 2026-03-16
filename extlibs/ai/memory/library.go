package memory

import (
	"context"
	"sync"
	"time"

	"github.com/paularlott/logger"
	"github.com/paularlott/snapshotkv"
	extai "github.com/paularlott/scriptling/extlibs/ai"
	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/object"
)

const MemoryLibraryName = "scriptling.ai.memory"

// storeRegistry maps a DB pointer to its single Store instance for this process.
var storeRegistry = struct {
	mu     sync.Mutex
	stores map[*snapshotkv.DB]*Store
}{
	stores: make(map[*snapshotkv.DB]*Store),
}

func getOrCreateStore(db *snapshotkv.DB, opts []Option) *Store {
	storeRegistry.mu.Lock()
	defer storeRegistry.mu.Unlock()
	if s, ok := storeRegistry.stores[db]; ok {
		// Apply any opts that update mutable config (e.g. AI client added after first creation)
		for _, o := range opts {
			o(&s.cfg)
		}
		return s
	}
	s := New(db, opts...)
	storeRegistry.stores[db] = s
	return s
}

// Register registers the scriptling.ai.memory library.
func Register(registrar interface{ RegisterLibrary(*object.Library) }, log ...logger.Logger) {
	var l logger.Logger
	if len(log) > 0 {
		l = log[0]
	}
	registrar.RegisterLibrary(buildLibrary(l))
}

func buildLibrary(log logger.Logger) *object.Library {
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

		var opts []Option

		if log != nil {
			opts = append(opts, WithLogger(log))
		}

		// ai_client kwarg (or positional arg[1]) for Mode 2 compaction
		var clientObj object.Object
		if c := kwargs.Get("ai_client"); c != nil {
			clientObj = c
		} else if len(args) > 1 {
			clientObj = args[1]
		}
		if client := extai.AIClientFromObject(clientObj); client != nil {
			model := kwargs.MustGetString("model", "")
			opts = append(opts, WithAIClient(client, model))
		}

		store := getOrCreateStore(db, opts)
		return newMemoryObject(store)
	}, `new(kv_store, ai_client=None, model="") - Create a memory store backed by a kv store

Parameters:
  kv_store: A kv store object (e.g. kv.default or kv.open(...))
  ai_client (optional): An ai.Client instance to enable Mode 2 LLM compaction
  model (str, optional): Model name to use for LLM compaction (required if ai_client provided)

Returns:
  Memory store object with remember, recall, forget, list, count, compact methods

Example:
  import scriptling.runtime.kv as kv
  import scriptling.ai.memory as memory
  import scriptling.ai as ai

  mem = memory.new(kv.default)
  mem.remember("User's name is Alice", type="fact", importance=0.9)

  # With LLM compaction (Mode 2)
  client = ai.Client("http://127.0.0.1:1234/v1")
  mem = memory.new(kv.default, ai_client=client, model="qwen3-8b")

  # ai_client=None disables Mode 2 (same as omitting it)
  mem = memory.new(kv.default, ai_client=None)`)

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
  importance (float, optional): 0.0-1.0 (default: 0.5)

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
				HelpText: `recall(query="", limit=10, type="") - Search memories by keyword and semantic similarity

Parameters:
  query (str, optional): Keyword search query; empty returns memories ranked by recency/importance
  limit (int, optional): Maximum results (default: 10, use -1 for unlimited)
  type (str, optional): Filter by type: "fact", "preference", "event", "note", or "!type" to exclude

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

			"count": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					return object.NewInteger(int64(store.Count()))
				},
				HelpText: `count() - Return the total number of stored memories`,
			},

			"compact": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					before := store.Count()
					remaining := store.Compact()
					return &object.Dict{Pairs: map[string]object.DictPair{
						"removed":   {Key: &object.String{Value: "removed"}, Value: object.NewInteger(int64(before - remaining))},
						"remaining": {Key: &object.String{Value: "remaining"}, Value: object.NewInteger(int64(remaining))},
					}}
				},
				HelpText: `compact() - Manually trigger compaction; returns removed and remaining counts`,
			},

		},
		HelpText: "Memory store object — call .remember(), .recall(), .forget(), .count(), .compact()",
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
