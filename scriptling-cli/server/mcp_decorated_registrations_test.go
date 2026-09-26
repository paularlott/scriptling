package server

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paularlott/logger"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/extlibs/secretprovider"
	"github.com/paularlott/scriptling/scriptling-cli/setup"
)

// TestMCPDecoratedRegistrations serves a tools folder whose single .py file
// registers a tool, a static resource, a resource template, a prompt and a
// skill via the decorators, then exercises every one of them through a real
// in-process MCP client session.
func TestMCPDecoratedRegistrations(t *testing.T) {
	libDir := t.TempDir()
	setup.Factories([]string{libDir}, nil, nil, secretprovider.NewRegistry(), logger.NewNullLogger(), "", "")
	extlibs.ResetRuntime()

	toolsDir := t.TempDir()
	writeFile(t, filepath.Join(toolsDir, "everything.py"), []byte(`
import scriptling.runtime.mcp as mcp

@mcp.tool("Double a number", params={"n": {"type": "int", "description": "Number to double"}})
def double(n):
    return str(n * 2)

@mcp.resource("config://app", name="App config", mime_type="application/json")
def app_config():
    return {"debug": True, "version": "1.2.3"}

@mcp.resource("user://docs/{topic}", template=True, mime_type="text/markdown")
def user_doc(topic):
    return "# doc: " + topic

@mcp.prompt(description="Summarise text")
def summarise(text, style="brief"):
    return "style=" + style + " :: " + text

@mcp.skill(files={"regions.md": "eu-west: Europe\n"})
def region_guide():
    return "---\nname: region_guide\ndescription: How to pick a region\n---\n\n# Regions\nSee regions.md."
`))

	s := &Server{
		config: ServerConfig{
			MCPToolsDir: toolsDir,
			LibDirs:     []string{libDir},
		},
		mcpHandler: &reloadableMCPHandler{},
	}
	server, err := s.createMCPServer()
	if err != nil {
		t.Fatalf("createMCPServer: %v", err)
	}
	s.mcpHandler.server.Store(server)

	// The decorated skill must be tracked for reload like every other kind.
	if len(s.mcpFolderEntries.skills) != 1 || s.mcpFolderEntries.skills[0] != "region_guide" {
		t.Fatalf("folder skills = %v, want [region_guide]", s.mcpFolderEntries.skills)
	}

	client, cleanup := pipeClientServer(t, server)
	defer cleanup()
	ctx := context.Background()

	// Tool
	result, err := client.CallTool(ctx, "double", map[string]any{"n": 21})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if len(result.Content) != 1 || result.Content[0].Text != "42" {
		t.Fatalf("double(21) = %+v, want '42'", result.Content)
	}

	// Static resource: dict return must arrive as JSON with the right type.
	resources, err := client.ListResources(ctx)
	if err != nil {
		t.Fatalf("ListResources: %v", err)
	}
	var foundStatic bool
	for _, r := range resources {
		if r.URI == "config://app" {
			foundStatic = true
			if r.Name != "App config" {
				t.Errorf("resource name: %q", r.Name)
			}
		}
	}
	if !foundStatic {
		t.Fatalf("config://app not listed: %+v", resources)
	}
	rr, err := client.ReadResource(ctx, "config://app")
	if err != nil {
		t.Fatalf("ReadResource static: %v", err)
	}
	if len(rr.Contents) != 1 || rr.Contents[0].MimeType != "application/json" {
		t.Fatalf("static resource contents: %+v", rr.Contents)
	}
	if !strings.Contains(rr.Contents[0].Text, `"debug":true`) {
		t.Fatalf("static resource text: %q", rr.Contents[0].Text)
	}

	// Resource template: string return, template variable bound to the
	// function parameter.
	templates, err := client.ListResourceTemplates(ctx)
	if err != nil {
		t.Fatalf("ListResourceTemplates: %v", err)
	}
	var foundTemplate bool
	for _, tmpl := range templates {
		if tmpl.URITemplate == "user://docs/{topic}" {
			foundTemplate = true
		}
	}
	if !foundTemplate {
		t.Fatalf("template not listed: %+v", templates)
	}
	tr, err := client.ReadResource(ctx, "user://docs/regions")
	if err != nil {
		t.Fatalf("ReadResource template: %v", err)
	}
	if len(tr.Contents) != 1 || tr.Contents[0].Text != "# doc: regions" {
		t.Fatalf("template contents: %+v", tr.Contents)
	}
	if tr.Contents[0].MimeType != "text/markdown" {
		t.Errorf("template mimeType: %q", tr.Contents[0].MimeType)
	}

	// Prompt: arguments inferred from the signature, function called with
	// them on prompts/get.
	prompts, err := client.ListPrompts(ctx)
	if err != nil {
		t.Fatalf("ListPrompts: %v", err)
	}
	var foundPrompt bool
	for _, p := range prompts {
		if p.Name == "summarise" {
			foundPrompt = true
			if p.Description != "Summarise text" {
				t.Errorf("prompt description: %q", p.Description)
			}
			if len(p.Arguments) != 2 || p.Arguments[0].Name != "text" || !p.Arguments[0].Required {
				t.Errorf("prompt arguments: %+v", p.Arguments)
			}
		}
	}
	if !foundPrompt {
		t.Fatalf("summarise not listed: %+v", prompts)
	}
	pr, err := client.GetPrompt(ctx, "summarise", map[string]string{"text": "hello", "style": "long"})
	if err != nil {
		t.Fatalf("GetPrompt: %v", err)
	}
	if len(pr.Messages) != 1 || pr.Messages[0].Content.Text != "style=long :: hello" {
		t.Fatalf("prompt messages: %+v", pr.Messages)
	}

	// Skill: listed with verbatim frontmatter, SKILL.md and the supporting
	// file both readable as skill:// resources.
	skills, err := client.ListSkills(ctx)
	if err != nil {
		t.Fatalf("ListSkills: %v", err)
	}
	var foundSkill bool
	for _, sk := range skills {
		if sk.URI == "skill://region_guide/SKILL.md" {
			foundSkill = true
			if sk.Frontmatter["name"] != "region_guide" || sk.Frontmatter["description"] != "How to pick a region" {
				t.Errorf("skill frontmatter: %+v", sk.Frontmatter)
			}
			if len(sk.Resources) != 2 {
				t.Errorf("skill resources (SKILL.md + regions.md): %+v", sk.Resources)
			}
		}
	}
	if !foundSkill {
		t.Fatalf("region-guide not listed: %+v", skills)
	}
	skillContent, err := client.ReadResource(ctx, "skill://region_guide/SKILL.md")
	if err != nil {
		t.Fatalf("ReadResource SKILL.md: %v", err)
	}
	if !strings.Contains(skillContent.Contents[0].Text, "See regions.md.") {
		t.Fatalf("SKILL.md content: %q", skillContent.Contents[0].Text)
	}
	supportContent, err := client.ReadResource(ctx, "skill://region_guide/regions.md")
	if err != nil {
		t.Fatalf("ReadResource supporting file: %v", err)
	}
	if !strings.Contains(supportContent.Contents[0].Text, "eu-west: Europe") {
		t.Fatalf("supporting file content: %q", supportContent.Contents[0].Text)
	}
}

