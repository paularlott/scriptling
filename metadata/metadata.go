// Package metadata parses and verifies PEP 723-style inline script metadata:
// a TOML block carried in comments before a script's first statement.
//
//	# /// script
//	# requires-scriptling = ">=0.24"
//	#
//	# dependencies = [
//	#   "requests",
//	#   "scriptling.sql via sql >= 0.23",
//	# ]
//	#
//	# plugins = [
//	#   "knot >= 1.2.3",
//	# ]
//	# ///
//
// requires-scriptling carries a version constraint matched against the
// running host. dependencies name libraries the script imports; an entry may
// add "via <plugin>" to name the plugin that provides the library — with the
// same optional version constraint as the plugins list ("scriptling.sql via
// sql >= 0.23") — and when that library does not resolve the plugin (with
// its constraint) becomes required. A compiled-in or otherwise provided
// library satisfies the entry without the plugin. plugins name external
// plugin processes directly, with an optional version constraint matched
// against the version each plugin declared in its handshake.
//
// Plugin names match the declared name from the plugin's handshake, and a
// bare name additionally matches the same name under scriptling's host-owned
// namespaces: "sql" matches a plugin declaring "scriptling.sql" (the
// first-party database plugins), "hello" matches one declaring the bare name
// "hello" (registered as "plugin.hello"), while "knot" matches only a plugin
// declaring "knot".
//
// Tables under [tool.*] are reserved for tool and host configuration: they
// are accepted and ignored so a future need cannot break existing scripts.
// Any other unknown key is an error, so a typo fails loudly instead of
// silently doing nothing.
package metadata

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	blockOpen  = "# /// script"
	blockClose = "# ///"
)

var (
	moduleNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*$`)
	pluginNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`)
	digitsOnlyRe = regexp.MustCompile(`^[0-9]+$`)
)

// Dependency is one required library. Library is the name the script
// imports, checked by resolution. Plugin, when set, names the plugin that
// provides the library in the same "name [constraint]" syntax the plugins
// list accepts: if the library does not resolve, the plugin (with its
// constraint) becomes required and its absence is reported as a plugin
// failure with the plugin's own remedy, rather than an unresolved library.
type Dependency struct {
	Library    string
	Plugin     string
	Constraint string
}

// PluginRequirement is one required external plugin process. Constraint, when
// set, is an operator and version (">=1.2.3", "==2.0") matched against the
// version the plugin declared in its handshake.
type PluginRequirement struct {
	Plugin     string
	Constraint string
}

// Metadata is the parsed script metadata block.
type Metadata struct {
	RequiresScriptling string
	Dependencies       []Dependency
	Plugins            []PluginRequirement
}

// Parse finds and parses the metadata block. It reports ok=false when the
// source has no block; every malformed block is an error, because a script
// that tried to declare its requirements should never run as if it had none.
// The block must appear before the first statement and at most once.
func Parse(source []byte) (m Metadata, ok bool, err error) {
	tomlSource, ok, err := extractBlock(string(source))
	if err != nil || !ok {
		return Metadata{}, false, err
	}

	// Decode into a map and validate keys manually: the dependencies list
	// mixes strings and tables, and decoding those into untyped fields would
	// leave their inner keys untracked, so Undecoded() cannot police the
	// schema for us.
	var raw map[string]any
	if _, err := toml.Decode(tomlSource, &raw); err != nil {
		return Metadata{}, false, fmt.Errorf("invalid TOML in script metadata: %w", err)
	}
	for key := range raw {
		if key != "requires-scriptling" && key != "dependencies" && key != "plugins" && key != "tool" {
			return Metadata{}, false, fmt.Errorf("unknown key %q in script metadata (tool and host configuration goes under [tool.<name>])", key)
		}
	}

	m = Metadata{}
	if v, ok := raw["requires-scriptling"]; ok {
		s, ok := v.(string)
		if !ok {
			return Metadata{}, false, fmt.Errorf("requires-scriptling must be a version constraint string, e.g. \">=0.24\"")
		}
		m.RequiresScriptling = strings.TrimSpace(s)
		if m.RequiresScriptling != "" {
			if _, _, err := parseConstraint(m.RequiresScriptling); err != nil {
				return Metadata{}, false, fmt.Errorf("requires-scriptling: %v", err)
			}
		}
	}
	if v, ok := raw["dependencies"]; ok {
		list := toAnySlice(v)
		if list == nil {
			return Metadata{}, false, fmt.Errorf("dependencies must be a list of library names and { library = ..., plugin = ... } tables")
		}
		for i, entry := range list {
			dep, err := parseDependency(entry, i)
			if err != nil {
				return Metadata{}, false, err
			}
			m.Dependencies = append(m.Dependencies, dep)
		}
	}
	if v, ok := raw["plugins"]; ok {
		list := toAnySlice(v)
		if list == nil {
			return Metadata{}, false, fmt.Errorf("plugins must be a list of plugin names with optional version constraints")
		}
		for i, entry := range list {
			s, ok := entry.(string)
			if !ok {
				return Metadata{}, false, fmt.Errorf("plugins[%d] must be a string like \"knot\" or \"knot >= 1.2.3\"", i)
			}
			req, err := parsePluginRequirement(s, i)
			if err != nil {
				return Metadata{}, false, err
			}
			m.Plugins = append(m.Plugins, req)
		}
	}
	return m, true, nil
}

