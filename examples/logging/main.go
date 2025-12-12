package main

import (
	"fmt"
	"os"
	"path/filepath"

	logslog "github.com/paularlott/logger/slog"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
)

func main() {
	// Create a logger for this environment
	loggerInstance := logslog.New(logslog.Config{
		Level:  "debug", // Show all messages
		Format: "console",
		Writer: os.Stdout,
	})

	// Create a new scriptling environment
	p := scriptling.New()

	// Register the logging library with our logger instance
	// The logger is specific to this environment and not shared
	extlibs.RegisterLoggingLibrary(p, loggerInstance.WithGroup("scriptling"))

	// Example of using the Go logger directly (not accessible from Python)
	loggerInstance.Info("Starting logging example", "version", "1.0")

	// Read and execute the Python example script
	scriptPath := filepath.Join("example.py")
	scriptContent, err := os.ReadFile(scriptPath)
	if err != nil {
		fmt.Printf("Error reading script: %v\n", err)
		os.Exit(1)
	}

	// Execute the Python script
	result, err := p.Eval(string(scriptContent))
	if err != nil {
		fmt.Printf("Error executing script: %v\n", err)
		os.Exit(1)
	}

	// Print the result
	fmt.Printf("\nScript execution completed. Result: %v\n", result.Inspect())
}
