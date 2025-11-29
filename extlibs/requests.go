package extlibs

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
	"golang.org/x/net/http2"
)

var httpClient *http.Client

func init() {
	// Create HTTP/2 transport with connection pooling and self-signed cert support
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // Accept self-signed certificates
		},
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
	}

	// Enable HTTP/2
	http2.ConfigureTransport(transport)

	httpClient = &http.Client{
		Transport: transport,
	}
}

// Response class for HTTP responses

// ResponseClass defines the Response class with its methods
var ResponseClass = &object.Class{
	Name: "Response",
	Methods: map[string]object.Object{
		"json": &object.Builtin{
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) != 1 {
					return errors.NewArgumentError(len(args), 1)
				}
				if instance, ok := args[0].(*object.Instance); ok {
					if body, ok := instance.Fields["body"].(*object.String); ok {
						var result interface{}
						if err := json.Unmarshal([]byte(body.Value), &result); err != nil {
							return errors.NewError("JSONDecodeError: %s", err.Error())
						}
						return convertJSONToObject(result)
					}
				}
				return errors.NewError("json() called on non-Response object")
			},
			HelpText: `json() - Parses the response body as JSON and returns the parsed object`,
		},
		"raise_for_status": &object.Builtin{
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) != 1 {
					return errors.NewArgumentError(len(args), 1)
				}
				if instance, ok := args[0].(*object.Instance); ok {
					if statusCode, ok := instance.Fields["status_code"].(*object.Integer); ok {
						if statusCode.Value >= 400 {
							if statusCode.Value >= 500 {
								return errors.NewError("HTTPError: %d Server Error", statusCode.Value)
							} else {
								return errors.NewError("HTTPError: %d Client Error", statusCode.Value)
							}
						}
						return &object.Null{}
					}
				}
				return errors.NewError("raise_for_status() called on non-Response object")
			},
			HelpText: `raise_for_status() - Raises an exception if the status code indicates an error`,
		},
	},
}

// createResponseInstance creates a new Response instance
func createResponseInstance(statusCode int, headers map[string]string, body []byte, url string) *object.Instance {
	// Convert headers to object.Dict
	headerPairs := make(map[string]object.DictPair)
	for k, v := range headers {
		headerPairs[k] = object.DictPair{
			Key:   &object.String{Value: k},
			Value: &object.String{Value: v},
		}
	}

	return &object.Instance{
		Class: ResponseClass,
		Fields: map[string]object.Object{
			"status_code": &object.Integer{Value: int64(statusCode)},
			"text":        &object.String{Value: string(body)},
			"headers":     &object.Dict{Pairs: headerPairs},
			"body":        &object.String{Value: string(body)},
			"url":         &object.String{Value: url},
		},
	}
}

// Exception types for requests library
var requestExceptionType = &object.String{Value: "RequestException"}
var httpErrorType = &object.String{Value: "HTTPError"}

// Create exceptions namespace dict
var exceptionsNamespace = &object.Dict{
	Pairs: map[string]object.DictPair{
		"RequestException": {
			Key:   &object.String{Value: "RequestException"},
			Value: requestExceptionType,
		},
		"HTTPError": {
			Key:   &object.String{Value: "HTTPError"},
			Value: httpErrorType,
		},
	},
}

// parseRequestOptions parses the options dict and returns timeout and headers
func parseRequestOptions(options map[string]object.Object) (int, map[string]string) {
	timeout := 5
	headers := make(map[string]string)
	if timeoutPair, ok := options["timeout"]; ok {
		if timeoutInt, ok := timeoutPair.AsInt(); ok {
			timeout = int(timeoutInt)
		}
	}
	if headersPair, ok := options["headers"]; ok {
		if headersDict, ok := headersPair.AsDict(); ok {
			headers = extractHeaders(headersDict)
		}
	}
	return timeout, headers
}

