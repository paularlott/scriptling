package scriptling

import (
	"testing"

	"github.com/paularlott/scriptling/stdlib"
)

func TestLibraryNotAvailableWithoutImport(t *testing.T) {
	p := New()

	// Try to use json without importing
	_, err := p.Eval(`result = json.loads('{"test": "value"}')`)
	if err == nil {
		t.Error("Expected error when using json without import, got nil")
	}

	// Try to use math without importing
	_, err = p.Eval(`result = math.sqrt(16)`)
	if err == nil {
		t.Error("Expected error when using math without import, got nil")
	}
}

func TestLibraryAvailableAfterImport(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.JSONLibraryName, stdlib.JSONLibrary)

	// Import and use json
	_, err := p.Eval(`
import json
result = json.loads('{"test": "value"}')
`)
	if err != nil {
		t.Errorf("Expected no error after importing json, got: %v", err)
	}

	_, objErr := p.GetVar("result")
	if objErr != nil {
		t.Error("Expected result variable to be set")
	}

	// Should be able to access the parsed data
	_, err = p.Eval(`test_value = result["test"]`)
	if err != nil {
		t.Errorf("Expected no error accessing parsed data, got: %v", err)
	}

	testValue, objErr := p.GetVar("test_value")
	if objErr != nil || testValue != "value" {
		t.Errorf("Expected test_value to be 'value', got: %v", testValue)
	}
}

func TestMultipleLibraryImports(t *testing.T) {
	p := New()
	stdlib.RegisterAll(p)

	_, err := p.Eval(`
import json
import math
import time

# Test json
data = json.loads('{"number": 16}')
number = data["number"]

# Test math
sqrt_result = math.sqrt(number)

# Test time
current_time = time.time()
`)

	if err != nil {
		t.Errorf("Expected no error with multiple imports, got: %v", err)
	}

	// Verify all libraries work
	sqrtResult, objErr := p.GetVar("sqrt_result")
	if objErr != nil || sqrtResult != 4.0 {
		t.Errorf("Expected sqrt_result to be 4.0, got: %v", sqrtResult)
	}

	currentTime, objErr := p.GetVar("current_time")
	if objErr != nil {
		t.Error("Expected current_time to be set")
	}

	// Should be a float (timestamp)
	if _, ok := currentTime.(float64); !ok {
		t.Errorf("Expected current_time to be float64, got: %T", currentTime)
	}
}
