package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/template"

	"github.com/paularlott/scriptling/build"
)

// formulaTemplate works for every CLI variant: Prefix is the artifact name
// (scriptling, scriptling-slim, scriptling-onepassword,
// scriptling-slim-onepassword), Class the formula class, Desc the description
// line, BinName the executable inside the zip, and InstallAs the name it
// installs as. Every zip carries the binary named scriptling, so each variant
// conflicts with the others.
const formulaTemplate = `class {{ .Class }} < Formula
	desc "{{ .Desc }}"
	homepage "https://github.com/paularlott/scriptling"
	license "MIT"
	version "{{ .Version }}"
{{ range .Conflicts }}	conflicts_with "{{ . }}", because: "both install a scriptling binary"
{{ end }}	if OS.mac?
		if Hardware::CPU.arm?
			url "https://github.com/paularlott/scriptling/releases/download/v#{version}/{{ .Prefix }}-darwin-arm64.zip"
			sha256 "{{ .Checksum.DarwinArm64 }}"
		else
			url "https://github.com/paularlott/scriptling/releases/download/v#{version}/{{ .Prefix }}-darwin-amd64.zip"
			sha256 "{{ .Checksum.DarwinAmd64 }}"
		end
	elsif OS.linux?
		if Hardware::CPU.arm?
			url "https://github.com/paularlott/scriptling/releases/download/v#{version}/{{ .Prefix }}-linux-arm64.zip"
			sha256 "{{ .Checksum.LinuxArm64 }}"
		else
			url "https://github.com/paularlott/scriptling/releases/download/v#{version}/{{ .Prefix }}-linux-amd64.zip"
			sha256 "{{ .Checksum.LinuxAmd64 }}"
		end
	end

	def install
{{ if ne .BinName .InstallAs }}		bin.install "{{ .BinName }}" => "{{ .InstallAs }}"
{{ else }}		bin.install "{{ .BinName }}"
{{ end }}	end

	def caveats
		<<~EOS
{{ .Caveats }}		EOS
	end
end
`

// pluginsTemplate installs the database plugin binaries under libexec; the
// scriptling formula finds them via --plugin-dir / SCRIPTLING_PLUGIN_DIR.
// One plugins-<os>-<arch>.zip per platform carries all four binaries.
const pluginsTemplate = `class ScriptlingPlugins < Formula
	desc "Database plugins for Scriptling (sqlite, sql, valkey, badger)"
	homepage "https://github.com/paularlott/scriptling"
	license "MIT"
	version "{{ .Version }}"
	if OS.mac?
		if Hardware::CPU.arm?
			url "https://github.com/paularlott/scriptling/releases/download/v#{version}/plugins-darwin-arm64.zip"
			sha256 "{{ .Checksum.DarwinArm64 }}"
		else
			url "https://github.com/paularlott/scriptling/releases/download/v#{version}/plugins-darwin-amd64.zip"
			sha256 "{{ .Checksum.DarwinAmd64 }}"
		end
	elsif OS.linux?
		if Hardware::CPU.arm?
			url "https://github.com/paularlott/scriptling/releases/download/v#{version}/plugins-linux-arm64.zip"
			sha256 "{{ .Checksum.LinuxArm64 }}"
		else
			url "https://github.com/paularlott/scriptling/releases/download/v#{version}/plugins-linux-amd64.zip"
			sha256 "{{ .Checksum.LinuxAmd64 }}"
		end
	end

	def install
		# The zip holds all four plugin binaries named plainly; installing
		# them together gives a directory --plugin-dir can point at.
		(libexec/"plugins").install Dir["*"]
	end

	def caveats
		<<~EOS
			The database plugin binaries are in:
			  #{opt_libexec}/plugins

			Load them with either:
			  export SCRIPTLING_PLUGIN_DIR="#{opt_libexec}/plugins"
			or pass to each run:
			  scriptling --plugin-dir #{opt_libexec}/plugins script.py

			scriptling users do not need this formula — the plugins are
			compiled in.
		EOS
	end
end
`

type checksums struct {
	DarwinArm64 string
	DarwinAmd64 string
	LinuxArm64  string
	LinuxAmd64  string
}

