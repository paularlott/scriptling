package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/object"
)

// JSON-RPC 2.0 error codes (per spec).
const (
	jsonrpcParseError     = -32700
	jsonrpcInvalidRequest = -32600
	jsonrpcMethodNotFound = -32601
	jsonrpcInvalidParams  = -32602
	jsonrpcInternalError  = -32603
	jsonrpcServerError    = -32000
)

// jsonrpcVersion is the protocol version emitted on every response.
const jsonrpcVersion = "2.0"

// jsonrpcFrame is a single inbound request/notification. ID is kept as raw
// bytes so absent id (notification) is distinguishable from explicit null, and
// so the original id shape round-trips into the response unchanged.
type jsonrpcFrame struct {
	JSONRPC json.RawMessage `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
}

func (f *jsonrpcFrame) isNotification() bool {
	return len(f.ID) == 0
}

// jsonrpcResponseOut is the wire shape of an outbound response.
type jsonrpcResponseOut struct {
	JSONRPC string           `json:"jsonrpc"`
	Result  interface{}      `json:"result,omitempty"`
	Error   *jsonrpcErrorOut `json:"error,omitempty"`
	ID      json.RawMessage  `json:"id"`
}

type jsonrpcErrorOut struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// collectJSONRPCMethods copies registered methods/notifications out of
// RuntimeState into the Server so dispatch is lock-free during serving.
func (s *Server) collectJSONRPCMethods() {
	extlibs.RuntimeState.RLock()
	s.jsonrpcMethods = make(map[string]string, len(extlibs.RuntimeState.JSONRPCMethods))
	for name, handler := range extlibs.RuntimeState.JSONRPCMethods {
		s.jsonrpcMethods[name] = handler
	}
	s.jsonrpcNotifications = make(map[string]string, len(extlibs.RuntimeState.JSONRPCNotifications))
	for name, handler := range extlibs.RuntimeState.JSONRPCNotifications {
		s.jsonrpcNotifications[name] = handler
	}
	extlibs.RuntimeState.RUnlock()

	for name, handler := range s.jsonrpcMethods {
		Log.Info("Registered JSON-RPC method", "method", name, "handler", handler)
	}
	for name, handler := range s.jsonrpcNotifications {
		Log.Info("Registered JSON-RPC notification", "name", name, "handler", handler)
	}
}

// RunJSONRPCStdio serves JSON-RPC 2.0 requests from stdin, writing one response
// per line to stdout. Each request runs on a fresh Scriptling evaluator in its
// own goroutine; writes are serialised via a mutex. The call blocks until stdin
// closes (or returns on a fatal read/write error).
func (s *Server) RunJSONRPCStdio(ctx context.Context) error {
	return s.runJSONRPC(ctx, os.Stdin, os.Stdout)
}

func (s *Server) runJSONRPC(ctx context.Context, in io.Reader, out io.Writer) error {
	decoder := json.NewDecoder(bufio.NewReader(in))
	writeMu := &sync.Mutex{}
	encoder := json.NewEncoder(out)

	var wg sync.WaitGroup
	var firstErr error
	var firstErrMu sync.Mutex
	recordErr := func(err error) {
		if err == nil {
			return
		}
		firstErrMu.Lock()
		if firstErr == nil {
			firstErr = err
		}
		firstErrMu.Unlock()
	}

	// Cancel inbound work on signal; lets long-running handlers bail out.
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

	for {
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			if err == io.EOF {
				wg.Wait()
				return firstErr
			}
			// Unrecoverable stream corruption: emit a parse error if we can.
			writeMu.Lock()
			encErr := encoder.Encode(jsonrpcResponseOut{
				JSONRPC: jsonrpcVersion,
				Error:   &jsonrpcErrorOut{Code: jsonrpcParseError, Message: "parse error: " + err.Error()},
				ID:      json.RawMessage("null"),
			})
			writeMu.Unlock()
			recordErr(encErr)
			wg.Wait()
			return err
		}

		trimmed := bytes.TrimLeft(raw, " \t\r\n")
		if len(trimmed) == 0 {
			continue
		}

		// Batch: a JSON array of requests. Each element is dispatched
		// concurrently on a fresh evaluator; the responses are collected and
		// emitted as a single JSON array (per JSON-RPC 2.0). Notifications
		// inside a batch produce no entry in the output array. An
		// all-notification batch produces no output at all.
		if trimmed[0] == '[' {
			var frames []jsonrpcFrame
			if err := json.Unmarshal(raw, &frames); err != nil {
				writeMu.Lock()
				encoder.Encode(jsonrpcResponseOut{
					JSONRPC: jsonrpcVersion,
					Error:   &jsonrpcErrorOut{Code: jsonrpcParseError, Message: "parse error: " + err.Error()},
					ID:      json.RawMessage("null"),
				})
				writeMu.Unlock()
				continue
			}
			if len(frames) == 0 {
				writeMu.Lock()
				encoder.Encode(jsonrpcResponseOut{
					JSONRPC: jsonrpcVersion,
					Error:   &jsonrpcErrorOut{Code: jsonrpcInvalidRequest, Message: "invalid request: empty batch"},
					ID:      json.RawMessage("null"),
				})
				writeMu.Unlock()
				continue
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				s.processJSONRPCBatch(ctx, frames, encoder, writeMu)
			}()
			continue
		}

		// Single request/notification.
		var frame jsonrpcFrame
		if err := json.Unmarshal(raw, &frame); err != nil {
			writeMu.Lock()
			encoder.Encode(jsonrpcResponseOut{
				JSONRPC: jsonrpcVersion,
				Error:   &jsonrpcErrorOut{Code: jsonrpcParseError, Message: "parse error: " + err.Error()},
				ID:      json.RawMessage("null"),
			})
			writeMu.Unlock()
			continue
		}

		if frame.isNotification() {
			wg.Add(1)
			go func() {
				defer wg.Done()
				s.dispatchJSONRPCNotification(ctx, frame)
			}()
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, _ := s.dispatchJSONRPCRequest(ctx, frame)
			if resp != nil {
				writeMu.Lock()
				recordErr(encoder.Encode(resp))
				writeMu.Unlock()
			}
		}()
	}
}

// processJSONRPCBatch fans a batch of frames out concurrently, then writes the
// collected responses as a single JSON array. Notifications contribute no
// response and an all-notification batch writes nothing at all.
func (s *Server) processJSONRPCBatch(ctx context.Context, frames []jsonrpcFrame, encoder *json.Encoder, writeMu *sync.Mutex) {
	var responses []jsonrpcResponseOut
	var respMu sync.Mutex
	var batchWg sync.WaitGroup

	for i := range frames {
		frame := frames[i]
		if frame.isNotification() {
			batchWg.Add(1)
			go func() {
				defer batchWg.Done()
				s.dispatchJSONRPCNotification(ctx, frame)
			}()
			continue
		}
		batchWg.Add(1)
		go func() {
			defer batchWg.Done()
			resp, _ := s.dispatchJSONRPCRequest(ctx, frame)
			if resp != nil {
				respMu.Lock()
				responses = append(responses, *resp)
				respMu.Unlock()
			}
		}()
	}
	batchWg.Wait()

	if len(responses) == 0 {
		return
	}
	writeMu.Lock()
	encoder.Encode(responses)
	writeMu.Unlock()
}

// dispatchJSONRPCRequest resolves and invokes the handler for a request frame
// on a fresh evaluator, returning the outbound response (never nil) and the
// handler reference used (for logging).
func (s *Server) dispatchJSONRPCRequest(ctx context.Context, frame jsonrpcFrame) (*jsonrpcResponseOut, string) {
	id := frame.ID
	if len(id) == 0 {
		id = json.RawMessage("null")
	}

	if frame.JSONRPC != nil && strings.TrimSpace(string(frame.JSONRPC)) != `"2.0"` && len(frame.JSONRPC) > 0 {
		return &jsonrpcResponseOut{
			JSONRPC: jsonrpcVersion,
			Error:   &jsonrpcErrorOut{Code: jsonrpcInvalidRequest, Message: "invalid request: jsonrpc must be \"2.0\""},
			ID:      id,
		}, ""
	}
	if frame.Method == "" {
		return &jsonrpcResponseOut{
			JSONRPC: jsonrpcVersion,
			Error:   &jsonrpcErrorOut{Code: jsonrpcInvalidRequest, Message: "invalid request: missing method"},
			ID:      id,
		}, ""
	}

	handlerRef, ok := s.jsonrpcMethods[frame.Method]
	if !ok {
		return &jsonrpcResponseOut{
			JSONRPC: jsonrpcVersion,
			Error:   &jsonrpcErrorOut{Code: jsonrpcMethodNotFound, Message: "method not found: " + frame.Method},
			ID:      id,
		}, handlerRef
	}

	params, perr := decodeParams(frame.Params)
	if perr != nil {
		return &jsonrpcResponseOut{
			JSONRPC: jsonrpcVersion,
			Error:   &jsonrpcErrorOut{Code: jsonrpcInvalidParams, Message: "invalid params: " + perr.Error()},
			ID:      id,
		}, handlerRef
	}

	result := s.runJSONRPCHandler(handlerRef, params)
	if ctx.Err() != nil {
		return &jsonrpcResponseOut{
			JSONRPC: jsonrpcVersion,
			Error:   &jsonrpcErrorOut{Code: jsonrpcInternalError, Message: "request cancelled"},
			ID:      id,
		}, handlerRef
	}

	// Handler returned an explicit JSON-RPC error object.
	if extlibs.IsJSONRPCError(result) {
		code, _ := result.(*object.Instance).Fields["code"].AsInt()
		message, _ := result.(*object.Instance).Fields["message"].AsString()
		errOut := &jsonrpcErrorOut{Code: int(code), Message: message}
		if data, present := result.(*object.Instance).Fields["data"]; present {
			if _, isNull := data.(*object.Null); !isNull {
				errOut.Data = conversion.ToGo(data)
			}
		}
		return &jsonrpcResponseOut{
			JSONRPC: jsonrpcVersion,
			Error:   errOut,
			ID:      id,
		}, handlerRef
	}

	// Scriptling error / exception -> server error.
	if errObj, ok := result.(*object.Error); ok {
		return &jsonrpcResponseOut{
			JSONRPC: jsonrpcVersion,
			Error:   &jsonrpcErrorOut{Code: jsonrpcServerError, Message: errObj.Message},
			ID:      id,
		}, handlerRef
	}
	if exObj, ok := result.(*object.Exception); ok {
		return &jsonrpcResponseOut{
			JSONRPC: jsonrpcVersion,
			Error:   &jsonrpcErrorOut{Code: jsonrpcServerError, Message: exObj.Message},
			ID:      id,
		}, handlerRef
	}

	return &jsonrpcResponseOut{
		JSONRPC: jsonrpcVersion,
		Result:  conversion.ToGo(result),
		ID:      id,
	}, handlerRef
}

// dispatchJSONRPCNotification resolves and invokes a notification handler on a
// fresh evaluator. The result is discarded (no response is written).
func (s *Server) dispatchJSONRPCNotification(ctx context.Context, frame jsonrpcFrame) {
	handlerRef, ok := s.jsonrpcNotifications[frame.Method]
	if !ok {
		// Unknown notifications are silently ignored per JSON-RPC 2.0.
		return
	}
	params, perr := decodeParams(frame.Params)
	if perr != nil {
		Log.Debug("JSON-RPC notification params decode error", "name", frame.Method, "error", perr)
		return
	}
	result := s.runJSONRPCHandler(handlerRef, params)
	if errObj, ok := result.(*object.Error); ok {
		Log.Debug("JSON-RPC notification handler error", "name", frame.Method, "error", errObj.Message)
	}
	if exObj, ok := result.(*object.Exception); ok {
		Log.Debug("JSON-RPC notification handler exception", "name", frame.Method, "error", exObj.Message)
	}
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
