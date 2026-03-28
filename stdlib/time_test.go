package stdlib

import (
	"context"
	"strings"
	"testing"

	"github.com/paularlott/scriptling/object"
)

func TestTimeNow(t *testing.T) {
	ctx := context.Background()

	result := TimeLibrary.Functions()["now"].Fn(ctx, object.NewKwargs(nil))
	if result.Type() != object.STRING_OBJ {
		t.Fatalf("expected STRING_OBJ, got %v", result.Type())
	}

	str := result.(*object.String).Value

	// Should be in ISO 8601 format: YYYY-MM-DDTHH:MM:SS.ffffff
	if !strings.Contains(str, "T") {
		t.Errorf("now() result %q should contain 'T' separator", str)
	}
	if len(str) < 19 {
		t.Errorf("now() result %q too short, expected ISO 8601 format", str)
	}

	// Check date part format (YYYY-MM-DD)
	datePart := str[:10]
	if datePart[4] != '-' || datePart[7] != '-' {
		t.Errorf("now() date part %q should be YYYY-MM-DD format", datePart)
	}

	// Check time part format (HH:MM:SS)
	timePart := str[11:19]
	if timePart[2] != ':' || timePart[5] != ':' {
		t.Errorf("now() time part %q should be HH:MM:SS format", timePart)
	}
}

func TestTimeTime(t *testing.T) {
	ctx := context.Background()

	result := TimeLibrary.Functions()["time"].Fn(ctx, object.NewKwargs(nil))
	if result.Type() != object.FLOAT_OBJ {
		t.Fatalf("expected FLOAT_OBJ, got %v", result.Type())
	}

	ts := result.(*object.Float).Value
	if ts <= 0 {
		t.Errorf("time() returned %f, expected positive Unix timestamp", ts)
	}
}