func checksumFor(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func checksumSet(prefix string) (checksums, error) {
	var set checksums
	var err error
	pairs := map[string]*string{
		prefix + "-darwin-amd64.zip": &set.DarwinAmd64,
		prefix + "-darwin-arm64.zip": &set.DarwinArm64,
		prefix + "-linux-amd64.zip":  &set.LinuxAmd64,
		prefix + "-linux-arm64.zip":  &set.LinuxArm64,
	}
	artifactDir := os.Getenv("SCRIPTLING_FORMULA_BIN_DIR")
	if artifactDir == "" {
		artifactDir = "bin"
	}
	for file, target := range pairs {
		if *target, err = checksumFor(filepath.Join(artifactDir, file)); err != nil {
			return set, err
		}
	}
	return set, nil
}

// fail reports a generation error on stderr and exits non-zero: Taskfile
// redirects stdout into the formula file, so an error printed to stdout (with
// a zero exit status) would silently replace the formula with an error
// string while release automation stays green.
func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func main() {
	mode := ""
	for _, arg := range os.Args[1:] {
		switch arg {
		case "-slim", "-plugins", "-onepassword", "-slim-onepassword":
			mode = arg
		}
	}

	switch mode {
	case "-plugins":
		data := struct {
			Version  string
			Checksum checksums
		}{Version: build.Version}
		var err error
		if data.Checksum, err = checksumSet("plugins"); err != nil {
			fail("Error checksumming plugin zips: %v", err)
		}
		if err := template.Must(template.New("plugins").Parse(pluginsTemplate)).Execute(os.Stdout, data); err != nil {
			fail("Error executing template: %v", err)
		}
		return
	}

	v, ok := variants[mode]
	if !ok {
		fail("unknown formula %q", mode)
	}
	data := struct {
		Version   string
		Prefix    string
		Class     string
		Desc      string
		BinName   string
		InstallAs string
		Conflicts []string
		Caveats   string
		Checksum  checksums
	}{
		Version:   build.Version,
		Prefix:    v.prefix,
		Class:     v.class,
		Desc:      v.desc,
		BinName:   "scriptling",
		InstallAs: "scriptling",
		Caveats:   v.caveats,
	}
	for _, other := range cliVariants {
		if other != v.prefix {
			data.Conflicts = append(data.Conflicts, other)
		}
	}
	var err error
	if data.Checksum, err = checksumSet(v.prefix); err != nil {
		fail("Error opening file: %v", err)
	}
	if err := template.Must(template.New("formula").Parse(formulaTemplate)).Execute(os.Stdout, data); err != nil {
		fail("Error executing template: %v", err)
	}
}

// cliVariants are the formulas that install a scriptling binary; each
// conflicts with all the others.
var cliVariants = []string{"scriptling", "scriptling-slim", "scriptling-onepassword", "scriptling-slim-onepassword"}

type variant struct {
	prefix, class, desc, caveats string
}

// variants maps the generator's flag to the CLI build it describes; the
// default (no flag) is the standard scriptling build.
var variants = map[string]variant{
	"": {
		prefix: "scriptling",
		class:  "Scriptling",
		desc:   "A powerful scripting language with Python-like syntax and database plugins built in",
		caveats: "The sqlite, sql, valkey and badger database plugins are compiled in,\n" +
			"so the scriptling-plugins formula is not needed. For a leaner\n" +
			"binary without them: brew install scriptling-slim. For 1Password\n" +
			"service accounts in scriptling.secret: brew install scriptling-onepassword.\n\n",
	},
	"-slim": {
		prefix: "scriptling-slim",
		class:  "ScriptlingSlim",
		desc:   "Scriptling without the database plugins compiled in (use scriptling-plugins)",
		caveats: "This formula installs Scriptling without the database plugins\n" +
			"compiled in. For the database plugins: brew install scriptling\n" +
			"(this binary plus sqlite/sql/valkey/badger compiled in), or keep\n" +
			"this lean build and brew install scriptling-plugins, then run with\n" +
			"SCRIPTLING_PLUGIN_DIR=\"$(brew --prefix)/opt/scriptling-plugins/libexec/plugins\".\n\n",
	},
	"-onepassword": {
		prefix: "scriptling-onepassword",
		class:  "ScriptlingOnepassword",
		desc:   "Scriptling with database plugins and 1Password service accounts built in",
		caveats: "This formula installs Scriptling with the database plugins and\n" +
			"1Password service account support compiled in: scriptling.secret's\n" +
			"onepassword provider accepts service_account_token as well as Connect.\n" +
			"Without the database plugins: brew install scriptling-slim-onepassword.\n\n",
	},
	"-slim-onepassword": {
		prefix: "scriptling-slim-onepassword",
		class:  "ScriptlingSlimOnepassword",
		desc:   "Scriptling with 1Password service accounts built in, without the database plugins",
		caveats: "This formula installs Scriptling with 1Password service account\n" +
			"support compiled in (scriptling.secret's onepassword provider accepts\n" +
			"service_account_token) but without the database plugins. Add them with\n" +
			"brew install scriptling-plugins, or install scriptling-onepassword.\n\n",
	},
}
