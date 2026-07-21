package pack

import (
	"strings"
	"testing"
	"testing/fstest"
)

// bundleFromMap builds a Bundle from a manifest TOML string + file map.
func bundleFromMap(t *testing.T, manifest string, files map[string]string) *Bundle {
	t.Helper()
	m := fstest.MapFS{"manifest.toml": &fstest.MapFile{Data: []byte(manifest)}}
	for name, content := range files {
		m[name] = &fstest.MapFile{Data: []byte(content)}
	}
	b, err := OpenBundle(m, "test")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestLoaderMultiLibs(t *testing.T) {
	b := bundleFromMap(t, "name=\"a\"\nversion=\"1\"\nlibs=[\"lib\", \"vendor\"]\n", map[string]string{
		"lib/app.py":      "app",
		"vendor/dep.py":   "dep",
		"lib/nested/m.py": "nested",
	})
	l := NewLoader()
	l.AddBundle(b)

	for name, want := range map[string]string{
		"app":       "app",
		"dep":       "dep",
		"nested.m":  "nested",
		"nested/m.x": "",
	} {
		src, found, err := l.Load(name)
		if err != nil {
			t.Fatalf("Load(%s): %v", name, err)
		}
		if want == "" {
			if found {
				t.Errorf("Load(%s) found %q, want not found", name, src)
			}
			continue
		}
		if !found || src != want {
			t.Errorf("Load(%s) = %q, %v; want %q", name, src, found, want)
		}
	}
}

func TestLoaderLibsPriority(t *testing.T) {
	// Same module in both libs dirs: declared order wins.
	b := bundleFromMap(t, "name=\"a\"\nversion=\"1\"\nlibs=[\"lib\", \"vendor\"]\n", map[string]string{
		"lib/dup.py":    "from-lib",
		"vendor/dup.py": "from-vendor",
	})
	l := NewLoader()
	l.AddBundle(b)
	src, found, _ := l.Load("dup")
	if !found || src != "from-lib" {
		t.Errorf("Load(dup) = %q, %v; want from-lib", src, found)
	}

	// Reversed declared order flips the winner.
	b2 := bundleFromMap(t, "name=\"a\"\nversion=\"1\"\nlibs=[\"vendor\", \"lib\"]\n", map[string]string{
		"lib/dup.py":    "from-lib",
		"vendor/dup.py": "from-vendor",
	})
	l2 := NewLoader()
	l2.AddBundle(b2)
	src, found, _ = l2.Load("dup")
	if !found || src != "from-vendor" {
		t.Errorf("Load(dup) = %q, %v; want from-vendor", src, found)
	}
}

func TestLoaderCrossBundlePriority(t *testing.T) {
	b1 := bundleFromMap(t, "name=\"a\"\nversion=\"1\"\n", map[string]string{"lib/dup.py": "first"})
	b2 := bundleFromMap(t, "name=\"b\"\nversion=\"1\"\n", map[string]string{"lib/dup.py": "second"})
	l := NewLoader()
	l.AddBundle(b1)
	l.AddBundle(b2)
	src, found, _ := l.Load("dup")
	if !found || src != "second" {
		t.Errorf("Load(dup) = %q; want second (last added wins)", src)
	}
}

func TestLoaderResolutionOrder(t *testing.T) {
	b := bundleFromMap(t, "name=\"a\"\nversion=\"1\"\n", map[string]string{
		"lib/pkg/mod.py":         "folder",
		"lib/pkg/__init__.py":    "pkginit",
		"lib/flat.mod.py":        "flat",
		"lib/pkg2/__init__.py":   "pkg2init",
	})
	l := NewLoader()
	l.AddBundle(b)

	cases := map[string]string{
		"pkg":       "pkginit",  // package __init__.py
		"pkg.mod":   "folder",   // folder structure
		"flat.mod":  "flat",     // flat fallback
		"pkg2":      "pkg2init", // __init__ for single-part name
		"pkg2.mod":  "",         // missing
	}
	for name, want := range cases {
		src, found, _ := l.Load(name)
		if want == "" {
			if found {
				t.Errorf("Load(%s) found %q, want not found", name, src)
			}
		} else if !found || src != want {
			t.Errorf("Load(%s) = %q, %v; want %q", name, src, found, want)
		}
	}
}

func TestResolveMain(t *testing.T) {
	tests := []struct {
		name       string
		manifest   string
		files      map[string]string
		wantScript string // non-empty => expect script entry with this content
		wantModule string
		wantFunc   string
		wantFound  bool
		wantErr    bool
	}{
		{
			name:      "module.function",
			manifest:  "name=\"a\"\nversion=\"1\"\nmain=\"demo.run\"\n",
			files:     map[string]string{"lib/demo.py": "def run():\n    pass\n"},
			wantModule: "demo", wantFunc: "run", wantFound: true,
		},
		{
			name:       "py script file preferred",
			manifest:   "name=\"a\"\nversion=\"1\"\nmain=\"setup.py\"\n",
			files:      map[string]string{"setup.py": "print('hi')\n"},
			wantScript: "print('hi')\n", wantFound: true,
		},
		{
			name:       "nested py script",
			manifest:   "name=\"a\"\nversion=\"1\"\nmain=\"app/setup.py\"\n",
			files:      map[string]string{"app/setup.py": "x=1\n"},
			wantScript: "x=1\n", wantFound: true,
		},
		{
			name:      "foo.py falls back to module.function",
			manifest:  "name=\"a\"\nversion=\"1\"\nmain=\"foo.py\"\n",
			files:     map[string]string{"lib/foo.py": "def py():\n    pass\n"},
			wantModule: "foo", wantFunc: "py", wantFound: true,
		},
		{
			name:      "no main",
			manifest:  "name=\"a\"\nversion=\"1\"\n",
			files:     nil,
			wantFound: false,
		},
		{
			name:     "unresolvable main",
			manifest: "name=\"a\"\nversion=\"1\"\nmain=\"nodots\"\n",
			files:    nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := NewLoader()
			l.AddBundle(bundleFromMap(t, tt.manifest, tt.files))
			entry, found, err := l.ResolveMain()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", entry)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if found != tt.wantFound {
				t.Fatalf("found = %v, want %v", found, tt.wantFound)
			}
			if !found {
				return
			}
			if tt.wantScript != "" {
				if string(entry.Script) != tt.wantScript {
					t.Errorf("script = %q, want %q", entry.Script, tt.wantScript)
				}
			} else {
				if entry.Script != nil {
					t.Errorf("unexpected script entry: %q", entry.Script)
				}
				if entry.Module != tt.wantModule || entry.Function != tt.wantFunc {
					t.Errorf("entry = %s.%s, want %s.%s", entry.Module, entry.Function, tt.wantModule, tt.wantFunc)
				}
			}
		})
	}
}

