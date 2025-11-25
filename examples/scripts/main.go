package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
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

	// Register requests library for scripts that need it
	p.RegisterLibrary("requests", extlibs.RequestsLibrary())

	start := time.Now()
	_, err = p.EvalWithTimeout(30*time.Second, string(content))
	elapsed := time.Since(start)
	fmt.Printf("Execution took: %v\n", elapsed)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
