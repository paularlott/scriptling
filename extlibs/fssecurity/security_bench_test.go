package fssecurity

import (
	"os"
	"path/filepath"
	"testing"
)

func benchPaths(b *testing.B) (Config, string, string, string) {
	b.Helper()
	dir, _ := os.MkdirTemp("", "fsbench")
	b.Cleanup(func() { os.RemoveAll(dir) })
	jail := filepath.Join(dir, "jail")
	os.MkdirAll(filepath.Join(jail, "a", "b"), 0o755)
	existing := filepath.Join(jail, "a", "b")
	os.WriteFile(filepath.Join(existing, "f.txt"), []byte("x"), 0o644)
	cfg := Config{AllowedPaths: []string{jail}}
	return cfg, filepath.Join(existing, "f.txt"), filepath.Join(jail, "a", "b", "new.txt"), filepath.Join(jail, "x", "y", "z.txt")
}

func BenchmarkIsPathAllowed_Existing(b *testing.B) {
	cfg, existing, _, _ := benchPaths(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg.IsPathAllowed(existing)
	}
}

func BenchmarkIsPathAllowed_NewFileInJail(b *testing.B) {
	cfg, _, newFile, _ := benchPaths(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg.IsPathAllowed(newFile)
	}
}

func BenchmarkIsPathAllowed_DeepNew(b *testing.B) {
	cfg, _, _, deep := benchPaths(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg.IsPathAllowed(deep)
	}
}