// TestMCPDecoratedSkillFrontmatterMismatch verifies the server skips a
// decorated skill whose SKILL.md frontmatter name does not match the function
// name, without failing the whole server (the skills-folder rule).
func TestMCPDecoratedSkillFrontmatterMismatch(t *testing.T) {
	libDir := t.TempDir()
	setup.Factories([]string{libDir}, nil, nil, secretprovider.NewRegistry(), logger.NewNullLogger(), "", "")
	extlibs.ResetRuntime()

	toolsDir := t.TempDir()
	writeFile(t, filepath.Join(toolsDir, "broken.py"), []byte(`
import scriptling.runtime.mcp as mcp

@mcp.skill()
def guide():
    return "---\nname: something-else\ndescription: d\n---\n\nbody"

@mcp.tool("Still works")
def fine():
    return "yes"
`))

	s := &Server{
		config: ServerConfig{
			MCPToolsDir: toolsDir,
			LibDirs:     []string{libDir},
		},
		mcpHandler: &reloadableMCPHandler{},
	}
	server, err := s.createMCPServer()
	if err != nil {
		t.Fatalf("createMCPServer: %v", err)
	}
	s.mcpHandler.server.Store(server)
	client, cleanup := pipeClientServer(t, server)
	defer cleanup()

	ctx := context.Background()
	skills, err := client.ListSkills(ctx)
	if err != nil {
		t.Fatalf("ListSkills: %v", err)
	}
	if len(skills) != 0 {
		t.Fatalf("mismatched skill must be skipped, got: %+v", skills)
	}
	if result, err := client.CallTool(ctx, "fine", map[string]any{}); err != nil || len(result.Content) != 1 || result.Content[0].Text != "yes" {
		t.Fatalf("tool after bad skill: err=%v result=%+v", err, result)
	}
}

