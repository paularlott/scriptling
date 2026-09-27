package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/paularlott/logger"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/extlibs/secretprovider"
	"github.com/paularlott/scriptling/scriptling-cli/setup"
)

// newNestedWatchServer builds a Server serving a skills tree and a
// resources tree, with a real fsnotify watcher built the same way setupMCP
// builds it (recursive for both trees) and a serve-style event loop routing
// events through handleWatchEvent.
func newNestedWatchServer(t *testing.T, skillsDir, resourcesDir string) *Server {
	t.Helper()
	libDir := t.TempDir()
	setup.Factories([]string{libDir}, nil, nil, secretprovider.NewRegistry(), logger.NewNullLogger(), "", "")
	extlibs.ResetRuntime()
	s := &Server{
		config: ServerConfig{
			MCPSkillsDir:    skillsDir,
			MCPResourcesDir: resourcesDir,
			LibDirs:         []string{libDir},
		},
		mcpHandler:       &reloadableMCPHandler{},
		debounceDuration: 25 * time.Millisecond,
	}
	server, err := s.createMCPServer()
	if err != nil {
		t.Fatalf("createMCPServer: %v", err)
	}
	s.mcpHandler.server.Store(server)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("fsnotify: %v", err)
	}
	if n := watchTree(watcher, skillsDir); n == 0 {
		t.Fatal("skills tree not watched")
	}
	if n := watchTree(watcher, resourcesDir); n == 0 {
		t.Fatal("resources tree not watched")
	}
	s.watcher = watcher

	done := make(chan struct{})
	t.Cleanup(func() {
		watcher.Close()
		<-done
	})
	go func() {
		defer close(done)
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				s.handleWatchEvent(event)
			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
			}
		}
	}()
	return s
}

// waitFor polls until fn passes or the deadline expires.
func waitFor(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}

// skillListed checks the live server's skills listing for a URI suffix.
func skillListed(t *testing.T, s *Server, suffix string) bool {
	t.Helper()
	client, cleanup := pipeClientServer(t, s.mcpHandler.server.Load())
	defer cleanup()
	skills, err := client.ListSkills(context.Background())
	if err != nil {
		return false
	}
	for _, sk := range skills {
		if strings.HasSuffix(sk.URI, suffix) {
			return true
		}
	}
	return false
}

func resourceReadable(t *testing.T, s *Server, uri, wantText string) bool {
	t.Helper()
	client, cleanup := pipeClientServer(t, s.mcpHandler.server.Load())
	defer cleanup()
	res, err := client.ReadResource(context.Background(), uri)
	if err != nil || len(res.Contents) == 0 {
		return false
	}
	return res.Contents[0].Text == wantText
}

// TestWatcherPicksUpNestedResourceAndSkill proves the recursive watcher plus
// the extension-agnostic event handler reload on changes nested inside the
// resources and skills trees: a new skill directory two levels of files deep,
// a new resource three directories down, and an in-place edit of a skill's
// SKILL.md (a .md file, which the old .toml/.py filter ignored).
func TestWatcherPicksUpNestedResourceAndSkill(t *testing.T) {
	skillsDir := t.TempDir()
	writeSkill(t, skillsDir, "first-skill", "first-skill", "original body")
	resourcesDir := t.TempDir()
	nested := filepath.Join(resourcesDir, "docs", "guides", "deep")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "intro.md"), []byte("intro v1"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := newNestedWatchServer(t, skillsDir, resourcesDir)

	// Baseline: the pre-existing nested resource and skill are served.
	if !resourceReadable(t, s, "docs://guides/deep/intro.md", "intro v1") {
		t.Fatal("baseline nested resource not served")
	}
	if !skillListed(t, s, "skill://first-skill/SKILL.md") {
		t.Fatal("baseline skill not listed")
	}

	// Add a new skill and a deeply nested resource; the watcher must fire on
	// events INSIDE the trees and the reload must register both.
	writeSkill(t, skillsDir, "added-skill", "added-skill", "added extra")
	if err := os.WriteFile(filepath.Join(nested, "second.md"), []byte("second v1"), 0o644); err != nil {
		t.Fatal(err)
	}

	waitFor(t, 5*time.Second, func() bool {
		return skillListed(t, s, "skill://added-skill/SKILL.md") &&
			resourceReadable(t, s, "docs://guides/deep/second.md", "second v1")
	})

	// In-place edit of a .md file (SKILL.md body and the nested resource):
	// the old extension filter would have ignored both.
	skillMD := filepath.Join(skillsDir, "first-skill", "SKILL.md")
	if err := os.WriteFile(skillMD, []byte("---\nname: first-skill\ndescription: Skill first-skill\n---\n\nedited body"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "intro.md"), []byte("intro v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 5*time.Second, func() bool {
		return resourceReadable(t, s, "skill://first-skill/SKILL.md", "---\nname: first-skill\ndescription: Skill first-skill\n---\n\nedited body") &&
			resourceReadable(t, s, "docs://guides/deep/intro.md", "intro v2")
	})
}

// TestWatchTreeCoversNewSubdirectories guards the known limitation the
// recursive watch carries over from fsnotify: directories created AFTER the
// tree walk are not watched. The reload triggered by the create event in the
// parent still registers the new skill, but edits inside it need the parent
// event; document the behaviour so a regression in either half is visible.
func TestWatchTreeCoversNewSubdirectories(t *testing.T) {
	dir := t.TempDir()
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()
	if n := watchTree(watcher, dir); n != 1 {
		t.Fatalf("expected root dir watched, got %d", n)
	}

	// A pre-existing subdirectory is watched: writes inside fire.
	sub := filepath.Join(dir, "existing")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if n := watchTree(watcher, dir); n != 2 {
		t.Fatalf("expected root+sub watched, got %d", n)
	}
	if err := os.WriteFile(filepath.Join(sub, "file.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case <-watcher.Events:
	case <-time.After(2 * time.Second):
		t.Fatal("write inside pre-watched subdirectory did not fire")
	}
}
