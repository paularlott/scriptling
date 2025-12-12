# Multi-Environment Logging Example

This example demonstrates how the logging library maintains proper isolation between different Scriptling environments.

## What It Shows

When you run this example, it creates two separate Scriptling environments, each with its own logger instance:

1. **Environment 1** - Uses a logger with group name "scriptling1"
2. **Environment 2** - Uses a logger with group name "scriptling2"

Both environments execute the same Python script (`example.py`) but produce output with different group prefixes, proving that:

- Each environment has its own logger instance
- Loggers are not shared between environments
- Each environment can have different logging configurations

## Running the Example

```bash
go build -o multi_env_example main.go
./multi_env_example
```

## Key Points

- The `RegisterLoggingLibrary` function creates a new library instance for each environment
- Each library instance has its own logger that was passed during registration
- This allows different environments to log to different destinations with different configurations
- There's no global state shared between environments