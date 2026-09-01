package metadata

import (
	"errors"
	"strings"
	"testing"
)

func TestParseNoBlock(t *testing.T) {
	for _, source := range []string{
		"print('hi')\n",
		"# just a comment\nprint('hi')\n",
		"#!/usr/bin/env scriptling\nprint('hi')\n",
		"// not a comment\n",
	} {
		m, ok, err := Parse([]byte(source))
		if err != nil || ok || m.RequiresScriptling != "" {
			t.Errorf("Parse(%q) = %v, %v, %v; want zero, false, nil", source, m, ok, err)
		}
	}
}

func TestParseFullBlock(t *testing.T) {
	source := `#!/usr/bin/env scriptling
# a leading comment

# /// script
# requires-scriptling = ">=0.24"
# dependencies = [
#   "requests",
#   "scriptling.sql via sql",
#   "knot.space via knot >= 1.2",
# ]
# plugins = [
#   "knot >= 1.2.3",
#   "plainplugin",
# ]
#
# [tool.knot]
# anything = "ignored"
# ///

print("hello")
`
	m, ok, err := Parse([]byte(source))
	if err != nil || !ok {
		t.Fatalf("Parse: ok=%v err=%v", ok, err)
	}
	if m.RequiresScriptling != ">=0.24" {
		t.Errorf("RequiresScriptling = %q", m.RequiresScriptling)
	}
	wantDeps := []Dependency{
		{Library: "requests"},
		{Library: "scriptling.sql", Plugin: "sql"},
		{Library: "knot.space", Plugin: "knot", Constraint: ">=1.2"},
	}
	if len(m.Dependencies) != len(wantDeps) {
		t.Fatalf("Dependencies = %v", m.Dependencies)
	}
	for i, want := range wantDeps {
		if m.Dependencies[i] != want {
			t.Errorf("Dependencies[%d] = %v, want %v", i, m.Dependencies[i], want)
		}
	}
	wantPlugins := []PluginRequirement{
		{Plugin: "knot", Constraint: ">=1.2.3"},
		{Plugin: "plainplugin"},
	}
	if len(m.Plugins) != len(wantPlugins) {
		t.Fatalf("Plugins = %v", m.Plugins)
	}
	for i, want := range wantPlugins {
		if m.Plugins[i] != want {
			t.Errorf("Plugins[%d] = %v, want %v", i, m.Plugins[i], want)
		}
	}
}

func TestParseEmptyBlock(t *testing.T) {
	m, ok, err := Parse([]byte("# /// script\n# ///\nprint(1)\n"))
	if err != nil || !ok {
		t.Fatalf("Parse: ok=%v err=%v", ok, err)
	}
	if m.RequiresScriptling != "" || m.Dependencies != nil || m.Plugins != nil {
		t.Errorf("Parse empty block = %+v, want zero metadata", m)
	}
}

func TestParseBlockWithoutTrailingCode(t *testing.T) {
	// The whole file is the block: still a valid, closed block.
	_, ok, err := Parse([]byte("# /// script\n# requires-scriptling = \">=0.24\"\n# ///\n"))
	if err != nil || !ok {
		t.Fatalf("Parse: ok=%v err=%v", ok, err)
	}
}

