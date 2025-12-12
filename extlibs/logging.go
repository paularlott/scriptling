package extlibs

import (
	"context"
	"os"

	"github.com/paularlott/logger"
	logslog "github.com/paularlott/logger/slog"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// Logger object wrapper for Python getLogger() functionality
type loggerWrapper struct {
	logger.Logger
}

// Type() returns the logger type
func (l *loggerWrapper) Type() object.ObjectType { return object.INSTANCE_OBJ }

// Inspect returns a string representation
func (l *loggerWrapper) Inspect() string { return "<logging.Logger>" }

// Implementation of object.Object interface
func (l *loggerWrapper) AsString() (string, bool)                 { return l.Inspect(), true }
func (l *loggerWrapper) AsInt() (int64, bool)                     { return 0, false }
func (l *loggerWrapper) AsFloat() (float64, bool)                 { return 0, false }
func (l *loggerWrapper) AsBool() (bool, bool)                     { return true, true }
func (l *loggerWrapper) AsList() ([]object.Object, bool)          { return nil, false }
func (l *loggerWrapper) AsDict() (map[string]object.Object, bool) { return nil, false }

// CallMethod handles method calls on logger objects
func (l *loggerWrapper) CallMethod(method string, args ...object.Object) object.Object {
	switch method {
	case "debug":
		return l.logAtLevel(args, l.Logger.Debug)
	case "info":
		return l.logAtLevel(args, l.Logger.Info)
	case "warning":
		return l.logAtLevel(args, l.Logger.Warn)
	case "warn": // Python compatibility
		return l.logAtLevel(args, l.Logger.Warn)
	case "error":
		return l.logAtLevel(args, l.Logger.Error)
	case "critical":
		return l.logAtLevel(args, l.Logger.Error) // Map critical to error in Go
	default:
		return errors.NewError("logging.Logger has no method '%s'", method)
	}
}

// Helper function for logging at different levels
func (l *loggerWrapper) logAtLevel(args []object.Object, logFunc func(msg string, keysAndValues ...any)) object.Object {
	if len(args) == 0 {
		return errors.NewError("missing log message")
	}

	msg, ok := args[0].(*object.String)
	if !ok {
		return errors.NewError("log message must be a string")
	}

	logFunc(msg.Value)
	return &object.Boolean{Value: true}
}

// RegisterLoggingLibrary registers the logging library with the given registrar and optional logger
// Each environment gets its own logger instance
func RegisterLoggingLibrary(registrar interface{ RegisterLibrary(string, *object.Library) }, loggerInstance logger.Logger) {
	// Create the default logger for this environment
	var envLogger logger.Logger
	if loggerInstance != nil {
		envLogger = loggerInstance
	} else {
		// Create a default logger with stdout output
		envLogger = logslog.New(logslog.Config{
			Level:  "info",
			Format: "console",
			Writer: os.Stdout,
		})
	}

	// Create library with the environment-specific logger
	loggingLibrary := createLoggingLibrary(envLogger)
	registrar.RegisterLibrary(LoggingLibraryName, loggingLibrary)
}

// RegisterLoggingLibraryDefault registers the logging library with default configuration
func RegisterLoggingLibraryDefault(registrar interface{ RegisterLibrary(string, *object.Library) }) {
	RegisterLoggingLibrary(registrar, nil)
}

// createLoggingLibrary creates a logging library instance with the given logger
func createLoggingLibrary(defaultLogger logger.Logger) *object.Library {
	return object.NewLibrary(
		map[string]*object.Builtin{
			"getLogger": {
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
					// Optional name parameter
					var loggerName string = "scriptling"

					if len(args) > 0 {
						if name, ok := args[0].(*object.String); ok {
							loggerName = name.Value
						} else {
							return errors.NewError("logger name must be a string")
						}
					}

					// Create logger with the specified group
					logInstance := defaultLogger.WithGroup(loggerName)
					wrapper := &loggerWrapper{
						Logger: logInstance,
					}

					// Wrap as Python object
					return &object.Instance{
						Class: &object.Class{
							Name: "Logger",
							Methods: map[string]object.Object{
								"debug": &object.Builtin{Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
									// Skip the first arg (self) when it's a method call
									if len(args) > 0 {
										return wrapper.CallMethod("debug", args[1:]...)
									}
									return wrapper.CallMethod("debug")
								}},
								"info": &object.Builtin{Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
									if len(args) > 0 {
										return wrapper.CallMethod("info", args[1:]...)
									}
									return wrapper.CallMethod("info")
								}},
								"warning": &object.Builtin{Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
									if len(args) > 0 {
										return wrapper.CallMethod("warning", args[1:]...)
									}
									return wrapper.CallMethod("warning")
								}},
								"warn": &object.Builtin{Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
									if len(args) > 0 {
										return wrapper.CallMethod("warn", args[1:]...)
									}
									return wrapper.CallMethod("warn")
								}},
								"error": &object.Builtin{Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
									if len(args) > 0 {
										return wrapper.CallMethod("error", args[1:]...)
									}
									return wrapper.CallMethod("error")
								}},
								"critical": &object.Builtin{Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
									if len(args) > 0 {
										return wrapper.CallMethod("critical", args[1:]...)
									}
									return wrapper.CallMethod("critical")
								}},
							},
						},
						Fields: map[string]object.Object{
							"_internal": wrapper,
						},
					}
				},
			},
			"debug": {
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
					return logWithLogger(ctx, args, defaultLogger.Debug)
				},
			},
			"info": {
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
					return logWithLogger(ctx, args, defaultLogger.Info)
				},
			},
			"warning": {
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
					return logWithLogger(ctx, args, defaultLogger.Warn)
				},
			},
			"warn": {
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
					return logWithLogger(ctx, args, defaultLogger.Warn)
				},
			},
			"error": {
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
					return logWithLogger(ctx, args, defaultLogger.Error)
				},
			},
			"critical": {
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
					return logWithLogger(ctx, args, defaultLogger.Error)
				},
			},
		},
		map[string]object.Object{
			"DEBUG":    object.NewInteger(10),
			"INFO":     object.NewInteger(20),
			"WARNING":  object.NewInteger(30),
			"WARN":     object.NewInteger(30),
			"ERROR":    object.NewInteger(40),
			"CRITICAL": object.NewInteger(50),
		},
		"Python-style logging library",
	)
}

// Helper function for logging with a specific logger
func logWithLogger(ctx context.Context, args []object.Object, logFunc func(msg string, keysAndValues ...any)) object.Object {
	if len(args) == 0 {
		return errors.NewError("missing log message")
	}

	msg, ok := args[0].(*object.String)
	if !ok {
		return errors.NewError("log message must be a string")
	}

	logFunc(msg.Value)
	return &object.Boolean{Value: true}
}