func TestResolveMainLastBundleWins(t *testing.T) {
	l := NewLoader()
	l.AddBundle(bundleFromMap(t, "name=\"a\"\nversion=\"1\"\nmain=\"one.run\"\n", nil))
	l.AddBundle(bundleFromMap(t, "name=\"b\"\nversion=\"1\"\nmain=\"two.run\"\n", nil))
	entry, found, err := l.ResolveMain()
	if err != nil || !found {
		t.Fatalf("ResolveMain: %v %v", found, err)
	}
	if entry.Module != "two" {
		t.Errorf("module = %q, want two (last bundle wins)", entry.Module)
	}
}

func TestResolveMainModuleFunctionCompat(t *testing.T) {
	l := NewLoader()
	l.AddBundle(bundleFromMap(t, "name=\"a\"\nversion=\"1\"\nmain=\"demo.run\"\n", nil))
	entry, found, err := l.ResolveMain()
	if err != nil || !found || entry.Module != "demo" || entry.Function != "run" {
		t.Errorf("ResolveMain = %s.%s found=%v err=%v", entry.Module, entry.Function, found, err)
	}
}

func TestLoaderUnresolvableMainErrorMentionsBundle(t *testing.T) {
	l := NewLoader()
	l.AddBundle(bundleFromMap(t, "name=\"a\"\nversion=\"1\"\nmain=\"nodots\"\n", nil))
	_, _, err := l.ResolveMain()
	if err == nil || !strings.Contains(err.Error(), "nodots") {
		t.Errorf("error = %v, want mention of the main value", err)
	}
}