// toAnySlice normalises the two slice shapes BurntSushi can produce for a
// TOML array decoded into interface{} — []interface{} for mixed content and
// []map[string]interface{} when every element is a table — returning nil when
// the value is neither.
func toAnySlice(v any) []any {
	switch list := v.(type) {
	case []any:
		return list
	case []map[string]any:
		out := make([]any, 0, len(list))
		for _, item := range list {
			out = append(out, item)
		}
		return out
	}
	return nil
}

// extractBlock returns the TOML content between the "# /// script" opener and
// its "# ///" closer, with the comment markers stripped. It scans only the
// leading comment region: a block after code has started is an ordinary
// comment, not metadata, so a "# /// script" line inside a string literal can
// never confuse the parser.
func extractBlock(source string) (string, bool, error) {
	var content []string
	inBlock := false
	found := false
	for i, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == blockOpen:
			if inBlock {
				return "", false, fmt.Errorf("script metadata block is not closed (expected \"# ///\")")
			}
			if found {
				return "", false, fmt.Errorf("at most one script metadata block is allowed (second starts at line %d)", i+1)
			}
			inBlock, found = true, true
		case trimmed == blockClose:
			if inBlock {
				inBlock = false
			}
		case trimmed == "":
			// Blank lines inside the block carry no TOML; skip them.
		case strings.HasPrefix(trimmed, "#"):
			if inBlock {
				body := trimmed[1:]
				if len(body) > 0 && (body[0] == ' ' || body[0] == '\t') {
					body = body[1:]
				}
				content = append(content, body)
			}
		default:
			// Code has started.
			if inBlock {
				return "", false, fmt.Errorf("script metadata block must be closed with \"# ///\" before any code (line %d)", i+1)
			}
			if found {
				return strings.Join(content, "\n"), true, nil
			}
			return "", false, nil
		}
	}
	if inBlock {
		return "", false, fmt.Errorf("script metadata block is not closed (expected \"# ///\")")
	}
	if found {
		return strings.Join(content, "\n"), true, nil
	}
	return "", false, nil
}

// parseDependency parses one dependencies entry: a library name, optionally
// followed by " via " and the plugin that provides it, itself optionally
// carrying a version constraint — "scriptling.sql via sql >= 0.23".
func parseDependency(entry any, index int) (Dependency, error) {
	s, ok := entry.(string)
	if !ok {
		return Dependency{}, fmt.Errorf("dependencies[%d] must be a library name, optionally followed by \" via \" and the plugin that provides it", index)
	}
	parts := strings.SplitN(s, " via ", 2)
	library := strings.TrimSpace(parts[0])
	if !moduleNameRe.MatchString(library) {
		return Dependency{}, fmt.Errorf("dependencies[%d]: %q is not a valid library name", index, library)
	}
	if len(parts) == 1 {
		return Dependency{Library: library}, nil
	}
	req, err := parsePluginSpec(parts[1])
	if err != nil {
		return Dependency{}, fmt.Errorf("dependencies[%d]: plugin: %v", index, err)
	}
	return Dependency{Library: library, Plugin: req.Plugin, Constraint: req.Constraint}, nil
}

func parsePluginRequirement(entry string, index int) (PluginRequirement, error) {
	req, err := parsePluginSpec(entry)
	if err != nil {
		return PluginRequirement{}, fmt.Errorf("plugins[%d]: %v", index, err)
	}
	return req, nil
}

