package gossip

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/paularlott/gossip"
	"github.com/paularlott/gossip/codec"
	"github.com/paularlott/gossip/compression"
	"github.com/paularlott/gossip/encryption"
	"github.com/paularlott/gossip/leader"
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
	log logger.Logger
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

	tags := n.GetTags()
	tagElems := make([]object.Object, len(tags))
	for i, t := range tags {
		tagElems[i] = &object.String{Value: t}
	}

	return object.NewStringDict(map[string]object.Object{
		"id":       &object.String{Value: n.ID.String()},
		"addr":     &object.String{Value: n.AdvertisedAddr()},
		"state":    &object.String{Value: state},
		"metadata": object.NewStringDict(mdPairs),
		"tags":     &object.List{Elements: tagElems},
	})
}

func nodesToList(nodes []*gossip.Node) *object.List {
	elements := make([]object.Object, len(nodes))
	for i, n := range nodes {
		elements[i] = nodeToObject(n)
	}
	return &object.List{Elements: elements}
}

func extractStringMap(raw object.Object) (map[string]string, error) {
	if raw == nil {
		return nil, nil
	}
	dict, ok := raw.(*object.Dict)
	if !ok {
		return nil, fmt.Errorf("expected a dict")
	}
	result := make(map[string]string, len(dict.Pairs))
	for _, pair := range dict.Pairs {
		key := pair.Key.Inspect()
		val, ok := pair.Value.(*object.String)
		if !ok {
			val = &object.String{Value: pair.Value.Inspect()}
		}
		result[key] = val.Value
	}
	return result, nil
}

func buildNodeGroupObject(ng *gossip.NodeGroup, c *gossip.Cluster, eval evaliface.Evaluator, env *object.Environment) *object.Builtin {
	return &object.Builtin{
		Attributes: map[string]object.Object{
			"nodes": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					return nodesToList(ng.GetNodes(nil))
				},
				HelpText: `nodes() - Get all nodes in this group

Returns:
  list of node dicts`,
			},
			"contains": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.MinArgs(args, 1); err != nil {
						return err
					}
					nodeIDStr, idErr := args[0].AsString()
					if idErr != nil {
						return idErr
					}
					node := c.GetNodeByIDString(nodeIDStr)
					if node == nil {
						return &object.Boolean{Value: false}
					}
					return &object.Boolean{Value: ng.Contains(node.ID)}
				},
				HelpText: `contains(node_id) - Check if a node is in this group

Parameters:
  node_id (string): Node UUID to check

Returns:
  bool`,
			},
			"count": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					return object.NewInteger(int64(ng.Count()))
				},
				HelpText: `count() - Get number of nodes in this group

Returns:
  int`,
			},
			"send_to_peers": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.MinArgs(args, 2); err != nil {
						return err
					}
					msgType, mtErr := args[0].AsInt()
					if mtErr != nil {
						return errors.NewError("message_type must be an integer (>= 128)")
					}
					if msgType < 128 {
						return errors.NewError("message_type must be >= 128")
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
						sendErr = ng.SendToPeersReliable(gossip.MessageType(msgType), payload)
					} else {
						sendErr = ng.SendToPeers(gossip.MessageType(msgType), payload)
					}
					if sendErr != nil {
						return errors.NewError("send_to_peers failed: %s", sendErr.Error())
					}
					return &object.Null{}
				},
				HelpText: `send_to_peers(message_type, data, reliable=False) - Send a message to all peers in the group

Parameters:
  message_type (int): Message type (must be >= 128)
  data: Message payload
  reliable (bool): Use reliable transport (default: False)`,
			},
			"close": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					ng.Close()
					return &object.Null{}
				},
				HelpText: `close() - Close the node group and release resources`,
			},
		},
		HelpText: "Gossip node group (metadata-criteria-based)",
	}
}

