package object

import (
	"errors"
	"testing"
)

func TestStartTaskAccounting(t *testing.T) {
	var none *ResourceGuard
	if end, err := none.StartTask(); err != nil || end == nil {
		t.Fatalf("nil guard: %v", err)
	}

	g := NewResourceGuard()
	if g.MaxBackgroundTasks() != 0 {
		t.Fatalf("default limit = %d, want 0", g.MaxBackgroundTasks())
	}
	var ends []func()
	for range 5 {
		end, err := g.StartTask()
		if err != nil {
			t.Fatal(err)
		}
		ends = append(ends, end)
	}
	if g.Usage().BackgroundTasks != 5 || g.Usage().MemoryBytes != 0 {
		t.Fatalf("no limits: %+v", g.Usage())
	}

	g.SetMaxBackgroundTasks(5)
	if _, err := g.StartTask(); !errors.Is(err, ErrTaskLimitExceeded) {
		t.Fatalf("over the task limit: %v", err)
	}
	ends[0]()
	end, err := g.StartTask()
	if err != nil {
		t.Fatalf("after a task ended: %v", err)
	}
	ends[0] = end

	// Under a memory limit each live task holds TaskBytes until it ends.
	g.SetMaxBackgroundTasks(0)
	g.SetMemoryLimit(10 * TaskBytes)
	charged := []func(){}
	for range 3 {
		end, err := g.StartTask()
		if err != nil {
			t.Fatal(err)
		}
		charged = append(charged, end)
	}
	if got := g.Usage().MemoryBytes; got != 3*TaskBytes {
		t.Fatalf("charged %d, want %d", got, 3*TaskBytes)
	}
	for _, end := range charged {
		end()
	}
	if got := g.Usage().MemoryBytes; got != 0 {
		t.Fatalf("%d bytes still charged", got)
	}
	for _, end := range ends {
		end()
	}
	if g.Usage().BackgroundTasks != 0 {
		t.Fatalf("%d tasks still counted", g.Usage().BackgroundTasks)
	}

	// Over the memory limit the task is refused and nothing stays charged.
	g.SetMemoryLimit(TaskBytes / 2)
	if _, err := g.StartTask(); !errors.Is(err, ErrMemoryLimitExceeded) {
		t.Fatalf("over the memory limit: %v", err)
	}
	if u := g.Usage(); u.MemoryBytes != 0 || u.BackgroundTasks != 0 {
		t.Fatalf("refused task left %+v", u)
	}
}

func TestBackgroundTaskLimitInheritance(t *testing.T) {
	SetDefaultMaxBackgroundTasks(7)
	defer SetDefaultMaxBackgroundTasks(0)
	g := NewResourceGuard()
	if g.MaxBackgroundTasks() != 7 {
		t.Fatalf("inherits default: %d", g.MaxBackgroundTasks())
	}
	g.SetMaxBackgroundTasks(3)
	c := NewResourceGuard()
	c.CopyLimits(g)
	if c.MaxBackgroundTasks() != 3 {
		t.Fatalf("CopyLimits: %d", c.MaxBackgroundTasks())
	}
	g.SetMaxBackgroundTasks(InheritLimit)
	if g.MaxBackgroundTasks() != 7 {
		t.Fatalf("InheritLimit: %d", g.MaxBackgroundTasks())
	}
}