// TestMCPDecoratedFailuresAtRead verifies unhappy paths surface as MCP errors
// rather than crashing the server: a raising resource function and a raising
// prompt function both return errors, and the rest of the server keeps
// working.
func TestMCPDecoratedFailuresAtRead(t *testing.T) {
	libDir := t.TempDir()
	setup.Factories([]string{libDir}, nil, nil, secretprovider.NewRegistry(), logger.NewNullLogger(), "", "")
	extlibs.ResetRuntime()

	toolsDir := t.TempDir()
	writeFile(t, filepath.Join(toolsDir, "broken.py"), []byte(`
import scriptling.runtime.mcp as mcp

@mcp.resource("boom://resource")
def boom_resource():
    raise ValueError("resource exploded")

@mcp.prompt(description="Broken prompt")
def boom_prompt(text):
    raise ValueError("prompt exploded")

@mcp.tool("Still fine")
def fine():
    return "yes"
`))

	s := &Server{
		config: ServerConfig{
			MCPToolsDir: toolsDir,
			LibDirs:     []string{libDir},
		},
		mcpHandler: &reloadableMCPHandler{},
	}
	server, err := s.createMCPServer()
	if err != nil {
		t.Fatalf("createMCPServer: %v", err)
	}
	s.mcpHandler.server.Store(server)
	client, cleanup := pipeClientServer(t, server)
	defer cleanup()
	ctx := context.Background()

	if _, err := client.ReadResource(ctx, "boom://resource"); err == nil || !strings.Contains(err.Error(), "resource exploded") {
		t.Fatalf("resource raise must surface its message, got: %v", err)
	}
	if _, err := client.GetPrompt(ctx, "boom_prompt", map[string]string{"text": "x"}); err == nil || !strings.Contains(err.Error(), "prompt exploded") {
		t.Fatalf("prompt raise must surface its message, got: %v", err)
	}
	if result, err := client.CallTool(ctx, "fine", map[string]any{}); err != nil || len(result.Content) != 1 || result.Content[0].Text != "yes" {
		t.Fatalf("server must keep working after read errors: err=%v result=%+v", err, result)
	}
}

// TestMCPPromptRequiredArguments verifies the spec behaviour for prompt
// arguments across both dynamic prompt styles: prompts/get with every
// argument renders, and prompts/get missing a required argument returns
// -32602 (Invalid params) before any handler runs, for decorated and
// .toml+.py prompts alike.
func TestMCPPromptRequiredArguments(t *testing.T) {
	libDir := t.TempDir()
	setup.Factories([]string{libDir}, nil, nil, secretprovider.NewRegistry(), logger.NewNullLogger(), "", "")
	extlibs.ResetRuntime()

	toolsDir := t.TempDir()
	writeFile(t, filepath.Join(toolsDir, "decorated_prompt.py"), []byte(`
import scriptling.runtime.mcp as mcp

@mcp.prompt(description="Summarise sales")
def sales_summary(period, region="all"):
    return "Summarise sales for " + period + " in " + region
`))

	promptsDir := t.TempDir()
	writeFile(t, filepath.Join(promptsDir, "legacy_review.toml"), []byte(`description = "Review code"

[[arguments]]
name = "language"
description = "Language of the code"
required = true

[[arguments]]
name = "code"
description = "The code"
required = true
`))
	writeFile(t, filepath.Join(promptsDir, "legacy_review.py"), []byte("import scriptling.mcp.tool as tool\ntool.return_string('Review this ' + tool.get_string('language') + ' code.')\n"))

	s := &Server{
		config: ServerConfig{
			MCPToolsDir:   toolsDir,
			MCPPromptsDir: promptsDir,
			LibDirs:       []string{libDir},
		},
		mcpHandler: &reloadableMCPHandler{},
	}
	server, err := s.createMCPServer()
	if err != nil {
		t.Fatalf("createMCPServer: %v", err)
	}
	s.mcpHandler.server.Store(server)
	client, cleanup := pipeClientServer(t, server)
	defer cleanup()
	ctx := context.Background()

	// Decorated: full arguments render, optional argument defaults.
	pr, err := client.GetPrompt(ctx, "sales_summary", map[string]string{"period": "Q1"})
	if err != nil {
		t.Fatalf("GetPrompt decorated: %v", err)
	}
	if len(pr.Messages) != 1 || pr.Messages[0].Content.Text != "Summarise sales for Q1 in all" {
		t.Fatalf("decorated messages: %+v", pr.Messages)
	}

	// Decorated: missing required argument is -32602 before the handler runs.
	_, err = client.GetPrompt(ctx, "sales_summary", map[string]string{})
	if err == nil || !strings.Contains(err.Error(), "missing required argument: period") {
		t.Fatalf("decorated missing-arg error, got: %v", err)
	}

	// Legacy .toml+.py: full arguments render through the script handler.
	lr, err := client.GetPrompt(ctx, "legacy_review", map[string]string{"language": "go", "code": "x"})
	if err != nil {
		t.Fatalf("GetPrompt legacy: %v", err)
	}
	if len(lr.Messages) != 1 || !strings.Contains(lr.Messages[0].Content.Text, "Review this go code.") {
		t.Fatalf("legacy messages: %+v", lr.Messages)
	}

	// Legacy: missing required argument is -32602 too.
	_, err = client.GetPrompt(ctx, "legacy_review", map[string]string{"language": "go"})
	if err == nil || !strings.Contains(err.Error(), "missing required argument: code") {
		t.Fatalf("legacy missing-arg error, got: %v", err)
	}
}