var RequestsLibrary = object.NewLibrary(map[string]*object.Builtin{
	// Exceptions namespace - returns dict with exception types
	"exceptions": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			return exceptionsNamespace
		},
	},
	"get": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) < 1 || len(args) > 2 {
				return errors.NewArgumentError(len(args), 1)
			}
			if args[0].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}
			url, _ := args[0].AsString()
			timeout := 5 // Default 5 seconds
			headers := make(map[string]string)

			if len(args) == 2 {
				if args[1].Type() != object.DICT_OBJ {
					return errors.NewTypeError("DICT", args[1].Type().String())
				}
				options, _ := args[1].AsDict()
				timeout, headers = parseRequestOptions(options)
			}
			return httpRequestWithContext(ctx, "GET", url, "", timeout, headers)
		},
		HelpText: `get(url, options={}) - Send a GET request

Sends an HTTP GET request to the specified URL.

Parameters:
  url (string): The URL to send the request to
  options (dict, optional): Request options
    - timeout (int): Request timeout in seconds (default: 5)
    - headers (dict): HTTP headers as key-value pairs

Returns:
  Response object with status_code, text, headers, body, url, and json() method`,
	},
	"post": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) < 2 || len(args) > 3 {
				return errors.NewArgumentError(len(args), 2)
			}
			if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", "mixed types")
			}
			url, _ := args[0].AsString()
			body, _ := args[1].AsString()
			timeout := 5
			headers := make(map[string]string)

			if len(args) == 3 {
				if args[2].Type() != object.DICT_OBJ {
					return errors.NewTypeError("DICT", args[2].Type().String())
				}
				options, _ := args[2].AsDict()
				timeout, headers = parseRequestOptions(options)
			}
			return httpRequestWithContext(ctx, "POST", url, body, timeout, headers)
		},
		HelpText: `post(url, data, options={}) - Send a POST request

Sends an HTTP POST request to the specified URL with the given data.

Parameters:
  url (string): The URL to send the request to
  data (string): The request body data
  options (dict, optional): Request options
    - timeout (int): Request timeout in seconds (default: 5)
    - headers (dict): HTTP headers as key-value pairs

Returns:
  Response object with status_code, text, headers, body, url, and json() method`,
	},
	"put": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) < 2 || len(args) > 3 {
				return errors.NewArgumentError(len(args), 2)
			}
			if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", "mixed types")
			}
			url, _ := args[0].AsString()
			body, _ := args[1].AsString()
			timeout := 5
			headers := make(map[string]string)

			if len(args) == 3 {
				if args[2].Type() != object.DICT_OBJ {
					return errors.NewTypeError("DICT", args[2].Type().String())
				}
				options, _ := args[2].AsDict()
				timeout, headers = parseRequestOptions(options)
			}
			return httpRequestWithContext(ctx, "PUT", url, body, timeout, headers)
		},
		HelpText: `put(url, data, options={}) - Send a PUT request

Sends an HTTP PUT request to the specified URL with the given data.

Parameters:
  url (string): The URL to send the request to
  data (string): The request body data
  options (dict, optional): Request options
    - timeout (int): Request timeout in seconds (default: 5)
    - headers (dict): HTTP headers as key-value pairs

Returns:
  Response object with status_code, text, headers, body, url, and json() method`,
	},
	"delete": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) < 1 || len(args) > 2 {
				return errors.NewArgumentError(len(args), 1)
			}
			if args[0].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", args[0].Type().String())
			}
			url, _ := args[0].AsString()
			timeout := 5
			headers := make(map[string]string)

			if len(args) == 2 {
				if args[1].Type() != object.DICT_OBJ {
					return errors.NewTypeError("DICT", args[1].Type().String())
				}
				options, _ := args[1].AsDict()
				timeout, headers = parseRequestOptions(options)
			}
			return httpRequestWithContext(ctx, "DELETE", url, "", timeout, headers)
		},
		HelpText: `delete(url, options={}) - Send a DELETE request

Sends an HTTP DELETE request to the specified URL.

Parameters:
  url (string): The URL to send the request to
  options (dict, optional): Request options
    - timeout (int): Request timeout in seconds (default: 5)
    - headers (dict): HTTP headers as key-value pairs

Returns:
  Response object with status_code, text, headers, body, url, and json() method`,
	},
	"patch": {
		Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
			if len(args) < 2 || len(args) > 3 {
				return errors.NewArgumentError(len(args), 2)
			}
			if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", "mixed types")
			}
			url, _ := args[0].AsString()
			body, _ := args[1].AsString()
			timeout := 5
			headers := make(map[string]string)

			if len(args) == 3 {
				if args[2].Type() != object.DICT_OBJ {
					return errors.NewTypeError("DICT", args[2].Type().String())
				}
				options, _ := args[2].AsDict()
				timeout, headers = parseRequestOptions(options)
			}
			return httpRequestWithContext(ctx, "PATCH", url, body, timeout, headers)
		},
		HelpText: `patch(url, data, options={}) - Send a PATCH request

Sends an HTTP PATCH request to the specified URL with the given data.

Parameters:
  url (string): The URL to send the request to
  data (string): The request body data
  options (dict, optional): Request options
    - timeout (int): Request timeout in seconds (default: 5)
    - headers (dict): HTTP headers as key-value pairs

Returns:
  Response object with status_code, text, headers, body, url, and json() method`,
	},
}, map[string]object.Object{
	// Exception types as constants (for except clause matching)
	"RequestException": requestExceptionType,
	"HTTPError":        httpErrorType,
	"Response":         ResponseClass,
}, "HTTP requests library")