// parsePluginSpec parses a plugin reference — "name", or "name" followed by
// a version constraint — shared by the plugins list and the plugin field of
// a dependency entry.
func parsePluginSpec(s string) (PluginRequirement, error) {
	name, constraint := splitConstraint(s)
	name = strings.TrimSpace(name)
	if !pluginNameRe.MatchString(name) {
		return PluginRequirement{}, fmt.Errorf("%q is not a valid plugin name", name)
	}
	if constraint == "" {
		return PluginRequirement{Plugin: name}, nil
	}
	if _, _, err := parseConstraint(constraint); err != nil {
		return PluginRequirement{}, err
	}
	return PluginRequirement{Plugin: name, Constraint: constraint}, nil
}

// constraintOps are matched longest-first when splitting, so ">=" is never
// read as ">" followed by "=1.2.3".
var constraintOps = []string{">=", "<=", "==", "!=", "<", ">"}

// splitConstraint splits "name >= 1.2.3" into "name " and ">=1.2.3". With no
// operator present it returns the whole string as the name.
func splitConstraint(s string) (string, string) {
	best, bestAt := "", -1
	for _, op := range constraintOps {
		at := strings.Index(s, op)
		if at >= 0 && (bestAt == -1 || at < bestAt) {
			best, bestAt = op, at
		}
	}
	if bestAt < 0 {
		return s, ""
	}
	name := s[:bestAt]
	constraint := best + strings.TrimSpace(s[bestAt+len(best):])
	return name, constraint
}

// parseConstraint validates "op version" and returns its parts.
func parseConstraint(constraint string) (string, string, error) {
	for _, op := range constraintOps {
		if strings.HasPrefix(constraint, op) {
			version := strings.TrimSpace(constraint[len(op):])
			if _, err := ParseVersion(version); err != nil {
				return "", "", fmt.Errorf("invalid constraint %q: %v", constraint, err)
			}
			return op, version, nil
		}
	}
	return "", "", fmt.Errorf("invalid constraint %q: expected an operator and version, e.g. \">=0.24\"", constraint)
}

// Version is a dotted numeric version: 0.24, 1.2.3.
type Version []int

// ParseVersion parses a dotted numeric version. Segments are non-negative
// integers with no leading conventions beyond their numeric value.
func ParseVersion(s string) (Version, error) {
	if s == "" {
		return nil, fmt.Errorf("version is empty")
	}
	parts := strings.Split(s, ".")
	nums := make(Version, 0, len(parts))
	for _, part := range parts {
		if !digitsOnlyRe.MatchString(part) {
			return nil, fmt.Errorf("version %q is not dotted numeric (e.g. 1.2.3)", s)
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("version %q is not dotted numeric (e.g. 1.2.3)", s)
		}
		nums = append(nums, n)
	}
	return nums, nil
}

