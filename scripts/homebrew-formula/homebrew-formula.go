package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"text/template"

	"github.com/paularlott/scriptling/build"
)

// formulaTemplate works for the lean and full builds: Prefix is the artifact
// name (scriptling / scriptling-full), Class the formula class, Desc the
// description line, BinName the executable inside the zip, and InstallAs the
// name it installs as (full installs as `scriptling`, hence the conflict
// with the core formula).
const formulaTemplate = `class {{ .Class }} < Formula
	desc "{{ .Desc }}"
	homepage "https://github.com/paularlott/scriptling"
	license "MIT"
	version "{{ .Version }}"
{{ if .Conflicts }}	conflicts_with "{{ .Conflicts }}", because: "both install a scriptling binary"
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
		bin.install "{{ .BinName }}" => "{{ .InstallAs }}"
	end

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

			scriptling-full users do not need this formula — the plugins are
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
	for file, target := range pairs {
		if *target, err = checksumFor("bin/" + file); err != nil {
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
		case "-full", "-plugins":
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
	case "-full":
		data := struct {
			Version   string
			Prefix    string
			Class     string
			Desc      string
			BinName   string
			InstallAs string
			Conflicts string
			Caveats   string
			Checksum  checksums
		}{
			Version:   build.Version,
			Prefix:    "scriptling-full",
			Class:     "ScriptlingFull",
			Desc:      "Scriptling with the sqlite, sql, valkey and badger database plugins compiled in",
			BinName:   "scriptling-full",
			InstallAs: "scriptling",
			Conflicts: "scriptling",
			Caveats: "This formula replaces the plain `scriptling` binary with the full build\n" +
				"(all database plugins compiled in); brew uninstall scriptling first.\n" +
				"The scriptling-plugins formula is not needed with this build.\n\n",
		}
		var err error
		if data.Checksum, err = checksumSet("scriptling-full"); err != nil {
			fail("Error opening file: %v", err)
		}
		if err := template.Must(template.New("formula").Parse(formulaTemplate)).Execute(os.Stdout, data); err != nil {
			fail("Error executing template: %v", err)
		}
		return
	}

	data := struct {
		Version   string
		Prefix    string
		Class     string
		Desc      string
		BinName   string
		InstallAs string
		Conflicts string
		Caveats   string
		Checksum  checksums
	}{
		Version:   build.Version,
		Prefix:    "scriptling",
		Class:     "Scriptling",
		Desc:      "A powerful scripting language with Python-like syntax and Go performance",
		BinName:   "scriptling",
		InstallAs: "scriptling",
		Conflicts: "scriptling-full",
		Caveats: "For the database plugins: brew install scriptling-full (this binary plus\n" +
			"sqlite/sql/valkey/badger compiled in), or keep this lean build and\n" +
			"brew install scriptling-plugins, then run with\n" +
			"SCRIPTLING_PLUGIN_DIR=\"$(brew --prefix)/opt/scriptling-plugins/libexec/plugins\".\n\n",
	}
	var err error
	if data.Checksum, err = checksumSet("scriptling"); err != nil {
		fail("Error opening file: %v", err)
	}
	if err := template.Must(template.New("formula").Parse(formulaTemplate)).Execute(os.Stdout, data); err != nil {
		fail("Error executing template: %v", err)
	}
}
