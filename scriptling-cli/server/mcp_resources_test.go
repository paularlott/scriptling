package server

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/paularlott/logger"
	mcplib "github.com/paularlott/mcp"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/extlibs/secretprovider"
	"github.com/paularlott/scriptling/scriptling-cli/setup"
)

// buildResourceTestServer builds a scriptling Server with one folder tool
// (greet), a resources tree (a static file and a template), and two prompts
// (one static .md, one dynamic .toml+.py). The MCP server is stored on the
// handler so reloadMCP works.
func buildResourceTestServer(t *testing.T) *Server {
	t.Helper()
	libDir := t.TempDir()
	setup.Factories([]string{libDir}, nil, nil, secretprovider.NewRegistry(), logger.NewNullLogger(), "", "")
	extlibs.ResetRuntime()

	toolsDir := t.TempDir()
	writeFile(t, filepath.Join(toolsDir, "greet.toml"), []byte("description = \"Greet a name\"\nkeywords=[\"hi\"]\n\n[[parameters]]\nname=\"name\"\ntype=\"string\"\ndescription=\"Name to greet\"\nrequired=true\n"))
	writeFile(t, filepath.Join(toolsDir, "greet.py"), []byte("import scriptling.mcp.tool as tool\ntool.return_string('hi ' + name)\n"))

	// Resources tree: top-level dir = scheme. A static file and a template.
	resDir := t.TempDir()
	writeFile(t, filepath.Join(resDir, "docs/readme.md"), []byte("# Hello\nThis is a static resource."))
	writeFile(t, filepath.Join(resDir, "greeting/{name}.py"), []byte("import scriptling.mcp.tool as tool\ntool.return_string('Hello, ' + tool.get_string('name') + '!')\n"))

	// Prompts: one static .md, one dynamic toml+py.
	promptsDir := t.TempDir()
	writeFile(t, filepath.Join(promptsDir, "static.md"), []byte("Summarise the following content."))
	writeFile(t, filepath.Join(promptsDir, "review.toml"), []byte("description = \"Review code\"\n\n[[arguments]]\nname=\"language\"\ntype=\"string\"\ndescription=\"Language\"\nrequired=true\n"))
	writeFile(t, filepath.Join(promptsDir, "review.py"), []byte("import scriptling.mcp.tool as tool\ntool.return_object({\"messages\": [{\"role\": \"user\", \"content\": \"Review this \" + tool.get_string('language') + \" code.\"}]})\n"))

	s := &Server{
		config: ServerConfig{
			MCPToolsDir:     toolsDir,
			MCPResourcesDir: resDir,
			MCPPromptsDir:   promptsDir,
			MCPExecTool:     true,
			LibDirs:         []string{libDir},
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

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

// pipeClientServer wires an mcplib stream client to the scriptling MCP server
// over in-process pipes and returns the client plus a cleanup function.
func pipeClientServer(t *testing.T, mcpServer *mcplib.Server) (*mcplib.Client, func()) {
	t.Helper()
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()

	ctx, cancel := context.WithCancel(context.Background())
	serveDone := make(chan struct{})
	go func() {
		defer close(serveDone)
		_ = mcpServer.ServeStream(ctx, serverReader, serverWriter)
	}()

	client := mcplib.NewStreamClient(clientReader, clientWriter, "")
	cleanup := func() {
		cancel()
		client.Close()
		clientWriter.Close()
		serverReader.Close()
		<-serveDone
		serverWriter.Close()
		clientReader.Close()
	}
	return client, cleanup
}

// TestMCPServerMultiVarTemplate covers a template with multiple variables
// across multiple segments, e.g. greeting://{first}/person/{last}, to verify
// the file path -> URI template mapping, multi-var matching, and that all
// extracted vars reach the script.
func TestMCPServerMultiVarTemplate(t *testing.T) {
	libDir := t.TempDir()
	setup.Factories([]string{libDir}, nil, nil, secretprovider.NewRegistry(), logger.NewNullLogger(), "", "")
	extlibs.ResetRuntime()

	resDir := t.TempDir()
	// resources/greeting/{first}/person/{last}.py  ->  greeting://{first}/person/{last}
	writeFile(t, filepath.Join(resDir, "greeting/{first}/person/{last}.py"),
		[]byte("import scriptling.mcp.tool as tool\ntool.return_string(tool.get_string('first') + '.' + tool.get_string('last') + '@example.com')\n"))

	s := &Server{
		config:     ServerConfig{MCPResourcesDir: resDir, LibDirs: []string{libDir}},
		mcpHandler: &reloadableMCPHandler{},
	}
	server, err := s.createMCPServer()
	if err != nil {
		t.Fatalf("createMCPServer: %v", err)
	}
	s.mcpHandler.server.Store(server)

	client, cleanup := pipeClientServer(t, s.mcpHandler.server.Load())
	defer cleanup()
	ctx := context.Background()

	templates, err := client.ListResourceTemplates(ctx)
	if err != nil {
		t.Fatalf("ListResourceTemplates: %v", err)
	}
	if !templateURIsContain(templates, "greeting://{first}/person/{last}") {
		t.Fatalf("expected greeting://{first}/person/{last} template, got %+v", templateURIs(templates))
	}

	// Read an expanded URI: both vars must be extracted and reach the script.
	rr, err := client.ReadResource(ctx, "greeting://ada/person/lovelace")
	if err != nil {
		t.Fatalf("ReadResource multi-var: %v", err)
	}
	if len(rr.Contents) != 1 || rr.Contents[0].Text != "ada.lovelace@example.com" {
		t.Fatalf("expected ada.lovelace@example.com, got %+v", rr)
	}
}

func TestMCPServerResourcesAndPrompts(t *testing.T) {
	s := buildResourceTestServer(t)
	mcpServer := s.mcpHandler.server.Load()

	client, cleanup := pipeClientServer(t, mcpServer)
	defer cleanup()
	ctx := context.Background()

	// resources/list includes the static file resource and built-in server info.
	resources, err := client.ListResources(ctx)
	if err != nil {
		t.Fatalf("ListResources: %v", err)
	}
	if !resourceURIsContain(resources, "docs://readme.md") {
		t.Fatalf("expected docs://readme.md static resource, got %+v", resourceURIs(resources))
	}

	// resources/templates/list includes the file-tree template.
	templates, err := client.ListResourceTemplates(ctx)
	if err != nil {
		t.Fatalf("ListResourceTemplates: %v", err)
	}
	if !templateURIsContain(templates, "greeting://{name}") {
		t.Fatalf("expected greeting://{name} template, got %+v", templateURIs(templates))
	}

	// resources/read on the static resource returns the file content.
	rr, err := client.ReadResource(ctx, "docs://readme.md")
	if err != nil {
		t.Fatalf("ReadResource static: %v", err)
	}
	if len(rr.Contents) != 1 || !strings.Contains(rr.Contents[0].Text, "static resource") {
		t.Fatalf("expected static file text, got %+v", rr)
	}

	// resources/read on an expanded template runs the .py handler.
	rg, err := client.ReadResource(ctx, "greeting://Ada")
	if err != nil {
		t.Fatalf("ReadResource template: %v", err)
	}
	if len(rg.Contents) != 1 || rg.Contents[0].Text != "Hello, Ada!" {
		t.Fatalf("expected template-rendered text, got %+v", rg)
	}

	// prompts/list includes both the static and dynamic prompts.
	prompts, err := client.ListPrompts(ctx)
	if err != nil {
		t.Fatalf("ListPrompts: %v", err)
	}
	if !promptNamesContain(prompts, "static") || !promptNamesContain(prompts, "review") {
		t.Fatalf("expected static and review prompts, got %+v", promptNames(prompts))
	}

	// prompts/get on the static prompt returns the file content as a message.
	ps, err := client.GetPrompt(ctx, "static", nil)
	if err != nil {
		t.Fatalf("GetPrompt static: %v", err)
	}
	if len(ps.Messages) != 1 || !strings.Contains(ps.Messages[0].Content.Text, "Summarise") {
		t.Fatalf("expected static prompt message, got %+v", ps)
	}

	// prompts/get on the dynamic prompt runs the .py handler with args.
	pr, err := client.GetPrompt(ctx, "review", map[string]string{"language": "go"})
	if err != nil {
		t.Fatalf("GetPrompt dynamic: %v", err)
	}
	if len(pr.Messages) != 1 || !strings.Contains(pr.Messages[0].Content.Text, "Review this go code") {
		t.Fatalf("expected rendered dynamic prompt, got %+v", pr)
	}
}

func TestMCPServerReloadEmitsNotification(t *testing.T) {
	s := buildResourceTestServer(t)
	mcpServer := s.mcpHandler.server.Load()

	client, cleanup := pipeClientServer(t, mcpServer)
	defer cleanup()
	ctx := context.Background()

	changed := make(chan struct{}, 4)
	client.OnToolsChanged(func() {
		select {
		case changed <- struct{}{}:
		default:
		}
	})

	// Prime the client: initialize + cache the tool list. Both directions work,
	// so the notification path (server sink + client reader) is wired afterwards.
	tools, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools) < 1 {
		t.Fatalf("expected at least 1 tool, got %d", len(tools))
	}

	// Trigger a reload: this mutates the live server in place and emits
	// notifications/tools/listChanged over the stream.
	s.reloadMCP()

	select {
	case <-changed:
		// notification received -> cache invalidated
	case <-time.After(3 * time.Second):
		t.Fatal("OnToolsChanged did not fire after reloadMCP")
	}

	// The client cache was invalidated; a re-list reflects current state.
	if _, err := client.ListTools(ctx); err != nil {
		t.Fatalf("ListTools after reload: %v", err)
	}
}

func resourceURIs(rs []mcplib.MCPResource) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.URI
	}
	return out
}
func resourceURIsContain(rs []mcplib.MCPResource, want string) bool {
	for _, r := range rs {
		if r.URI == want {
			return true
		}
	}
	return false
}
func templateURIs(ts []mcplib.MCPResourceTemplate) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.URITemplate
	}
	return out
}
func templateURIsContain(ts []mcplib.MCPResourceTemplate, want string) bool {
	for _, t := range ts {
		if t.URITemplate == want {
			return true
		}
	}
	return false
}
func promptNames(ps []mcplib.MCPPrompt) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.Name
	}
	return out
}
func promptNamesContain(ps []mcplib.MCPPrompt, want string) bool {
	for _, p := range ps {
		if p.Name == want {
			return true
		}
	}
	return false
}