func TestParseErrors(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{"unclosed", "# /// script\n# requires-scriptling = \">=0.24\"\n", "not closed"},
		{"code before close", "# /// script\nprint(1)\n# ///\n", "before any code"},
		{"second block", "# /// script\n# ///\n\n# /// script\n# ///\nprint(1)\n", "at most one"},
		{"unclosed before second opener", "# /// script\n# /// script\n# ///\n", "not closed"},
		{"invalid toml", "# /// script\n# requires-scriptling =\n# ///\nprint(1)\n", "invalid TOML"},
		{"unknown key", "# /// script\n# frobnicate = true\n# ///\nprint(1)\n", "unknown key"},
		{"bad requires constraint", "# /// script\n# requires-scriptling = \"next\"\n# ///\nprint(1)\n", "requires-scriptling"},
		{"bad library name", "# /// script\n# dependencies = [\"sql >= 1\"]\n# ///\nprint(1)\n", "not a valid library name"},
		{"via without a plugin", "# /// script\n# dependencies = [\"scriptling.sql via\"]\n# ///\nprint(1)\n", "not a valid library name"},
		{"via with a bad plugin constraint", "# /// script\n# dependencies = [\"a via b >= banana\"]\n# ///\nprint(1)\n", "dotted numeric"},
		{"bad plugin constraint", "# /// script\n# plugins = [\"knot >= banana\"]\n# ///\nprint(1)\n", "dotted numeric"},
		{"bad plugin name", "# /// script\n# plugins = [\"a b\"]\n# ///\nprint(1)\n", "not a valid plugin name"},
		{"dependency wrong type", "# /// script\n# dependencies = [42]\n# ///\nprint(1)\n", "must be a library name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, ok, err := Parse([]byte(tc.source))
			if ok || err == nil {
				t.Fatalf("Parse accepted %q: ok=%v err=%v", tc.source, ok, err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.want)
			}
		})
	}
}

func TestParseBlockAfterCodeIsIgnored(t *testing.T) {
	// A "# /// script" line after code has started is an ordinary comment,
	// not a second metadata block.
	source := "print(1)\n# /// script\n# requires-scriptling = \">=0.24\"\n# ///\n"
	_, ok, err := Parse([]byte(source))
	if err != nil || ok {
		t.Fatalf("Parse: ok=%v err=%v, want false, nil", ok, err)
	}
}

func TestSatisfies(t *testing.T) {
	cases := []struct {
		version    string
		constraint string
		want       bool
	}{
		{"0.24.0", ">=0.24", true},
		{"0.24.0", ">=0.25", false},
		{"0.23.1", ">=0.24", false},
		{"0.24", "==0.24.0", true},
		{"1.2.0", "!=1.2", false},
		{"1.2.1", "!=1.2", true},
		{"2.0", ">1.9.9", true},
		{"1.9", "<2", true},
		{"1.9", "<=1.9", true},
	}
	for _, tc := range cases {
		got, err := Satisfies(tc.version, tc.constraint)
		if err != nil {
			t.Fatalf("Satisfies(%q, %q): %v", tc.version, tc.constraint, err)
		}
		if got != tc.want {
			t.Errorf("Satisfies(%q, %q) = %v, want %v", tc.version, tc.constraint, got, tc.want)
		}
	}
	if _, err := Satisfies("0.24", "latest"); err == nil {
		t.Error("Satisfies accepted an operator-less constraint")
	}
	if _, err := Satisfies("beta", ">=0.24"); err == nil {
		t.Error("Satisfies accepted a non-numeric version")
	}
}

func TestVerifyVersion(t *testing.T) {
	m := Metadata{RequiresScriptling: ">=0.24"}
	err := m.Verify(Env{HostVersion: "0.23.1"})
	var check *CheckError
	if !errors.As(err, &check) {
		t.Fatalf("Verify = %v, want CheckError", err)
	}
	if !check.Has(FailureVersion) || len(check.Failures) != 1 {
		t.Errorf("failures = %+v", check.Failures)
	}
	if !strings.Contains(check.Error(), "needs scriptling >=0.24") {
		t.Errorf("error %q missing version message", check.Error())
	}
	if err := (Metadata{RequiresScriptling: ">=0.24"}).Verify(Env{HostVersion: "0.24.0"}); err != nil {
		t.Errorf("satisfied version failed: %v", err)
	}
}

