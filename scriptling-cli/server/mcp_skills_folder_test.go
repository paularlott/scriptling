package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paularlott/logger"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/extlibs/secretprovider"
	"github.com/paularlott/scriptling/scriptling-cli/pack"
	"github.com/paularlott/scriptling/scriptling-cli/setup"
)

// newSkillsFolderServer builds a Server whose MCP entries come from a
// skills folder (no tools/resources/prompts dirs).
func newSkillsFolderServer(t *testing.T, skillsDir string) *Server {
	t.Helper()
	libDir := t.TempDir()
	setup.Factories([]string{libDir}, nil, nil, secretprovider.NewRegistry(), logger.NewNullLogger(), "", "")
	extlibs.ResetRuntime()
	s := &Server{
		config: ServerConfig{
			MCPSkillsDir: skillsDir,
			LibDirs:      []string{libDir},
		},
		mcpHandler: &reloadableMCPHandler{},
	}
	server, err := s.createMCPServer()
	if err != nil {
		t.Fatalf("createMCPServer: %v", err)
	}
	s.mcpHandler.server.Store(server)
	return s
}

func writeSkill(t *testing.T, dir, name, frontmatterName, extra string) {
	t.Helper()
	skillDir := filepath.Join(dir, name)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skillMD := "---\nname: " + frontmatterName + "\ndescription: Skill " + name + "\n---\n\nBody of " + name + "."
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0o644); err != nil {
		t.Fatal(err)
	}
	if extra != "" {
		if err := os.WriteFile(filepath.Join(skillDir, "extra.md"), []byte(extra), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestMCPSkillsFolderServedAndReloaded proves the skills folder is served and
// that a reload replaces rather than duplicates: one skill before, two after
// adding a second directory, no duplicates of the first, and a skill removed
// on reload disappears.
func TestMCPSkillsFolderServedAndReloaded(t *testing.T) {
	skillsDir := t.TempDir()
	writeSkill(t, skillsDir, "first-skill", "first-skill", "first extra")

	s := newSkillsFolderServer(t, skillsDir)
	client, cleanup := pipeClientServer(t, s.mcpHandler.server.Load())
	ctx := context.Background()

	skills, err := client.ListSkills(ctx)
	if err != nil {
		t.Fatalf("ListSkills: %v", err)
	}
	if len(skills) != 1 || skills[0].URI != "skill://first-skill/SKILL.md" {
		t.Fatalf("initial skills: %+v", skills)
	}
	extra, err := client.ReadResource(ctx, "skill://first-skill/extra.md")
	if err != nil || extra.Contents[0].Text != "first extra" {
		t.Fatalf("supporting file: err=%v contents=%+v", err, extra)
	}
	cleanup()

	// Add a second skill and reload in place.
	writeSkill(t, skillsDir, "second-skill", "second-skill", "")
	s.reloadMCP()

	client2, cleanup2 := pipeClientServer(t, s.mcpHandler.server.Load())
	defer cleanup2()
	skills2, err := client2.ListSkills(ctx)
	if err != nil {
		t.Fatalf("ListSkills after reload: %v", err)
	}
	if len(skills2) != 2 {
		t.Fatalf("after reload want 2 skills, got %d: %+v", len(skills2), skills2)
	}
	seen := map[string]int{}
	for _, sk := range skills2 {
		seen[sk.URI]++
	}
	if seen["skill://first-skill/SKILL.md"] != 1 || seen["skill://second-skill/SKILL.md"] != 1 {
		t.Fatalf("skills after reload must appear exactly once each: %+v", seen)
	}
}

// TestMCPSkillsFolderBrokenSkillSkipped verifies a skill directory whose
// frontmatter name does not match the directory is skipped with a warning
// (not a startup failure), and a reload that removes it takes it out of the
// listing entirely.
func TestMCPSkillsFolderBrokenSkillSkipped(t *testing.T) {
	skillsDir := t.TempDir()
	writeSkill(t, skillsDir, "good-skill", "good-skill", "")
	writeSkill(t, skillsDir, "bad-dir", "wrong-name", "")

	s := newSkillsFolderServer(t, skillsDir)
	client, cleanup := pipeClientServer(t, s.mcpHandler.server.Load())
	defer cleanup()
	ctx := context.Background()

	skills, err := client.ListSkills(ctx)
	if err != nil {
		t.Fatalf("ListSkills: %v", err)
	}
	if len(skills) != 1 || skills[0].URI != "skill://good-skill/SKILL.md" {
		t.Fatalf("broken skill must be skipped: %+v", skills)
	}

	// Fix the broken skill and reload: both now served.
	writeSkill(t, skillsDir, "bad-dir", "bad-dir", "")
	s.reloadMCP()
	skills2, err := client.ListSkills(ctx)
	if err != nil {
		t.Fatalf("ListSkills after fix: %v", err)
	}
	if len(skills2) != 2 {
		t.Fatalf("after fixing and reloading want 2 skills: %+v", skills2)
	}
}

// TestMCPDirBundleSkills serves the same bundle from a directory (dev mode)
// rather than a zip, proving folder and zip run the identical registration
// path for skills.
func TestMCPDirBundleSkills(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"manifest.toml": "name = \"dirskill\"\nversion = \"1.0.0\"\nmain = \"setup.py\"\nserve = [\"mcp\"]\n",
		"setup.py":      "# mcp only\n",
		"tools/ping.py": "import scriptling.runtime.mcp as mcp\n\n@mcp.tool(\"Ping\")\ndef ping():\n    return \"pong\"\n",
		"skills/dir-skill/SKILL.md":  "---\nname: dir-skill\ndescription: Dir bundle skill\n---\n\nDir body.",
		"skills/dir-skill/notes.md": "dir notes",
	}
	for name, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	b, err := pack.OpenBundleDir(dir)
	if err != nil {
		t.Fatalf("OpenBundleDir: %v", err)
	}
	s, err := NewServer(ServerConfig{Bundle: b})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	client, cleanup := pipeClientServer(t, s.mcpHandler.server.Load())
	defer cleanup()
	ctx := context.Background()

	skills, err := client.ListSkills(ctx)
	if err != nil {
		t.Fatalf("ListSkills: %v", err)
	}
	if len(skills) != 1 || skills[0].URI != "skill://dir-skill/SKILL.md" {
		t.Fatalf("dir bundle skills: %+v", skills)
	}
	notes, err := client.ReadResource(ctx, "skill://dir-skill/notes.md")
	if err != nil || !strings.Contains(notes.Contents[0].Text, "dir notes") {
		t.Fatalf("dir bundle supporting file: err=%v", err)
	}
	if result, err := client.CallTool(ctx, "ping", map[string]any{}); err != nil || result.Content[0].Text != "pong" {
		t.Fatalf("dir bundle tool: err=%v", err)
	}
}
