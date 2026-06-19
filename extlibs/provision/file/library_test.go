package file

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paularlott/scriptling"
)

func TestProvisionFileRegistration(t *testing.T) {
	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
`)
	if err != nil {
		t.Fatalf("Failed to import provision.file library: %v", err)
	}
}

func TestEnsureCreate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "test.txt")

	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
status = file.ensure("` + path + `", "hello world")
if status != file.CREATED:
    raise Exception("expected CREATED")
`)
	if err != nil {
		t.Fatalf("Failed to run ensure: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(content) != "hello world" {
		t.Errorf("Expected 'hello world', got %s", string(content))
	}
}

func TestEnsureUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("same content"), 0o644)

	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
status = file.ensure("` + path + `", "same content")
if status != file.UNCHANGED:
    raise Exception("expected UNCHANGED")
`)
	if err != nil {
		t.Fatalf("Failed to run ensure: %v", err)
	}

	status, _ := p.GetVar("status")
	if status.(string) != "unchanged" {
		t.Errorf("Expected 'unchanged', got %s", status)
	}
}

func TestEnsureUpdated(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("old content"), 0o644)

	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
status = file.ensure("` + path + `", "new content")
if status != file.UPDATED:
    raise Exception("expected UPDATED")
`)
	if err != nil {
		t.Fatalf("Failed to run ensure: %v", err)
	}

	content, _ := os.ReadFile(path)
	if string(content) != "new content" {
		t.Errorf("Expected 'new content', got %s", string(content))
	}
}

func TestEnsureWithMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret.txt")

	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
status = file.ensure("` + path + `", "secret", mode=0o600)
`)
	if err != nil {
		t.Fatalf("Failed to run ensure with mode: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("Expected mode 0o600, got %o", info.Mode().Perm())
	}
}

func TestEnsureCreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "c", "test.txt")

	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
status = file.ensure("` + path + `", "nested")
`)
	if err != nil {
		t.Fatalf("Failed to run ensure with nested dirs: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("Expected file to exist at %s", path)
	}
}

func TestEnsureWrongArgCount(t *testing.T) {
	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
try:
    file.ensure("only_one_arg")
    error_caught = False
except:
    error_caught = True
`)
	if err != nil {
		t.Fatalf("Failed to run wrong arg count test: %v", err)
	}

	caught, _ := p.GetVar("error_caught")
	if caught.(bool) != true {
		t.Errorf("Expected error for missing argument")
	}
}

func TestEnsureOverwriteDifferentMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("same"), 0o644)

	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
status = file.ensure("` + path + `", "same", mode=0o600)
if status != file.UNCHANGED:
    raise Exception("expected UNCHANGED")
`)
	if err != nil {
		t.Fatalf("Failed to run ensure: %v", err)
	}
}

func TestEnsureCreateOnlySkipsExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("original"), 0o644)

	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
status = file.ensure("` + path + `", "different", create_only=True)
if status != file.UNCHANGED:
    raise Exception("expected UNCHANGED")
`)
	if err != nil {
		t.Fatalf("Failed to run ensure with create_only: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(content) != "original" {
		t.Errorf("create_only should not modify existing file, got %s", string(content))
	}
}

func TestEnsureCreateOnlyCreatesNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "test.txt")

	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
status = file.ensure("` + path + `", "fresh", create_only=True)
if status != file.CREATED:
    raise Exception("expected CREATED")
`)
	if err != nil {
		t.Fatalf("Failed to run ensure with create_only: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(content) != "fresh" {
		t.Errorf("Expected 'fresh', got %s", string(content))
	}
}

func TestEnsureCreateOnlyFalseUpdates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("old"), 0o644)

	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
status = file.ensure("` + path + `", "new", create_only=False)
if status != file.UPDATED:
    raise Exception("expected UPDATED")
`)
	if err != nil {
		t.Fatalf("Failed to run ensure: %v", err)
	}

	content, _ := os.ReadFile(path)
	if string(content) != "new" {
		t.Errorf("Expected 'new', got %s", string(content))
	}
}

func TestAbsentRemoves(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("data"), 0o644)

	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
status = file.absent("` + path + `")
if status != file.REMOVED:
    raise Exception("expected REMOVED")
`)
	if err != nil {
		t.Fatalf("Failed to run absent: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("Expected file to be removed")
	}
}

func TestAbsentAlreadyGone(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.txt")

	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
status = file.absent("` + path + `")
if status != file.ABSENT:
    raise Exception("expected ABSENT")
`)
	if err != nil {
		t.Fatalf("Failed to run absent: %v", err)
	}
}

func TestDirectoryCreate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "c")

	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
status = file.ensure_directory("` + path + `")
if status != file.CREATED:
    raise Exception("expected CREATED")
`)
	if err != nil {
		t.Fatalf("Failed to run ensure_directory: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Expected dir to exist: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("Expected %s to be a directory", path)
	}
}

func TestEnsureDirectoryExists(t *testing.T) {
	dir := t.TempDir()

	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
status = file.ensure_directory("` + dir + `")
if status != file.EXISTS:
    raise Exception("expected EXISTS")
`)
	if err != nil {
		t.Fatalf("Failed to run ensure_directory: %v", err)
	}
}

func TestEnsureDirectoryNotADir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	os.WriteFile(path, []byte("data"), 0o644)

	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
try:
    file.ensure_directory("` + path + `")
    error_caught = False
except:
    error_caught = True
`)
	if err != nil {
		t.Fatalf("Failed to run ensure_directory: %v", err)
	}

	caught, _ := p.GetVar("error_caught")
	if caught.(bool) != true {
		t.Errorf("Expected error when path is a file")
	}
}

func TestEnsureDirectoryWithMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secretdir")

	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
status = file.ensure_directory("` + path + `", mode=0o700)
if status != file.CREATED:
    raise Exception("expected CREATED")
`)
	if err != nil {
		t.Fatalf("Failed to run ensure_directory: %v", err)
	}

	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o700 {
		t.Errorf("Expected mode 0o700, got %o", info.Mode().Perm())
	}
}

func TestAbsentDirectoryRemoves(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "emptydir")
	os.Mkdir(path, 0o755)

	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
status = file.absent_directory("` + path + `")
if status != file.REMOVED:
    raise Exception("expected REMOVED")
`)
	if err != nil {
		t.Fatalf("Failed to run absent_directory: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("Expected directory to be removed")
	}
}

func TestAbsentDirectoryAlreadyGone(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent")

	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
status = file.absent_directory("` + path + `")
if status != file.ABSENT:
    raise Exception("expected ABSENT")
`)
	if err != nil {
		t.Fatalf("Failed to run absent_directory: %v", err)
	}
}

func TestAbsentDirectoryNotEmpty(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "mydir")
	os.Mkdir(subdir, 0o755)
	os.WriteFile(filepath.Join(subdir, "file.txt"), []byte("data"), 0o644)

	p := scriptling.New()
	Register(p)

	_, err := p.Eval(`
import scriptling.provision.file as file
try:
    file.absent_directory("` + subdir + `")
    error_caught = False
except:
    error_caught = True
`)
	if err != nil {
		t.Fatalf("Failed to run absent_directory: %v", err)
	}

	caught, _ := p.GetVar("error_caught")
	if caught.(bool) != true {
		t.Errorf("Expected error for non-empty directory")
	}
}
