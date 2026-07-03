package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/paularlott/jsonrpc"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/object"
)

// JSON-RPC 2.0 error codes (per spec). These mirror the codes exported by the
// jsonrpc package and are retained here for readability at the call sites.
const (
	jsonrpcParseError     = jsonrpc.CodeParseError
	jsonrpcInvalidRequest = jsonrpc.CodeInvalidRequest
	jsonrpcMethodNotFound = jsonrpc.CodeMethodNotFound
	jsonrpcInvalidParams  = jsonrpc.CodeInvalidParams
	jsonrpcInternalError  = jsonrpc.CodeInternalError
	jsonrpcServerError    = jsonrpc.CodeServerError
)

// collectJSONRPCMethods copies registered methods/notifications out of
// RuntimeState into the Server so dispatch is lock-free during serving.
// Caller must hold RuntimeState.Lock() (write lock) — no additional locking
// is done here, matching collectRoutes which reads fields the same way.
func (s *Server) collectJSONRPCMethods() {
	s.jsonrpcMethods = make(map[string]string, len(extlibs.RuntimeState.JSONRPCMethods))
	for name, handler := range extlibs.RuntimeState.JSONRPCMethods {
		s.jsonrpcMethods[name] = handler
	}
	s.jsonrpcNotifications = make(map[string]string, len(extlibs.RuntimeState.JSONRPCNotifications))
	for name, handler := range extlibs.RuntimeState.JSONRPCNotifications {
		s.jsonrpcNotifications[name] = handler
	}

	for name, handler := range s.jsonrpcMethods {
		Log.Info("Registered JSON-RPC method", "method", name, "handler", handler)
	}
	for name, handler := range s.jsonrpcNotifications {
		Log.Info("Registered JSON-RPC notification", "name", name, "handler", handler)
	}
}

// jsonrpcServerInstance builds (once) and returns the jsonrpc.Server that backs
// both the stdio and HTTP transports. The framing, batching, notification and
// error-code handling all live in the jsonrpc package; scriptling only supplies
// the per-method handlers that run script on a fresh evaluator.
func (s *Server) jsonrpcServerInstance() *jsonrpc.Server {
	s.jsonrpcServerOnce.Do(func() {
		s.jsonrpcServer = s.buildJSONRPCServer()
	})
	return s.jsonrpcServer
}

// buildJSONRPCServer registers a handler for every method/notification name.
// A JSON-RPC notification is a request without an id, so both call shapes reach
// the same handler; jsonrpc.IsNotification(ctx) distinguishes them, letting us
// preserve scriptling's separate method/notification registration semantics:
//
//   - a request (has id) for a name that is only a notification -> method not found
//   - a notification (no id) for a name that is only a method   -> ignored, handler not run
//   - a name registered as both dispatches to the matching handler for each shape
func (s *Server) buildJSONRPCServer() *jsonrpc.Server {
	srv := jsonrpc.NewServer(jsonrpc.WithErrorHandler(func(method string, err error) {
		Log.Debug("JSON-RPC handler error", "method", method, "error", err)
	}))

	names := make(map[string]struct{}, len(s.jsonrpcMethods)+len(s.jsonrpcNotifications))
	for name := range s.jsonrpcMethods {
		names[name] = struct{}{}
	}
	for name := range s.jsonrpcNotifications {
		names[name] = struct{}{}
	}

	for name := range names {
		methodRef, hasMethod := s.jsonrpcMethods[name]
		notifRef, hasNotif := s.jsonrpcNotifications[name]
		name := name
		srv.Handle(name, func(ctx context.Context, params json.RawMessage) (any, error) {
			if jsonrpc.IsNotification(ctx) {
				if !hasNotif {
					// Name is a method only: notifications for it are ignored
					// (the handler must not run and no response is written).
					return nil, nil
				}
				return s.invokeJSONRPCHandler(ctx, notifRef, params)
			}
			if !hasMethod {
				return nil, jsonrpc.MethodNotFound("method not found: " + name)
			}
			return s.invokeJSONRPCHandler(ctx, methodRef, params)
		})
	}

	return srv
}