func buildLeaderElectionObject(le *leader.LeaderElection, eval evaliface.Evaluator, env *object.Environment) *object.Builtin {
	return &object.Builtin{
		Attributes: map[string]object.Object{
			"start": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					le.Start()
					return &object.Null{}
				},
				HelpText: `start() - Start the leader election process`,
			},
			"stop": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					le.Stop()
					return &object.Null{}
				},
				HelpText: `stop() - Stop the leader election process`,
			},
			"is_leader": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					return &object.Boolean{Value: le.IsLeader()}
				},
				HelpText: `is_leader() - Check if this node is the leader

Returns:
  bool`,
			},
			"has_leader": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					return &object.Boolean{Value: le.HasLeader()}
				},
				HelpText: `has_leader() - Check if a leader is currently elected

Returns:
  bool`,
			},
			"get_leader_id": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if !le.HasLeader() {
						return &object.Null{}
					}
					return &object.String{Value: le.GetLeaderID().String()}
				},
				HelpText: `get_leader_id() - Get the current leader's node ID

Returns:
  string or None if no leader`,
			},
			"send_to_peers": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.MinArgs(args, 2); err != nil {
						return err
					}
					msgType, mtErr := args[0].AsInt()
					if mtErr != nil {
						return errors.NewError("message_type must be an integer (>= 128)")
					}
					if msgType < 128 {
						return errors.NewError("message_type must be >= 128")
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
						sendErr = le.SendToPeersReliable(gossip.MessageType(msgType), payload)
					} else {
						sendErr = le.SendToPeers(gossip.MessageType(msgType), payload)
					}
					if sendErr != nil {
						return errors.NewError("send_to_peers failed: %s", sendErr.Error())
					}
					return &object.Null{}
				},
				HelpText: `send_to_peers(message_type, data, reliable=False) - Send to eligible leader election peers

Parameters:
  message_type (int): Message type (must be >= 128)
  data: Message payload
  reliable (bool): Use reliable transport (default: False)`,
			},
			"on_event": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if eval == nil {
						return errors.NewError("no evaluator available for handler registration")
					}
					if err := errors.MinArgs(args, 2); err != nil {
						return err
					}
					eventTypeStr, etErr := args[0].AsString()
					if etErr != nil {
						return etErr
					}

					var eventType leader.EventType
					switch eventTypeStr {
					case "elected":
						eventType = leader.LeaderElectedEvent
					case "lost":
						eventType = leader.LeaderLostEvent
					case "became_leader":
						eventType = leader.BecameLeaderEvent
					case "stepped_down":
						eventType = leader.SteppedDownEvent
					default:
						return errors.NewError("unknown event type: %s (use: elected, lost, became_leader, stepped_down)", eventTypeStr)
					}

					handlerFn := args[1]
					le.HandleEventFunc(eventType, func(et leader.EventType, nodeID gossip.NodeID) {
						eval.CallObjectFunction(ctx, handlerFn, []object.Object{
							&object.String{Value: eventTypeStr},
							&object.String{Value: nodeID.String()},
						}, nil, env)
					})
					return &object.Null{}
				},
				HelpText: `on_event(event_type, handler) - Register a leader election event handler

Parameters:
  event_type (string): One of "elected", "lost", "became_leader", "stepped_down"
  handler (function): Handler function(event_type, node_id)`,
			},
		},
		HelpText: "Gossip leader election object",
	}
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
  - sender: dict with id, addr, state, metadata, tags
  - payload: decoded message payload`,
			},
			"handle_with_reply": &object.Builtin{
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

					handleErr := c.HandleFuncWithReply(gossip.MessageType(msgType), func(sender *gossip.Node, packet *gossip.Packet) (interface{}, error) {
						var payload interface{}
						if unmarshalErr := packet.Unmarshal(&payload); unmarshalErr != nil {
							return nil, unmarshalErr
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
							return nil, fmt.Errorf("handler error: %s", errObj.Message)
						}
						return conversion.ToGo(result), nil
					})

					if handleErr != nil {
						return errors.NewError("handle_with_reply failed: %s", handleErr.Error())
					}
					return &object.Null{}
				},
				HelpText: `handle_with_reply(message_type, handler) - Register a request/reply message handler

Parameters:
  message_type (int): Message type to handle (must be >= 128)
  handler (function): Handler function(message_dict) -> reply_data

The handler receives the same dict as handle() and must return the reply data.`,
			},
			"send_request": &object.Builtin{
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
					var response interface{}
					if sendErr := c.SendToWithResponse(node, gossip.MessageType(msgType), payload, &response); sendErr != nil {
						return errors.NewError("send_request failed: %s", sendErr.Error())
					}

					if response == nil {
						return &object.Null{}
					}
					return conversion.FromGo(response)
				},
				HelpText: `send_request(node_id, message_type, data) - Send a request and wait for a reply

