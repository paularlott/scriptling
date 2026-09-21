package mcp

import (
	"testing"
)

func TestParseToolMetadata_UI(t *testing.T) {
	t.Run("resourceUri and visibility", func(t *testing.T) {
		meta, err := parseToolMetadata([]byte(`description = "Get the sales report"

[ui]
resourceUri = "ui://sales-dashboard/dashboard"
visibility = ["model", "app"]
`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if meta.UI == nil {
			t.Fatal("expected UI to be set")
		}
		if meta.UI.ResourceURI != "ui://sales-dashboard/dashboard" {
			t.Errorf("ResourceURI = %q", meta.UI.ResourceURI)
		}
		if len(meta.UI.Visibility) != 2 || meta.UI.Visibility[0] != "model" || meta.UI.Visibility[1] != "app" {
			t.Errorf("Visibility = %v", meta.UI.Visibility)
		}
	})

	t.Run("resourceUri only, visibility omitted", func(t *testing.T) {
		meta, err := parseToolMetadata([]byte(`description = "d"

[ui]
resourceUri = "ui://x"
`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if meta.UI == nil || meta.UI.ResourceURI != "ui://x" {
			t.Fatalf("UI = %+v", meta.UI)
		}
		if len(meta.UI.Visibility) != 0 {
			t.Errorf("Visibility = %v, want empty", meta.UI.Visibility)
		}
	})

	t.Run("no ui table leaves UI nil", func(t *testing.T) {
		meta, err := parseToolMetadata([]byte(`description = "plain tool"`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if meta.UI != nil {
			t.Errorf("UI = %+v, want nil", meta.UI)
		}
	})

	// resourceUri is optional per spec: an "app"-only action tool — one only
	// ever called by a view that's already open, such as a form submission —
	// has no rendering purpose of its own and can declare just visibility.
	t.Run("visibility only, resourceUri omitted", func(t *testing.T) {
		meta, err := parseToolMetadata([]byte(`description = "d"

[ui]
visibility = ["app"]
`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if meta.UI == nil {
			t.Fatal("expected UI to be set")
		}
		if meta.UI.ResourceURI != "" {
			t.Errorf("ResourceURI = %q, want empty", meta.UI.ResourceURI)
		}
		if len(meta.UI.Visibility) != 1 || meta.UI.Visibility[0] != "app" {
			t.Errorf("Visibility = %v", meta.UI.Visibility)
		}
	})

	t.Run("ui table with neither resourceUri nor visibility is an error", func(t *testing.T) {
		_, err := parseToolMetadata([]byte(`description = "d"

[ui]
`))
		if err == nil {
			t.Fatal("expected an error for [ui] with neither resourceUri nor visibility")
		}
	})
}

func TestParseToolMetadata_Icons(t *testing.T) {
	t.Run("one icon with all fields", func(t *testing.T) {
		meta, err := parseToolMetadata([]byte(`description = "Get the weather"

[[icons]]
src = "https://example.com/weather.png"
mimeType = "image/png"
sizes = ["48x48"]
theme = "light"
`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(meta.Icons) != 1 {
			t.Fatalf("expected 1 icon, got %d", len(meta.Icons))
		}
		icon := meta.Icons[0]
		if icon.Src != "https://example.com/weather.png" || icon.MimeType != "image/png" || icon.Theme != "light" {
			t.Errorf("icon = %+v", icon)
		}
		if len(icon.Sizes) != 1 || icon.Sizes[0] != "48x48" {
			t.Errorf("icon sizes = %v", icon.Sizes)
		}
	})

	t.Run("no icons table leaves Icons nil", func(t *testing.T) {
		meta, err := parseToolMetadata([]byte(`description = "d"`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(meta.Icons) != 0 {
			t.Errorf("Icons = %+v, want none", meta.Icons)
		}
	})

	t.Run("icon missing src is an error", func(t *testing.T) {
		_, err := parseToolMetadata([]byte(`description = "d"

[[icons]]
mimeType = "image/png"
`))
		if err == nil {
			t.Fatal("expected an error for an icon missing src")
		}
	})
}

func TestParseToolMetadata(t *testing.T) {
	tests := []struct {
		name     string
		toml     string
		wantDesc string
		wantKeys []string
		wantDisc bool
		wantNPar int
		wantErr  bool
	}{
		{
			name: "full metadata",
			toml: `description = "Greet someone"
keywords = ["hello", "greet"]
discoverable = true

[[parameters]]
name = "name"
type = "string"
description = "Who to greet"
required = true

[[parameters]]
name = "count"
type = "int"
description = "Times"
required = false
`,
			wantDesc: "Greet someone",
			wantKeys: []string{"hello", "greet"},
			wantDisc: true,
			wantNPar: 2,
		},
		{
			name:     "empty file",
			toml:     "",
			wantDesc: "",
			wantKeys: nil,
			wantDisc: false,
			wantNPar: 0,
		},
		{
			name: "unknown keys ignored",
			toml: `description = "d"
future_field = "whatever"

[[parameters]]
name = "x"
type = "string"
unknown_param_key = 42
`,
			wantDesc: "d",
			wantNPar: 1,
		},
		{
			name:    "malformed toml",
			toml:    `description = "unclosed`,
			wantErr: true,
		},
		{
			name:    "wrong type",
			toml:    `description = 42`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta, err := parseToolMetadata([]byte(tt.toml))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", meta)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if meta.Description != tt.wantDesc {
				t.Errorf("description = %q, want %q", meta.Description, tt.wantDesc)
			}
			if len(meta.Keywords) != len(tt.wantKeys) {
				t.Errorf("keywords = %v, want %v", meta.Keywords, tt.wantKeys)
			}
			if meta.Discoverable != tt.wantDisc {
				t.Errorf("discoverable = %v, want %v", meta.Discoverable, tt.wantDisc)
			}
			if len(meta.Parameters) != tt.wantNPar {
				t.Errorf("parameters = %d, want %d", len(meta.Parameters), tt.wantNPar)
			}
		})
	}
}

func TestParseToolMetadataParameterFields(t *testing.T) {
	meta, err := parseToolMetadata([]byte(`
[[parameters]]
name = "name"
type = "string"
description = "Who"
required = true
`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(meta.Parameters) != 1 {
		t.Fatalf("parameters = %d, want 1", len(meta.Parameters))
	}
	p := meta.Parameters[0]
	if p.Name != "name" || p.Type != "string" || p.Description != "Who" || !p.Required {
		t.Errorf("parameter = %+v", p)
	}
}

func TestParseResourceMetadata(t *testing.T) {
	name, desc, mime, uiMeta, err := parseResourceMetadata([]byte(`
name = "My Resource"
description = "A thing"
mimeType = "text/html"
`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "My Resource" || desc != "A thing" || mime != "text/html" {
		t.Errorf("got %q %q %q", name, desc, mime)
	}
	if uiMeta != nil {
		t.Errorf("uiMeta = %+v, want nil (no [ui] table)", uiMeta)
	}

	// Empty input is valid, all fields empty.
	name, desc, mime, uiMeta, err = parseResourceMetadata(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "" || desc != "" || mime != "" || uiMeta != nil {
		t.Errorf("got %q %q %q %+v, want all empty", name, desc, mime, uiMeta)
	}

	if _, _, _, _, err = parseResourceMetadata([]byte(`name = "x`)); err == nil {
		t.Error("expected error for malformed toml")
	}
}

func TestParseResourceMetadata_UI(t *testing.T) {
	_, _, mime, uiMeta, err := parseResourceMetadata([]byte(`
mimeType = "text/html;profile=mcp-app"

[ui]
domain = "example.claudemcpcontent.com"
prefersBorder = true

[ui.csp]
resourceDomains = ["https://cdn.jsdelivr.net"]
connectDomains = ["https://api.example.com"]

[ui.permissions]
camera = true
`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mime != "text/html;profile=mcp-app" {
		t.Errorf("mime = %q", mime)
	}
	if uiMeta == nil {
		t.Fatal("expected uiMeta to be set")
	}
	if uiMeta.Domain != "example.claudemcpcontent.com" {
		t.Errorf("Domain = %q", uiMeta.Domain)
	}
	if uiMeta.PrefersBorder == nil || !*uiMeta.PrefersBorder {
		t.Errorf("PrefersBorder = %v", uiMeta.PrefersBorder)
	}
	if uiMeta.CSP == nil || len(uiMeta.CSP.ResourceDomains) != 1 || uiMeta.CSP.ResourceDomains[0] != "https://cdn.jsdelivr.net" {
		t.Errorf("CSP.ResourceDomains = %+v", uiMeta.CSP)
	}
	if uiMeta.CSP == nil || len(uiMeta.CSP.ConnectDomains) != 1 || uiMeta.CSP.ConnectDomains[0] != "https://api.example.com" {
		t.Errorf("CSP.ConnectDomains = %+v", uiMeta.CSP)
	}
	if uiMeta.Permissions == nil || !uiMeta.Permissions.Camera {
		t.Errorf("Permissions = %+v", uiMeta.Permissions)
	}
}

func TestParsePromptMetadata(t *testing.T) {
	desc, args, err := parsePromptMetadata([]byte(`
description = "Summarize text"

[[arguments]]
name = "text"
description = "Text to summarize"
required = true

[[arguments]]
name = "style"
description = "Summary style"
required = false
`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if desc != "Summarize text" {
		t.Errorf("description = %q", desc)
	}
	if len(args) != 2 {
		t.Fatalf("args = %d, want 2", len(args))
	}
	if args[0].Name != "text" || !args[0].Required {
		t.Errorf("arg0 = %+v", args[0])
	}
	if args[1].Name != "style" || args[1].Required {
		t.Errorf("arg1 = %+v", args[1])
	}

	// No arguments section.
	desc, args, err = parsePromptMetadata([]byte(`description = "d"`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if desc != "d" || len(args) != 0 {
		t.Errorf("got %q %v", desc, args)
	}

	if _, _, err = parsePromptMetadata([]byte(`[[arguments]`)); err == nil {
		t.Error("expected error for malformed toml")
	}
}