// CompareVersions compares dotted numeric versions, padding the shorter with
// zeros so 1.2 equals 1.2.0.
func CompareVersions(a, b Version) int {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		var x, y int
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

// Satisfies reports whether version satisfies a constraint such as ">=0.24"
// or "==1.2.3".
func Satisfies(version, constraint string) (bool, error) {
	op, target, err := parseConstraint(constraint)
	if err != nil {
		return false, err
	}
	have, err := ParseVersion(version)
	if err != nil {
		return false, err
	}
	want, err := ParseVersion(target)
	if err != nil {
		return false, err
	}
	c := CompareVersions(have, want)
	switch op {
	case ">=":
		return c >= 0, nil
	case "<=":
		return c <= 0, nil
	case ">":
		return c > 0, nil
	case "<":
		return c < 0, nil
	case "==":
		return c == 0, nil
	case "!=":
		return c != 0, nil
	}
	return false, fmt.Errorf("unsupported operator in constraint %q", constraint)
}

// Env supplies everything Verify needs to know about the running host.
type Env struct {
	// HostVersion is the running host's version. The CLI passes scriptling's
	// build.Version; an embedding application passes its own version, since
	// from a script's perspective the host is the interpreter it runs on.
	HostVersion string

	// Resolves reports whether a library or module name is available to
	// scripts: a registered library, a built-in module, or module source a
	// package loader can produce.
	Resolves func(name string) bool

	// PluginVersion returns the version a loaded plugin declared in its
	// handshake, by plugin name. A nil function means no plugins are loaded.
	PluginVersion func(name string) (string, bool)
}

// FailureKind classifies a requirement failure.
type FailureKind string

const (
	FailureVersion       FailureKind = "version"
	FailureLibrary       FailureKind = "library"
	FailurePluginMissing FailureKind = "plugin-missing"
	FailurePluginVersion FailureKind = "plugin-version"
)

// Failure is one unmet requirement with a rendered, neutral message.
type Failure struct {
	Kind    FailureKind
	Message string
}

// CheckError is the aggregated result of a failed Verify, carrying every
// failure so a script learns all of them at once instead of one per run.
type CheckError struct {
	Failures []Failure
}

func (e *CheckError) Error() string {
	lines := make([]string, 0, len(e.Failures))
	for _, f := range e.Failures {
		lines = append(lines, f.Message)
	}
	return "script requirements not met:\n  - " + strings.Join(lines, "\n  - ")
}

// Has reports whether any failure has the given kind; hosts use it to attach
// their own remedy hints (the CLI adds how to load plugins).
func (e *CheckError) Has(kind FailureKind) bool {
	for _, f := range e.Failures {
		if f.Kind == kind {
			return true
		}
	}
	return false
}

// lookupPlugin resolves a required plugin name to a loaded plugin's declared
// version. The exact declared name wins; a bare name (no dot) also matches
// the same name under scriptling's host-owned namespaces, so "sql" finds the
// first-party plugin that declares "scriptling.sql" and "hello" finds a
// plugin declaring the bare name "hello" (registered as "plugin.hello").
// Bare names never match third-party dotted namespaces like "knot.space".
func lookupPlugin(env Env, name string) (string, bool) {
	if env.PluginVersion == nil {
		return "", false
	}
	if version, ok := env.PluginVersion(name); ok {
		return version, true
	}
	if !strings.Contains(name, ".") {
		if version, ok := env.PluginVersion("scriptling." + name); ok {
			return version, true
		}
		return env.PluginVersion("plugin." + name)
	}
	return "", false
}

// Verify checks the metadata against the host environment. Dependencies are
// checked first: a dependency that resolves is satisfied however the
// environment provides it, and only an unresolved dependency promotes its
// declared plugin into the required set. The plugin check then reports every
// missing or too-old plugin — declared directly or promoted — in one pass.
// When a plugin is named both directly and by a promoted dependency, the
// direct entry (with its constraint) is the intentional statement and wins.
func (m Metadata) Verify(env Env) error {
	var failures []Failure

	if m.RequiresScriptling != "" {
		ok, err := Satisfies(env.HostVersion, m.RequiresScriptling)
		switch {
		case err != nil:
			failures = append(failures, Failure{FailureVersion, fmt.Sprintf("requires-scriptling %s cannot be checked against host %s: %v", m.RequiresScriptling, env.HostVersion, err)})
		case !ok:
			failures = append(failures, Failure{FailureVersion, fmt.Sprintf("this script needs scriptling %s, but this host is %s", m.RequiresScriptling, env.HostVersion)})
		}
	}

	required := make([]PluginRequirement, 0, len(m.Plugins))
	required = append(required, m.Plugins...)
	seen := make(map[string]bool, len(required))
	for _, req := range required {
		seen[req.Plugin] = true
	}
	provides := map[string]string{}
	for _, dep := range m.Dependencies {
		if env.Resolves != nil && env.Resolves(dep.Library) {
			continue
		}
		if dep.Plugin == "" {
			failures = append(failures, Failure{FailureLibrary, fmt.Sprintf("required library %q is not available in this environment", dep.Library)})
			continue
		}
		if !seen[dep.Plugin] {
			seen[dep.Plugin] = true
			required = append(required, PluginRequirement{Plugin: dep.Plugin, Constraint: dep.Constraint})
		}
		provides[dep.Plugin] = dep.Library
	}

	for _, req := range required {
		version, loaded := lookupPlugin(env, req.Plugin)
		if !loaded {
			message := fmt.Sprintf("required plugin %q is not loaded", req.Plugin)
			if library, ok := provides[req.Plugin]; ok && library != req.Plugin {
				message = fmt.Sprintf("required plugin %q is not loaded (it provides the library %q)", req.Plugin, library)
			}
			failures = append(failures, Failure{FailurePluginMissing, message})
			continue
		}
		if req.Constraint == "" {
			continue
		}
		ok, err := Satisfies(version, req.Constraint)
		switch {
		case err != nil:
			failures = append(failures, Failure{FailurePluginVersion, fmt.Sprintf("plugin %q version %q cannot be checked against %s: %v", req.Plugin, version, req.Constraint, err)})
		case !ok:
			failures = append(failures, Failure{FailurePluginVersion, fmt.Sprintf("plugin %q is version %s, but this script needs %s", req.Plugin, version, req.Constraint)})
		}
	}

	if len(failures) > 0 {
		return &CheckError{Failures: failures}
	}
	return nil
}
