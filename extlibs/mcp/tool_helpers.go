package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	mcptoon "github.com/paularlott/mcp/toon"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/evaluator"
	"github.com/paularlott/scriptling/object"
)

const (
	// MCPParamsVarName is the environment variable name for MCP tool parameters
	MCPParamsVarName = "__mcp_params"
	// MCPResponseVarName is the environment variable name for MCP tool response
	MCPResponseVarName = "__mcp_response"
)

// getParamValue retrieves a parameter value from the __mcp_params dict
// Returns nil if the parameter doesn't exist or environment is not available
func getParamValue(ctx context.Context, name string) object.Object {
	// Get environment from context
	env := evaluator.GetEnvFromContext(ctx)
	if env == nil {
		return nil
	}

	// Get __mcp_params from environment
	paramsObj, ok := env.Get(MCPParamsVarName)
	if !ok {
		return nil
	}

	// Get parameter from params dict
	paramsDict, ok := paramsObj.(*object.Dict)
	if !ok {
		return nil
	}

	pair, exists := paramsDict.Pairs[name]
	if !exists {
		return nil
	}

	return pair.Value
}

// setResponseAndExit sets the __mcp_response environment variable and returns a SystemExit
func setResponseAndExit(ctx context.Context, response string, exitCode int) object.Object {
	// Get environment from context
	env := evaluator.GetEnvFromContext(ctx)
	if env == nil {
		return &object.Error{Message: "environment not available"}
	}

	// Set __mcp_response
	env.Set(MCPResponseVarName, &object.String{Value: response})

	// Exit with specified code
	return object.NewSystemExit(exitCode, "")
}

