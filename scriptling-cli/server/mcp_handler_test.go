package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	mcp_lib "github.com/paularlott/mcp"
	"github.com/paularlott/scriptling/extlibs/secretprovider"
	mcpcli "github.com/paularlott/scriptling/scriptling-cli/mcp"
	"github.com/paularlott/scriptling/scriptling-cli/pack"
	"github.com/paularlott/scriptling/scriptling-cli/bootstrap"
)

// TestBuildToolHandlerPassesThroughServerPackLoader verifies that the server's
// handlerConfig() plumbs the pack loader (and configured lib dirs) into the
// shared BuildToolHandler factory. This is the server-package regression test
// for the refactor that moved tool/resource/prompt handler factories into
// scriptling-cli/mcp. Sibling imports require the tools dir to be configured
// as a lib dir explicitly (the CLI's -L flag).
func TestBuildToolHandlerPassesThroughServerPackLoader(t *testing.T) {
	toolsDir := t.TempDir()
	packSrcDir := t.TempDir()

	manifestContent := "name = \"pkg\"\nversion = \"1.0.0\"\n"
	if err := os.WriteFile(filepath.Join(packSrcDir, "manifest.toml"), []byte(manifestContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(packSrcDir, "lib"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packSrcDir, "lib", "packmod.py"), []byte("def value():\n    return 'pack'\n"), 0644); err != nil {
		t.Fatal(err)
	}

	pkgFile := filepath.Join(t.TempDir(), "pkg.zip")
	if _, _, err := pack.Pack(packSrcDir, pkgFile, false); err != nil {
		t.Fatal(err)
	}

	packLoader, err := bootstrap.NewPackLoader([]string{pkgFile}, false, "")
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(toolsDir, "localmod.py"), []byte("def value():\n    return 'local'\n"), 0644); err != nil {
		t.Fatal(err)
	}

	scriptPath := filepath.Join(toolsDir, "tool.py")
	script := `
import localmod
import packmod
import scriptling.mcp.tool as tool

tool.return_string(localmod.value() + "+" + packmod.value())
`
	if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
		t.Fatal(err)
	}

	s := &Server{
		config: ServerConfig{
			LibDirs:        []string{toolsDir},
			SecretRegistry: secretprovider.NewRegistry(),
		},
		packLoader: packLoader,
	}

	handler, err := mcpcli.BuildToolHandler(scriptPath, s.handlerConfig())
	if err != nil {
		t.Fatal(err)
	}

	resp, err := handler(context.Background(), mcp_lib.NewToolRequest(map[string]interface{}{}))
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if len(resp.Content) != 1 || resp.Content[0].Text != "local+pack" {
		t.Fatalf("expected local+pack response, got %+v", resp.Content)
	}
}
