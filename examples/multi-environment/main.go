package main

import (
	"bytes"
	"fmt"
	"os"

	logslog "github.com/paularlott/logger/slog"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
)

func main() {
	fmt.Println("=== Multi-Environment Logging Example ===")
	fmt.Println("Demonstrating environment isolation with different loggers...\n")

	// Create two different output buffers to capture logs
	var buf1, buf2 bytes.Buffer

	// Create first logger for environment 1
	logger1 := logslog.New(logslog.Config{
		Level:  "debug",
		Format: "console",
		Writer: &buf1,
	}).WithGroup("scriptling1")

	// Create second logger for environment 2
	logger2 := logslog.New(logslog.Config{
		Level:  "debug",
		Format: "console",
		Writer: &buf2,
	}).WithGroup("scriptling2")

	// Create first scriptling environment
	p1 := scriptling.New()
	extlibs.RegisterLoggingLibrary(p1, logger1)

	// Create second scriptling environment
	p2 := scriptling.New()
	extlibs.RegisterLoggingLibrary(p2, logger2)

	// Read the example script
	scriptContent, err := os.ReadFile("example.py")
	if err != nil {
		fmt.Printf("Error reading script: %v\n", err)
		os.Exit(1)
	}

	// Execute script in first environment
	fmt.Println("Executing in environment 1 (scriptling1):")
	_, err = p1.Eval(string(scriptContent))
	if err != nil {
		fmt.Printf("Error in environment 1: %v\n", err)
	}

	// Execute script in second environment
	fmt.Println("\nExecuting in environment 2 (scriptling2):")
	_, err = p2.Eval(string(scriptContent))
	if err != nil {
		fmt.Printf("Error in environment 2: %v\n", err)
	}

	// Execute script in first environment
	fmt.Println("Executing in environment 1 (scriptling1):")
	_, err = p1.Eval(string(scriptContent))
	if err != nil {
		fmt.Printf("Error in environment 1: %v\n", err)
	}

	// Show the captured output from each environment
	fmt.Println("\n=== Environment 1 Output ===")
	fmt.Print(buf1.String())

	fmt.Println("\n=== Environment 2 Output ===")
	fmt.Print(buf2.String())

	fmt.Println("\nNote: Each environment used its own logger instance with different group names.")
	fmt.Println("Environment 1 logs have '[scriptling1]' prefix.")
	fmt.Println("Environment 2 logs have '[scriptling2]' prefix.")

	// Example completed - each environment maintained its own logger
}