Parameters:
  node_id (string): Target node UUID
  message_type (int): Message type (must be >= 128)
  data: Message payload

Returns:
  The reply payload from the target node`,
			},
			"unhandle": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.MinArgs(args, 1); err != nil {
						return err
					}
					msgType, mtErr := args[0].AsInt()
					if mtErr != nil {
						return errors.NewError("message_type must be an integer (>= 128)")
					}
					if msgType < 128 {
						return errors.NewError("message_type must be >= 128 (user messages)")
					}
					return &object.Boolean{Value: c.UnregisterMessageType(gossip.MessageType(msgType))}
				},
				HelpText: `unhandle(message_type) - Remove a previously registered message handler

Parameters:
  message_type (int): Message type to unregister (must be >= 128)

Returns:
  bool - True if a handler was removed`,
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
			"on_metadata_change": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if eval == nil {
						return errors.NewError("no evaluator available for handler registration")
					}
					if err := errors.MinArgs(args, 1); err != nil {
						return err
					}
					handlerFn := args[0]

					c.HandleNodeMetadataChangeFunc(func(node *gossip.Node) {
						eval.CallObjectFunction(ctx, handlerFn, []object.Object{
							nodeToObject(node),
						}, nil, env)
					})
					return &object.Null{}
				},
				HelpText: `on_metadata_change(handler) - Register a remote node metadata change handler

