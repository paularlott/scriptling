package extlibs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/pool"
)

func RegisterRequestsLibrary(registrar interface{ RegisterLibrary(*object.Library) }) {
	registrar.RegisterLibrary(RequestsLibrary)
}

// Response class for HTTP responses

// ResponseClass defines the Response class with its methods
var ResponseClass = &object.Class{
	Name: "Response",
	Methods: map[string]object.Object{
		"json": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil { return err }
				if instance, ok := args[0].(*object.Instance); ok {
					if body, err := instance.Fields["body"].AsString(); err == nil {
						return conversion.MustParseJSON(body)
					}
				}
				return errors.NewError("json() called on non-Response object")
			},
			HelpText: `json() - Parses the response body as JSON and returns the parsed object`,
		},
		"raise_for_status": &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if err := errors.ExactArgs(args, 1); err != nil { return err }
				if instance, ok := args[0].(*object.Instance); ok {
					if statusCode, err := instance.Fields["status_code"].AsInt(); err == nil {
						if statusCode >= 400 {
							kind := "Client"
							if statusCode >= 500 {
								kind = "Server"
							}
							return &object.Exception{
								ExceptionType: "HTTPError",
								Message:       fmt.Sprintf("HTTPError: %d %s Error", statusCode, kind),
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
	headerDict := &object.Dict{Pairs: make(map[string]object.DictPair)}
	for k, v := range headers {
		headerDict.SetByString(k, object.NewString(v))
	}

	return &object.Instance{
		Class: ResponseClass,
		Fields: map[string]object.Object{
			"status_code": object.NewInteger(int64(statusCode)),
			"text":        object.NewString(string(body)),
			"headers":     headerDict,
			"body":        object.NewString(string(body)),
			"url":         object.NewString(url),
		},
	}
}

// Exception types for requests library
var requestExceptionType = object.NewString("RequestException")
var httpErrorType = object.NewString("HTTPError")

// Create exceptions namespace dict
var exceptionsNamespace = object.NewStringDict(map[string]object.Object{
	"RequestException": requestExceptionType,
	"HTTPError":        httpErrorType,
})

// parseRequestOptions parses the options dict and returns timeout, headers, params, user, pass
func parseRequestOptions(options map[string]object.Object) (int, map[string]string, map[string]string, string, string) {
	timeout := 5
	headers := make(map[string]string)
	params := make(map[string]string)
	user := ""
	pass := ""
	if timeoutPair, ok := options["timeout"]; ok {
		if timeoutInt, err := timeoutPair.AsInt(); err == nil {
			timeout = int(timeoutInt)
		}
	}
	if headersPair, ok := options["headers"]; ok {
		if headersDict, err := headersPair.AsDict(); err == nil {
			headers = extractHeaders(headersDict)
		}
	}
	if paramsPair, ok := options["params"]; ok {
		if paramsDict, err := paramsPair.AsDict(); err == nil {
			params = extractParams(paramsDict)
		}
	}
	if authPair, ok := options["auth"]; ok {
		if authList, err := authPair.AsList(); err == nil {
			if len(authList) == 2 {
				if u, err := authList[0].AsString(); err == nil {
					user = u
				}
				if p, err := authList[1].AsString(); err == nil {
					pass = p
				}
			}
		}
	}
	return timeout, headers, params, user, pass
}

// extractParams extracts params from a dict, converting all values to strings
func extractParams(dict map[string]object.Object) map[string]string {
	params := make(map[string]string)
	for key, value := range dict {
		// Convert any value type to string (int, float, bool, etc.)
		if strVal, err := value.CoerceString(); err == nil {
			params[key] = strVal
		}
	}
	return params
}

// buildURLWithParams appends query parameters to a URL
func buildURLWithParams(baseURL string, params map[string]string) string {
	if len(params) == 0 {
		return baseURL
	}

	// Parse existing URL to preserve any existing query params
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		// If parsing fails, just append params naively
		return baseURL + "?" + encodeParams(params)
	}

	// Add new params to existing query
	query := parsedURL.Query()
	for key, value := range params {
		query.Set(key, value)
	}
	parsedURL.RawQuery = query.Encode()
	return parsedURL.String()
}

// encodeParams encodes params to URL query string format
func encodeParams(params map[string]string) string {
	values := url.Values{}
	for key, value := range params {
		values.Set(key, value)
	}
	return values.Encode()
}

// extractRequestArgs extracts URL, optional data, and remaining options from kwargs and args.
// hasData should be true for POST/PUT/PATCH methods, false for GET/DELETE.
// Returns (url, data, options, errorOrNil).
func extractRequestArgs(kwargs object.Kwargs, args []object.Object, hasData bool) (string, string, map[string]object.Object, object.Object) {
	var url, data string
	options := make(map[string]object.Object)

	// 1. Handle kwargs
	for k, v := range kwargs.Kwargs {
		if k == "url" {
			if s, err := v.AsString(); err == nil {
				url = s
			} else {
				return "", "", nil, errors.NewTypeError("STRING", v.Type().String())
			}
		} else if hasData && k == "data" {
			if s, err := v.AsString(); err == nil {
				data = s
			} else {
				return "", "", nil, errors.NewTypeError("STRING", v.Type().String())
			}
		} else if hasData && k == "json" {
			// Handle json parameter - convert to JSON string
			jsonBytes, err := json.Marshal(conversion.ToGo(v))
			if err != nil {
				return "", "", nil, errors.NewError("failed to encode json: %s", err.Error())
			}
			data = string(jsonBytes)
			// Set Content-Type header if not already set
			if _, hasHeaders := options["headers"]; !hasHeaders {
				contentTypeDict := &object.Dict{Pairs: make(map[string]object.DictPair)}
				contentTypeDict.SetByString("Content-Type", object.NewString("application/json"))
				options["headers"] = contentTypeDict
			}
		} else {
			options[k] = v
		}
	}

	// 2. Handle positional args
	argIdx := 0
	if url == "" && len(args) > argIdx {
		if s, err := args[argIdx].AsString(); err == nil {
			url = s
			argIdx++
		} else {
			return "", "", nil, errors.NewTypeError("STRING", args[argIdx].Type().String())
		}
	}

	if hasData && data == "" && len(args) > argIdx {
		if s, err := args[argIdx].AsString(); err == nil {
			data = s
			argIdx++
		} else if args[argIdx].Type() != object.DICT_OBJ {
			// Not a string and not a dict (options), error
			return "", "", nil, errors.NewTypeError("STRING", args[argIdx].Type().String())
		}
		// If it's a dict, we'll process it as options below
	}

	// Check for legacy options dict
	if len(args) > argIdx {
		if d, err := args[argIdx].AsDict(); err == nil {
			for k, v := range d {
				options[k] = v
			}
		}
	}

	if url == "" {
		return "", "", nil, errors.NewArgumentError(0, 1)
	}

	return url, data, options, nil
}

var RequestsLibrary = object.NewLibrary(RequestsLibraryName, map[string]*object.Builtin{
	// Exceptions namespace - returns dict with exception types
	"exceptions": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return exceptionsNamespace
		},
	},
	"get": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			rawURL, _, options, err := extractRequestArgs(kwargs, args, false)
			if err != nil {
				return err
			}
			timeout, headers, params, user, pass := parseRequestOptions(options)
			// Build URL with query params
			fullURL := buildURLWithParams(rawURL, params)
			return httpRequestWithContext(ctx, "GET", fullURL, "", timeout, headers, user, pass)
		},
		HelpText: `get(url, **kwargs) - Send a GET request

Sends an HTTP GET request to the specified URL.

Parameters:
  url (string): The URL to send the request to
  **kwargs: Optional arguments
    - timeout (int): Request timeout in seconds (default: 5)
    - headers (dict): HTTP headers as key-value pairs
    - params (dict): Query parameters to append to URL
    - auth (tuple/list): Basic authentication as (username, password)

Returns:
  Response object with status_code, text, headers, body, url, and json() method`,
	},
	"post": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			url, data, options, err := extractRequestArgs(kwargs, args, true)
			if err != nil {
				return err
			}
			timeout, headers, params, user, pass := parseRequestOptions(options)
			// POST can also have query params
			fullURL := buildURLWithParams(url, params)
			return httpRequestWithContext(ctx, "POST", fullURL, data, timeout, headers, user, pass)
		},
		HelpText: `post(url, data=None, json=None, **kwargs) - Send a POST request

Sends an HTTP POST request to the specified URL with the given data.

Parameters:
  url (string): The URL to send the request to
  data (string, optional): The request body data as a string
  json (dict/list, optional): Data to be JSON-encoded and sent (sets Content-Type to application/json)
  **kwargs: Optional arguments
    - timeout (int): Request timeout in seconds (default: 5)
    - headers (dict): HTTP headers as key-value pairs
    - auth (tuple/list): Basic authentication as (username, password)

Note: Use either 'data' or 'json', not both.

Returns:
  Response object with status_code, text, headers, body, url, and json() method`,
	},
	"put": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			url, data, options, err := extractRequestArgs(kwargs, args, true)
			if err != nil {
				return err
			}
			timeout, headers, params, user, pass := parseRequestOptions(options)
			fullURL := buildURLWithParams(url, params)
			return httpRequestWithContext(ctx, "PUT", fullURL, data, timeout, headers, user, pass)
		},
		HelpText: `put(url, data=None, json=None, **kwargs) - Send a PUT request

Sends an HTTP PUT request to the specified URL with the given data.

Parameters:
  url (string): The URL to send the request to
  data (string, optional): The request body data as a string
  json (dict/list, optional): Data to be JSON-encoded and sent (sets Content-Type to application/json)
  **kwargs: Optional arguments
    - timeout (int): Request timeout in seconds (default: 5)
    - headers (dict): HTTP headers as key-value pairs
    - auth (tuple/list): Basic authentication as (username, password)

Note: Use either 'data' or 'json', not both.

Returns:
  Response object with status_code, text, headers, body, url, and json() method`,
	},
	"delete": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			rawURL, _, options, err := extractRequestArgs(kwargs, args, false)
			if err != nil {
				return err
			}
			timeout, headers, params, user, pass := parseRequestOptions(options)
			fullURL := buildURLWithParams(rawURL, params)
			return httpRequestWithContext(ctx, "DELETE", fullURL, "", timeout, headers, user, pass)
		},
		HelpText: `delete(url, **kwargs) - Send a DELETE request

Sends an HTTP DELETE request to the specified URL.

Parameters:
  url (string): The URL to send the request to
  **kwargs: Optional arguments
    - timeout (int): Request timeout in seconds (default: 5)
    - headers (dict): HTTP headers as key-value pairs
    - auth (tuple/list): Basic authentication as (username, password)

Returns:
  Response object with status_code, text, headers, body, url, and json() method`,
	},
	"patch": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			url, data, options, err := extractRequestArgs(kwargs, args, true)
			if err != nil {
				return err
			}
			timeout, headers, params, user, pass := parseRequestOptions(options)
			fullURL := buildURLWithParams(url, params)
			return httpRequestWithContext(ctx, "PATCH", fullURL, data, timeout, headers, user, pass)
		},
		HelpText: `patch(url, data=None, json=None, **kwargs) - Send a PATCH request

Sends an HTTP PATCH request to the specified URL with the given data.

Parameters:
  url (string): The URL to send the request to
  data (string, optional): The request body data as a string
  json (dict/list, optional): Data to be JSON-encoded and sent (sets Content-Type to application/json)
  **kwargs: Optional arguments
    - timeout (int): Request timeout in seconds (default: 5)
    - headers (dict): HTTP headers as key-value pairs
    - auth (tuple/list): Basic authentication as (username, password)

Note: Use either 'data' or 'json', not both.

Returns:
  Response object with status_code, text, headers, body, url, and json() method`,
	},
	"parallel": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) < 1 {
				return errors.NewArgumentError(1, 0)
			}
			requestList, err := args[0].AsList()
			if err != nil {
				return errors.NewTypeError("LIST", args[0].Type().String())
			}
			if len(requestList) == 0 {
				return &object.List{Elements: []object.Object{}}
			}

			maxParallel := int64(4)
			if mp, mpErr := kwargs.GetInt("max_parallel", 4); mpErr == nil {
				maxParallel = mp
			}
			if maxParallel < 1 {
				maxParallel = 1
			}

			results := make([]object.Object, len(requestList))
			sem := make(chan struct{}, maxParallel)
			var wg sync.WaitGroup

			for i, reqObj := range requestList {
				reqDict, dictErr := reqObj.AsDict()
				if dictErr != nil {
					results[i] = errors.NewTypeError("DICT", reqObj.Type().String())
					continue
				}

				wg.Add(1)
				go func(idx int, rd map[string]object.Object) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()

					results[idx] = executeParallelRequest(ctx, rd)
				}(i, reqDict)
			}

			wg.Wait()
			return &object.List{Elements: results}
		},
		HelpText: `parallel(requests, max_parallel=4) - Execute multiple HTTP requests in parallel

Sends multiple HTTP requests concurrently with a configurable concurrency limit.
Results are returned in the same order as the input requests.

Parameters:
  requests (list): List of request dicts, each with:
    - method (string): HTTP method ("GET", "POST", "PUT", "DELETE", "PATCH")
    - url (string): The URL to send the request to
    - data (string, optional): Request body as string
    - json (dict/list, optional): Data to JSON-encode as body
    - headers (dict, optional): HTTP headers
    - params (dict, optional): Query parameters
    - auth (list/tuple, optional): [username, password] for basic auth
    - timeout (int, optional): Timeout in seconds (default: 30)
  max_parallel (int): Maximum concurrent requests (default: 4)

Returns:
  list: List of Response objects in the same order as input requests.
        Failed requests return a Response with status_code=0 and the error in body.

Example:
  results = requests.parallel([
      {"method": "GET", "url": "https://api.example.com/item/1"},
      {"method": "POST", "url": "https://api.example.com/item/2", "json": {"key": "val"}},
  ], max_parallel=4)
  for resp in results:
      if resp.status_code == 200:
          data = resp.json()`,
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
		if strVal, err := value.AsString(); err == nil {
			headers[key] = strVal
		}
	}
	return headers
}

