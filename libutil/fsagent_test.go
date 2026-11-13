package libutil

import (
	"encoding/json"
	"testing"
)

// TestErrorJSONBodyEscapesPayload pins the contract protocol error bodies rely
// on: an error message carrying quotes, backslashes, newlines and control
// characters must survive as a valid JSON document that round-trips exactly.
func TestErrorJSONBodyEscapesPayload(t *testing.T) {
	message := "handler \"failed\"\nline two\t\x01back\\slash"
	body := ErrorJSONBody(message)

	var decoded struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		t.Fatalf("ErrorJSONBody produced invalid JSON: %v\nbody: %s", err, body)
	}
	if decoded.Error != message {
		t.Fatalf("message did not round-trip:\n got: %q\nwant: %q", decoded.Error, message)
	}

	if plain := ErrorJSONBody("simple"); plain != `{"error":"simple"}` {
		t.Fatalf("simple message rendered as %s", plain)
	}
}
