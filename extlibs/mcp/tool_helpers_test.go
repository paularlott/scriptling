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

	script := `
from scriptling.mcp.tool import get_int

count = get_int("count", 0)
limit = get_int("limit", 10)
missing = get_int("missing", 99)
`

	params := map[string]interface{}{
		"count": 42,
		"limit": "100", // String that should coerce to int
	}

	_, exitCode, err := mcp.RunToolScript(context.Background(), sl, script, params)
	if err != nil {
		t.Fatalf("RunToolScript failed: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

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

	script := `
from scriptling.mcp.tool import get_string

name = get_string("name", "guest")
missing = get_string("missing", "default")
`

	params := map[string]interface{}{
		"name": "  Alice  ", // With whitespace
	}

	_, exitCode, err := mcp.RunToolScript(context.Background(), sl, script, params)
	if err != nil {
		t.Fatalf("RunToolScript failed: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	name, objErr := sl.GetVar("name")
	if objErr != nil || name != "Alice" { // Should be trimmed
		t.Errorf("Expected name='Alice', got %v", name)
	}

	missing, objErr := sl.GetVar("missing")
	if objErr != nil || missing != "default" {
		t.Errorf("Expected missing='default', got %v", missing)
	}
}

// TestToolHelpersGetFloat tests get_float function
func TestToolHelpersGetFloat(t *testing.T) {
	sl := scriptling.New()
	mcp.RegisterToolHelpers(sl)

	script := `
from scriptling.mcp.tool import get_float

price = get_float("price", 0.0)
percentage = get_float("percentage", 100.0)
missing = get_float("missing", 50.5)
`

	params := map[string]interface{}{
		"price":      19.99,
		"percentage": "75.5",
	}

	_, exitCode, err := mcp.RunToolScript(context.Background(), sl, script, params)
	if err != nil {
		t.Fatalf("RunToolScript failed: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	price, objErr := sl.GetVar("price")
	if objErr != nil || price != 19.99 {
		t.Errorf("Expected price=19.99, got %v", price)
	}

	percentage, objErr := sl.GetVar("percentage")
	if objErr != nil || percentage != 75.5 {
		t.Errorf("Expected percentage=75.5, got %v", percentage)
	}

	missing, objErr := sl.GetVar("missing")
	if objErr != nil || missing != 50.5 {
		t.Errorf("Expected missing=50.5 (default), got %v", missing)
	}
}

// TestToolHelpersGetBool tests get_bool function
func TestToolHelpersGetBool(t *testing.T) {
	sl := scriptling.New()
	mcp.RegisterToolHelpers(sl)

	script := `
from scriptling.mcp.tool import get_bool

enabled = get_bool("enabled", False)
verbose = get_bool("verbose", False)
missing = get_bool("missing", True)
`

	params := map[string]interface{}{
		"enabled": true,
		"verbose": "true", // String that should parse to bool
	}

	_, exitCode, err := mcp.RunToolScript(context.Background(), sl, script, params)
	if err != nil {
		t.Fatalf("RunToolScript failed: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

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

// TestToolHelpersGetList tests get_list function
func TestToolHelpersGetList(t *testing.T) {
	sl := scriptling.New()
	mcp.RegisterToolHelpers(sl)

	script := `
from scriptling.mcp.tool import get_list

tags = get_list("tags")
ids = get_list("ids")
missing = get_list("missing")
`

	params := map[string]interface{}{
		"tags": "tag1, tag2, tag3",
		"ids":  []int{1, 2, 3},
	}

	_, exitCode, err := mcp.RunToolScript(context.Background(), sl, script, params)
	if err != nil {
		t.Fatalf("RunToolScript failed: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	tagsObj, objErr := sl.GetVarAsObject("tags")
	if objErr != nil {
		t.Fatalf("Failed to get tags: %v", objErr)
	}
	tagsList, ok := tagsObj.(*object.List)
	if !ok {
		t.Fatalf("Expected List object, got %T", tagsObj)
	}
	if len(tagsList.Elements) != 3 {
		t.Errorf("Expected 3 tags, got %d", len(tagsList.Elements))
	}

	idsObj, objErr := sl.GetVarAsObject("ids")
	if objErr != nil {
		t.Fatalf("Failed to get ids: %v", objErr)
	}
	idsList, ok := idsObj.(*object.List)
	if !ok {
		t.Fatalf("Expected List object, got %T", idsObj)
	}
	if len(idsList.Elements) != 3 {
		t.Errorf("Expected 3 ids, got %d", len(idsList.Elements))
	}

	missingObj, objErr := sl.GetVarAsObject("missing")
	if objErr != nil {
		t.Fatalf("Failed to get missing: %v", objErr)
	}
	missingList, ok := missingObj.(*object.List)
	if !ok {
		t.Fatalf("Expected List object, got %T", missingObj)
	}
	if len(missingList.Elements) != 0 {
		t.Errorf("Expected empty list, got %d elements", len(missingList.Elements))
	}
}

// TestToolHelpersGetStringList tests get_string_list function
func TestToolHelpersGetStringList(t *testing.T) {
	sl := scriptling.New()
	mcp.RegisterToolHelpers(sl)

	script := `
from scriptling.mcp.tool import get_string_list

args = get_string_list("args")
missing = get_string_list("missing", ["default"])
`

	params := map[string]interface{}{
		"args": []string{"--verbose", "-o", "file.txt"},
	}

	_, exitCode, err := mcp.RunToolScript(context.Background(), sl, script, params)
	if err != nil {
		t.Fatalf("RunToolScript failed: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	argsObj, objErr := sl.GetVarAsObject("args")
	if objErr != nil {
		t.Fatalf("Failed to get args: %v", objErr)
	}
	argsList, ok := argsObj.(*object.List)
	if !ok {
		t.Fatalf("Expected List object, got %T", argsObj)
	}
	if len(argsList.Elements) != 3 {
		t.Errorf("Expected 3 args, got %d", len(argsList.Elements))
	}

	missingObj, objErr := sl.GetVarAsObject("missing")
	if objErr != nil {
		t.Fatalf("Failed to get missing: %v", objErr)
	}
	missingList, ok := missingObj.(*object.List)
	if !ok {
		t.Fatalf("Expected List object, got %T", missingObj)
	}
	if len(missingList.Elements) != 1 {
		t.Errorf("Expected 1 element (default), got %d", len(missingList.Elements))
	}
}

// TestToolHelpersGetIntList tests get_int_list function
func TestToolHelpersGetIntList(t *testing.T) {
	sl := scriptling.New()
	mcp.RegisterToolHelpers(sl)

	script := `
from scriptling.mcp.tool import get_int_list

ids = get_int_list("ids")
missing = get_int_list("missing")
`

	params := map[string]interface{}{
		"ids": []int{1, 2, 3},
	}

	_, exitCode, err := mcp.RunToolScript(context.Background(), sl, script, params)
	if err != nil {
		t.Fatalf("RunToolScript failed: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	idsObj, objErr := sl.GetVarAsObject("ids")
	if objErr != nil {
		t.Fatalf("Failed to get ids: %v", objErr)
	}
	idsList, ok := idsObj.(*object.List)
	if !ok {
		t.Fatalf("Expected List object, got %T", idsObj)
	}
	if len(idsList.Elements) != 3 {
		t.Errorf("Expected 3 ids, got %d", len(idsList.Elements))
	}
}

// TestToolHelpersGetFloatList tests get_float_list function
func TestToolHelpersGetFloatList(t *testing.T) {
	sl := scriptling.New()
	mcp.RegisterToolHelpers(sl)

	script := `
from scriptling.mcp.tool import get_float_list

prices = get_float_list("prices")
missing = get_float_list("missing")
`

	params := map[string]interface{}{
		"prices": []float64{19.99, 29.99},
	}

	_, exitCode, err := mcp.RunToolScript(context.Background(), sl, script, params)
	if err != nil {
		t.Fatalf("RunToolScript failed: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	pricesObj, objErr := sl.GetVarAsObject("prices")
	if objErr != nil {
		t.Fatalf("Failed to get prices: %v", objErr)
	}
	pricesList, ok := pricesObj.(*object.List)
	if !ok {
		t.Fatalf("Expected List object, got %T", pricesObj)
	}
	if len(pricesList.Elements) != 2 {
		t.Errorf("Expected 2 prices, got %d", len(pricesList.Elements))
	}
}

// TestToolHelpersGetBoolList tests get_bool_list function
func TestToolHelpersGetBoolList(t *testing.T) {
	sl := scriptling.New()
	mcp.RegisterToolHelpers(sl)

	script := `
from scriptling.mcp.tool import get_bool_list

flags = get_bool_list("flags")
missing = get_bool_list("missing")
`

	params := map[string]interface{}{
		"flags": []bool{true, false, true},
	}

	_, exitCode, err := mcp.RunToolScript(context.Background(), sl, script, params)
	if err != nil {
		t.Fatalf("RunToolScript failed: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	flagsObj, objErr := sl.GetVarAsObject("flags")
	if objErr != nil {
		t.Fatalf("Failed to get flags: %v", objErr)
	}
	flagsList, ok := flagsObj.(*object.List)
	if !ok {
		t.Fatalf("Expected List object, got %T", flagsObj)
	}
	if len(flagsList.Elements) != 3 {
		t.Errorf("Expected 3 flags, got %d", len(flagsList.Elements))
	}
}

// TestToolHelpersReturnToon tests return_toon function
func TestToolHelpersReturnToon(t *testing.T) {
	sl := scriptling.New()
	mcp.RegisterToolHelpers(sl)

	script := `
from scriptling.mcp.tool import return_toon

return_toon({"status": "success", "items": [1, 2, 3]})
`

	result, err := sl.Eval(script)
	if err != nil {
		t.Fatalf("Failed to evaluate script: %v", err)
	}

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

	responseObj, err := sl.GetVarAsObject(mcp.MCPResponseVarName)
	if err != nil {
		t.Fatalf("Failed to get __mcp_response: %v", err)
	}

	strObj, ok := responseObj.(*object.String)
	if !ok {
		t.Fatalf("Expected String object, got %T", responseObj)
	}

	if strObj.Value == "" {
		t.Errorf("Expected TOON response, got empty string")
	}

	if !strings.Contains(strObj.Value, "status") || !strings.Contains(strObj.Value, "success") {
		t.Errorf("Expected TOON to contain status and success, got %q", strObj.Value)
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
