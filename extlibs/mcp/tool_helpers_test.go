package mcp_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/mcp"
	"github.com/paularlott/scriptling/object"
)

// TestRunToolScript tests the RunToolScript Go helper function
func TestRunToolScript(t *testing.T) {
	sl := scriptling.New()
	mcp.RegisterToolHelpers(sl)

	script := `
import scriptling.mcp.tool as tool

name = tool.get_string("name", "guest")
age = tool.get_int("age", 0)

tool.return_object({
    "greeting": f"Hello, {name}!",
    "age": age
})
`

	params := map[string]interface{}{
		"name": "Alice",
		"age":  30,
	}

	response, exitCode, err := mcp.RunToolScript(context.Background(), sl, script, params)

	if err != nil {
		t.Fatalf("RunToolScript failed: %v", err)
	}

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	// Parse JSON response
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	if result["greeting"] != "Hello, Alice!" {
		t.Errorf("Expected greeting='Hello, Alice!', got %v", result["greeting"])
	}

	// JSON numbers become float64
	if result["age"] != float64(30) {
		t.Errorf("Expected age=30, got %v", result["age"])
	}
}

// TestRunToolScriptError tests the RunToolScript error handling
func TestRunToolScriptError(t *testing.T) {
	sl := scriptling.New()
	mcp.RegisterToolHelpers(sl)

	script := `
import scriptling.mcp.tool as tool

age = tool.get_int("age", 0)

if age < 0:
    tool.return_error("Age must be positive")
`

	params := map[string]interface{}{
		"age": -5,
	}

	response, exitCode, err := mcp.RunToolScript(context.Background(), sl, script, params)

	if err == nil {
		t.Errorf("Expected error, got nil")
	}

	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}

	// Parse JSON error response
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		t.Fatalf("Failed to parse error response JSON: %v", err)
	}

	errorMsg, ok := result["error"].(string)
	if !ok || !strings.Contains(errorMsg, "Age must be positive") {
		t.Errorf("Expected error message 'Age must be positive', got %v", result["error"])
	}
}


// TestToolHelpersGetInt tests get_int function
func TestToolHelpersGetInt(t *testing.T) {
	sl := scriptling.New()
	mcp.RegisterToolHelpers(sl)

	// Set up __mcp_params
	paramsDict := &object.Dict{
		Pairs: map[string]object.DictPair{
			"count": {
				Key:   &object.String{Value: "count"},
				Value: object.NewInteger(42),
			},
			"limit": {
				Key:   &object.String{Value: "limit"},
				Value: &object.String{Value: "100"}, // String that should coerce to int
			},
		},
	}
	sl.SetObjectVar(mcp.MCPParamsVarName, paramsDict)

	script := `
from scriptling.mcp.tool import get_int

count = get_int("count", 0)
limit = get_int("limit", 10)
missing = get_int("missing", 99)
`

	_, err := sl.Eval(script)
	if err != nil {
		t.Fatalf("Failed to evaluate script: %v", err)
	}

	// Check results
	count, objErr := sl.GetVar("count")
	if objErr != nil || count != int64(42) {
		t.Errorf("Expected count=42, got %v", count)
	}

	limit, objErr := sl.GetVar("limit")
	if objErr != nil || limit != int64(100) {
		t.Errorf("Expected limit=100, got %v", limit)
	}

	missing, objErr := sl.GetVar("missing")
	if objErr != nil || missing != int64(99) {
		t.Errorf("Expected missing=99 (default), got %v", missing)
	}
}

// TestToolHelpersGetString tests get_string function
func TestToolHelpersGetString(t *testing.T) {
	sl := scriptling.New()
	mcp.RegisterToolHelpers(sl)

	// Set up __mcp_params
	paramsDict := &object.Dict{
		Pairs: map[string]object.DictPair{
			"name": {
				Key:   &object.String{Value: "name"},
				Value: &object.String{Value: "  Alice  "}, // With whitespace
			},
		},
	}
	sl.SetObjectVar(mcp.MCPParamsVarName, paramsDict)

	script := `
from scriptling.mcp.tool import get_string

name = get_string("name", "guest")
missing = get_string("missing", "default")
`

	_, err := sl.Eval(script)
	if err != nil {
		t.Fatalf("Failed to evaluate script: %v", err)
	}

	// Check results
	name, objErr := sl.GetVar("name")
	if objErr != nil || name != "Alice" { // Should be trimmed
		t.Errorf("Expected name='Alice', got %v", name)
	}

	missing, objErr := sl.GetVar("missing")
	if objErr != nil || missing != "default" {
		t.Errorf("Expected missing='default', got %v", missing)
	}
}

// TestToolHelpersGetBool tests get_bool function
func TestToolHelpersGetBool(t *testing.T) {
	sl := scriptling.New()
	mcp.RegisterToolHelpers(sl)

	// Set up __mcp_params
	paramsDict := &object.Dict{
		Pairs: map[string]object.DictPair{
			"enabled": {
				Key:   &object.String{Value: "enabled"},
				Value: &object.Boolean{Value: true},
			},
			"verbose": {
				Key:   &object.String{Value: "verbose"},
				Value: &object.String{Value: "true"}, // String that should parse to bool
			},
		},
	}
	sl.SetObjectVar(mcp.MCPParamsVarName, paramsDict)

	script := `
from scriptling.mcp.tool import get_bool

enabled = get_bool("enabled", False)
verbose = get_bool("verbose", False)
missing = get_bool("missing", True)
`

	_, err := sl.Eval(script)
	if err != nil {
		t.Fatalf("Failed to evaluate script: %v", err)
	}

	// Check results
	enabled, objErr := sl.GetVar("enabled")
	if objErr != nil || enabled != true {
		t.Errorf("Expected enabled=true, got %v", enabled)
	}

	verbose, objErr := sl.GetVar("verbose")
	if objErr != nil || verbose != true {
		t.Errorf("Expected verbose=true, got %v", verbose)
	}

	missing, objErr := sl.GetVar("missing")
	if objErr != nil || missing != true {
		t.Errorf("Expected missing=true (default), got %v", missing)
	}
}

