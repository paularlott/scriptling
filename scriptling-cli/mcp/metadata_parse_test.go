package mcp

import (
	"testing"
)

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
	name, desc, mime, err := parseResourceMetadata([]byte(`
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

	// Empty input is valid, all fields empty.
	name, desc, mime, err = parseResourceMetadata(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "" || desc != "" || mime != "" {
		t.Errorf("got %q %q %q, want all empty", name, desc, mime)
	}

	if _, _, _, err = parseResourceMetadata([]byte(`name = "x`)); err == nil {
		t.Error("expected error for malformed toml")
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
