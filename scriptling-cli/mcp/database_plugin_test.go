package mcp

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	scriptlingplugin "github.com/paularlott/scriptling/plugin"
)

func buildSQLitePlugin(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "sqlite")
	cmd := exec.Command("go", "build", "-o", bin, "./plugins/sqlite/cmd")
	cmd.Dir = "../.."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build sqlite plugin: %v\n%s", err, out)
	}
	return bin
}

// TestPrepareScriptlingExternalPlugin proves MCP handler environments carry
// the database plugins: prepareScriptling (one per MCP session) registers
// the plugin's script library, and a tool using the ORM resolves.
func TestPrepareScriptlingExternalPlugin(t *testing.T) {
	bin := buildSQLitePlugin(t)
	manager := scriptlingplugin.NewManager(nil)
	defer manager.Close()
	if _, err := manager.LoadPlugin(context.Background(), bin, nil); err != nil {
		t.Fatalf("load plugin: %v", err)
	}

	cfg := NewHandlerConfig(nil)
	WithPlugins(manager)(&cfg)
	p := prepareScriptling(cfg, nil)
	if p == nil {
		t.Fatal("prepareScriptling returned nil")
	}

	result, err := p.Eval(`
import scriptling.sqlite as sqlite
conn = sqlite.connect()
orm = conn.get_orm()
orm.drop_table("mcp")
(orm.create_table("mcp")
 .column("id", "integer", primary_key=True, autoincrement=True)
 .column("name", "text")
 .execute())
ins = orm.insert("mcp", {"name": "tool"})
row = orm.select("mcp", "name").one()
orm.drop_table("mcp")
conn.close()
return row["name"] + ":" + str(ins.last_insert_id)
`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if result.Inspect() != "tool:1" {
		t.Fatalf("result: %s", result.Inspect())
	}
}
