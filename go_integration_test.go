package scriptling

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

// TestGoIntegration_BasicUsage validates basic interpreter creation and execution examples
func TestGoIntegration_BasicUsage(t *testing.T) {
	t.Run("CreateInterpreter", func(t *testing.T) {
		p := New()
		stdlib.RegisterAll(p)

		_, err := p.Eval(`x = 5 + 3`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		result, objErr := p.GetVarAsInt("x")
		if objErr != nil || result != 8 {
			t.Errorf("expected 8, got %d", result)
		}
	})

	t.Run("SimpleExecution", func(t *testing.T) {
		p := New()
		result, err := p.Eval("x = 42")
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		x, objErr := p.GetVarAsInt("x")
		if objErr != nil || x != 42 {
			t.Errorf("expected 42, got %d", x)
		}

		// Result should be the last expression
		if resultInt, err := result.AsInt(); err == nil {
			if resultInt != 42 {
				t.Errorf("expected result 42, got %d", resultInt)
			}
		}
	})

	t.Run("MultiLineScript", func(t *testing.T) {
		p := New()

		script := `
def fibonacci(n):
    if n <= 1:
        return n
    else:
        return fibonacci(n - 1) + fibonacci(n - 2)

result = fibonacci(10)
`
		_, err := p.Eval(script)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		result, objErr := p.GetVarAsInt("result")
		if objErr != nil || result != 55 {
			t.Errorf("expected 55, got %d", result)
		}
	})
}

// TestGoIntegration_VariableExchange validates variable exchange examples
func TestGoIntegration_VariableExchange(t *testing.T) {
	t.Run("SetVariablesFromGo", func(t *testing.T) {
		p := New()

		// Set variables from Go
		p.SetVar("db_host", "prod.db.example.com")
		p.SetVar("cache_size", int64(1000))
		p.SetVar("debug", true)

		// Get them back
		dbHost, objErr := p.GetVarAsString("db_host")
		if objErr != nil || dbHost != "prod.db.example.com" {
			t.Errorf("expected 'prod.db.example.com', got %s", dbHost)
		}

		cacheSize, objErr := p.GetVarAsInt("cache_size")
		if objErr != nil || cacheSize != 1000 {
			t.Errorf("expected 1000, got %d", cacheSize)
		}

		debug, objErr := p.GetVarAsBool("debug")
		if objErr != nil || debug != true {
			t.Errorf("expected true, got %t", debug)
		}
	})

	t.Run("GetVariablesFromScriptling", func(t *testing.T) {
		p := New()

		// Execute script that sets variables
		_, err := p.Eval(`
x = 42
name = "Alice"
result = {"status": "success", "count": 10}
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		// Get variables using convenience methods
		if value, objErr := p.GetVarAsInt("x"); objErr == nil {
			if value != 42 {
				t.Errorf("expected 42, got %d", value)
			}
		} else {
			t.Error("Variable 'x' not found")
		}

		if name, objErr := p.GetVarAsString("name"); objErr == nil {
			if name != "Alice" {
				t.Errorf("expected 'Alice', got %s", name)
			}
		} else {
			t.Error("Variable 'name' not found")
		}

		// Get variables using generic GetVar (returns Go native types)
		if value, objErr := p.GetVar("result"); objErr == nil {
			if dict, ok := value.(map[string]interface{}); ok {
				if status, ok := dict["status"]; ok {
					if statusStr, ok := status.(string); ok {
						if statusStr != "success" {
							t.Errorf("expected 'success', got %s", statusStr)
						}
					}
				}
			}
		}
	})

	t.Run("GetComplexTypes", func(t *testing.T) {
		p := New()

		_, err := p.Eval(`
numbers = [1, 2, 3, 4, 5]
config = {"host": "localhost", "port": 8080}
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		// Get list
		if numbers, objErr := p.GetVarAsList("numbers"); objErr == nil {
			if len(numbers) != 5 {
				t.Errorf("expected 5 elements, got %d", len(numbers))
			}
		} else {
			t.Error("Variable 'numbers' not found or not a list")
		}

		// Get dict
		if config, objErr := p.GetVarAsDict("config"); objErr == nil {
			if host, ok := config["host"]; ok {
				if hostStr, err := host.AsString(); err == nil {
					if hostStr != "localhost" {
						t.Errorf("expected 'localhost', got %s", hostStr)
					}
				}
			}
		} else {
			t.Error("Variable 'config' not found or not a dict")
		}
	})
}

// TestGoIntegration_ScriptReturnValues validates script return value examples
func TestGoIntegration_ScriptReturnValues(t *testing.T) {
	t.Run("BasicReturnValues", func(t *testing.T) {
		p := New()

		result, err := p.Eval(`
x = 42
y = 24
x + y  # Last expression becomes return value
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		// Access the return value
		if intResult, err := result.AsInt(); err == nil {
			if intResult != 66 {
				t.Errorf("expected 66, got %d", intResult)
			}
		} else {
			t.Errorf("expected integer result, got error: %v", err)
		}
	})

	t.Run("ComplexReturnValues", func(t *testing.T) {
		p := New()

		result, err := p.Eval(`
data = {"name": "Alice", "age": 30, "active": True}
numbers = [1, 2, 3, 4, 5]

# Return computed result
{
    "user": data,
    "count": len(numbers),
    "sum": sum(numbers),
    "status": "success"
}
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		// Access dictionary return value
		if dict, err := result.AsDict(); err == nil {
			if status, ok := dict["status"]; ok {
				if statusStr, err := status.AsString(); err == nil {
					if statusStr != "success" {
						t.Errorf("expected 'success', got %s", statusStr)
					}
				}
			}
			if count, ok := dict["count"]; ok {
				if countVal, err := count.AsInt(); err == nil {
					if countVal != 5 {
						t.Errorf("expected 5, got %d", countVal)
					}
				}
			}
		} else {
			t.Errorf("expected dict result, got error: %v", err)
		}
	})

	t.Run("ReturnValueTypes", func(t *testing.T) {
		p := New()

		scripts := []struct {
			script    string
			typeCheck func(object.Object) error
		}{
			{`42`, func(o object.Object) error {
				if _, err := o.AsInt(); err != nil {
					return fmt.Errorf("expected integer, got %s", o.Type())
				}
				return nil
			}},
			{`"hello"`, func(o object.Object) error {
				if _, err := o.AsString(); err != nil {
					return fmt.Errorf("expected string, got %s", o.Type())
				}
				return nil
			}},
			{`3.14`, func(o object.Object) error {
				if _, err := o.AsFloat(); err != nil {
					return fmt.Errorf("expected float, got %s", o.Type())
				}
				return nil
			}},
			{`True`, func(o object.Object) error {
				if _, err := o.AsBool(); err != nil {
					return fmt.Errorf("expected boolean, got %s", o.Type())
				}
				return nil
			}},
			{`[1, 2, 3]`, func(o object.Object) error {
				if _, err := o.AsList(); err != nil {
					return fmt.Errorf("expected list, got %s", o.Type())
				}
				return nil
			}},
			{`{"key": "value"}`, func(o object.Object) error {
				if _, err := o.AsDict(); err != nil {
					return fmt.Errorf("expected dict, got %s", o.Type())
				}
				return nil
			}},
		}

		for _, tt := range scripts {
			result, err := p.Eval(tt.script)
			if err != nil {
				t.Errorf("script %q failed: %v", tt.script, err)
				continue
			}
			if err := tt.typeCheck(result); err != nil {
				t.Errorf("script %q type check failed: %v", tt.script, err)
			}
		}
	})

	t.Run("ProcessingReturnValues", func(t *testing.T) {
		p := New()

		result, err := p.Eval(`
# Process input data
input = [10, 20, 30, 40, 50]
filtered = [x for x in input if x > 25]
total = sum(filtered)

# Return processed result
{
    "original_count": len(input),
    "filtered": filtered,
    "total": total,
    "average": total / len(filtered)
}
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		// Process the returned dictionary
		if resultDict, err := result.AsDict(); err == nil {
			if count, ok := resultDict["original_count"]; ok {
				if countVal, err := count.AsInt(); err == nil {
					if countVal != 5 {
						t.Errorf("expected 5 original items, got %d", countVal)
					}
				}
			}

			if total, ok := resultDict["total"]; ok {
				if totalVal, err := total.AsInt(); err == nil {
					if totalVal != 120 {
						t.Errorf("expected 120 total, got %d", totalVal)
					}
				}
			}

			if filtered, ok := resultDict["filtered"]; ok {
				if filteredList, err := filtered.AsList(); err == nil {
					if len(filteredList) != 3 {
						t.Errorf("expected 3 filtered items, got %d", len(filteredList))
					}
				}
			}
		}
	})
}

// TestGoIntegration_CallFunction validates CallFunction examples
func TestGoIntegration_CallFunction(t *testing.T) {
	t.Run("CallingRegisteredFunctions", func(t *testing.T) {
		p := New()

		p.RegisterFunc("multiply", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			a, _ := args[0].AsInt()
			b, _ := args[1].AsInt()
			return &object.Integer{Value: a * b}
		})

		// Call with Go arguments
		result, err := p.CallFunction("multiply", int64(6), int64(7))
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		product, _ := result.AsInt()
		if product != 42 {
			t.Errorf("expected 42, got %d", product)
		}
	})

	t.Run("CallingScriptDefinedFunctions", func(t *testing.T) {
		p := New()

		// Define a function in script
		_, err := p.Eval(`
def greet(name):
    return 'Hello, ' + name
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		// Call it directly from Go
		result, err := p.CallFunction("greet", "World")
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		message, _ := result.AsString()
		if message != "Hello, World" {
			t.Errorf("expected 'Hello, World', got %s", message)
		}
	})

	t.Run("TypeConversions", func(t *testing.T) {
		p := New()

		p.RegisterFunc("identity", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return args[0]
		})

		// Test various Go types
		tests := []struct {
			input     interface{}
			typeCheck func(object.Object) bool
		}{
			{int64(42), func(o object.Object) bool { _, ok := o.AsInt(); return ok == nil }},
			{3.14, func(o object.Object) bool { _, ok := o.AsFloat(); return ok == nil }},
			{"hello", func(o object.Object) bool { _, ok := o.AsString(); return ok == nil }},
			{true, func(o object.Object) bool { _, ok := o.AsBool(); return ok == nil }},
			{[]int{1, 2, 3}, func(o object.Object) bool { _, ok := o.AsList(); return ok == nil }},
			{map[string]int{"a": 1}, func(o object.Object) bool { _, ok := o.AsDict(); return ok == nil }},
		}

		for _, tt := range tests {
			result, err := p.CallFunction("identity", tt.input)
			if err != nil {
				t.Errorf("CallFunction(%v) failed: %v", tt.input, err)
				continue
			}
			if !tt.typeCheck(result) {
				t.Errorf("Type conversion failed for %v", tt.input)
			}
		}
	})

	t.Run("ErrorHandling", func(t *testing.T) {
		p := New()

		// Test calling non-existent function
		_, err := p.CallFunction("nonexistent", "arg")
		if err == nil {
			t.Error("expected error for non-existent function, got nil")
		}

		// Test function that returns an error
		p.RegisterFunc("fail", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return &object.Error{Message: "intentional error"}
		})

		_, err = p.CallFunction("fail")
		if err == nil {
			t.Error("expected error for function returning Error object, got nil")
		}
		// Error message should contain the Error object's message
		if err != nil && !strings.Contains(err.Error(), "intentional error") {
			t.Errorf("expected error to contain 'intentional error', got %v", err)
		}
	})

	t.Run("UsingContext", func(t *testing.T) {
		p := New()

		p.RegisterFunc("slow_operation", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			select {
			case <-ctx.Done():
				return &object.Error{Message: "cancelled"}
			case <-time.After(10 * time.Millisecond):
				return &object.String{Value: "completed"}
			}
		})

		// Test with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
		defer cancel()

		_, err := p.CallFunctionWithContext(ctx, "slow_operation")
		if err == nil {
			t.Error("expected error due to timeout, got nil")
		}
		// Error should be due to cancellation
		if err != nil && !strings.Contains(err.Error(), "cancelled") {
			t.Errorf("expected error to contain 'cancelled', got %v", err)
		}
	})

	t.Run("UsingKeywordArguments", func(t *testing.T) {
		p := New()

		// Register a function with keyword arguments
		p.RegisterFunc("format", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			text, _ := args[0].AsString()
			prefix := kwargs.MustGetString("prefix", "")
			suffix := kwargs.MustGetString("suffix", "")
			return &object.String{Value: prefix + text + suffix}
		})

		// Call with keyword arguments
		result, err := p.CallFunction("format", "hello",
			Kwargs{
				"prefix": ">> ",
				"suffix": " <<",
			})
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		message, _ := result.AsString()
		if message != ">> hello <<" {
			t.Errorf("expected '>> hello <<', got %s", message)
		}
	})

	t.Run("ScriptFunctionsWithKwargs", func(t *testing.T) {
		p := New()

		// Define a script function with default keyword arguments
		_, err := p.Eval(`
def greet(name, greeting="Hello", punctuation="!"):
    return greeting + ", " + name + punctuation
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		// Call with positional args only
		result, err := p.CallFunction("greet", "World")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		text, _ := result.AsString()
		if text != "Hello, World!" {
			t.Errorf("expected 'Hello, World!', got %s", text)
		}

		// Call with keyword arguments
		result, err = p.CallFunction("greet", "Alice",
			Kwargs{
				"greeting":    "Hi",
				"punctuation": "?",
			})
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		text, _ = result.AsString()
		if text != "Hi, Alice?" {
			t.Errorf("expected 'Hi, Alice?', got %s", text)
		}
	})
}

// TestGoIntegration_ProgrammaticLibraryImport validates programmatic import examples
func TestGoIntegration_ProgrammaticLibraryImport(t *testing.T) {
	t.Run("ImportBeforeExecution", func(t *testing.T) {
		p := New()
		stdlib.RegisterAll(p)

		// Import libraries programmatically
		p.Import("json")
		p.Import("math")

		// Now use libraries in scripts without import statements
		_, err := p.Eval(`
data = json.dumps({"numbers": [1, 2, 3]})
result = math.sqrt(16)
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		result, objErr := p.GetVarAsFloat("result")
		if objErr != nil || result != 4.0 {
			t.Errorf("expected 4.0, got %f", result)
		}
	})
}

// TestGoIntegration_ObjectTypes validates object type handling examples
func TestGoIntegration_ObjectTypes(t *testing.T) {
	t.Run("StringObjects", func(t *testing.T) {
		strObj := &object.String{Value: "hello"}
		if strObj.Value != "hello" {
			t.Errorf("expected 'hello', got %s", strObj.Value)
		}
	})

	t.Run("IntegerObjects", func(t *testing.T) {
		intObj := &object.Integer{Value: 42}
		if intObj.Value != 42 {
			t.Errorf("expected 42, got %d", intObj.Value)
		}
	})

	t.Run("BooleanObjects", func(t *testing.T) {
		boolObj := &object.Boolean{Value: true}
		if boolObj.Value != true {
			t.Errorf("expected true, got %t", boolObj.Value)
		}
	})

	t.Run("FloatObjects", func(t *testing.T) {
		floatObj := &object.Float{Value: 3.14}
		if floatObj.Value != 3.14 {
			t.Errorf("expected 3.14, got %f", floatObj.Value)
		}
	})
}

// TestGoIntegration_OutputCapture validates output capture examples
func TestGoIntegration_OutputCapture(t *testing.T) {
	t.Run("DefaultBehavior", func(t *testing.T) {
		p := New()

		// Just verify it doesn't crash - output goes to stdout
		_, err := p.Eval(`print("Hello World")`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
	})

	t.Run("CaptureOutput", func(t *testing.T) {
		p := New()
		p.EnableOutputCapture()

		p.Eval(`
print("Line 1")
print("Line 2")
print("Result:", 42)
`)

		// Get captured output
		output := p.GetOutput()
		expected := "Line 1\nLine 2\nResult: 42\n"
		if output != expected {
			t.Errorf("expected %q, got %q", expected, output)
		}

		// Buffer is cleared after GetOutput()
		output2 := p.GetOutput()
		if output2 != "" {
			t.Errorf("expected empty buffer, got %q", output2)
		}
	})

	t.Run("TestingExample", func(t *testing.T) {
		p := New()
		p.EnableOutputCapture()

		p.Eval(`print("test result:", 42)`)
		output := p.GetOutput()

		expected := "test result: 42\n"
		if output != expected {
			t.Errorf("Expected %q, got %q", expected, output)
		}
	})
}

// TestGoIntegration_ErrorHandling validates error handling examples
func TestGoIntegration_ErrorHandling(t *testing.T) {
	t.Run("CheckEvalErrors", func(t *testing.T) {
		p := New()

		// Test syntax error
		_, err := p.Eval(`if x`)
		if err == nil {
			t.Error("expected syntax error, got nil")
		}

		// Test runtime error
		_, err = p.Eval(`y = 1 / 0`)
		if err == nil {
			t.Error("expected runtime error, got nil")
		}
	})

	t.Run("CheckVariableExists", func(t *testing.T) {
		p := New()

		p.Eval(`x = 42`)

		// Variable exists
		if value, objErr := p.GetVar("x"); objErr == nil {
			if valueInt, ok := value.(int64); ok {
				if valueInt != 42 {
					t.Errorf("expected 42, got %d", valueInt)
				}
			}
		} else {
			t.Error("Variable 'x' should exist")
		}

		// Variable doesn't exist
		_, objErr := p.GetVar("nonexistent")
		if objErr == nil {
			t.Error("expected error for non-existent variable")
		}
	})
}

// TestGoIntegration_CommonPatterns validates common pattern examples
func TestGoIntegration_CommonPatterns(t *testing.T) {
	t.Run("ConfigurationScripts", func(t *testing.T) {
		p := New()
		p.SetVar("env", "production")

		configScript := `
if env == "production":
    db_host = "prod.db.example.com"
    cache_size = 1000
else:
    db_host = "dev.db.example.com"
    cache_size = 100
`

		_, err := p.Eval(configScript)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		dbHost, objErr := p.GetVarAsString("db_host")
		if objErr != nil || dbHost != "prod.db.example.com" {
			t.Errorf("expected 'prod.db.example.com', got %s", dbHost)
		}

		cacheSize, objErr := p.GetVarAsInt("cache_size")
		if objErr != nil || cacheSize != 1000 {
			t.Errorf("expected 1000, got %d", cacheSize)
		}
	})

	t.Run("DataProcessingPipeline", func(t *testing.T) {
		p := New()
		stdlib.RegisterAll(p)

		jsonString := `{"items": [{"name": "a", "active": true}, {"name": "b", "active": false}, {"name": "c", "active": true}]}`

		p.SetVar("raw_data", jsonString)

		pipeline := `
import json

data = json.parse(raw_data)
processed = []

for item in data["items"]:
    if item["active"]:
        processed.append(item["name"])

result = json.stringify(processed)
`

		_, err := p.Eval(pipeline)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		result, objErr := p.GetVarAsString("result")
		if objErr != nil {
			t.Errorf("expected string result, got error: %v", objErr)
		}

		// Result should be JSON array of active item names
		if !strings.Contains(result, "a") || !strings.Contains(result, "c") {
			t.Errorf("expected result to contain 'a' and 'c', got %s", result)
		}
	})

	t.Run("ReuseInterpreters", func(t *testing.T) {
		p := New()

		scripts := []string{
			`x = 1`,
			`x = x + 1`,
			`x = x + 1`,
		}

		for _, script := range scripts {
			_, err := p.Eval(script)
			if err != nil {
				t.Fatalf("error: %v", err)
			}
		}

		result, objErr := p.GetVarAsInt("x")
		if objErr != nil || result != 3 {
			t.Errorf("expected 3, got %d", result)
		}
	})
}

// TestGoIntegration_CompleteExample validates the complete integration example
func TestGoIntegration_CompleteExample(t *testing.T) {
	p := New()

	// Register libraries
	stdlib.RegisterAll(p)
	extlibs.RegisterRequestsLibrary(p)

	// Set configuration from Go
	p.SetVar("api_base", "https://api.example.com")
	p.SetVar("timeout", 30)

	// Simulated response (since we can't make real HTTP requests in tests)
	p.SetVar("mock_response", `{"status": 200, "body": "[{\"name\":\"Alice\",\"active\":true},{\"name\":\"Bob\",\"active\":false}]"}`)

	// Execute automation script (modified to use mock response)
	script := `
import json

# Use mock response instead of making real HTTP request
response_data = json.parse(mock_response)

if response_data["status"] == 200:
    users = json.parse(response_data["body"])
    print("Found " + str(len(users)) + " users")

    # Process each user
    processed_count = 0
    for user in users:
        if user["active"]:
            print("Processing user: " + user["name"])
            processed_count = processed_count + 1

    success = True
else:
    print("API call failed: " + str(response_data["status"]))
    processed_count = 0
    success = False

# Return summary
{
    "success": success,
    "total_users": len(users),
    "processed_count": processed_count,
    "api_status": response_data["status"]
}
`

	// Enable output capture to verify print statements
	p.EnableOutputCapture()

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script error: %v", err)
	}

	// Check captured output
	output := p.GetOutput()
	if !strings.Contains(output, "Found 2 users") {
		t.Errorf("Expected output to contain 'Found 2 users', got: %s", output)
	}
	if !strings.Contains(output, "Processing user: Alice") {
		t.Errorf("Expected output to contain 'Processing user: Alice', got: %s", output)
	}

	// Access return value from script
	if resultDict, err := result.AsDict(); err == nil {
		if success, ok := resultDict["success"]; ok {
			if successVal, err := success.AsBool(); err == nil {
				if !successVal {
					t.Errorf("expected success=true, got false")
				}
			}
		}
		if processed, ok := resultDict["processed_count"]; ok {
			if count, err := processed.AsInt(); err == nil {
				if count != 1 {
					t.Errorf("expected processed_count=1, got %d", count)
				}
			}
		}
		if totalUsers, ok := resultDict["total_users"]; ok {
			if count, err := totalUsers.AsInt(); err == nil {
				if count != 2 {
					t.Errorf("expected total_users=2, got %d", count)
				}
			}
		}
	}
}