// invokeJSONRPCHandler decodes params, runs the script handler on a fresh
// evaluator, and maps the result into a jsonrpc result value or *jsonrpc.Error.
func (s *Server) invokeJSONRPCHandler(ctx context.Context, handlerRef string, rawParams json.RawMessage) (any, error) {
	params, perr := decodeParams(rawParams)
	if perr != nil {
		return nil, jsonrpc.InvalidParams("invalid params: " + perr.Error())
	}

	result := s.runJSONRPCHandler(handlerRef, params)

	if ctx.Err() != nil {
		return nil, jsonrpc.InternalError("request cancelled")
	}

	// Handler returned an explicit JSON-RPC error object.
	if extlibs.IsJSONRPCError(result) {
		inst := result.(*object.Instance)
		code, _ := inst.Field("code").AsInt()
		message, _ := inst.Field("message").AsString()
		errOut := jsonrpc.NewError(int(code), message, nil)
		if data, present := inst.GetField("data"); present {
			if _, isNull := data.(*object.Null); !isNull {
				errOut.Data = conversion.ToGo(data)
			}
		}
		return nil, errOut
	}

	// Scriptling error / exception -> server error.
	if errObj, ok := result.(*object.Error); ok {
		return nil, jsonrpc.ServerError(errObj.Message)
	}
	if exObj, ok := result.(*object.Exception); ok {
		return nil, jsonrpc.ServerError(exObj.Message)
	}

	return conversion.ToGo(result), nil
}

// RunJSONRPCStdio serves JSON-RPC 2.0 requests from stdin, writing one response
// per line to stdout. Each request runs on a fresh Scriptling evaluator in its
// own goroutine. The call blocks until stdin closes (or returns on a fatal
// read/write error).
func (s *Server) RunJSONRPCStdio(ctx context.Context) error {
	return s.runJSONRPC(ctx, os.Stdin, os.Stdout)
}

// runJSONRPC installs signal-driven cancellation and then delegates the
// newline-delimited serve loop to the jsonrpc package. Cancelling on a signal
// lets long-running handlers bail out via their context.
func (s *Server) runJSONRPC(ctx context.Context, in io.Reader, out io.Writer) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)
	go func() {
		select {
		case <-sigChan:
			Log.Info("JSON-RPC server received signal, cancelling in-flight work")
			cancel()
		case <-ctx.Done():
		}
	}()

	Log.Info("JSON-RPC stdio server ready")

	return s.jsonrpcServerInstance().ServeStream(ctx, in, out)
}

// handleJSONRPCHTTP serves JSON-RPC 2.0 over HTTP. Requests use POST with a
// single JSON-RPC object or batch array. Notifications produce 204 No Content.
func (s *Server) handleJSONRPCHTTP(w http.ResponseWriter, r *http.Request) {
	Log.Trace("JSON-RPC HTTP request", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr)
	s.jsonrpcServerInstance().ServeHTTP(w, r)
}

// runJSONRPCHandler imports the handler's library into a fresh evaluator and
// invokes the function with the decoded params. Mirrors runHandler in http.go.
func (s *Server) runJSONRPCHandler(handlerRef string, params object.Object) object.Object {
	libName, _, ok := strings.Cut(handlerRef, ".")
	if !ok {
		Log.Error("Invalid JSON-RPC handler reference", "handler", handlerRef)
		return &object.Error{Message: "invalid handler reference: " + handlerRef}
	}

	p := scriptling.New()
	s.setupScriptling(p)
	s.applyPackLoader(p)

	if err := p.Import(libName); err != nil {
		Log.Error("Failed to import library", "library", libName, "error", err)
		return &object.Error{Message: fmt.Sprintf("failed to import library %s: %v", libName, err)}
	}

	result, err := p.CallFunction(handlerRef, params)
	if err != nil {
		Log.Error("JSON-RPC handler error", "handler", handlerRef, "error", err)
		return &object.Error{Message: err.Error()}
	}
	return result
}

// decodeParams unmarshals raw JSON params into a Scriptling object, preserving
// integer precision via UseNumber. Absent params yield Null.
func decodeParams(raw json.RawMessage) (object.Object, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return &object.Null{}, nil
	}
	var goVal interface{}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&goVal); err != nil {
		return nil, err
	}
	return conversion.FromGo(goVal), nil
}
