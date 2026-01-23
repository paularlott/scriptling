package main

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
)

// MockWebSocketConn simulates a websocket connection with separate input/output streams
type MockWebSocketConn struct {
	input  io.Reader
	output io.Writer
}

func main() {
	fmt.Println("=== Custom I/O Streams Example ===")

	// Example 1: Simple custom I/O
	example1()

	// Example 2: Simulated websocket scenario
	example2()

	// Example 3: Multiple parallel sessions
	example3()
}

func example1() {
	fmt.Println("Example 1: Basic custom I/O")
	fmt.Println("----------------------------")

	p := scriptling.New()
	extlibs.RegisterConsoleLibrary(p)

	// Setup custom streams
	input := strings.NewReader("World\n")
	output := &bytes.Buffer{}

	p.SetInputReader(input)
	p.SetOutputWriter(output)

	script := `
import sl.console as console
name = console.input("Enter your name: ")
print("Hello, " + name + "!")
`

	_, err := p.Eval(script)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Output: %s\n", output.String())
}

func example2() {
	fmt.Println("\nExample 2: Simulated WebSocket")
	fmt.Println("-------------------------------")

	// Simulate a websocket connection
	clientInput := "Alice 42\n"
	serverOutput := &bytes.Buffer{}

	conn := &MockWebSocketConn{
		input:  strings.NewReader(clientInput),
		output: serverOutput,
	}

	// Server-side: Create scriptling instance with websocket I/O
	p := scriptling.New()
	extlibs.RegisterConsoleLibrary(p)

	p.SetInputReader(conn.input)
	p.SetOutputWriter(conn.output)

	// Execute remote script
	script := `
import sl.console as console

print("Welcome to remote execution!")
data = console.input("Enter name and age: ")
parts = data.split()
name = parts[0]
age = parts[1]

print("User: " + name + ", Age: " + age)
print("Script completed successfully")
`

	_, err := p.Eval(script)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Server output (sent to client):\n%s", serverOutput.String())
}

func example3() {
	fmt.Println("\nExample 3: Multiple Parallel Sessions")
	fmt.Println("--------------------------------------")

	// Simulate 3 concurrent websocket connections
	sessions := []struct {
		name  string
		input string
	}{
		{"Session-A", "UserA\n"},
		{"Session-B", "UserB\n"},
		{"Session-C", "UserC\n"},
	}

	for _, session := range sessions {
		output := &bytes.Buffer{}

		p := scriptling.New()
		extlibs.RegisterConsoleLibrary(p)

		p.SetInputReader(strings.NewReader(session.input))
		p.SetOutputWriter(output)

		script := `
import sl.console as console
user = console.input("Username: ")
print("Authenticated: " + user)
`

		_, err := p.Eval(script)
		if err != nil {
			fmt.Printf("%s Error: %v\n", session.name, err)
			continue
		}

		fmt.Printf("%s Output: %s", session.name, output.String())
	}
}
