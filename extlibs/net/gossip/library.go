package gossip

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/paularlott/gossip"
	"github.com/paularlott/gossip/codec"
	"github.com/paularlott/gossip/compression"
	"github.com/paularlott/gossip/encryption"
	"github.com/paularlott/logger"
	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/evaliface"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/object"
)

const (
	LibraryName = "scriptling.net.gossip"
	LibraryDesc = "Gossip protocol cluster membership and messaging"
)

var (
	library     *object.Library
	libraryOnce sync.Once
	clusters    = struct {
		sync.RWMutex
		m map[string]*gossip.Cluster
	}{m: make(map[string]*gossip.Cluster)}
)

func nodeToObject(n *gossip.Node) *object.Dict {
	state := "unknown"
	switch n.GetObservedState() {
	case gossip.NodeAlive:
		state = "alive"
	case gossip.NodeSuspect:
		state = "suspect"
	case gossip.NodeDead:
		state = "dead"
	case gossip.NodeLeaving:
		state = "leaving"
	}

	md := n.Metadata.GetAllAsString()
	mdPairs := make(map[string]object.Object, len(md))
	for k, v := range md {
		mdPairs[k] = &object.String{Value: v}
	}

	return object.NewStringDict(map[string]object.Object{
		"id":        &object.String{Value: n.ID.String()},
		"addr":      &object.String{Value: n.AdvertisedAddr()},
		"state":     &object.String{Value: state},
		"metadata":  object.NewStringDict(mdPairs),
	})
}

func nodesToList(nodes []*gossip.Node) *object.List {
	elements := make([]object.Object, len(nodes))
	for i, n := range nodes {
		elements[i] = nodeToObject(n)
	}
	return &object.List{Elements: elements}
}

