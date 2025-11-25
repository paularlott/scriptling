package scriptling

import (
	"context"
	"testing"

	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/object"
)

func TestRegisterFunc(t *testing.T) {
	p := New()
	p.RegisterFunc("double", func(ctx context.Context, args ...object.Object) object.Object {
		if len(args) != 1 {
			return &object.Error{Message: "need 1 argument"}
		}
		val := args[0].(*object.Integer).Value
		return &object.Integer{Value: val * 2}
	})

	_, err := p.Eval("result = double(5)")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	result, ok := p.GetVar("result")
	if !ok || result != int64(10) {
		t.Errorf("expected 10, got %v", result)
	}
}

func TestRegisterLibrary(t *testing.T) {
	p := New()
	myLib := object.NewLibrary(map[string]*object.Builtin{
		"greet": {
			Fn: func(ctx context.Context, args ...object.Object) object.Object {
				return &object.String{Value: "Hello!"}
			},
		},
	})
	p.RegisterLibrary("mylib", myLib)

	_, err := p.Eval(`
import mylib
msg = mylib.greet()
`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	msg, ok := p.GetVar("msg")
	if !ok || msg != "Hello!" {
		t.Errorf("expected Hello!, got %v", msg)
	}
}

func TestImportBuiltin(t *testing.T) {
	p := New()
	_, err := p.Eval(`
import json
data = json.parse('{"key":"value"}')
result = data["key"]
`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	result, ok := p.GetVar("result")
	if !ok || result != "value" {
		t.Errorf("expected value, got %v", result)
	}
}

func TestModuloOperator(t *testing.T) {
	p := New()
	_, err := p.Eval("result = 10 % 3")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	result, ok := p.GetVar("result")
	if !ok || result != int64(1) {
		t.Errorf("expected 1, got %v", result)
	}
}

func TestBooleanOperators(t *testing.T) {
	p := New()
	_, err := p.Eval(`
and_result = True and False
or_result = True or False
`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	andResult, _ := p.GetVar("and_result")
	if andResult != false {
		t.Errorf("expected false, got %v", andResult)
	}

	orResult, _ := p.GetVar("or_result")
	if orResult != true {
		t.Errorf("expected true, got %v", orResult)
	}
}

func TestComparisonOperators(t *testing.T) {
	p := New()
	_, err := p.Eval(`
lte = 5 <= 5
gte = 10 >= 5
`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	lte, _ := p.GetVar("lte")
	if lte != true {
		t.Errorf("expected true for <=, got %v", lte)
	}

	gte, _ := p.GetVar("gte")
	if gte != true {
		t.Errorf("expected true for >=, got %v", gte)
	}
}

func TestDotNotation(t *testing.T) {
	p := New()
	_, err := p.Eval(`
import json
data = json.parse('{"name":"Alice"}')
result = data["name"]
`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	result, ok := p.GetVar("result")
	if !ok || result != "Alice" {
		t.Errorf("expected Alice, got %v", result)
	}
}

func TestHTTPLibrary(t *testing.T) {
	p := New()
	p.RegisterLibrary("requests", extlibs.RequestsLibrary())
	_, err := p.Eval(`
import requests
options = {"timeout": 10}
response = requests.get("https://httpbin.org/status/200", options)
status = response.status_code
`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	status, ok := p.GetVar("status")
	if !ok || status != int64(200) {
		t.Errorf("expected 200, got %v", status)
	}
}