// buildToolHelpersLibrary creates the scriptling.mcp.tool sub-library
// This provides parameter access and result functions for MCP tools
func buildToolHelpersLibrary() *object.Library {
	builder := object.NewLibraryBuilder("scriptling.mcp.tool", "MCP tool parameter access and result functions")

	// get_int(name, default=0) - Get integer parameter
	builder.FunctionWithHelp("get_int", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		if len(args) == 0 {
			return &object.Error{Message: "get_int() requires at least 1 argument (name)"}
		}

		name, err := args[0].AsString()
		if err != nil {
			return err
		}

		// Get default value if provided
		var defaultVal int64 = 0
		if len(args) > 1 {
			defaultVal, err = args[1].CoerceInt()
			if err != nil {
				return err
			}
		}

		// Get parameter value
		paramValue := getParamValue(ctx, name)
		if paramValue == nil {
			return object.NewInteger(defaultVal)
		}

		// Coerce to int
		val, err := paramValue.CoerceInt()
		if err != nil {
			return object.NewInteger(defaultVal)
		}

		return object.NewInteger(val)
	}, `get_int(name, default=0) - Get a parameter as integer

Safely gets a parameter and converts it to an integer, handling None, empty strings, and whitespace.
Returns the default value if the parameter doesn't exist, is None, empty, or whitespace-only.

Parameters:
  name: The parameter name
  default: Default value (optional, defaults to 0)

Example:
  project_id = mcp.tool.get_int("project_id", 0)
  limit = mcp.tool.get_int("limit", 100)`)

	// get_float(name, default=0.0) - Get float parameter
	builder.FunctionWithHelp("get_float", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		if len(args) == 0 {
			return &object.Error{Message: "get_float() requires at least 1 argument (name)"}
		}

		name, err := args[0].AsString()
		if err != nil {
			return err
		}

		// Get default value if provided
		var defaultVal float64 = 0.0
		if len(args) > 1 {
			defaultVal, err = args[1].CoerceFloat()
			if err != nil {
				return err
			}
		}

		// Get parameter value
		paramValue := getParamValue(ctx, name)
		if paramValue == nil {
			return &object.Float{Value: defaultVal}
		}

		// Coerce to float
		val, err := paramValue.CoerceFloat()
		if err != nil {
			return &object.Float{Value: defaultVal}
		}

		return &object.Float{Value: val}
	}, `get_float(name, default=0.0) - Get a parameter as float

Safely gets a parameter and converts it to a float, handling None, empty strings, and whitespace.
Returns the default value if the parameter doesn't exist, is None, empty, or whitespace-only.

Parameters:
  name: The parameter name
  default: Default value (optional, defaults to 0.0)

Example:
  price = mcp.tool.get_float("price", 0.0)
  percentage = mcp.tool.get_float("percentage", 100.0)`)

	// get_string(name, default="") - Get string parameter
	builder.FunctionWithHelp("get_string", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		if len(args) == 0 {
			return &object.Error{Message: "get_string() requires at least 1 argument (name)"}
		}

		name, err := args[0].AsString()
		if err != nil {
			return err
		}

		// Get default value if provided
		defaultVal := ""
		if len(args) > 1 {
			defaultVal, err = args[1].CoerceString()
			if err != nil {
				return err
			}
		}

		// Get parameter value
		paramValue := getParamValue(ctx, name)
		if paramValue == nil {
			return &object.String{Value: defaultVal}
		}

		// Coerce to string
		val, err := paramValue.CoerceString()
		if err != nil {
			return &object.String{Value: defaultVal}
		}

		// Trim whitespace
		val = strings.TrimSpace(val)
		if val == "" {
			return &object.String{Value: defaultVal}
		}

		return &object.String{Value: val}
	}, `get_string(name, default="") - Get a parameter as string

Safely gets a parameter as a string, handling None, empty strings, and whitespace.
Trims whitespace and returns the default value if the parameter doesn't exist, is None, empty, or whitespace-only.

Parameters:
  name: The parameter name
  default: Default value (optional, defaults to "")

Example:
  name = mcp.tool.get_string("name", "guest")
  query = mcp.tool.get_string("query")`)

	// get_bool(name, default=false) - Get boolean parameter
	builder.FunctionWithHelp("get_bool", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		if len(args) == 0 {
			return &object.Error{Message: "get_bool() requires at least 1 argument (name)"}
		}

		name, err := args[0].AsString()
		if err != nil {
			return err
		}

		// Get default value if provided
		defaultVal := false
		if len(args) > 1 {
			defaultVal, err = args[1].AsBool()
			if err != nil {
				return err
			}
		}

		// Get parameter value
		paramValue := getParamValue(ctx, name)
		if paramValue == nil {
			return &object.Boolean{Value: defaultVal}
		}

		// Handle string "true"/"false"
		if strVal, ok := paramValue.(*object.String); ok {
			lower := strings.ToLower(strings.TrimSpace(strVal.Value))
			if lower == "true" || lower == "1" {
				return &object.Boolean{Value: true}
			}
			if lower == "false" || lower == "0" {
				return &object.Boolean{Value: false}
			}
		}

		// Coerce to bool
		val, err := paramValue.AsBool()
		if err != nil {
			return &object.Boolean{Value: defaultVal}
		}

		return &object.Boolean{Value: val}
	}, `get_bool(name, default=false) - Get a parameter as boolean

Safely gets a parameter and converts it to a boolean.
Handles string values "true"/"false" (case-insensitive) and numeric 0/1.
Returns the default value if the parameter doesn't exist or cannot be converted.

Parameters:
  name: The parameter name
  default: Default value (optional, defaults to false)

Example:
  enabled = mcp.tool.get_bool("enabled", true)
  verbose = mcp.tool.get_bool("verbose")`)

	// get_list(name, default=None) - Get list parameter
	builder.FunctionWithHelp("get_list", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		if len(args) == 0 {
			return &object.Error{Message: "get_list() requires at least 1 argument (name)"}
		}

		name, err := args[0].AsString()
		if err != nil {
			return err
		}

		// Get default value if provided
		var defaultVal *object.List
		if len(args) > 1 {
			if list, ok := args[1].(*object.List); ok {
				defaultVal = list
			} else {
				return &object.Error{Message: "get_list() default must be a list"}
			}
		}
		if defaultVal == nil {
			defaultVal = &object.List{Elements: []object.Object{}}
		}

		// Get parameter value
		paramValue := getParamValue(ctx, name)
		if paramValue == nil {
			return defaultVal
		}

		// If already a list, return it
		if list, ok := paramValue.(*object.List); ok {
			return list
		}

		// If string, split by comma
		if strVal, ok := paramValue.(*object.String); ok {
			val := strings.TrimSpace(strVal.Value)
			if val == "" {
				return defaultVal
			}
			parts := strings.Split(val, ",")
			elements := make([]object.Object, 0, len(parts))
			for _, part := range parts {
				trimmed := strings.TrimSpace(part)
				if trimmed != "" {
					elements = append(elements, &object.String{Value: trimmed})
				}
			}
			return &object.List{Elements: elements}
		}

		return defaultVal
	}, `get_list(name, default=None) - Get a parameter as list

Gets a list parameter. If the value is a string, splits it by comma.
Returns the default value if the parameter doesn't exist.

Parameters:
  name: The parameter name
  default: Default value (optional, defaults to empty list)

Example:
  ids = mcp.tool.get_list("ids")              # "1,2,3" → ["1", "2", "3"]
  tags = mcp.tool.get_list("tags", ["all"])   # "tag1, tag2" → ["tag1", "tag2"]`)

	// return_string(text) - Return string result
	builder.FunctionWithHelp("return_string", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		if len(args) == 0 {
			return &object.Error{Message: "return_string() requires a text argument"}
		}

		text, err := args[0].CoerceString()
		if err != nil {
			return err
		}

		return setResponseAndExit(ctx, text, 0)
	}, `return_string(text) - Return a string result from the tool and stop execution

Sets the tool's return value to the given string and stops script execution.
No code after this call will execute.

Example:
  mcp.tool.return_string("Search completed successfully")
  # Code here will not execute`)

	// return_object(obj) - Return object as JSON
	builder.FunctionWithHelp("return_object", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		if len(args) == 0 {
			return &object.Error{Message: "return_object() requires an object argument"}
		}

		// Convert scriptling object to Go native types
		goObj := scriptling.ToGo(args[0])

		// Marshal to JSON
		jsonBytes, err := json.Marshal(goObj)
		if err != nil {
			return &object.Error{Message: fmt.Sprintf("Failed to serialize object to JSON: %v", err)}
		}

		return setResponseAndExit(ctx, string(jsonBytes), 0)
	}, `return_object(obj) - Return an object as JSON from the tool and stop execution

Serializes the object to JSON and sets it as the tool's return value.
Stops script execution immediately - no code after this call will execute.

Example:
  mcp.tool.return_object({"status": "success", "count": 42})
  # Code here will not execute`)

	// return_toon(obj) - Return object as TOON
	builder.FunctionWithHelp("return_toon", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		if len(args) == 0 {
			return &object.Error{Message: "return_toon() requires an object argument"}
		}

		// Convert to Go object
		goObj := scriptling.ToGo(args[0])

		// Encode to TOON
		toonStr, err := mcptoon.Encode(goObj)
		if err != nil {
			return &object.Error{Message: fmt.Sprintf("Failed to encode to TOON: %v", err)}
		}

		return setResponseAndExit(ctx, toonStr, 0)
	}, `return_toon(obj) - Return an object encoded as TOON from the tool and stop execution

Serializes the object to TOON format and sets it as the tool's return value.
TOON is a compact text format optimized for LLM consumption.
Stops script execution immediately - no code after this call will execute.

Example:
  mcp.tool.return_toon({"result": data})
  # Code here will not execute`)

	// return_error(message) - Return error
	builder.FunctionWithHelp("return_error", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		if len(args) == 0 {
			return &object.Error{Message: "return_error() requires a message argument"}
		}

		message, err := args[0].CoerceString()
		if err != nil {
			return err
		}

		// Set error response as JSON
		errorJSON := fmt.Sprintf(`{"error": "%s"}`, message)
		return setResponseAndExit(ctx, errorJSON, 1)
	}, `return_error(message) - Return an error from the tool and stop execution

Returns an error message to the MCP client and stops script execution immediately.

Arguments:
  message (str): The error message

Example:
  mcp.tool.return_error("Customer not found")
  mcp.tool.return_error("Invalid input: project ID is required")`)

	return builder.Build()
}

