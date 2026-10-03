package gossip

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/paularlott/gossip"
	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/evaliface"
	"github.com/paularlott/scriptling/object"
)

// Streams carry a reply of any size from one node to one caller, outside the
// packet size limit: cluster.open_stream() sends a request and returns a
// reader; the target's cluster.handle_stream() handler writes the reply.

// messageObject builds the dict a handler receives for a packet: type,
// sender and the decoded payload.
func messageObject(sender *gossip.Node, packet *gossip.Packet) (object.Object, error) {
	var payload interface{}
	if err := packet.Unmarshal(&payload); err != nil {
		return nil, err
	}
	var payloadObj object.Object
	if str, ok := payload.(string); ok {
		payloadObj = object.NewString(str)
	} else if payload != nil {
		payloadObj = conversion.FromGo(payload)
	} else {
		payloadObj = &object.Null{}
	}
	return object.NewStringDict(map[string]object.Object{
		"type":    object.NewInteger(int64(packet.MessageType)),
		"sender":  nodeToObject(sender),
		"payload": payloadObj,
	}), nil
}

// streamBytes converts data passed to write() into bytes: bytes as-is,
// strings as UTF-8.
func streamBytes(data object.Object) ([]byte, object.Object) {
	switch d := data.(type) {
	case *object.Bytes:
		return d.BytesValue(), nil
	case *object.String:
		return []byte(d.StringValue()), nil
	}
	return nil, errors.NewTypeError("str or bytes", data.Type().String())
}

// ---------------------------------------------------------------------------
// Writer: the handler side
// ---------------------------------------------------------------------------

// streamWriterData is the reply writer handed to a stream handler. It is only
// valid while the handler runs. mu serialises writes and the end of the
// handler; it is only ever taken without the interpreter lock held (inside
// RunBlocking, or on the gossip goroutine), so it cannot deadlock with it.
type streamWriterData struct {
	mu     sync.Mutex
	w      io.Writer
	closed bool
}

// finish marks the writer unusable once the handler has returned, waiting
// for any write in progress.
func (d *streamWriterData) finish() {
	d.mu.Lock()
	d.closed = true
	d.mu.Unlock()
}

func writerFrom(args []object.Object) (*streamWriterData, object.Object) {
	if len(args) < 1 {
		return nil, errors.NewError("missing self")
	}
	inst, ok := args[0].(*object.Instance)
	if !ok {
		return nil, errors.NewError("expected a StreamWriter")
	}
	d, ok := inst.NativeData.(*streamWriterData)
	if !ok {
		return nil, errors.NewError("expected a StreamWriter")
	}
	return d, nil
}

var streamWriterClass = &object.Class{
	Name: "StreamWriter",
	Methods: map[string]object.Object{
		"write": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 2); err != nil {
					return err
				}
				d, errObj := writerFrom(args)
				if errObj != nil {
					return errObj
				}
				data, errObj := streamBytes(args[1])
				if errObj != nil {
					return errObj
				}
				var n int
				var werr error
				closed := false
				object.RunBlocking(ctx, func() {
					d.mu.Lock()
					defer d.mu.Unlock()
					if d.closed {
						closed = true
						return
					}
					n, werr = d.w.Write(data)
				})
				if closed {
					return errors.NewError("stream is closed: write() is only valid inside the stream handler")
				}
				if werr != nil {
					return errors.NewError("stream write failed: %s", werr.Error())
				}
				return object.NewInteger(int64(n))
			},
			HelpText: `write(data) - Send str (as UTF-8) or bytes to the caller; returns the byte count`,
		},
	},
}

// ---------------------------------------------------------------------------
// Reader: the caller side
// ---------------------------------------------------------------------------

// streamReaderData is the caller's side of a stream. readMu serialises reads
// and is only taken inside RunBlocking (interpreter lock released); close()
// takes no lock at all, so closing from another task can never deadlock with
// a read blocked waiting for data. Closing aborts any read in progress.
type streamReaderData struct {
	readMu sync.Mutex
	rc     io.ReadCloser
	r      *bufio.Reader
	cancel context.CancelFunc
	closed atomic.Bool
}

func (d *streamReaderData) close() {
	if d.closed.Swap(true) {
		return
	}
	d.cancel()
	_ = d.rc.Close()
}

func readerFrom(args []object.Object) (*streamReaderData, object.Object) {
	if len(args) < 1 {
		return nil, errors.NewError("missing self")
	}
	inst, ok := args[0].(*object.Instance)
	if !ok {
		return nil, errors.NewError("expected a Stream")
	}
	d, ok := inst.NativeData.(*streamReaderData)
	if !ok {
		return nil, errors.NewError("expected a Stream")
	}
	return d, nil
}

// streamReadError turns a reader error into a script error. io.EOF is the
// normal end of a complete reply and is not an error.
func streamReadError(err error) object.Object {
	return errors.NewError("stream read failed: %s", err.Error())
}

