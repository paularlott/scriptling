package sqlite

import (
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
)

// configureEmbeddedInstance is the minimal embedder recipe: a plain
// interpreter with the runtime library and the database drivers registered.
// Hosts that compile the drivers in do this on every instance they spin up —
// including background/sandbox factories.
func configureEmbeddedInstance(p *scriptling.Scriptling) {
	extlibs.RegisterRuntimeLibraryAll(p, nil)
	RegisterInProcess(p, nil)
}

// TestBackgroundTaskEnvironment proves plugin libraries survive into
// runtime.background tasks: the handler runs against the calling script's
// environment, so scriptling.sqlite (and its ORM) imports inside the task
// resolve against the registering instance. Hosts release queued tasks once
// setup completes — the CLI does this after evaluating the main script —
// so the test queues, releases, then awaits: the host contract in miniature.
func TestBackgroundTaskEnvironment(t *testing.T) {
	extlibs.SetBackgroundFactory(func() extlibs.SandboxInstance {
		p := scriptling.New()
		configureEmbeddedInstance(p)
		return p
	})
	defer extlibs.SetBackgroundFactory(nil)

	p := scriptling.New()
	configureEmbeddedInstance(p)

	queue := `
import scriptling.runtime as runtime
import scriptling.sqlite as sqlite

def db_task():
    conn = sqlite.connect()
    orm = conn.get_orm()
    orm.drop_table("bg")
    (orm.create_table("bg")
     .column("id", "integer", primary_key=True, autoincrement=True)
     .column("name", "text")
     .execute())
    ins = orm.insert("bg", {"name": "from-background"})
    rows = orm.select("bg", "name").fetch()
    orm.drop_table("bg")
    conn.close()
    return rows[0]["name"] + ":" + str(ins.last_insert_id)

promise = runtime.background("dbjob", "db_task")
`
	if _, err := p.Eval(queue); err != nil {
		t.Fatalf("queue eval: %v", err)
	}
	extlibs.ReleaseBackgroundTasks()

	result, err := p.Eval(`
value = promise.get()
if value != "from-background:1":
    return "wrong: " + str(value)
return "ok"
`)
	if err != nil {
		t.Fatalf("await eval: %v", err)
	}
	if result.Inspect() != "ok" {
		t.Fatalf("script result: %s", result.Inspect())
	}
}
