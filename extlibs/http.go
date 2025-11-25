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

var requestsLibrary = object.NewLibrary(map[string]*object.Builtin{
	// Exception classes (as strings for except clause matching)
	"RequestException": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			return requestExceptionType
		},
	},
	"HTTPError": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			return httpErrorType
		},
	},
	// Exceptions namespace - returns dict with exception types
	"exceptions": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			return exceptionsNamespace
		},
	},
	"get": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) < 1 || len(args) > 2 {
				return errors.NewArgumentError(len(args), 1)
			}
			if args[0].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", string(args[0].Type()))
			}
			url := args[0].(*object.String).Value
			timeout := 5 // Default 5 seconds
			headers := make(map[string]string)

			if len(args) == 2 {
				if args[1].Type() != object.DICT_OBJ {
					return errors.NewTypeError("DICT", string(args[1].Type()))
				}
				options := args[1].(*object.Dict)
				if timeoutPair, ok := options.Pairs["timeout"]; ok {
					if timeoutInt, ok := timeoutPair.Value.(*object.Integer); ok {
						timeout = int(timeoutInt.Value)
					}
				}
				if headersPair, ok := options.Pairs["headers"]; ok {
					if headersDict, ok := headersPair.Value.(*object.Dict); ok {
						headers = extractHeaders(headersDict)
					}
				}
			}
			return httpRequestWithContext(ctx, "GET", url, "", timeout, headers)
		},
	},
	"post": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) < 2 || len(args) > 3 {
				return errors.NewArgumentError(len(args), 2)
			}
			if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", "mixed types")
			}
			url := args[0].(*object.String).Value
			body := args[1].(*object.String).Value
			timeout := 5
			headers := make(map[string]string)

			if len(args) == 3 {
				if args[2].Type() != object.DICT_OBJ {
					return errors.NewTypeError("DICT", string(args[2].Type()))
				}
				options := args[2].(*object.Dict)
				if timeoutPair, ok := options.Pairs["timeout"]; ok {
					if timeoutInt, ok := timeoutPair.Value.(*object.Integer); ok {
						timeout = int(timeoutInt.Value)
					}
				}
				if headersPair, ok := options.Pairs["headers"]; ok {
					if headersDict, ok := headersPair.Value.(*object.Dict); ok {
						headers = extractHeaders(headersDict)
					}
				}
			}
			return httpRequestWithContext(ctx, "POST", url, body, timeout, headers)
		},
	},
	"put": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) < 2 || len(args) > 3 {
				return errors.NewArgumentError(len(args), 2)
			}
			if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", "mixed types")
			}
			url := args[0].(*object.String).Value
			body := args[1].(*object.String).Value
			timeout := 5
			headers := make(map[string]string)

			if len(args) == 3 {
				if args[2].Type() != object.DICT_OBJ {
					return errors.NewTypeError("DICT", string(args[2].Type()))
				}
				options := args[2].(*object.Dict)
				if timeoutPair, ok := options.Pairs["timeout"]; ok {
					if timeoutInt, ok := timeoutPair.Value.(*object.Integer); ok {
						timeout = int(timeoutInt.Value)
					}
				}
				if headersPair, ok := options.Pairs["headers"]; ok {
					if headersDict, ok := headersPair.Value.(*object.Dict); ok {
						headers = extractHeaders(headersDict)
					}
				}
			}
			return httpRequestWithContext(ctx, "PUT", url, body, timeout, headers)
		},
	},
	"delete": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) < 1 || len(args) > 2 {
				return errors.NewArgumentError(len(args), 1)
			}
			if args[0].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", string(args[0].Type()))
			}
			url := args[0].(*object.String).Value
			timeout := 5
			headers := make(map[string]string)

			if len(args) == 2 {
				if args[1].Type() != object.DICT_OBJ {
					return errors.NewTypeError("DICT", string(args[1].Type()))
				}
				options := args[1].(*object.Dict)
				if timeoutPair, ok := options.Pairs["timeout"]; ok {
					if timeoutInt, ok := timeoutPair.Value.(*object.Integer); ok {
						timeout = int(timeoutInt.Value)
					}
				}
				if headersPair, ok := options.Pairs["headers"]; ok {
					if headersDict, ok := headersPair.Value.(*object.Dict); ok {
						headers = extractHeaders(headersDict)
					}
				}
			}
			return httpRequestWithContext(ctx, "DELETE", url, "", timeout, headers)
		},
	},
	"patch": {
		Fn: func(ctx context.Context, args ...object.Object) object.Object {
			if len(args) < 2 || len(args) > 3 {
				return errors.NewArgumentError(len(args), 2)
			}
			if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
				return errors.NewTypeError("STRING", "mixed types")
			}
			url := args[0].(*object.String).Value
			body := args[1].(*object.String).Value
			timeout := 5
			headers := make(map[string]string)

			if len(args) == 3 {
				if args[2].Type() != object.DICT_OBJ {
					return errors.NewTypeError("DICT", string(args[2].Type()))
				}
				options := args[2].(*object.Dict)
				if timeoutPair, ok := options.Pairs["timeout"]; ok {
					if timeoutInt, ok := timeoutPair.Value.(*object.Integer); ok {
						timeout = int(timeoutInt.Value)
					}
				}
				if headersPair, ok := options.Pairs["headers"]; ok {
					if headersDict, ok := headersPair.Value.(*object.Dict); ok {
						headers = extractHeaders(headersDict)
					}
				}
			}
			return httpRequestWithContext(ctx, "PATCH", url, body, timeout, headers)
		},
	},
})