// TestToolHelpersReturnString tests return_string function
func TestToolHelpersReturnString(t *testing.T) {
	sl := scriptling.New()
	mcp.RegisterToolHelpers(sl)

	script := `
from scriptling.mcp.tool import return_string

return_string("Success!")
# This should not execute
raise Exception("Should not reach here")
`

	result, err := sl.Eval(script)
	if err != nil {
		t.Fatalf("Failed to evaluate script: %v", err)
	}

	// Should return Exception with SystemExit type
	exitObj, ok := result.(*object.Exception)
	if !ok {
		t.Fatalf("Expected Exception object, got %T", result)
	}

	if !exitObj.IsSystemExit() {
		t.Errorf("Expected SystemExit exception")
	}

	if exitObj.Code != 0 {
		t.Errorf("Expected exit code 0, got %d", exitObj.Code)
	}

	// Check __mcp_response
	responseObj, err := sl.GetVarAsObject(mcp.MCPResponseVarName)
	if err != nil {
		t.Fatalf("Failed to get __mcp_response: %v", err)
	}

	strObj, ok := responseObj.(*object.String)
	if !ok {
		t.Fatalf("Expected String object, got %T", responseObj)
	}

	if strObj.Value != "Success!" {
		t.Errorf("Expected response='Success!', got %q", strObj.Value)
	}
}

// TestToolHelpersReturnObject tests return_object function
func TestToolHelpersReturnObject(t *testing.T) {
	sl := scriptling.New()
	mcp.RegisterToolHelpers(sl)

	script := `
from scriptling.mcp.tool import return_object

return_object({"status": "success", "count": 42})
`

	result, err := sl.Eval(script)
	if err != nil {
		t.Fatalf("Failed to evaluate script: %v", err)
	}

	// Should return Exception with SystemExit type
	exitObj, ok := result.(*object.Exception)
	if !ok {
		t.Fatalf("Expected Exception object, got %T", result)
	}

	if !exitObj.IsSystemExit() {
		t.Errorf("Expected SystemExit exception")
	}

	if exitObj.Code != 0 {
		t.Errorf("Expected exit code 0, got %d", exitObj.Code)
	}

	// Check __mcp_response contains JSON
	responseObj, err := sl.GetVarAsObject(mcp.MCPResponseVarName)
	if err != nil {
		t.Fatalf("Failed to get __mcp_response: %v", err)
	}

	strObj, ok := responseObj.(*object.String)
	if !ok {
		t.Fatalf("Expected String object, got %T", responseObj)
	}

	// Should be valid JSON
	jsonStr := strObj.Value
	if jsonStr == "" {
		t.Errorf("Expected JSON response, got empty string")
	}

	// Verify it's valid JSON
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		t.Errorf("Expected valid JSON, got error: %v", err)
	}

	// Check content
	if data["status"] != "success" {
		t.Errorf("Expected status='success', got %v", data["status"])
	}
	if data["count"] != float64(42) { // JSON numbers become float64
		t.Errorf("Expected count=42, got %v", data["count"])
	}
}

// TestToolHelpersReturnError tests return_error function
func TestToolHelpersReturnError(t *testing.T) {
	sl := scriptling.New()
	mcp.RegisterToolHelpers(sl)

	script := `
from scriptling.mcp.tool import return_error

return_error("Something went wrong")
`

	result, err := sl.Eval(script)
	// For error exit (code 1), both result and err are populated
	if err == nil {
		t.Fatalf("Expected error from SystemExit(1), got nil")
	}

	// Should return Exception with SystemExit type and code 1
	exitObj, ok := result.(*object.Exception)
	if !ok {
		t.Fatalf("Expected Exception object, got %T", result)
	}

	if !exitObj.IsSystemExit() {
		t.Errorf("Expected SystemExit exception")
	}

	if exitObj.Code != 1 {
		t.Errorf("Expected exit code 1, got %d", exitObj.Code)
	}

	// Check __mcp_response contains error message
	responseObj, getErr := sl.GetVarAsObject(mcp.MCPResponseVarName)
	if getErr != nil {
		t.Fatalf("Failed to get __mcp_response: %v", getErr)
	}

	strObj, ok := responseObj.(*object.String)
	if !ok {
		t.Fatalf("Expected String object, got %T", responseObj)
	}

	// Should contain error JSON
	jsonStr := strObj.Value
	if jsonStr == "" {
		t.Errorf("Expected error response, got empty string")
	}

	// Verify it's valid JSON with error field
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		t.Errorf("Expected valid JSON, got error: %v", err)
	}

	errorMsg, ok := data["error"].(string)
	if !ok || !strings.Contains(errorMsg, "Something went wrong") {
		t.Errorf("Expected error message to contain 'Something went wrong', got %v", data["error"])
	}
}
