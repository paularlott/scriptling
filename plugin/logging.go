package plugin

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/paularlott/logger"
)

type runtimeLogger struct {
	runtime *serverRuntime
	values  []any
}

// Logger returns a call-scoped proxy to the manager's host logger. It uses ctx
// to find the active plugin call runtime, so plugin code should request it from
// functions, constructors, methods, or property accessors and should not store
// it globally. If ctx is not attached to an active plugin call, Logger returns
// a no-op logger.
func Logger(ctx context.Context) logger.Logger {
	runtime, ok := ctx.Value(callbackRuntimeKey{}).(*serverRuntime)
	if !ok || runtime == nil {
		return logger.NewNullLogger()
	}
	return &runtimeLogger{runtime: runtime}
}

func (l *runtimeLogger) Trace(msg string, keysAndValues ...any) {
	l.log("trace", msg, keysAndValues...)
}

func (l *runtimeLogger) Debug(msg string, keysAndValues ...any) {
	l.log("debug", msg, keysAndValues...)
}

func (l *runtimeLogger) Info(msg string, keysAndValues ...any) {
	l.log("info", msg, keysAndValues...)
}

func (l *runtimeLogger) Warn(msg string, keysAndValues ...any) {
	l.log("warn", msg, keysAndValues...)
}

func (l *runtimeLogger) Error(msg string, keysAndValues ...any) {
	l.log("error", msg, keysAndValues...)
}

func (l *runtimeLogger) Fatal(msg string, keysAndValues ...any) {
	l.log("fatal", msg, keysAndValues...)
	os.Exit(1)
}

func (l *runtimeLogger) With(key string, value any) logger.Logger {
	next := make([]any, 0, len(l.values)+2)
	next = append(next, l.values...)
	next = append(next, key, value)
	return &runtimeLogger{runtime: l.runtime, values: next}
}

func (l *runtimeLogger) WithError(err error) logger.Logger {
	return l.With("error", err)
}

func (l *runtimeLogger) WithGroup(group string) logger.Logger {
	return l.With("group", group)
}

func (l *runtimeLogger) log(level, msg string, keysAndValues ...any) {
	if l == nil || l.runtime == nil {
		return
	}
	args := make([]Value, 0, len(l.values)+len(keysAndValues))
	for _, value := range l.values {
		args = append(args, goValueToTransport(value))
	}
	for _, value := range keysAndValues {
		args = append(args, goValueToTransport(value))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = l.runtime.callHostLog(ctx, logParams{
		Level:   level,
		Message: msg,
		Args:    args,
	})
}

func (r *serverRuntime) callHostLog(ctx context.Context, params logParams) error {
	id := r.nextID.Add(1)
	ch := make(chan rpcResponse, 1)
	r.mu.Lock()
	r.pending[id] = ch
	r.mu.Unlock()

	req := rpcRequest{JSONRPC: "2.0", ID: id, Method: "host.log", Params: params}
	r.writeMu.Lock()
	err := r.encoder.Encode(req)
	r.writeMu.Unlock()
	if err != nil {
		r.removePending(id)
		return err
	}

	select {
	case resp := <-ch:
		if resp.Error != nil {
			return resp.Error
		}
		if len(resp.Result) > 0 {
			var ignored Value
			_ = json.Unmarshal(resp.Result, &ignored)
		}
		return nil
	case <-ctx.Done():
		r.removePending(id)
		return ctx.Err()
	}
}