func extractHeaders(dict map[string]object.Object) map[string]string {
	headers := make(map[string]string)
	for key, value := range dict {
		if strVal, ok := value.AsString(); ok {
			headers[key] = strVal
		}
	}
	return headers
}

func httpRequestWithContext(parentCtx context.Context, method, url, body string, timeoutSecs int, headers map[string]string) object.Object {
	// Combine parent context with timeout
	ctx, cancel := context.WithTimeout(parentCtx, time.Duration(timeoutSecs)*time.Second)
	defer cancel()

	var req *http.Request
	var err error

	if body != "" {
		req, err = http.NewRequestWithContext(ctx, method, url, strings.NewReader(body))
		if err != nil {
			return errors.NewError("http request error: %s", err.Error())
		}
		if _, hasContentType := headers["Content-Type"]; !hasContentType {
			req.Header.Set("Content-Type", "application/json")
		}
	} else {
		req, err = http.NewRequestWithContext(ctx, method, url, nil)
		if err != nil {
			return errors.NewError("http request error: %s", err.Error())
		}
	}

	// Set custom headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return errors.NewError("http timeout after %d seconds", timeoutSecs)
		}
		return errors.NewError("http error: %s", err.Error())
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.NewError("http read error: %s", err.Error())
	}

	respHeaders := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			respHeaders[key] = values[0]
		}
	}

	return createResponseInstance(resp.StatusCode, respHeaders, respBody, url)
}

// convertJSONToObject converts Go's JSON interface{} to Scriptling objects
func convertJSONToObject(data interface{}) object.Object {
	switch v := data.(type) {
	case nil:
		return &object.Null{}
	case bool:
		return &object.Boolean{Value: v}
	case float64:
		// JSON numbers are always float64
		if v == float64(int64(v)) {
			return &object.Integer{Value: int64(v)}
		}
		return &object.Float{Value: v}
	case string:
		return &object.String{Value: v}
	case []interface{}:
		elements := make([]object.Object, len(v))
		for i, item := range v {
			elements[i] = convertJSONToObject(item)
		}
		return &object.List{Elements: elements}
	case map[string]interface{}:
		pairs := make(map[string]object.DictPair)
		for key, val := range v {
			pairs[key] = object.DictPair{
				Key:   &object.String{Value: key},
				Value: convertJSONToObject(val),
			}
		}
		return &object.Dict{Pairs: pairs}
	default:
		return &object.Null{}
	}
}