Parameters:
  handler (function): Handler function(node_dict) called when any node's metadata changes`,
			},
			"on_gossip_interval": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if eval == nil {
						return errors.NewError("no evaluator available for handler registration")
					}
					if err := errors.MinArgs(args, 1); err != nil {
						return err
					}
					handlerFn := args[0]

					c.HandleGossipFunc(func() {
						eval.CallObjectFunction(ctx, handlerFn, nil, nil, env)
					})
					return &object.Null{}
				},
				HelpText: `on_gossip_interval(handler) - Register a periodic gossip interval handler

Parameters:
  handler (function): Handler function() called every gossip interval`,
			},
			"create_node_group": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if eval == nil {
						return errors.NewError("no evaluator available for handler registration")
					}

					criteria, critErr := extractStringMap(kwargs.Get("criteria"))
					if critErr != nil {
						return errors.NewError("criteria: %s", critErr.Error())
					}
					if criteria == nil || len(criteria) == 0 {
						return errors.NewError("criteria is required and must be a non-empty dict")
					}

					opts := &gossip.NodeGroupOptions{}
					if onAddedFn := kwargs.Get("on_node_added"); onAddedFn != nil {
						addedFn := onAddedFn
						opts.OnNodeAdded = func(node *gossip.Node) {
							eval.CallObjectFunction(ctx, addedFn, []object.Object{nodeToObject(node)}, nil, env)
						}
					}
					if onRemovedFn := kwargs.Get("on_node_removed"); onRemovedFn != nil {
						removedFn := onRemovedFn
						opts.OnNodeRemoved = func(node *gossip.Node) {
							eval.CallObjectFunction(ctx, removedFn, []object.Object{nodeToObject(node)}, nil, env)
						}
					}

					ng := gossip.NewNodeGroup(c, criteria, opts)
					return buildNodeGroupObject(ng, c, eval, env)
				},
				HelpText: `create_node_group(criteria, on_node_added=None, on_node_removed=None) - Create a metadata-criteria-based node group

Parameters:
  criteria (dict): Metadata key-value pairs to match (use "*" for any value, "~value" for contains)
  on_node_added (function): Optional callback function(node_dict) when a node joins the group
  on_node_removed (function): Optional callback function(node_dict) when a node leaves the group

Returns:
  NodeGroup object with nodes(), contains(), count(), send_to_peers(), close()`,
			},
			"create_leader_election": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if eval == nil {
						return errors.NewError("no evaluator available for handler registration")
					}

					leaderCfg := leader.DefaultConfig()

					if ci := kwargs.Get("check_interval"); ci != nil {
						if v, e := ci.AsString(); e == nil {
							if d, pErr := time.ParseDuration(v); pErr == nil {
								leaderCfg.LeaderCheckInterval = d
							}
						}
					}
					if lt := kwargs.Get("leader_timeout"); lt != nil {
						if v, e := lt.AsString(); e == nil {
							if d, pErr := time.ParseDuration(v); pErr == nil {
								leaderCfg.LeaderTimeout = d
							}
						}
					}
					if hmt := kwargs.Get("heartbeat_msg_type"); hmt != nil {
						if v, e := hmt.AsInt(); e == nil {
							leaderCfg.HeartbeatMessageType = gossip.MessageType(v)
						}
					}
					if qp := kwargs.Get("quorum_percentage"); qp != nil {
						if v, e := qp.AsInt(); e == nil {
							leaderCfg.QuorumPercentage = int(v)
						}
					}
					if raw := kwargs.Get("metadata_criteria"); raw != nil {
						pairs, pErr := extractStringMap(raw)
						if pErr != nil {
							return errors.NewError("metadata_criteria: %s", pErr.Error())
						}
						if pairs != nil && len(pairs) > 0 {
							leaderCfg.MetadataCriteria = pairs
						}
					}

					le := leader.NewLeaderElection(c, leaderCfg)
					return buildLeaderElectionObject(le, eval, env)
				},
				HelpText: `create_leader_election(check_interval="1s", leader_timeout="3s", heartbeat_msg_type=65, quorum_percentage=60, metadata_criteria=None) - Create a leader election manager

Parameters:
  check_interval (string): Duration between leader checks (default: "1s")
  leader_timeout (string): Duration without heartbeat before leader is considered lost (default: "3s")
  heartbeat_msg_type (int): Message type for heartbeat messages (default: 65, reserved range)
  quorum_percentage (int): Percentage of nodes required for quorum 1-100 (default: 60)
  metadata_criteria (dict): Optional metadata criteria to limit eligible nodes

Returns:
  LeaderElection object with start(), stop(), is_leader(), has_leader(), get_leader_id(), on_event()`,
			},
			"nodes": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					return nodesToList(c.Nodes())
				},
				HelpText: `nodes() - Get all known nodes

Returns:
  list of node dicts with id, addr, state, metadata, tags`,
			},
			"alive_nodes": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					return nodesToList(c.AliveNodes())
				},
				HelpText: `alive_nodes() - Get all alive nodes

Returns:
  list of node dicts`,
			},
			"nodes_by_tag": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.MinArgs(args, 1); err != nil {
						return err
					}
					tag, tagErr := args[0].AsString()
					if tagErr != nil {
						return tagErr
					}
					return nodesToList(c.GetNodesByTag(tag))
				},
				HelpText: `nodes_by_tag(tag) - Get all nodes that have a specific tag

Parameters:
  tag (string): Tag to filter by

Returns:
  list of node dicts`,
			},
			"get_node": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.MinArgs(args, 1); err != nil {
						return err
					}
					nodeIDStr, idErr := args[0].AsString()
					if idErr != nil {
						return idErr
					}
					node := c.GetNodeByIDString(nodeIDStr)
					if node == nil {
						return &object.Null{}
					}
					return nodeToObject(node)
				},
				HelpText: `get_node(node_id) - Get a specific node by ID

Parameters:
  node_id (string): Node UUID

Returns:
  node dict or None if not found`,
			},
			"local_node": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					return nodeToObject(c.LocalNode())
				},
				HelpText: `local_node() - Get the local node info

Returns:
  dict with id, addr, state, metadata, tags`,
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
			"num_suspect": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					return object.NewInteger(int64(c.NumSuspectNodes()))
				},
				HelpText: `num_suspect() - Get number of suspect nodes`,
			},
			"num_dead": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					return object.NewInteger(int64(c.NumDeadNodes()))
				},
				HelpText: `num_dead() - Get number of dead nodes`,
			},
			"is_local": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.MinArgs(args, 1); err != nil {
						return err
					}
					nodeIDStr, idErr := args[0].AsString()
					if idErr != nil {
						return idErr
					}
					node := c.GetNodeByIDString(nodeIDStr)
					if node == nil {
						return &object.Boolean{Value: false}
					}
					return &object.Boolean{Value: c.NodeIsLocal(node)}
				},
				HelpText: `is_local(node_id) - Check if a node ID refers to the local node

Parameters:
  node_id (string): Node UUID to check

Returns:
  bool`,
			},
			"candidates": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					return nodesToList(c.GetCandidates())
				},
				HelpText: `candidates() - Get a random subset of nodes for gossiping

Returns:
  list of node dicts`,
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
				if comp := kwargs.Get("compression"); comp != nil {
					if bv, e := comp.AsBool(); e == nil {
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

				transport := "socket"
				if tr := kwargs.Get("transport"); tr != nil {
					if trv, e := tr.AsString(); e == nil {
						transport = trv
					}
				}

				config := gossip.DefaultConfig()
				config.BindAddr = bindAddr
				config.AdvertiseAddr = advertiseAddr
				config.NodeID = nodeID
				config.Tags = tags
				config.ApplicationVersion = appVersion
				config.BearerToken = bearerToken
				config.Logger = log.WithGroup("gossip")
				config.MsgCodec = codec.NewVmihailencoMsgpackCodec()

				if enableCompression {
					config.Compressor = compression.NewSnappyCompressor()
				}

				if encryptionKey != "" {
					config.EncryptionKey = []byte(encryptionKey)
					config.Cipher = encryption.NewAESEncryptor()
				}

				if v := kwargs.Get("compress_min_size"); v != nil {
					if iv, e := v.AsInt(); e == nil {
						config.CompressMinSize = int(iv)
					}
				}
				if v := kwargs.Get("gossip_interval"); v != nil {
					if sv, e := v.AsString(); e == nil {
						if d, pErr := time.ParseDuration(sv); pErr == nil {
							config.GossipInterval = d
						}
					}
				}
				if v := kwargs.Get("gossip_max_interval"); v != nil {
					if sv, e := v.AsString(); e == nil {
						if d, pErr := time.ParseDuration(sv); pErr == nil {
							config.GossipMaxInterval = d
						}
					}
				}
				if v := kwargs.Get("metadata_gossip_interval"); v != nil {
					if sv, e := v.AsString(); e == nil {
						if d, pErr := time.ParseDuration(sv); pErr == nil {
							config.MetadataGossipInterval = d
						}
					}
				}
				if v := kwargs.Get("state_gossip_interval"); v != nil {
					if sv, e := v.AsString(); e == nil {
						if d, pErr := time.ParseDuration(sv); pErr == nil {
							config.StateGossipInterval = d
						}
					}
				}
				if v := kwargs.Get("fan_out_multiplier"); v != nil {
					if fv, e := v.AsFloat(); e == nil {
						config.FanOutMultiplier = fv
					}
				}
				if v := kwargs.Get("ttl_multiplier"); v != nil {
					if fv, e := v.AsFloat(); e == nil {
						config.TTLMultiplier = fv
					}
				}
				if v := kwargs.Get("state_exchange_multiplier"); v != nil {
					if fv, e := v.AsFloat(); e == nil {
						config.StateExchangeMultiplier = fv
					}
				}
				if v := kwargs.Get("force_reliable_transport"); v != nil {
					if bv, e := v.AsBool(); e == nil {
						config.ForceReliableTransport = bv
					}
				}
				if v := kwargs.Get("prefer_ipv6"); v != nil {
					if bv, e := v.AsBool(); e == nil {
						config.PreferIPv6 = bv
					}
				}
				if v := kwargs.Get("node_cleanup_interval"); v != nil {
					if sv, e := v.AsString(); e == nil {
						if d, pErr := time.ParseDuration(sv); pErr == nil {
							config.NodeCleanupInterval = d
						}
					}
				}
				if v := kwargs.Get("node_retention_time"); v != nil {
					if sv, e := v.AsString(); e == nil {
						if d, pErr := time.ParseDuration(sv); pErr == nil {
							config.NodeRetentionTime = d
						}
					}
				}
				if v := kwargs.Get("leaving_node_timeout"); v != nil {
					if sv, e := v.AsString(); e == nil {
						if d, pErr := time.ParseDuration(sv); pErr == nil {
							config.LeavingNodeTimeout = d
						}
					}
				}
				if v := kwargs.Get("health_check_interval"); v != nil {
					if sv, e := v.AsString(); e == nil {
						if d, pErr := time.ParseDuration(sv); pErr == nil {
							config.HealthCheckInterval = d
						}
					}
				}
				if v := kwargs.Get("suspect_timeout"); v != nil {
					if sv, e := v.AsString(); e == nil {
						if d, pErr := time.ParseDuration(sv); pErr == nil {
							config.SuspectTimeout = d
						}
					}
				}
				if v := kwargs.Get("suspect_retry_interval"); v != nil {
					if sv, e := v.AsString(); e == nil {
						if d, pErr := time.ParseDuration(sv); pErr == nil {
							config.SuspectRetryInterval = d
						}
					}
				}
				if v := kwargs.Get("dead_node_timeout"); v != nil {
					if sv, e := v.AsString(); e == nil {
						if d, pErr := time.ParseDuration(sv); pErr == nil {
							config.DeadNodeTimeout = d
						}
					}
				}
				if v := kwargs.Get("peer_recovery_interval"); v != nil {
					if sv, e := v.AsString(); e == nil {
						if d, pErr := time.ParseDuration(sv); pErr == nil {
							config.PeerRecoveryInterval = d
						}
					}
				}
				if v := kwargs.Get("insecure_skip_verify"); v != nil {
					if bv, e := v.AsBool(); e == nil {
						config.InsecureSkipVerify = bv
					}
				}

				switch transport {
				case "http":
					config.Transport = gossip.NewHTTPTransport(config)
				case "socket":
					config.Transport = gossip.NewSocketTransport(config)
				default:
					return errors.NewError("unknown transport: %s (use 'socket' or 'http')", transport)
				}

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
			HelpText: `create(bind_addr="127.0.0.1:8000", node_id="", advertise_addr="", encryption_key="", tags=[], compression=False, bearer_token="", app_version="", transport="socket", ...) - Create a gossip cluster node

Parameters:
  bind_addr (string): Address to bind to (default: "127.0.0.1:8000")
  node_id (string): Unique node ID (auto-generated if empty)
  advertise_addr (string): Address to advertise to peers (default: same as bind_addr)
  encryption_key (string): Encryption key (16, 24, or 32 bytes for AES)
  tags (list): Tags for tag-based message routing
  compression (bool): Enable Snappy compression (default: False)
  bearer_token (string): Authentication bearer token
  app_version (string): Application version for compatibility checks
  transport (string): Transport type: "socket" or "http" (default: "socket")

Advanced configuration:
  compress_min_size (int): Min message size for compression (default: 256)
  gossip_interval (string): Gossip interval duration (default: "5s")
  gossip_max_interval (string): Max gossip interval (default: "20s")
  metadata_gossip_interval (string): Metadata gossip interval (default: "500ms")
  state_gossip_interval (string): State exchange interval (default: "45s")
  fan_out_multiplier (float): Fan-out scaling factor (default: 1.0)
  ttl_multiplier (float): TTL scaling factor (default: 1.0)
  state_exchange_multiplier (float): State exchange scaling (default: 0.8)
  force_reliable_transport (bool): Force TCP for all messages (default: False)
  prefer_ipv6 (bool): Prefer IPv6 for DNS resolution (default: False)
  node_cleanup_interval (string): Dead node cleanup interval (default: "20s")
  node_retention_time (string): How long to keep dead nodes (default: "1h")
  leaving_node_timeout (string): Timeout before moving leaving->dead (default: "30s")
  health_check_interval (string): Health check interval (default: "2s")
  suspect_timeout (string): Time before marking node suspect (default: "1.5s")
  suspect_retry_interval (string): Suspect node retry interval (default: "1s")
  dead_node_timeout (string): Time before marking suspect->dead (default: "15s")
  peer_recovery_interval (string): Peer recovery check interval (default: "30s")
  insecure_skip_verify (bool): Skip TLS verification for HTTP (default: False)

Returns:
  Cluster object with methods for membership and messaging`,
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

func Register(registrar interface{ RegisterLibrary(*object.Library) }, loggerInstance logger.Logger) {
	if loggerInstance == nil {
		loggerInstance = logger.NewNullLogger()
	}
	libraryOnce.Do(func() {
		log = loggerInstance
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