func httpRequestWithContext(parentCtx context.Context, method, url, body string, timeoutSecs int, headers map[string]string, user, pass string) object.Object {
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

	// Set basic auth if provided
	if user != "" {
		req.SetBasicAuth(user, pass)
	}

	resp, err := pool.GetHTTPClient().Do(req)
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

// executeParallelRequest executes a single request from a parallel batch spec dict.
func executeParallelRequest(ctx context.Context, spec map[string]object.Object) object.Object {
	// Extract method
	method := "GET"
	if methodObj, ok := spec["method"]; ok {
		if m, err := methodObj.AsString(); err == nil {
			method = strings.ToUpper(m)
		}
	}

	// Extract URL
	urlStr := ""
	if urlObj, ok := spec["url"]; ok {
		if u, err := urlObj.AsString(); err == nil {
			urlStr = u
		}
	}
	if urlStr == "" {
		return createResponseInstance(0, nil, []byte("parallel request missing 'url'"), "")
	}

	// Extract timeout (default 30s for parallel — longer than individual default)
	timeout := 30
	if timeoutObj, ok := spec["timeout"]; ok {
		if t, err := timeoutObj.AsInt(); err == nil {
			timeout = int(t)
		}
	}

	// Extract headers
	headers := make(map[string]string)
	if headersObj, ok := spec["headers"]; ok {
		if d, err := headersObj.AsDict(); err == nil {
			headers = extractHeaders(d)
		}
	}

	// Extract query params
	if paramsObj, ok := spec["params"]; ok {
		if d, err := paramsObj.AsDict(); err == nil {
			params := extractParams(d)
			urlStr = buildURLWithParams(urlStr, params)
		}
	}

	// Extract auth
	user, pass := "", ""
	if authObj, ok := spec["auth"]; ok {
		if authList, err := authObj.AsList(); err == nil && len(authList) == 2 {
			if u, err := authList[0].AsString(); err == nil {
				user = u
			}
			if p, err := authList[1].AsString(); err == nil {
				pass = p
			}
		}
	}

	// Extract body — either 'data' (string) or 'json' (dict/list to encode)
	body := ""
	if jsonObj, ok := spec["json"]; ok {
		jsonBytes, err := json.Marshal(conversion.ToGo(jsonObj))
		if err != nil {
			return createResponseInstance(0, nil, []byte(fmt.Sprintf("failed to encode json: %s", err.Error())), urlStr)
		}
		body = string(jsonBytes)
		if _, hasContentType := headers["Content-Type"]; !hasContentType {
			headers["Content-Type"] = "application/json"
		}
	} else if dataObj, ok := spec["data"]; ok {
		if d, err := dataObj.AsString(); err == nil {
			body = d
		}
	}

	return httpRequestWithContext(ctx, method, urlStr, body, timeout, headers, user, pass)
}
