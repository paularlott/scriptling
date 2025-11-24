package stdlib

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"
	"github.com/paularlott/scriptling/object"
)

func HTTPLibrary() map[string]*object.Builtin {
	return map[string]*object.Builtin{
		"get": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) < 1 || len(args) > 2 {
					return newError("wrong number of arguments. got=%d, want=1-2", len(args))
				}
				if args[0].Type() != object.STRING_OBJ {
					return newError("url must be STRING")
				}
				url := args[0].(*object.String).Value
				timeout := 5 // Default 5 seconds
				headers := make(map[string]string)
				
				if len(args) == 2 {
					if args[1].Type() != object.DICT_OBJ {
						return newError("options must be DICT")
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
				return httpRequest("GET", url, "", timeout, headers)
			},
		},
		"post": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) < 2 || len(args) > 3 {
					return newError("wrong number of arguments. got=%d, want=2-3", len(args))
				}
				if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
					return newError("url and body must be STRING")
				}
				url := args[0].(*object.String).Value
				body := args[1].(*object.String).Value
				timeout := 5
				headers := make(map[string]string)
				
				if len(args) == 3 {
					if args[2].Type() != object.DICT_OBJ {
						return newError("options must be DICT")
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
				return httpRequest("POST", url, body, timeout, headers)
			},
		},
		"put": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) < 2 || len(args) > 3 {
					return newError("wrong number of arguments. got=%d, want=2-3", len(args))
				}
				if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
					return newError("url and body must be STRING")
				}
				url := args[0].(*object.String).Value
				body := args[1].(*object.String).Value
				timeout := 5
				headers := make(map[string]string)
				
				if len(args) == 3 {
					if args[2].Type() != object.DICT_OBJ {
						return newError("options must be DICT")
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
				return httpRequest("PUT", url, body, timeout, headers)
			},
		},
		"delete": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) < 1 || len(args) > 2 {
					return newError("wrong number of arguments. got=%d, want=1-2", len(args))
				}
				if args[0].Type() != object.STRING_OBJ {
					return newError("url must be STRING")
				}
				url := args[0].(*object.String).Value
				timeout := 5
				headers := make(map[string]string)
				
				if len(args) == 2 {
					if args[1].Type() != object.DICT_OBJ {
						return newError("options must be DICT")
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
				return httpRequest("DELETE", url, "", timeout, headers)
			},
		},
		"patch": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) < 2 || len(args) > 3 {
					return newError("wrong number of arguments. got=%d, want=2-3", len(args))
				}
				if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
					return newError("url and body must be STRING")
				}
				url := args[0].(*object.String).Value
				body := args[1].(*object.String).Value
				timeout := 5
				headers := make(map[string]string)
				
				if len(args) == 3 {
					if args[2].Type() != object.DICT_OBJ {
						return newError("options must be DICT")
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
				return httpRequest("PATCH", url, body, timeout, headers)
			},
		},
	}
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

func httpRequest(method, url, body string, timeoutSecs int, headers map[string]string) object.Object {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSecs)*time.Second)
	defer cancel()

	var req *http.Request
	var err error

	if body != "" {
		req, err = http.NewRequestWithContext(ctx, method, url, strings.NewReader(body))
		if err != nil {
			return newError("http request error: %s", err.Error())
		}
		if _, hasContentType := headers["Content-Type"]; !hasContentType {
			req.Header.Set("Content-Type", "application/json")
		}
	} else {
		req, err = http.NewRequestWithContext(ctx, method, url, nil)
		if err != nil {
			return newError("http request error: %s", err.Error())
		}
	}

	// Set custom headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return newError("http timeout after %d seconds", timeoutSecs)
		}
		return newError("http error: %s", err.Error())
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return newError("http read error: %s", err.Error())
	}

	respHeaders := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			respHeaders[key] = values[0]
		}
	}

	pairs := make(map[string]object.DictPair)
	pairs["status"] = object.DictPair{
		Key:   &object.String{Value: "status"},
		Value: &object.Integer{Value: int64(resp.StatusCode)},
	}
	pairs["body"] = object.DictPair{
		Key:   &object.String{Value: "body"},
		Value: &object.String{Value: string(respBody)},
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

	return &object.Dict{Pairs: pairs}
}