func buildClusterObject(c *gossip.Cluster, clusterID string, eval evaliface.Evaluator, env *object.Environment) *object.Builtin {
	return &object.Builtin{
		Attributes: map[string]object.Object{
			"start": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					c.Start()
					return &object.Null{}
				},
				HelpText: `start() - Start the cluster node

Starts transport, health monitoring, and gossip routines.`,
			},
			"join": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.MinArgs(args, 1); err != nil {
						return err
					}

					var peers []string
					if list, ok := args[0].(*object.List); ok {
						for _, elem := range list.Elements {
							if s, e := elem.AsString(); e == nil {
								peers = append(peers, s)
							}
						}
					} else if s, e := args[0].AsString(); e == nil {
						peers = []string{s}
					} else {
						return errors.NewError("peers must be a string or list of strings")
					}

					if joinErr := c.Join(peers); joinErr != nil {
						return errors.NewError("join failed: %s", joinErr.Error())
					}
					return &object.Null{}
				},
				HelpText: `join(peers) - Join an existing cluster

Parameters:
  peers (string or list): One or more peer addresses to join`,
			},
			"leave": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					c.Leave()
					return &object.Null{}
				},
				HelpText: `leave() - Gracefully leave the cluster`,
			},
			"stop": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					c.Stop()
					clusters.Lock()
					delete(clusters.m, clusterID)
					clusters.Unlock()
					return &object.Null{}
				},
				HelpText: `stop() - Stop the cluster and clean up resources`,
			},
			"send": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.MinArgs(args, 2); err != nil {
						return err
					}
					msgType, mtErr := args[0].AsInt()
					if mtErr != nil {
						return errors.NewError("message_type must be an integer (>= 128)")
					}
					if msgType < 128 {
						return errors.NewError("message_type must be >= 128 (user messages)")
					}

					payload := conversion.ToGo(args[1])
					reliable := false
					if r := kwargs.Get("reliable"); r != nil {
						if b, e := r.AsBool(); e == nil {
							reliable = b
						}
					}

					var sendErr error
					if reliable {
						sendErr = c.SendReliable(gossip.MessageType(msgType), payload)
					} else {
						sendErr = c.Send(gossip.MessageType(msgType), payload)
					}
					if sendErr != nil {
						return errors.NewError("send failed: %s", sendErr.Error())
					}
					return &object.Null{}
				},
				HelpText: `send(message_type, data, reliable=False) - Broadcast a message to the cluster

Parameters:
  message_type (int): Message type (must be >= 128)
  data: Message payload (string, int, float, list, dict)
  reliable (bool): Use reliable transport (TCP) (default: False)`,
			},
			"send_tagged": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.MinArgs(args, 3); err != nil {
						return err
					}
					tag, tagErr := args[0].AsString()
					if tagErr != nil {
						return tagErr
					}
					msgType, mtErr := args[1].AsInt()
					if mtErr != nil {
						return errors.NewError("message_type must be an integer (>= 128)")
					}
					if msgType < 128 {
						return errors.NewError("message_type must be >= 128 (user messages)")
					}

					payload := conversion.ToGo(args[2])
					reliable := false
					if r := kwargs.Get("reliable"); r != nil {
						if b, e := r.AsBool(); e == nil {
							reliable = b
						}
					}

					var sendErr error
					if reliable {
						sendErr = c.SendTaggedReliable(tag, gossip.MessageType(msgType), payload)
					} else {
						sendErr = c.SendTagged(tag, gossip.MessageType(msgType), payload)
					}
					if sendErr != nil {
						return errors.NewError("send_tagged failed: %s", sendErr.Error())
					}
					return &object.Null{}
				},
				HelpText: `send_tagged(tag, message_type, data, reliable=False) - Send a tagged message

Parameters:
  tag (string): Tag for routing (only delivered to nodes with this tag)
  message_type (int): Message type (must be >= 128)
  data: Message payload
  reliable (bool): Use reliable transport (default: False)`,
			},
			"send_to": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.MinArgs(args, 3); err != nil {
						return err
					}

					nodeIDStr, idErr := args[0].AsString()
					if idErr != nil {
						return idErr
					}

					msgType, mtErr := args[1].AsInt()
					if mtErr != nil {
						return errors.NewError("message_type must be an integer (>= 128)")
					}
					if msgType < 128 {
						return errors.NewError("message_type must be >= 128 (user messages)")
					}

					node := c.GetNodeByIDString(nodeIDStr)
					if node == nil {
						return errors.NewError("node not found: %s", nodeIDStr)
					}

					payload := conversion.ToGo(args[2])
					reliable := false
					if r := kwargs.Get("reliable"); r != nil {
						if b, e := r.AsBool(); e == nil {
							reliable = b
						}
					}

					var sendErr error
					if reliable {
						sendErr = c.SendToReliable(node, gossip.MessageType(msgType), payload)
					} else {
						sendErr = c.SendTo(node, gossip.MessageType(msgType), payload)
					}
					if sendErr != nil {
						return errors.NewError("send_to failed: %s", sendErr.Error())
					}
					return &object.Null{}
				},
				HelpText: `send_to(node_id, message_type, data, reliable=False) - Send a direct message to a specific node

Parameters:
  node_id (string): Target node UUID
  message_type (int): Message type (must be >= 128)
  data: Message payload
  reliable (bool): Use reliable transport (default: False)`,
			},
			"handle": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if eval == nil {
						return errors.NewError("no evaluator available for handler registration")
					}
					if err := errors.MinArgs(args, 2); err != nil {
						return err
					}

					msgType, mtErr := args[0].AsInt()
					if mtErr != nil {
						return errors.NewError("message_type must be an integer (>= 128)")
					}
					if msgType < 128 {
						return errors.NewError("message_type must be >= 128 (user messages)")
					}

					handlerFn := args[1]

					handleErr := c.HandleFunc(gossip.MessageType(msgType), func(sender *gossip.Node, packet *gossip.Packet) error {
						var payload interface{}
						if unmarshalErr := packet.Unmarshal(&payload); unmarshalErr != nil {
							return unmarshalErr
						}

						var payloadObj object.Object
						if str, ok := payload.(string); ok {
							payloadObj = &object.String{Value: str}
						} else if payload != nil {
							payloadObj = conversion.FromGo(payload)
						} else {
							payloadObj = &object.Null{}
						}

						senderObj := nodeToObject(sender)
						msgObj := object.NewStringDict(map[string]object.Object{
							"type":    object.NewInteger(int64(packet.MessageType)),
							"sender":  senderObj,
							"payload": payloadObj,
						})

						result := eval.CallObjectFunction(ctx, handlerFn, []object.Object{msgObj}, nil, env)
						if errObj, ok := result.(*object.Error); ok {
							return fmt.Errorf("handler error: %s", errObj.Message)
						}
						return nil
					})

					if handleErr != nil {
						return errors.NewError("handle failed: %s", handleErr.Error())
					}
					return &object.Null{}
				},
				HelpText: `handle(message_type, handler) - Register a message handler

Parameters:
  message_type (int): Message type to handle (must be >= 128)
  handler (function): Handler function(message_dict) called for each message

The handler receives a dict with:
  - type: message type (int)
  - sender: dict with id, addr, state, metadata
  - payload: decoded message payload`,
			},
			"on_state_change": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if eval == nil {
						return errors.NewError("no evaluator available for handler registration")
					}
					if err := errors.MinArgs(args, 1); err != nil {
						return err
					}
					handlerFn := args[0]

					c.HandleNodeStateChangeFunc(func(node *gossip.Node, state gossip.NodeState) {
						stateStr := "unknown"
						switch state {
						case gossip.NodeAlive:
							stateStr = "alive"
						case gossip.NodeSuspect:
							stateStr = "suspect"
						case gossip.NodeDead:
							stateStr = "dead"
						case gossip.NodeLeaving:
							stateStr = "leaving"
						}
						// HandleNodeStateChangeFunc has no error return path; errors are logged
						// by the gossip transport and cannot be propagated from here.
						eval.CallObjectFunction(ctx, handlerFn, []object.Object{
							&object.String{Value: node.ID.String()},
							&object.String{Value: stateStr},
						}, nil, env)
					})
					return &object.Null{}
				},
				HelpText: `on_state_change(handler) - Register a node state change handler

Parameters:
  handler (function): Handler function(node_id, new_state)`,
			},
			"nodes": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					return nodesToList(c.Nodes())
				},
				HelpText: `nodes() - Get all known nodes

Returns:
  list of node dicts with id, addr, state, metadata`,
			},
			"alive_nodes": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					return nodesToList(c.AliveNodes())
				},
				HelpText: `alive_nodes() - Get all alive nodes

Returns:
  list of node dicts`,
			},
			"local_node": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					return nodeToObject(c.LocalNode())
				},
				HelpText: `local_node() - Get the local node info

Returns:
  dict with id, addr, state, metadata`,
			},
			"num_nodes": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					return object.NewInteger(int64(c.NumNodes()))
				},
				HelpText: `num_nodes() - Get total number of known nodes`,
			},
			"num_alive": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					return object.NewInteger(int64(c.NumAliveNodes()))
				},
				HelpText: `num_alive() - Get number of alive nodes`,
			},
			"set_metadata": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.MinArgs(args, 2); err != nil {
						return err
					}
					key, keyErr := args[0].AsString()
					if keyErr != nil {
						return keyErr
					}

					md := c.LocalMetadata()
					switch v := args[1].(type) {
					case *object.String:
						md.SetString(key, v.Value)
					case *object.Integer:
						md.SetInt64(key, v.Value)
					case *object.Float:
						md.SetFloat64(key, v.Value)
					case *object.Boolean:
						md.SetBool(key, v.Value)
					default:
						strVal, coerceErr := args[1].CoerceString()
						if coerceErr != nil {
							return errors.NewError("metadata value must be string, int, float, or bool")
						}
						md.SetString(key, strVal)
					}
					return &object.Null{}
				},
				HelpText: `set_metadata(key, value) - Set local node metadata

Parameters:
  key (string): Metadata key
  value: Metadata value (string, int, float, or bool)

Metadata is automatically gossiped to other nodes.`,
			},
			"get_metadata": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.MinArgs(args, 1); err != nil {
						return err
					}
					key, keyErr := args[0].AsString()
					if keyErr != nil {
						return keyErr
					}
					md := c.LocalMetadata()
					if !md.Exists(key) {
						return &object.Null{}
					}
					return &object.String{Value: md.GetString(key)}
				},
				HelpText: `get_metadata(key) - Get local node metadata value

Parameters:
  key (string): Metadata key

Returns:
  The metadata value as a string, or None if not found`,
			},
			"all_metadata": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					md := c.LocalMetadata().GetAllAsString()
					pairs := make(map[string]object.Object, len(md))
					for k, v := range md {
						pairs[k] = &object.String{Value: v}
					}
					return object.NewStringDict(pairs)
				},
				HelpText: `all_metadata() - Get all local node metadata

Returns:
  dict of all metadata key-value pairs`,
			},
			"delete_metadata": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.MinArgs(args, 1); err != nil {
						return err
					}
					key, keyErr := args[0].AsString()
					if keyErr != nil {
						return keyErr
					}
					c.LocalMetadata().Delete(key)
					return &object.Null{}
				},
				HelpText: `delete_metadata(key) - Delete a metadata key

Parameters:
  key (string): Metadata key to delete`,
			},
			"node_id": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					return &object.String{Value: c.LocalNode().ID.String()}
				},
				HelpText: `node_id() - Get the local node's unique ID`,
			},
		},
		HelpText: "Gossip cluster object",
	}
}

