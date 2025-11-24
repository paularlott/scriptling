package main

import (
	"fmt"
	"os"
	"path/filepath"

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
	
	// Register HTTP library for scripts that need it
	p.RegisterLibrary("http", extlibs.HTTPLibrary())
	
	_, err = p.Eval(string(content))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