func TestVerifyDependencies(t *testing.T) {
	m := Metadata{Dependencies: []Dependency{
		{Library: "requests"},
		{Library: "scriptling.sql", Plugin: "scriptling.sql"},
	}}

	// Everything resolves: no plugin needed even with none loaded.
	err := m.Verify(Env{HostVersion: "0.24", Resolves: func(string) bool { return true }})
	if err != nil {
		t.Fatalf("resolved dependencies failed: %v", err)
	}

	// Plain dependency unresolved, no provider declared: library failure.
	err = m.Verify(Env{Resolves: func(string) bool { return false }})
	var check *CheckError
	if !errors.As(err, &check) {
		t.Fatalf("Verify = %v, want CheckError", err)
	}
	if !check.Has(FailureLibrary) {
		t.Errorf("failures = %+v", check.Failures)
	}

	// Provider-backed dependency unresolved: promoted to a plugin failure.
	err = m.Verify(Env{
		Resolves:      func(name string) bool { return name == "requests" },
		PluginVersion: func(string) (string, bool) { return "", false },
	})
	if !errors.As(err, &check) {
		t.Fatalf("Verify = %v, want CheckError", err)
	}
	if !check.Has(FailurePluginMissing) {
		t.Fatalf("failures = %+v", check.Failures)
	}
	want := `required plugin "scriptling.sql" is not loaded`
	if !strings.Contains(check.Error(), want) {
		t.Errorf("error %q missing %q", check.Error(), want)
	}

	// Provider loaded: the dependency is satisfied through the plugin.
	err = m.Verify(Env{
		Resolves:      func(name string) bool { return name == "requests" },
		PluginVersion: func(string) (string, bool) { return "0.24.0", true },
	})
	if err != nil {
		t.Fatalf("plugin-provided dependency failed: %v", err)
	}
}

func TestVerifyPlugins(t *testing.T) {
	loaded := map[string]string{"knot": "1.2.3", "fresh": "2.0"}
	m := Metadata{Plugins: []PluginRequirement{
		{Plugin: "knot", Constraint: ">=1.2.3"},
		{Plugin: "missing"},
		{Plugin: "fresh", Constraint: ">=3"},
	}}

	err := m.Verify(Env{PluginVersion: func(name string) (string, bool) {
		v, ok := loaded[name]
		return v, ok
	}})
	var check *CheckError
	if !errors.As(err, &check) {
		t.Fatalf("Verify = %v, want CheckError", err)
	}
	if len(check.Failures) != 2 {
		t.Fatalf("failures = %+v", check.Failures)
	}
	if !check.Has(FailurePluginMissing) || !check.Has(FailurePluginVersion) {
		t.Errorf("failures = %+v", check.Failures)
	}
	if !strings.Contains(check.Error(), `plugin "fresh" is version 2.0, but this script needs >=3`) {
		t.Errorf("error %q missing version-too-old message", check.Error())
	}
}

func TestVerifyBarePluginNameMatchesScriptlingNamespace(t *testing.T) {
	loaded := map[string]string{"scriptling.sql": "0.23.1"}

	lookup := func(name string) (string, bool) {
		v, ok := loaded[name]
		return v, ok
	}

	// The bare name finds the first-party plugin and its constraint applies.
	m := Metadata{Plugins: []PluginRequirement{{Plugin: "sql", Constraint: ">=0.23"}}}
	if err := m.Verify(Env{PluginVersion: lookup}); err != nil {
		t.Errorf("bare name should match scriptling.sql: %v", err)
	}
	m = Metadata{Plugins: []PluginRequirement{{Plugin: "sql", Constraint: ">=0.24"}}}
	err := m.Verify(Env{PluginVersion: lookup})
	var check *CheckError
	if !errors.As(err, &check) || !check.Has(FailurePluginVersion) {
		t.Errorf("constraint should apply through the alias: %v", err)
	}

	// The exact declared name still matches.
	m = Metadata{Plugins: []PluginRequirement{{Plugin: "scriptling.sql", Constraint: ">=0.23"}}}
	if err := m.Verify(Env{PluginVersion: lookup}); err != nil {
		t.Errorf("declared name should match: %v", err)
	}

	// A bare name matches a plugin that declared a bare name (registered
	// under the host-owned plugin. namespace), with its constraint applied.
	loaded["plugin.hello"] = "2.1.0"
	m = Metadata{Plugins: []PluginRequirement{{Plugin: "hello", Constraint: ">=2.0"}}}
	if err := m.Verify(Env{PluginVersion: lookup}); err != nil {
		t.Errorf("bare name should match plugin.hello: %v", err)
	}

	// A bare name matches nothing dotted and third-party: only "knot" loads
	// "knot", and "knot" never matches "knot.space".
	m = Metadata{Plugins: []PluginRequirement{{Plugin: "space"}}}
	err = m.Verify(Env{PluginVersion: lookup})
	if !errors.As(err, &check) || !check.Has(FailurePluginMissing) {
		t.Errorf("bare name must not match third-party namespaces: %v", err)
	}
}

