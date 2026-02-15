package extlibs

import (
	"context"
	"testing"

	"github.com/paularlott/scriptling/object"
)

func TestHTTPGet(t *testing.T) {
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	// Reset routes
	ResetHTTPRoutes()

	getFn := HTTPLibrary.Functions()["get"]
	result := getFn.Fn(ctx, kwargs,
		&object.String{Value: "/api/test"},
		&object.String{Value: "handlers.test"},
	)

	if _, isNull := result.(*object.Null); !isNull {
		t.Errorf("get() should return None, got %T", result)
	}

	// Verify route was registered
	if HTTPRoutes.Routes["/api/test"] == nil {
		t.Error("route not registered")
	}
	route := HTTPRoutes.Routes["/api/test"]
	if route.Handler != "handlers.test" {
		t.Errorf("handler = %s, want handlers.test", route.Handler)
	}
	if len(route.Methods) != 1 || route.Methods[0] != "GET" {
		t.Errorf("methods = %v, want [GET]", route.Methods)
	}
}

func TestHTTPPost(t *testing.T) {
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	ResetHTTPRoutes()

	postFn := HTTPLibrary.Functions()["post"]
	result := postFn.Fn(ctx, kwargs,
		&object.String{Value: "/api/users"},
		&object.String{Value: "handlers.create_user"},
	)

	if _, isNull := result.(*object.Null); !isNull {
		t.Errorf("post() should return None, got %T", result)
	}

	route := HTTPRoutes.Routes["/api/users"]
	if route == nil {
		t.Fatal("route not registered")
	}
	if route.Handler != "handlers.create_user" {
		t.Errorf("handler = %s, want handlers.create_user", route.Handler)
	}
	if len(route.Methods) != 1 || route.Methods[0] != "POST" {
		t.Errorf("methods = %v, want [POST]", route.Methods)
	}
}

func TestHTTPRoute(t *testing.T) {
	ctx := context.Background()

	ResetHTTPRoutes()

	routeFn := HTTPLibrary.Functions()["route"]
	kwargs := object.NewKwargs(map[string]object.Object{
		"methods": &object.List{Elements: []object.Object{
			&object.String{Value: "GET"},
			&object.String{Value: "POST"},
		}},
	})
	result := routeFn.Fn(ctx, kwargs,
		&object.String{Value: "/api/data"},
		&object.String{Value: "handlers.data"},
	)

	if _, isNull := result.(*object.Null); !isNull {
		t.Errorf("route() should return None, got %T", result)
	}

	route := HTTPRoutes.Routes["/api/data"]
	if route == nil {
		t.Fatal("route not registered")
	}
	if len(route.Methods) != 2 {
		t.Errorf("methods count = %d, want 2", len(route.Methods))
	}
}

func TestHTTPMiddleware(t *testing.T) {
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	ResetHTTPRoutes()

	middlewareFn := HTTPLibrary.Functions()["middleware"]
	result := middlewareFn.Fn(ctx, kwargs, &object.String{Value: "auth.check"})

	if _, isNull := result.(*object.Null); !isNull {
		t.Errorf("middleware() should return None, got %T", result)
	}

	if HTTPRoutes.Middleware != "auth.check" {
		t.Errorf("middleware = %s, want auth.check", HTTPRoutes.Middleware)
	}
}

func TestHTTPStatic(t *testing.T) {
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	ResetHTTPRoutes()

	staticFn := HTTPLibrary.Functions()["static"]
	result := staticFn.Fn(ctx, kwargs,
		&object.String{Value: "/assets"},
		&object.String{Value: "./public"},
	)

	if _, isNull := result.(*object.Null); !isNull {
		t.Errorf("static() should return None, got %T", result)
	}

	route := HTTPRoutes.Routes["/assets"]
	if route == nil {
		t.Fatal("static route not registered")
	}
	if !route.Static {
		t.Error("route should be marked as static")
	}
	if route.StaticDir != "./public" {
		t.Errorf("staticDir = %s, want ./public", route.StaticDir)
	}
}