// RegisterToolHelpers registers the scriptling.mcp.tool sub-library
// This is optional and separate from the main MCP library registration
func RegisterToolHelpers(registrar interface{ RegisterLibrary(*object.Library) }) {
	toolLib := buildToolHelpersLibrary()
	registrar.RegisterLibrary(toolLib)
}

// RunToolScript is a Go helper that sets up MCP parameters, runs a script, and retrieves the response
// This simplifies calling MCP tool scripts from Go code
func RunToolScript(ctx context.Context, sl *scriptling.Scriptling, script string, params map[string]interface{}) (response string, exitCode int, err error) {
	// Convert params map to scriptling Dict
	paramsDict := &object.Dict{
		Pairs: make(map[string]object.DictPair),
	}
	for key, value := range params {
		obj := scriptling.FromGo(value)
		paramsDict.Pairs[key] = object.DictPair{
			Key:   &object.String{Value: key},
			Value: obj,
		}
	}

	// Set __mcp_params in environment
	if err := sl.SetObjectVar(MCPParamsVarName, paramsDict); err != nil {
		return "", 1, fmt.Errorf("failed to set params: %w", err)
	}

	// Run the script
	result, evalErr := sl.EvalWithContext(ctx, script)

	// Get the response
	responseObj, getErr := sl.GetVarAsObject(MCPResponseVarName)
	if getErr == nil {
		if strObj, ok := responseObj.(*object.String); ok {
			response = strObj.Value
		}
	}

	// Check for SystemExit
	if exc, ok := result.(*object.Exception); ok && exc.IsSystemExit() {
		exitCode = exc.GetExitCode()
		if exitCode != 0 && evalErr != nil {
			err = evalErr
		}
		return response, exitCode, err
	}

	// Check for other errors
	if evalErr != nil {
		return response, 1, evalErr
	}

	// Success
	return response, 0, nil
}