func buildLibrary() *object.Library {
	return object.NewLibrary(LibraryName, map[string]*object.Builtin{
		"create": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				eval := evaliface.FromContext(ctx)
				env := getEnvFromContext(ctx)

				bindAddr := "127.0.0.1:8000"
				if b := kwargs.Get("bind_addr"); b != nil {
					if bv, e := b.AsString(); e == nil {
						bindAddr = bv
					}
				}

				advertiseAddr := ""
				if a := kwargs.Get("advertise_addr"); a != nil {
					if av, e := a.AsString(); e == nil {
						advertiseAddr = av
					}
				}

				nodeID := ""
				if n := kwargs.Get("node_id"); n != nil {
					if nv, e := n.AsString(); e == nil {
						nodeID = nv
					}
				}

				encryptionKey := ""
				if ek := kwargs.Get("encryption_key"); ek != nil {
					if ekv, e := ek.AsString(); e == nil {
						encryptionKey = ekv
					}
				}

				var tags []string
				if t := kwargs.Get("tags"); t != nil {
					if list, e := t.AsList(); e == nil {
						for _, elem := range list {
							if s, se := elem.AsString(); se == nil {
								tags = append(tags, s)
							}
						}
					}
				}

				enableCompression := false
				if c := kwargs.Get("compression"); c != nil {
					if bv, e := c.AsBool(); e == nil {
						enableCompression = bv
					}
				}

				bearerToken := ""
				if bt := kwargs.Get("bearer_token"); bt != nil {
					if btv, e := bt.AsString(); e == nil {
						bearerToken = btv
					}
				}

				appVersion := ""
				if av := kwargs.Get("app_version"); av != nil {
					if avv, e := av.AsString(); e == nil {
						appVersion = avv
					}
				}

				config := gossip.DefaultConfig()
				config.BindAddr = bindAddr
				config.AdvertiseAddr = advertiseAddr
				config.NodeID = nodeID
				config.Tags = tags
				config.ApplicationVersion = appVersion
				config.BearerToken = bearerToken
				config.Logger = logger.NewNullLogger()
				config.MsgCodec = codec.NewShamatonMsgpackCodec()

				if enableCompression {
					config.Compressor = compression.NewSnappyCompressor()
				}

				if encryptionKey != "" {
					config.EncryptionKey = []byte(encryptionKey)
					config.Cipher = encryption.NewAESEncryptor()
				}

				config.Transport = gossip.NewSocketTransport(config)

				cluster, clusterErr := gossip.NewCluster(config)
				if clusterErr != nil {
					return errors.NewError("failed to create cluster: %s", clusterErr.Error())
				}

				clusterID := cluster.LocalNode().ID.String()
				clusters.Lock()
				clusters.m[clusterID] = cluster
				clusters.Unlock()

				return buildClusterObject(cluster, clusterID, eval, env)
			},
			HelpText: `create(bind_addr="127.0.0.1:8000", node_id="", advertise_addr="", encryption_key="", tags=[], compression=False, bearer_token="", app_version="") - Create a gossip cluster node

Parameters:
  bind_addr (string): Address to bind to (default: "127.0.0.1:8000")
  node_id (string): Unique node ID (auto-generated if empty)
  advertise_addr (string): Address to advertise to peers (default: same as bind_addr)
  encryption_key (string): Encryption key (16, 24, or 32 bytes for AES)
  tags (list): Tags for tag-based message routing
  compression (bool): Enable Snappy compression (default: False)
  bearer_token (string): Authentication bearer token
  app_version (string): Application version for compatibility checks

Returns:
  Cluster object with methods for membership and messaging

Example:
  import scriptling.net.gossip as gossip
  cluster = gossip.create(bind_addr="127.0.0.1:8000", tags=["web"])
  cluster.start()
  cluster.join(["127.0.0.1:8001"])
  cluster.handle(128, lambda msg: print(msg))
  cluster.send(128, "Hello!")
  cluster.stop()`,
		},
		"decode_json": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.MinArgs(args, 1); err != nil {
					return err
				}
				str, strErr := args[0].AsString()
				if strErr != nil {
					return strErr
				}
				var data interface{}
				if jsonErr := json.Unmarshal([]byte(str), &data); jsonErr != nil {
					return errors.NewError("JSON decode failed: %s", jsonErr.Error())
				}
				return conversion.FromGo(data)
			},
			HelpText: `decode_json(json_string) - Decode a JSON string to a scriptling value

Parameters:
  json_string (string): JSON string to decode

Returns:
  Decoded value (dict, list, string, int, float, bool, or None)`,
		},
	}, map[string]object.Object{
		"MSG_USER": object.NewInteger(128),
	}, LibraryDesc)
}

func getEnvFromContext(ctx context.Context) *object.Environment {
	if env, ok := ctx.Value("scriptling-env").(*object.Environment); ok {
		return env
	}
	return object.NewEnvironment()
}

func Register(registrar interface{ RegisterLibrary(*object.Library) }) {
	libraryOnce.Do(func() {
		library = buildLibrary()
		extlibs.RegisterCleanup(func() {
			clusters.Lock()
			for id, c := range clusters.m {
				c.Stop()
				delete(clusters.m, id)
			}
			clusters.Unlock()
		})
	})
	registrar.RegisterLibrary(library)
}