var streamReaderClass = &object.Class{
	Name: "Stream",
	Methods: map[string]object.Object{
		"read": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.RangeArgs(args, 1, 2); err != nil {
					return err
				}
				d, errObj := readerFrom(args)
				if errObj != nil {
					return errObj
				}
				size := int64(-1)
				if len(args) == 2 {
					if _, isNone := args[1].(*object.Null); !isNone {
						v, err := args[1].AsInt()
						if err != nil {
							return err
						}
						size = v
					}
				}
				if d.closed.Load() {
					return errors.NewError("read from a closed stream")
				}
				var data []byte
				var rerr error
				object.RunBlocking(ctx, func() {
					d.readMu.Lock()
					defer d.readMu.Unlock()
					// Memory grows with the data received, never with the
					// size asked for.
					var src io.Reader = d.r
					if size >= 0 {
						src = io.LimitReader(d.r, size)
					}
					data, rerr = io.ReadAll(src)
				})
				if d.closed.Load() {
					return errors.NewError("read from a closed stream")
				}
				if rerr != nil && rerr != io.EOF {
					return streamReadError(rerr)
				}
				return object.NewBytes(data)
			},
			HelpText: `read(size=-1) - Read up to size bytes, or everything when size is omitted; b'' at the end`,
		},
		"readline": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				d, errObj := readerFrom(args)
				if errObj != nil {
					return errObj
				}
				if d.closed.Load() {
					return errors.NewError("read from a closed stream")
				}
				var line []byte
				var rerr error
				object.RunBlocking(ctx, func() {
					d.readMu.Lock()
					defer d.readMu.Unlock()
					line, rerr = d.r.ReadBytes('\n')
				})
				if d.closed.Load() {
					return errors.NewError("read from a closed stream")
				}
				if rerr != nil && rerr != io.EOF {
					return streamReadError(rerr)
				}
				return object.NewBytes(line)
			},
			HelpText: `readline() - Read one line including its newline; b'' at the end`,
		},
		"close": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				d, errObj := readerFrom(args)
				if errObj != nil {
					return errObj
				}
				d.close()
				return &object.Null{}
			},
			HelpText: `close() - Close the stream; abandons any unread reply`,
		},
		"__enter__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				return args[0]
			},
		},
		"__exit__": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if d, errObj := readerFrom(args); errObj == nil {
					d.close()
				}
				return object.NewBoolean(false)
			},
		},
	},
}

// ---------------------------------------------------------------------------
// Cluster methods
// ---------------------------------------------------------------------------

// streamClusterMethods adds open_stream and handle_stream. clusterCtx ends
// when the cluster stops, which aborts any stream still open.
func streamClusterMethods(c *gossip.Cluster, clusterCtx context.Context, eval evaliface.Evaluator, env *object.Environment) map[string]object.Object {
	return map[string]object.Object{
		"open_stream": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 3); err != nil {
					return err
				}
				nodeIDStr, idErr := args[0].AsString()
				if idErr != nil {
					return idErr
				}
				msgType, mtErr := args[1].AsInt()
				if mtErr != nil || msgType < 128 {
					return errors.NewError("message_type must be an integer >= 128")
				}
				node := c.GetNodeByIDString(nodeIDStr)
				if node == nil {
					return errors.NewError("node not found: %s", nodeIDStr)
				}
				// The stream lives as long as both the script and the cluster,
				// unless closed first.
				streamCtx, cancelStream := context.WithCancel(ctx)
				stopWithCluster := context.AfterFunc(clusterCtx, cancelStream)
				cancel := func() {
					stopWithCluster()
					cancelStream()
				}
				var rc io.ReadCloser
				var openErr error
				object.RunBlocking(ctx, func() {
					rc, openErr = c.OpenStream(streamCtx, node, gossip.MessageType(msgType), conversion.ToGo(args[2]))
				})
				if openErr != nil {
					cancel()
					return errors.NewError("open_stream failed: %s", openErr.Error())
				}
				return object.NewInstanceWithData(streamReaderClass, map[string]object.Object{}, &streamReaderData{
					rc: rc, r: bufio.NewReader(rc), cancel: cancel,
				})
			},
			HelpText: `open_stream(node_id, message_type, data) - Request a streamed reply

Sends data to the node's handle_stream() handler for message_type and returns
a Stream to read the reply from: read(size=-1) and readline() return bytes
(b'' at the end), and close() abandons it. Use it in a with statement to
close it automatically. A handler error is raised by the read that reaches it.

Parameters:
  node_id (string): Target node UUID
  message_type (int): Message type (must be >= 128)
  data: Request payload`,
		},
		"handle_stream": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if eval == nil {
					return errors.NewError("no evaluator available for handler registration")
				}
				if err := errors.ExactArgs(args, 2); err != nil {
					return err
				}
				msgType, mtErr := args[0].AsInt()
				if mtErr != nil || msgType < 128 {
					return errors.NewError("message_type must be an integer >= 128")
				}
				handlerFn := args[1]
				err := c.HandleStreamFunc(gossip.MessageType(msgType), func(sender *gossip.Node, packet *gossip.Packet, w io.Writer) error {
					msgObj, err := messageObject(sender, packet)
					if err != nil {
						return err
					}
					wd := &streamWriterData{w: w}
					writer := object.NewInstanceWithData(streamWriterClass, map[string]object.Object{}, wd)
					result := dispatchSync(func() object.Object {
						return eval.CallObjectFunction(ctx, handlerFn, []object.Object{msgObj, writer}, nil, env)
					})
					// The writer is only valid while the handler runs.
					wd.finish()
					switch r := result.(type) {
					case *object.Error:
						return fmt.Errorf("%s", r.Message)
					case *object.Exception:
						return fmt.Errorf("%s: %s", r.ExceptionType, r.Message)
					}
					return nil
				})
				if err != nil {
					return errors.NewError("handle_stream failed: %s", err.Error())
				}
				return &object.Null{}
			},
			HelpText: `handle_stream(message_type, handler) - Serve streamed replies

Registers handler(msg, writer) for open_stream() requests of message_type.
msg is the same dict handle() receives; write the reply with
writer.write(data), where data is str (sent as UTF-8) or bytes. Returning
ends the reply; raising an exception sends the error to the caller.

Parameters:
  message_type (int): Message type to serve (must be >= 128)
  handler (function): handler(msg, writer)`,
		},
	}
}
