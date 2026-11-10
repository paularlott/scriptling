package mcp

import (
	"archive/zip"
	"context"
	"path/filepath"
	"testing"

	mcplib "github.com/paularlott/mcp"
)

// Each call runs in an interpreter of its own, which closes what the script
// left open when the call returns: here a zip, finished only once closed.
func TestToolHandlerClosesWhatTheCallLeftOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "left.zip")
	handler := BuildToolHandlerSource([]byte(`
import zipfile
z = zipfile.ZipFile("`+path+`", "w")
z.writestr("member.txt", "written by a tool")
`), NewHandlerConfig(nil))
	if _, err := handler(context.Background(), mcplib.NewToolRequest(nil)); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("the archive the tool left open was not closed: %v", err)
	}
	defer zr.Close()
	if len(zr.File) != 1 || zr.File[0].Name != "member.txt" {
		t.Fatalf("members: %v", zr.File)
	}
}
