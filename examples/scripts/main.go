package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/stdlib"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <script.py>\n", os.Args[0])
		os.Exit(1)
	}

	filename := os.Args[1]
	if filepath.Ext(filename) != ".py" {
		fmt.Fprintf(os.Stderr, "Error: File must have .py extension\n")
		os.Exit(1)
	}

	content, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	p := scriptling.New()

	// Register all standard libraries
	stdlib.RegisterAll(p)

	// Register extended libraries
	extlibs.RegisterRequestsLibrary(p)
	extlibs.RegisterSysLibrary(p)
	extlibs.RegisterSecretsLibrary(p)
	extlibs.RegisterSubprocessLibrary(p)
	extlibs.RegisterHTMLParserLibrary(p)
	extlibs.RegisterThreadsLibrary(p)
	extlibs.RegisterOSLibrary(p, []string{})
	extlibs.RegisterPathlibLibrary(p, []string{})

	// Register a test Scriptling library with documentation
	err = p.RegisterScriptLibrary("testlib", `
"""Test Library

This is a test library to demonstrate documentation features.
It provides basic utility functions.
"""

def greet(name):
    """Return a greeting for the given name.

    Args:
        name: The name to greet

    Returns:
        A greeting string
    """
    return "Hello, " + name + "!"

def add(a, b):
    """Add two numbers.

    Args:
        a: First number
        b: Second number

    Returns:
        The sum of a and b
    """
    return a + b

VERSION = "1.0.0"
`)
	if err != nil {
		fmt.Printf("Error registering test library: %v\n", err)
	}

	start := time.Now()
	_, err = p.EvalWithTimeout(30*time.Second, string(content))
	elapsed := time.Since(start)
	fmt.Printf("Execution took: %v\n", elapsed)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
