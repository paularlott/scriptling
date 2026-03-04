package main

import (
	"testing"
)

func TestBuildLibDirs(t *testing.T) {
	t.Run("base dir only when no extras", func(t *testing.T) {
		dirs := buildLibDirs("/app/scripts", nil)
		if len(dirs) != 1 || dirs[0] != "/app/scripts" {
			t.Errorf("expected [/app/scripts], got %v", dirs)
		}
	})

	t.Run("base dir first then extras", func(t *testing.T) {
		dirs := buildLibDirs("/app/scripts", []string{"/shared/libs", "/extra"})
		if len(dirs) != 3 {
			t.Fatalf("expected 3 dirs, got %d: %v", len(dirs), dirs)
		}
		if dirs[0] != "/app/scripts" {
			t.Errorf("expected base dir first, got %s", dirs[0])
		}
		if dirs[1] != "/shared/libs" {
			t.Errorf("expected /shared/libs second, got %s", dirs[1])
		}
		if dirs[2] != "/extra" {
			t.Errorf("expected /extra third, got %s", dirs[2])
		}
	})

	t.Run("empty strings in extras are skipped", func(t *testing.T) {
		dirs := buildLibDirs("/base", []string{"", "/valid", ""})
		if len(dirs) != 2 {
			t.Fatalf("expected 2 dirs, got %d: %v", len(dirs), dirs)
		}
		if dirs[0] != "/base" {
			t.Errorf("expected /base first, got %s", dirs[0])
		}
		if dirs[1] != "/valid" {
			t.Errorf("expected /valid second, got %s", dirs[1])
		}
	})

	t.Run("empty extras slice", func(t *testing.T) {
		dirs := buildLibDirs("/base", []string{})
		if len(dirs) != 1 || dirs[0] != "/base" {
			t.Errorf("expected [/base], got %v", dirs)
		}
	})

	t.Run("empty base dir is skipped", func(t *testing.T) {
		dirs := buildLibDirs("", []string{"/extra"})
		if len(dirs) != 1 || dirs[0] != "/extra" {
			t.Errorf("expected [/extra], got %v", dirs)
		}
	})

	t.Run("empty base dir and no extras returns empty", func(t *testing.T) {
		dirs := buildLibDirs("", nil)
		if len(dirs) != 0 {
			t.Errorf("expected empty slice, got %v", dirs)
		}
	})
}

func TestParseAllowedPaths(t *testing.T) {
	t.Run("empty string returns nil", func(t *testing.T) {
		if parseAllowedPaths("") != nil {
			t.Error("expected nil for empty string")
		}
	})

	t.Run("dash returns empty slice (deny all)", func(t *testing.T) {
		result := parseAllowedPaths("-")
		if result == nil || len(result) != 0 {
			t.Errorf("expected empty slice, got %v", result)
		}
	})

	t.Run("single path", func(t *testing.T) {
		result := parseAllowedPaths("/tmp")
		if len(result) != 1 || result[0] != "/tmp" {
			t.Errorf("expected [/tmp], got %v", result)
		}
	})

	t.Run("multiple paths", func(t *testing.T) {
		result := parseAllowedPaths("/tmp,/var/data, /home/user")
		if len(result) != 3 {
			t.Fatalf("expected 3 paths, got %d: %v", len(result), result)
		}
		if result[0] != "/tmp" || result[1] != "/var/data" || result[2] != "/home/user" {
			t.Errorf("unexpected paths: %v", result)
		}
	})

	t.Run("whitespace-only entries are ignored", func(t *testing.T) {
		result := parseAllowedPaths("/tmp, , /var")
		if len(result) != 2 {
			t.Fatalf("expected 2 paths, got %d: %v", len(result), result)
		}
	})
}
