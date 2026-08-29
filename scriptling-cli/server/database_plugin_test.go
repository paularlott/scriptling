package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	scriptlingplugin "github.com/paularlott/scriptling/plugin"
)

// dbSetupPy is served by a setup script; the handler opens its own sqlite
// connection per request — the pattern for stateless HTTP handlers — and
// exercises the ORM including the table builder.
const dbSetupPy = `
import scriptling.runtime.http as http
import scriptling.sqlite as sqlite

@http.get("/db")
def db(request):
    conn = sqlite.connect()
    orm = conn.get_orm()
    orm.drop_table("reqs")
    (orm.create_table("reqs")
     .column("id", "integer", primary_key=True, autoincrement=True)
     .column("name", "text")
     .execute())
    ins = orm.insert("reqs", {"name": request.query_param("name")})
    row = orm.select("reqs", "name").where("id", "=", ins.last_insert_id).one()
    orm.drop_table("reqs")
    conn.close()
    return http.json(200, {"name": row["name"], "id": ins.last_insert_id})
`

func newDBTestServer(t *testing.T, pm *scriptlingplugin.Manager) *httptest.Server {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "setup.py"), []byte(dbSetupPy), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := NewServer(ServerConfig{
		ScriptFile:    filepath.Join(dir, "setup.py"),
		LibDirs:       []string{dir},
		PluginManager: pm,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ts := httptest.NewServer(s.buildMux())
	t.Cleanup(ts.Close)
	return ts
}

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

// TestServerHandlerExternalPlugin proves the external sqlite plugin works
// inside HTTP request environments: every request spins up an interpreter,
// RegisterLibraries wires the script library in, and the handler opens and
// closes its own connection.
func TestServerHandlerExternalPlugin(t *testing.T) {
	bin := buildSQLitePlugin(t)
	manager := scriptlingplugin.NewManager(nil)
	defer manager.Close()
	if _, err := manager.LoadPlugin(context.Background(), bin, nil); err != nil {
		t.Fatalf("load plugin: %v", err)
	}

	ts := newDBTestServer(t, manager)

	resp, err := http.Get(ts.URL + "/db?name=ada")
	if err != nil {
		t.Fatalf("GET /db: %v", err)
	}
	defer resp.Body.Close()
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload["name"] != "ada" {
		t.Fatalf("handler result: %v", payload)
	}
	if id, ok := payload["id"].(float64); !ok || id != 1 {
		t.Fatalf("last_insert_id through the wire: %v", payload)
	}

	// A second request proves a fresh environment works too.
	resp2, err := http.Get(ts.URL + "/db?name=grace")
	if err != nil {
		t.Fatalf("second GET /db: %v", err)
	}
	defer resp2.Body.Close()
	var payload2 map[string]any
	if err := json.NewDecoder(resp2.Body).Decode(&payload2); err != nil {
		t.Fatalf("decode 2: %v", err)
	}
	if payload2["name"] != "grace" {
		t.Fatalf("second handler result: %v", payload2)
	}
}
