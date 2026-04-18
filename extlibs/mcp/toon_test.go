package mcp_test

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/mcp"
)

func TestToonLibraryRegistration(t *testing.T) {
	sl := scriptling.New()
	mcp.RegisterToon(sl)

	_, err := sl.Eval(`
import scriptling.toon as toon
`)
	if err != nil {
		t.Fatalf("failed to import toon library: %v", err)
	}
}

func TestToonEncodeDecodeRoundTrip(t *testing.T) {
	sl := scriptling.New()
	mcp.RegisterToon(sl)

	_, err := sl.Eval(`
import scriptling.toon as toon

data = {
    "name": "Alice",
    "age": 30,
    "active": True,
    "tags": ["red", "blue"],
    "meta": {"score": 9.5}
}

encoded = toon.encode(data)
decoded = toon.decode(encoded)
name = decoded["name"]
age = decoded["age"]
active = decoded["active"]
tag_count = len(decoded["tags"])
score = decoded["meta"]["score"]
`)
	if err != nil {
		t.Fatalf("toon round-trip failed: %v", err)
	}

	name, errObj := sl.GetVar("name")
	if errObj != nil || name != "Alice" {
		t.Fatalf("expected name=Alice, got %v (err=%v)", name, errObj)
	}

	ageObj, getErr := sl.GetVarAsObject("age")
	if getErr != nil {
		t.Fatalf("failed to get age object: %v", getErr)
	}
	age, numErr := ageObj.AsInt()
	if numErr != nil || age != 30 {
		t.Fatalf("expected age=30, got %v (err=%v)", ageObj, numErr)
	}

	active, errObj := sl.GetVar("active")
	if errObj != nil || active != true {
		t.Fatalf("expected active=true, got %v (err=%v)", active, errObj)
	}

	tagCount, errObj := sl.GetVar("tag_count")
	if errObj != nil || tagCount != int64(2) {
		t.Fatalf("expected tag_count=2, got %v (err=%v)", tagCount, errObj)
	}

	score, errObj := sl.GetVar("score")
	if errObj != nil || score != 9.5 {
		t.Fatalf("expected score=9.5, got %v (err=%v)", score, errObj)
	}
}

func TestToonEncodeOptionsAndDecodeOptions(t *testing.T) {
	sl := scriptling.New()
	mcp.RegisterToon(sl)

	_, err := sl.Eval(`
import scriptling.toon as toon

encoded = toon.encode_options({"items": [1, 2, 3]}, 4, ",")
decoded = toon.decode_options(encoded, True, 0)
items_len = len(decoded["items"])
`)
	if err != nil {
		t.Fatalf("toon options flow failed: %v", err)
	}

	encoded, errObj := sl.GetVar("encoded")
	if errObj != nil {
		t.Fatalf("failed to get encoded: %v", errObj)
	}

	encodedStr, ok := encoded.(string)
	if !ok {
		t.Fatalf("expected encoded string, got %T", encoded)
	}
	if encodedStr == "" {
		t.Fatal("expected non-empty encoded string")
	}

	itemsLenObj, getErr := sl.GetVarAsObject("items_len")
	if getErr != nil {
		t.Fatalf("failed to get items_len object: %v", getErr)
	}
	itemsLen, numErr := itemsLenObj.AsInt()
	if numErr != nil || itemsLen != 3 {
		t.Fatalf("expected items_len=3, got %v (err=%v)", itemsLenObj, numErr)
	}
}

func TestToonDecodeInvalidInput(t *testing.T) {
	sl := scriptling.New()
	mcp.RegisterToon(sl)

	_, err := sl.Eval(`
import scriptling.toon as toon

encoded = toon.encode_options({"items": [1, 2, 3]}, 2, ";")
toon.decode_options(encoded, True, 0)
`)
	if err == nil {
		t.Fatal("expected decode error for invalid TOON input")
	}
}

func TestToonArgumentValidation(t *testing.T) {
	tests := []struct {
		name    string
		script  string
		wantMsg string
	}{
		{
			name: "encode_missing_argument",
			script: `
import scriptling.toon as toon
toon.encode()
`,
			wantMsg: "argument error",
		},
		{
			name: "decode_non_string",
			script: `
import scriptling.toon as toon
toon.decode(123)
`,
			wantMsg: "must be a string",
		},
		{
			name: "encode_options_bad_indent",
			script: `
import scriptling.toon as toon
toon.encode_options({"x": 1}, "4", ",")
`,
			wantMsg: "must be an integer",
		},
		{
			name: "decode_options_bad_indent_size",
			script: `
import scriptling.toon as toon
toon.decode_options("{}", True, "bad")
`,
			wantMsg: "must be an integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sl := scriptling.New()
			mcp.RegisterToon(sl)

			_, err := sl.Eval(tt.script)
			if err == nil {
				t.Fatal("expected eval error")
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Fatalf("expected error containing %q, got %v", tt.wantMsg, err)
			}
		})
	}
}
