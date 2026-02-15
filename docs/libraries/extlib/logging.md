# logging

The `logging` library provides Python-style logging functionality, compatible with the basic Python logging interface. It uses the [paularlott/logger](https://github.com/paularlott/logger) library under the hood with slog integration.

**Note**: This is an extended library and not enabled by default. You must register it explicitly.

## Available Functions

| Function               | Description                               |
| ---------------------- | ----------------------------------------- |
| `getLogger(name=None)` | Get a logger instance with specified name |

## Registration

To use this library, you must register it:

```go
import "github.com/paularlott/scriptling/extlibs"
import logslog "github.com/paularlott/logger/slog"

// In your main function
p := scriptling.New()

// Register with a custom logger (recommended)
loggerInstance := logslog.New(logslog.Config{
    Level:  "debug",
    Format: "console",
    Writer: os.Stdout,
}).WithGroup("scriptling")
extlibs.RegisterLoggingLibrary(p, loggerInstance)
```

## Environment Isolation

Each Scriptling environment gets its own logger instance. This means:

- Multiple environments can have different loggers without interfering
- Loggers can have different output destinations, levels, and formats
- The logger instance is tied to the environment it's registered with

```go
// Environment 1
p1 := scriptling.New()
logger1 := logslog.New(logslog.Config{
    Writer: os.Stdout,
}).WithGroup("app1")
extlibs.RegisterLoggingLibrary(p1, logger1)

// Environment 2 (different logger)
p2 := scriptling.New()
logger2 := logslog.New(logslog.Config{
    Writer: someOtherWriter,
}).WithGroup("app2")
extlibs.RegisterLoggingLibrary(p2, logger2)
```

## Functions

### logging.getLogger(name=None)

Creates and returns a logger object with the specified name. If no name is provided, defaults to "scriptling".

**Parameters:**

- `name` (str, optional): Logger name. Used as the group name in the underlying logger.

The logger name will be displayed as a nested group in the log output, e.g., `[scriptling.componentName]`.

**Returns:**

- Logger object with methods for logging at different levels

### Module-level logging functions

These functions provide convenient access to logging without creating a logger instance:

- `logging.debug(msg)` - Log a debug message
- `logging.info(msg)` - Log an info message
- `logging.warning(msg)` - Log a warning message
- `logging.warn(msg)` - Alias for warning (Python compatibility)
- `logging.error(msg)` - Log an error message
- `logging.critical(msg)` - Log a critical message

## Logger Methods

When you get a logger using `getLogger()`, it returns an object with these methods:

- `logger.debug(msg)` - Log a debug message
- `logger.info(msg)` - Log an info message
- `logger.warning(msg)` - Log a warning message
- `logger.warn(msg)` - Alias for warning (Python compatibility)
- `logger.error(msg)` - Log an error message
- `logger.critical(msg)` - Log a critical message

## Constants

The library defines standard logging level constants:

- `logging.DEBUG = 10`
- `logging.INFO = 20`
- `logging.WARNING = 30`
- `logging.WARN = 30` (alias for WARNING)
- `logging.ERROR = 40`
- `logging.CRITICAL = 50`

## Examples

### Basic Usage

```python
import logging

# Simple logging using module functions
logging.warning('Watch out!')
logging.info('This is an info message')
logging.error('Something went wrong')
```

### Using Named Loggers

```python
import logging

# Get a logger with a specific name
logger = logging.getLogger('myApp')

# Log messages with the logger
logger.debug('debug message')
logger.info('info message')
logger.warning('warn message')
logger.error('error message')
logger.critical('critical message')
```

### Multiple Loggers

```python
import logging

# Different loggers for different components
app_logger = logging.getLogger('app')
db_logger = logging.getLogger('database')
api_logger = logging.getLogger('api')

app_logger.info('Application starting')
db_logger.debug('Connecting to database')
api_logger.warning('Rate limit approaching')
```

## Configuration

The default logger is configured with:

- Level: INFO
- Format: console (human-readable)
- Output: Standard output
- Group: "scriptling"

All loggers created with `getLogger()` will inherit these settings unless specified otherwise.

## Differences from Python's logging module

This is a simplified implementation focused on basic logging functionality:

1. No configuration API - loggers use the default configuration
2. No handlers - output is always to stdout
3. No formatters - output format is fixed
4. No filtering - all messages at the configured level are shown
5. No hierarchy - named loggers are independent (except for the shared group prefix)
6. No file logging - as per requirements

## Integration with Scriptling

The logging library integrates seamlessly with Scriptling's output capture system. All log messages will be captured along with any print statements or other output from your scripts.
