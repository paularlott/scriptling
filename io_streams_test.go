package scriptling

import (
	"bytes"
	"strings"
	"sync"
	"testing"

	"github.com/paularlott/scriptling/extlibs/console"
)

func TestCustomInputOutput(t *testing.T) {
	p := New()
	console.Register(p)

	// Setup custom input and output
	input := strings.NewReader("Alice\n")
	output := &bytes.Buffer{}

	p.SetInputReader(input)
	p.SetOutputWriter(output)

	// Test script that reads input and writes output
	script := `
import scriptling.console as console
name = console.input("Enter name: ")
print("Hello, " + name + "!")
`

	_, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	expected := "Enter name: Hello, Alice!\n"
	if output.String() != expected {
		t.Errorf("Expected output %q, got %q", expected, output.String())
	}
}

func TestParallelInputOutput(t *testing.T) {
	// Test that multiple scriptling instances can run in parallel with different I/O streams
	var wg sync.WaitGroup

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			p := New()
			console.Register(p)

			// Each instance has its own I/O streams
			input := strings.NewReader("User" + string(rune('A'+id)) + "\n")
			output := &bytes.Buffer{}

			p.SetInputReader(input)
			p.SetOutputWriter(output)

			script := `
import scriptling.console as console
name = console.input()
print("Hello from " + name)
`

			_, err := p.Eval(script)
			if err != nil {
				t.Errorf("Instance %d: Eval failed: %v", id, err)
				return
			}

			expected := "Hello from User" + string(rune('A'+id)) + "\n"
			if output.String() != expected {
				t.Errorf("Instance %d: Expected %q, got %q", id, expected, output.String())
			}
		}(i)
	}

	wg.Wait()
}

func TestDefaultInputOutput(t *testing.T) {
	// Test that defaults work (os.Stdin/os.Stdout)
	p := New()
	console.Register(p)

	// Just verify the library is registered and can be imported
	script := `
import scriptling.console as console
# Don't actually call input() as it would block waiting for stdin
"ok"
`

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	str, objErr := result.AsString()
	if objErr != nil {
		t.Fatalf("Result conversion failed: %v", objErr)
	}

	if str != "ok" {
		t.Errorf("Expected 'ok', got %q", str)
	}
}

func TestMultipleInputReads(t *testing.T) {
	p := New()
	console.Register(p)

	// Test reading a single line with multiple words
	input := strings.NewReader("Hello World Test\n")
	output := &bytes.Buffer{}

	p.SetInputReader(input)
	p.SetOutputWriter(output)

	script := `
import scriptling.console as console
line = console.input("Enter text: ")
words = line.split()
print("Word count: " + str(len(words)))
`

	_, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	expected := "Enter text: Word count: 3\n"
	if output.String() != expected {
		t.Errorf("Expected %q, got %q", expected, output.String())
	}
}

func TestInputOutputInFunction(t *testing.T) {
	p := New()
	console.Register(p)

	input := strings.NewReader("test\n")
	output := &bytes.Buffer{}

	p.SetInputReader(input)
	p.SetOutputWriter(output)

	script := `
import scriptling.console as console

def greet():
    name = console.input("Name: ")
    print("Hi " + name)

greet()
`

	_, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	expected := "Name: Hi test\n"
	if output.String() != expected {
		t.Errorf("Expected %q, got %q", expected, output.String())
	}
}
