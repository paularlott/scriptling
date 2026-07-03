package plugin

import (
	"context"
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
	if r.peer == nil {
		return nil
	}
	// Host log records are sent as JSON-RPC requests (the host responds with a
	// null result), matching the original synchronous behaviour.
	return r.peer.Client().Call(ctx, "host.log", params, nil)
}
