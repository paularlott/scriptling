package slack

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/extlibs/messaging/shared"
)

// File uploads read a local file only inside the allowed paths (vuln-0001).
func TestReadFileHonorsAllowedPaths(t *testing.T) {
	base := t.TempDir()
	jail := filepath.Join(base, "jail")
	os.MkdirAll(jail, 0o755)
	inside := filepath.Join(jail, "in.txt")
	outside := filepath.Join(base, "secret.txt")
	os.WriteFile(inside, []byte("inside"), 0o644)
	os.WriteFile(outside, []byte("secret"), 0o644)

	files := shared.FilePolicy([]string{jail})
	if _, err := readFile(files, outside); !errors.Is(err, fssecurity.ErrPathNotAllowed) {
		t.Fatalf("outside path: err = %v, want ErrPathNotAllowed", err)
	}
	if data, err := readFile(files, inside); err != nil || string(data) != "inside" {
		t.Fatalf("inside path: %q %v", data, err)
	}
	if data, err := readFile(nil, outside); err != nil || string(data) != "secret" {
		t.Fatalf("unrestricted: %q %v", data, err)
	}
}