func TestVerifyPluginWithoutConstraintAcceptsAnyVersion(t *testing.T) {
	// A requirement with no constraint is satisfied by any loaded version.
	m := Metadata{Plugins: []PluginRequirement{
		{Plugin: "knot"},
		{Plugin: "fresh"},
	}}
	err := m.Verify(Env{PluginVersion: func(name string) (string, bool) {
		switch name {
		case "knot":
			return "0.0.1-prehistoric", true
		case "fresh":
			return "99.0", true
		}
		return "", false
	}})
	if err != nil {
		t.Errorf("a constraintless requirement accepts any version: %v", err)
	}
}

func TestVerifyPromotesWithLibraryOrigin(t *testing.T) {
	m := Metadata{Dependencies: []Dependency{{Library: "knot.space", Plugin: "knot"}}}
	err := m.Verify(Env{})
	var check *CheckError
	if !errors.As(err, &check) {
		t.Fatalf("Verify = %v, want CheckError", err)
	}
	if !strings.Contains(check.Error(), `required plugin "knot" is not loaded (it provides the library "knot.space")`) {
		t.Errorf("error %q missing origin message", check.Error())
	}
}

func TestVerifyPromotedConstraintIsChecked(t *testing.T) {
	// The library does not resolve even though the plugin is loaded (a name
	// mismatch); the promoted constraint still applies.
	m := Metadata{Dependencies: []Dependency{{Library: "knot.space", Plugin: "knot", Constraint: ">=2.0"}}}
	err := m.Verify(Env{
		PluginVersion: func(string) (string, bool) { return "1.0", true },
	})
	var check *CheckError
	if !errors.As(err, &check) {
		t.Fatalf("Verify = %v, want CheckError", err)
	}
	if !check.Has(FailurePluginVersion) {
		t.Fatalf("failures = %+v", check.Failures)
	}
	if !strings.Contains(check.Error(), `plugin "knot" is version 1.0, but this script needs >=2.0`) {
		t.Errorf("error %q missing constraint message", check.Error())
	}
}

func TestVerifyDeduplicatesPromotedAndDeclared(t *testing.T) {
	m := Metadata{
		Dependencies: []Dependency{{Library: "knot.space", Plugin: "knot"}},
		Plugins:      []PluginRequirement{{Plugin: "knot", Constraint: ">=1.0"}},
	}
	err := m.Verify(Env{})
	var check *CheckError
	if !errors.As(err, &check) {
		t.Fatalf("Verify = %v, want CheckError", err)
	}
	if len(check.Failures) != 1 {
		t.Errorf("failures = %+v, want one deduplicated knot failure", check.Failures)
	}
}

func TestVerifyDirectEntryWinsOverPromoted(t *testing.T) {
	// knot is named directly with a constraint and promoted with a stricter
	// one; the direct entry is the intentional statement, so its >=1.0 is
	// what gets checked — and 1.5 satisfies it.
	m := Metadata{
		Dependencies: []Dependency{{Library: "knot.space", Plugin: "knot", Constraint: ">=9.0"}},
		Plugins:      []PluginRequirement{{Plugin: "knot", Constraint: ">=1.0"}},
	}
	err := m.Verify(Env{PluginVersion: func(string) (string, bool) { return "1.5", true }})
	if err != nil {
		t.Errorf("direct entry constraint should win: %v", err)
	}
}

func TestVerifyAllFailuresReportedTogether(t *testing.T) {
	m := Metadata{
		RequiresScriptling: ">=0.24",
		Dependencies:       []Dependency{{Library: "requests"}},
		Plugins:            []PluginRequirement{{Plugin: "knot"}},
	}
	err := m.Verify(Env{HostVersion: "0.23.1"})
	var check *CheckError
	if !errors.As(err, &check) {
		t.Fatalf("Verify = %v, want CheckError", err)
	}
	if len(check.Failures) != 3 {
		t.Errorf("failures = %+v, want three", check.Failures)
	}
}