func RequestsLibrary() *object.Library {
	return requestsLibrary
}

// HTTPLibrary is deprecated, use RequestsLibrary instead
func HTTPLibrary() *object.Library {
	return requestsLibrary
}

func extractHeaders(dict *object.Dict) map[string]string {
	headers := make(map[string]string)
	for _, pair := range dict.Pairs {
		if strVal, ok := pair.Value.(*object.String); ok {
			headers[pair.Key.Inspect()] = strVal.Value
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

	pairs := make(map[string]object.DictPair)
	statusCode := &object.Integer{Value: int64(resp.StatusCode)}
	bodyText := &object.String{Value: string(respBody)}

	// Requests-compatible keys
	pairs["status_code"] = object.DictPair{
		Key:   &object.String{Value: "status_code"},
		Value: statusCode,
	}
	pairs["text"] = object.DictPair{
		Key:   &object.String{Value: "text"},
		Value: bodyText,
	}

	headerPairs := make(map[string]object.DictPair)
	for k, v := range respHeaders {
		headerPairs[k] = object.DictPair{
			Key:   &object.String{Value: k},
			Value: &object.String{Value: v},
		}
	}
	pairs["headers"] = object.DictPair{
		Key:   &object.String{Value: "headers"},
		Value: &object.Dict{Pairs: headerPairs},
	}

	// Add json() method - parses response body as JSON
	pairs["json"] = object.DictPair{
		Key: &object.String{Value: "json"},
		Value: &object.Builtin{
			Fn: func(ctx context.Context, args ...object.Object) object.Object {
				if len(args) != 0 {
					return errors.NewArgumentError(len(args), 0)
				}

				// Parse the JSON body
				var result interface{}
				if err := json.Unmarshal(respBody, &result); err != nil {
					return errors.NewError("JSONDecodeError: %s", err.Error())
				}

				// Convert to Scriptling object
				return convertJSONToObject(result)
			},
		},
	}

	// Add raise_for_status() method - raises error if status >= 400
	pairs["raise_for_status"] = object.DictPair{
		Key: &object.String{Value: "raise_for_status"},
		Value: &object.Builtin{
			Fn: func(ctx context.Context, args ...object.Object) object.Object {
				if len(args) != 0 {
					return errors.NewArgumentError(len(args), 0)
				}

				if resp.StatusCode >= 400 {
					if resp.StatusCode >= 500 {
						return errors.NewError("HTTPError: %d Server Error", resp.StatusCode)
					} else {
						return errors.NewError("HTTPError: %d Client Error", resp.StatusCode)
					}
				}

				return &object.Null{}
			},
		},
	}

	return &object.Dict{Pairs: pairs}
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
