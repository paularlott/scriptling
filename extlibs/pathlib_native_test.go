package extlibs

import (
	"context"
	"strings"
	"testing"

	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/object"
)

func TestPathMethodsRejectMissingNativeData(t *testing.T) {
	lib := NewPathlibLibrary(fssecurity.Config{})
	pathClassObj := lib.Constants()["PathClass"]
	pathClass, ok := pathClassObj.(*object.Class)
	if !ok {
		t.Fatalf("expected PathClass constant, got %T", pathClassObj)
	}

	path := &object.Instance{
		Class:  pathClass,
		Fields: map[string]object.Object{},
	}

	for _, name := range []string{"joinpath", "exists"} {
		method, ok := pathClass.Methods[name].(*object.Builtin)
		if !ok {
			t.Fatalf("expected %s builtin, got %T", name, pathClass.Methods[name])
		}

		result := method.Fn(context.Background(), object.NewKwargs(nil), path)
		errObj, ok := result.(*object.Error)
		if !ok {
			t.Fatalf("%s returned %T, expected error", name, result)
		}
		if !strings.Contains(errObj.Message, "invalid native data") {
			t.Fatalf("%s error = %q, expected invalid native data", name, errObj.Message)
		}
	}
}
