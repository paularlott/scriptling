package pack

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// ListPackage summarizes a package file: the manifest's name, version and
// protocols, each convention directory with its file count, the total, and
// the sha256. A pre-deploy sanity check for what actually shipped in the
// artifact; the CLI's `pack --list` prints the returned summary verbatim.
func ListPackage(path string) (string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("failed to open package: %w", err)
	}
	defer zr.Close()

	byDir := map[string][]string{}
	var manifestName, manifestVersion, manifestServe string
	var total int
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		parts := strings.SplitN(f.Name, "/", 2)
		dir := "(root)"
		if len(parts) == 2 {
			dir = parts[0]
		}
		byDir[dir] = append(byDir[dir], f.Name)
		total++
		if f.Name == "manifest.toml" {
			if data, err := readZipEntry(f); err == nil {
				for _, line := range strings.Split(string(data), "\n") {
					line = strings.TrimSpace(line)
					for _, key := range []string{"name", "version", "serve"} {
						prefix := key + " = "
						if strings.HasPrefix(line, prefix) {
							v := strings.TrimSpace(strings.TrimPrefix(line, prefix))
							switch key {
							case "name":
								manifestName = strings.Trim(v, "\"")
							case "version":
								manifestVersion = strings.Trim(v, "\"")
							case "serve":
								manifestServe = parseServe(v)
							}
						}
					}
				}
			}
		}
	}

	var b strings.Builder
	if manifestName != "" {
		fmt.Fprintf(&b, "package: %s %s\n", manifestName, manifestVersion)
	}
	if manifestServe != "" {
		fmt.Fprintf(&b, "serves: %s\n", manifestServe)
	}
	dirs := make([]string, 0, len(byDir))
	for dir := range byDir {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	for _, dir := range dirs {
		fmt.Fprintf(&b, "  %-12s %d file(s)\n", dir+"/", len(byDir[dir]))
	}
	fmt.Fprintf(&b, "  %-12s %d\n", "total", total)

	if data, err := os.ReadFile(path); err == nil {
		fmt.Fprintf(&b, "sha256=%s\n", HashBytes(data))
	}
	return b.String(), nil
}

// parseServe turns a TOML value like `["mcp", "http"]` or `"mcp"` into a
// comma-separated list of the bare strings.
func parseServe(v string) string {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "[") {
		return strings.Trim(v, "\"")
	}
	parts := strings.Split(strings.Trim(v, "[]"), ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, strings.Trim(p, "\""))
		}
	}
	return strings.Join(out, ",")
}

func readZipEntry(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}