func TestHTTPJson(t *testing.T) {
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	jsonFn := HTTPLibrary.Functions()["json"]

	// Test with status and data
	result := jsonFn.Fn(ctx, kwargs,
		&object.Integer{Value: 200},
		&object.Dict{Pairs: map[string]object.DictPair{
			"status": {Key: &object.String{Value: "status"}, Value: &object.String{Value: "ok"}},
		}},
	)

	dict, ok := result.(*object.Dict)
	if !ok {
		t.Fatalf("json() should return Dict, got %T", result)
	}

	statusPair, exists := dict.Pairs["status"]
	if !exists {
		t.Fatal("missing status key")
	}
	status, ok := statusPair.Value.(*object.Integer)
	if !ok || status.Value != 200 {
		t.Errorf("status = %d, want 200", status.Value)
	}
}

func TestHTTPRedirect(t *testing.T) {
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	redirectFn := HTTPLibrary.Functions()["redirect"]
	result := redirectFn.Fn(ctx, kwargs,
		&object.String{Value: "/new-location"},
	)

	dict, ok := result.(*object.Dict)
	if !ok {
		t.Fatalf("redirect() should return Dict, got %T", result)
	}

	statusPair, exists := dict.Pairs["status"]
	if !exists {
		t.Fatal("missing status key")
	}
	status, ok := statusPair.Value.(*object.Integer)
	if !ok || status.Value != 302 {
		t.Errorf("status = %d, want 302", status.Value)
	}
}

func TestHTTPText(t *testing.T) {
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	textFn := HTTPLibrary.Functions()["text"]
	result := textFn.Fn(ctx, kwargs,
		&object.Integer{Value: 200},
		&object.String{Value: "Hello World"},
	)

	dict, ok := result.(*object.Dict)
	if !ok {
		t.Fatalf("text() should return Dict, got %T", result)
	}

	bodyPair, exists := dict.Pairs["body"]
	if !exists {
		t.Fatal("missing body key")
	}
	body, ok := bodyPair.Value.(*object.String)
	if !ok || body.Value != "Hello World" {
		t.Errorf("body = %s, want Hello World", body.Value)
	}
}

func TestHTTPHtml(t *testing.T) {
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	htmlFn := HTTPLibrary.Functions()["html"]
	result := htmlFn.Fn(ctx, kwargs,
		&object.Integer{Value: 200},
		&object.String{Value: "<h1>Hello</h1>"},
	)

	dict, ok := result.(*object.Dict)
	if !ok {
		t.Fatalf("html() should return Dict, got %T", result)
	}

	bodyPair, exists := dict.Pairs["body"]
	if !exists {
		t.Fatal("missing body key")
	}
	body, ok := bodyPair.Value.(*object.String)
	if !ok || body.Value != "<h1>Hello</h1>" {
		t.Errorf("body = %s, want <h1>Hello</h1>", body.Value)
	}
}

func TestHTTPParseQuery(t *testing.T) {
	ctx := context.Background()
	kwargs := object.NewKwargs(nil)

	parseQueryFn := HTTPLibrary.Functions()["parse_query"]
	result := parseQueryFn.Fn(ctx, kwargs,
		&object.String{Value: "name=John&age=30"},
	)

	dict, ok := result.(*object.Dict)
	if !ok {
		t.Fatalf("parse_query() should return Dict, got %T", result)
	}

	namePair, exists := dict.Pairs["name"]
	if !exists {
		t.Fatal("missing name key")
	}
	name, ok := namePair.Value.(*object.String)
	if !ok || name.Value != "John" {
		t.Errorf("name = %s, want John", name.Value)
	}

	agePair, exists := dict.Pairs["age"]
	if !exists {
		t.Fatal("missing age key")
	}
	age, ok := agePair.Value.(*object.String)
	if !ok || age.Value != "30" {
		t.Errorf("age = %s, want 30", age.Value)
	}
}

func TestCreateRequestInstance(t *testing.T) {
	req := CreateRequestInstance(
		"POST",
		"/api/test",
		`{"key": "value"}`,
		map[string]string{"content-type": "application/json"},
		map[string]string{"foo": "bar"},
	)

	if req.Class.Name != "Request" {
		t.Errorf("class name = %s, want Request", req.Class.Name)
	}

	method, ok := req.Fields["method"].(*object.String)
	if !ok || method.Value != "POST" {
		t.Errorf("method = %v, want POST", req.Fields["method"])
	}

	path, ok := req.Fields["path"].(*object.String)
	if !ok || path.Value != "/api/test" {
		t.Errorf("path = %v, want /api/test", req.Fields["path"])
	}

	body, ok := req.Fields["body"].(*object.String)
	if !ok || body.Value != `{"key": "value"}` {
		t.Errorf("body = %v, want {\"key\": \"value\"}", req.Fields["body"])
	}
}
