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
					return newError("wrong number of arguments. got=%d, want=1 or 2", len(args))
				}
				if args[0].Type() != object.STRING_OBJ {
					return newError("argument must be STRING")
				}
				url := args[0].(*object.String).Value
				timeout := 30
				if len(args) == 2 {
					if args[1].Type() != object.INTEGER_OBJ {
						return newError("timeout must be INTEGER")
					}
					timeout = int(args[1].(*object.Integer).Value)
				}
				return httpRequest("GET", url, "", timeout)
			},
		},
		"post": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) < 2 || len(args) > 3 {
					return newError("wrong number of arguments. got=%d, want=2 or 3", len(args))
				}
				if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
					return newError("arguments must be STRING")
				}
				url := args[0].(*object.String).Value
				body := args[1].(*object.String).Value
				timeout := 30
				if len(args) == 3 {
					if args[2].Type() != object.INTEGER_OBJ {
						return newError("timeout must be INTEGER")
					}
					timeout = int(args[2].(*object.Integer).Value)
				}
				return httpRequest("POST", url, body, timeout)
			},
		},
		"put": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) < 2 || len(args) > 3 {
					return newError("wrong number of arguments. got=%d, want=2 or 3", len(args))
				}
				if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
					return newError("arguments must be STRING")
				}
				url := args[0].(*object.String).Value
				body := args[1].(*object.String).Value
				timeout := 30
				if len(args) == 3 {
					if args[2].Type() != object.INTEGER_OBJ {
						return newError("timeout must be INTEGER")
					}
					timeout = int(args[2].(*object.Integer).Value)
				}
				return httpRequest("PUT", url, body, timeout)
			},
		},
		"delete": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) < 1 || len(args) > 2 {
					return newError("wrong number of arguments. got=%d, want=1 or 2", len(args))
				}
				if args[0].Type() != object.STRING_OBJ {
					return newError("argument must be STRING")
				}
				url := args[0].(*object.String).Value
				timeout := 30
				if len(args) == 2 {
					if args[1].Type() != object.INTEGER_OBJ {
						return newError("timeout must be INTEGER")
					}
					timeout = int(args[1].(*object.Integer).Value)
				}
				return httpRequest("DELETE", url, "", timeout)
			},
		},
		"patch": {
			Fn: func(args ...object.Object) object.Object {
				if len(args) < 2 || len(args) > 3 {
					return newError("wrong number of arguments. got=%d, want=2 or 3", len(args))
				}
				if args[0].Type() != object.STRING_OBJ || args[1].Type() != object.STRING_OBJ {
					return newError("arguments must be STRING")
				}
				url := args[0].(*object.String).Value
				body := args[1].(*object.String).Value
				timeout := 30
				if len(args) == 3 {
					if args[2].Type() != object.INTEGER_OBJ {
						return newError("timeout must be INTEGER")
					}
					timeout = int(args[2].(*object.Integer).Value)
				}
				return httpRequest("PATCH", url, body, timeout)
			},
		},
	}
}

func httpRequest(method, url, body string, timeoutSecs int) object.Object {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSecs)*time.Second)
	defer cancel()

	var req *http.Request
	var err error

	if body != "" {
		req, err = http.NewRequestWithContext(ctx, method, url, strings.NewReader(body))
		if err != nil {
			return newError("http request error: %s", err.Error())
		}
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequestWithContext(ctx, method, url, nil)
		if err != nil {
			return newError("http request error: %s", err.Error())
		}
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

	headers := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
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
	for k, v := range headers {
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
