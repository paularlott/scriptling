package scriptling_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
)

func TestPathlibSecurity(t *testing.T) {
	// Create a temporary directory for allowed paths
	allowedDir, err := os.MkdirTemp("", "scriptling_allowed")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(allowedDir)

	// Create a temporary directory for disallowed paths
	disallowedDir, err := os.MkdirTemp("", "scriptling_disallowed")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(disallowedDir)

	// Create a file in the disallowed directory
	disallowedFile := filepath.Join(disallowedDir, "secret.txt")
	if err := os.WriteFile(disallowedFile, []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}

	// Initialize Scriptling
	s := scriptling.New()

	// Register pathlib with restrictions
	extlibs.RegisterPathlibLibrary(s, []string{allowedDir})

	tests := []struct {
		name      string
		script    string
		shouldErr bool
	}{
		{
			name: "Allowed path creation",
			script: `
import pathlib
p = pathlib.Path("` + allowedDir + `/test.txt")
p.write_text("hello")
`,
			shouldErr: false,
		},
		{
			name: "Disallowed path read",
			script: `
import pathlib
p = pathlib.Path("` + disallowedFile + `")
p.read_text()
`,
			shouldErr: true,
		},
		{
			name: "Disallowed path check exists",
			script: `
import pathlib
p = pathlib.Path("` + disallowedFile + `")
p.exists()
`,
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.Eval(tt.script)
			if tt.shouldErr && err == nil {
				t.Errorf("Expected error for script %s, but got nil", tt.name)
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("Unexpected error for script %s: %v", tt.name, err)
			}
		})
	}
}
