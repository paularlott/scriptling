package scriptling

import (
	"testing"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/lexer"
	"github.com/paularlott/scriptling/parser"
)

func TestParseRelativeImport(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedLevel  int
		expectedModule string // empty string means nil module
		expectedNames  []string
	}{
		{
			name:           "single dot import",
			input:          "from . import api",
			expectedLevel:  1,
			expectedModule: "",
			expectedNames:  []string{"api"},
		},
		{
			name:           "double dot import",
			input:          "from .. import utils",
			expectedLevel:  2,
			expectedModule: "",
			expectedNames:  []string{"utils"},
		},
		{
			name:           "triple dot import",
			input:          "from ... import common",
			expectedLevel:  3,
			expectedModule: "",
			expectedNames:  []string{"common"},
		},
		{
			name:           "relative import with module",
			input:          "from . import api",
			expectedLevel:  1,
			expectedModule: "",
			expectedNames:  []string{"api"},
		},
		{
			name:           "relative import with submodule",
			input:          "from .submodule import func",
			expectedLevel:  1,
			expectedModule: "submodule",
			expectedNames:  []string{"func"},
		},
		{
			name:           "double dot with module",
			input:          "from ..parent import thing",
			expectedLevel:  2,
			expectedModule: "parent",
			expectedNames:  []string{"thing"},
		},
		{
			name:           "relative import multiple names",
			input:          "from . import foo, bar, baz",
			expectedLevel:  1,
			expectedModule: "",
			expectedNames:  []string{"foo", "bar", "baz"},
		},
		{
			name:           "absolute import (no dots)",
			input:          "from json import loads",
			expectedLevel:  0,
			expectedModule: "json",
			expectedNames:  []string{"loads"},
		},
		{
			name:           "relative import with alias",
			input:          "from . import api as myapi",
			expectedLevel:  1,
			expectedModule: "",
			expectedNames:  []string{"api"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := parser.New(l)
			program := p.ParseProgram()

			if len(p.Errors()) != 0 {
				t.Fatalf("parser errors: %v", p.Errors())
			}

			if len(program.Statements) != 1 {
				t.Fatalf("expected 1 statement, got %d", len(program.Statements))
			}

			fis, ok := program.Statements[0].(*ast.FromImportStatement)
			if !ok {
				t.Fatalf("expected FromImportStatement, got %T", program.Statements[0])
			}

			if fis.RelativeLevel != tt.expectedLevel {
				t.Errorf("expected relative level %d, got %d", tt.expectedLevel, fis.RelativeLevel)
			}

			if tt.expectedModule == "" {
				if fis.Module != nil {
					t.Errorf("expected nil module, got %s", fis.Module.Value)
				}
			} else {
				if fis.Module == nil {
					t.Errorf("expected module %s, got nil", tt.expectedModule)
				} else if fis.Module.Value != tt.expectedModule {
					t.Errorf("expected module %s, got %s", tt.expectedModule, fis.Module.Value)
				}
			}

			if len(fis.Names) != len(tt.expectedNames) {
				t.Errorf("expected %d names, got %d", len(tt.expectedNames), len(fis.Names))
			} else {
				for i, name := range tt.expectedNames {
					if fis.Names[i].Value != name {
						t.Errorf("expected name %d to be %s, got %s", i, name, fis.Names[i].Value)
					}
				}
			}
		})
	}
}

func TestRelativeImportResolution(t *testing.T) {
	tests := []struct {
		name            string
		currentModule   string
		relativeLevel   int
		module          string // empty means nil
		expectedImport  string
		expectError     bool
	}{
		{
			name:           "single dot from child module",
			currentModule:  "knot.space",
			relativeLevel:  1,
			module:         "",
			expectedImport: "knot.exported", // from . import exported -> knot.exported
		},
		{
			name:           "single dot with module from child",
			currentModule:  "knot.space",
			relativeLevel:  1,
			module:         "api",
			expectedImport: "knot.api",
		},
		{
			name:           "double dot from nested module",
			currentModule:  "a.b.c",
			relativeLevel:  2,
			module:         "",
			expectedImport: "a.exported", // from .. import exported -> a.exported
		},
		{
			name:           "double dot with module from nested",
			currentModule:  "a.b.c",
			relativeLevel:  2,
			module:         "d",
			expectedImport: "a.d",
		},
		{
			name:           "single dot from deeply nested",
			currentModule:  "a.b.c.d",
			relativeLevel:  1,
			module:         "sibling",
			expectedImport: "a.b.c.sibling",
		},
		{
			name:          "exceeds module depth",
			currentModule: "a.b",
			relativeLevel: 3,
			module:        "",
			expectError:   true,
		},
		{
			name:          "at root level",
			currentModule: "a",
			relativeLevel: 1,
			module:        "",
			expectError:   true, // Would result in empty module name
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New()

			// Register a mock library that captures what was imported
			var importedName string
			p.SetLibraryLoader(&mockLoader{
				loadFunc: func(name string) (string, bool, error) {
					importedName = name
					// Return a simple module with the requested name
					return `exported = "value"`, true, nil
				},
			})

			// Create a script library with the current module name
			// that does a relative import
			var script string
			if tt.module == "" {
				script = "from " + dots(tt.relativeLevel) + " import exported"
			} else {
				script = "from " + dots(tt.relativeLevel) + tt.module + " import exported"
			}

			err := p.RegisterScriptLibrary(tt.currentModule, script)
			if err != nil {
				t.Fatalf("failed to register script library: %v", err)
			}

			// Import the module which will trigger the relative import
			_, evalErr := p.Eval("import " + tt.currentModule)

			if tt.expectError {
				if evalErr == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if evalErr != nil {
				t.Errorf("unexpected error: %v", evalErr)
				return
			}

			if importedName != tt.expectedImport {
				t.Errorf("expected import of %q, got %q", tt.expectedImport, importedName)
			}
		})
	}
}

func dots(n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += "."
	}
	return result
}

// mockLoader is a simple library loader for testing
type mockLoader struct {
	loadFunc func(name string) (string, bool, error)
}

func (m *mockLoader) Load(name string) (string, bool, error) {
	if m.loadFunc != nil {
		return m.loadFunc(name)
	}
	return "", false, nil
}

func (m *mockLoader) Description() string {
	return "mock loader"
}
