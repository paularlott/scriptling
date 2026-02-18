package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLint_EmptyCode(t *testing.T) {
	result := Lint("", nil)
	if result.HasIssues() {
		t.Errorf("Expected no issues for empty code, got: %v", result.Errors)
	}
	if result.HasErrors {
		t.Error("Expected HasErrors to be false for empty code")
	}
	if result.FilesChecked != 1 {
		t.Errorf("Expected FilesChecked=1, got %d", result.FilesChecked)
	}
}

func TestLint_ValidCode(t *testing.T) {
	tests := []struct {
		name  string
		code  string
		valid bool
	}{
		{
			name:  "simple assignment",
			code:  "x = 42",
			valid: true,
		},
		{
			name:  "function definition",
			code:  "def add(a, b):\n    return a + b",
			valid: true,
		},
		{
			name:  "class definition",
			code:  "class Counter:\n    def __init__(self):\n        self.value = 0",
			valid: true,
		},
		{
			name:  "if statement",
			code:  "if x > 0:\n    print(x)",
			valid: true,
		},
		{
			name:  "for loop",
			code:  "for i in range(10):\n    print(i)",
			valid: true,
		},
		{
			name:  "list comprehension",
			code:  "squares = [x*x for x in range(10)]",
			valid: true,
		},
		{
			name:  "import statement",
			code:  "import json\njson.dumps({})",
			valid: true,
		},
		{
			name:  "try except",
			code:  "try:\n    risky()\nexcept Exception as e:\n    print(e)",
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Lint(tt.code, nil)
			if tt.valid && result.HasIssues() {
				t.Errorf("Expected no issues for valid code, got: %v", result.Errors)
			}
			if tt.valid && result.HasErrors {
				t.Error("Expected HasErrors to be false for valid code")
			}
		})
	}
}

func TestLint_InvalidCode(t *testing.T) {
	tests := []struct {
		name          string
		code          string
		expectError   bool
		expectedInMsg string // substring expected in error message
	}{
		{
			name:          "unclosed parenthesis",
			code:          "print(",
			expectError:   true,
			expectedInMsg: "",
		},
		{
			name:          "invalid syntax - missing colon",
			code:          "if True\n    print(1)",
			expectError:   true,
			expectedInMsg: ":", // The colon token is mentioned in error
		},
		{
			name:          "invalid expression",
			code:          "x = +",
			expectError:   true,
			expectedInMsg: "",
		},
		{
			name:          "break outside loop",
			code:          "break",
			expectError:   false, // Parser doesn't catch this, only runtime
			expectedInMsg: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Lint(tt.code, nil)
			if tt.expectError && !result.HasIssues() {
				t.Errorf("Expected issues for invalid code, but got none")
			}
			if tt.expectError && !result.HasErrors {
				t.Error("Expected HasErrors to be true for invalid code")
			}
			if tt.expectedInMsg != "" && result.HasIssues() {
				found := false
				for _, err := range result.Errors {
					if strings.Contains(err.Message, tt.expectedInMsg) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error message to contain %q, got: %v", tt.expectedInMsg, result.Errors)
				}
			}
		})
	}
}

func TestLint_WithFilename(t *testing.T) {
	code := "x = " // Invalid code
	result := Lint(code, &Options{Filename: "test.py"})

	if !result.HasIssues() {
		t.Error("Expected issues for invalid code")
	}

	if len(result.Errors) == 0 {
		t.Fatal("Expected at least one error")
	}

	if result.Errors[0].File != "test.py" {
		t.Errorf("Expected filename 'test.py', got %q", result.Errors[0].File)
	}
}

func TestLint_LineNumbers(t *testing.T) {
	// Multi-line code with error on line 3
	code := `x = 1
y = 2
z = +
w = 4`

	result := Lint(code, nil)
	if !result.HasIssues() {
		t.Fatal("Expected issues for invalid code")
	}

	// Check that the error is reported on the correct line
	found := false
	for _, err := range result.Errors {
		if err.Line == 3 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected error on line 3, got errors: %v", result.Errors)
	}
}

func TestLint_MultipleErrors(t *testing.T) {
	// Code with multiple errors
	code := `x = +
y = +`

	result := Lint(code, nil)
	if !result.HasIssues() {
		t.Fatal("Expected issues for invalid code")
	}

	if len(result.Errors) < 2 {
		t.Errorf("Expected at least 2 errors, got %d", len(result.Errors))
	}
}

func TestLintFile(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.py")

	// Test with valid code
	validCode := "x = 42"
	err := os.WriteFile(tmpFile, []byte(validCode), 0644)
	if err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	result, err := LintFile(tmpFile)
	if err != nil {
		t.Fatalf("LintFile failed: %v", err)
	}

	if result.HasIssues() {
		t.Errorf("Expected no issues for valid code, got: %v", result.Errors)
	}

	if result.FilesChecked != 1 {
		t.Errorf("Expected FilesChecked=1, got %d", result.FilesChecked)
	}
}

func TestLintFile_InvalidCode(t *testing.T) {
	// Create a temporary file with invalid code
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "invalid.py")

	invalidCode := "x = +"
	err := os.WriteFile(tmpFile, []byte(invalidCode), 0644)
	if err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	result, err := LintFile(tmpFile)
	if err != nil {
		t.Fatalf("LintFile failed: %v", err)
	}

	if !result.HasIssues() {
		t.Error("Expected issues for invalid code")
	}

	if !result.HasErrors {
		t.Error("Expected HasErrors to be true")
	}

	// Check filename is set
	if len(result.Errors) > 0 && result.Errors[0].File == "" {
		t.Error("Expected filename to be set in error")
	}
}

func TestLintFile_FileNotFound(t *testing.T) {
	_, err := LintFile("/nonexistent/path/to/file.py")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestLintFile_FilenameInErrorReport(t *testing.T) {
	// Create a temporary file with invalid code
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "my_script.py")

	invalidCode := "def broken(\n    pass"
	err := os.WriteFile(tmpFile, []byte(invalidCode), 0644)
	if err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	result, err := LintFile(tmpFile)
	if err != nil {
		t.Fatalf("LintFile failed: %v", err)
	}

	if !result.HasIssues() {
		t.Fatal("Expected issues for invalid code")
	}

	// Verify that the filename is correctly set in the error
	for i, lintErr := range result.Errors {
		// Check that the full path is used
		if lintErr.File != tmpFile {
			t.Errorf("Error %d: Expected File='%s', got File='%s'", i, tmpFile, lintErr.File)
		}
		if lintErr.Line < 1 {
			t.Errorf("Error %d: Expected Line >= 1, got Line=%d", i, lintErr.Line)
		}
		if lintErr.Message == "" {
			t.Errorf("Error %d: Expected non-empty Message", i)
		}
		if lintErr.Severity != SeverityError {
			t.Errorf("Error %d: Expected Severity='error', got Severity='%s'", i, lintErr.Severity)
		}
	}
}

func TestLintFile_DifferentFilenames(t *testing.T) {
	// Test that different filenames are correctly preserved in error reports
	tests := []string{
		"script.py",
		"my_script.py",
		"path/to/script.py",
		"scripts/deeply/nested/path/to/script.py",
	}

	invalidCode := "x = +"

	for _, filename := range tests {
		t.Run(filename, func(t *testing.T) {
			tmpDir := t.TempDir()
		 fullPath := filepath.Join(tmpDir, filename)

			// Create parent directories if needed
			dir := filepath.Dir(fullPath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatalf("Failed to create directories: %v", err)
			}

			err := os.WriteFile(fullPath, []byte(invalidCode), 0644)
			if err != nil {
				t.Fatalf("Failed to write temp file: %v", err)
			}

			result, err := LintFile(fullPath)
			if err != nil {
				t.Fatalf("LintFile failed: %v", err)
			}

			if !result.HasIssues() {
				t.Fatal("Expected issues for invalid code")
			}

			// Verify full path is preserved
			for _, lintErr := range result.Errors {
				if lintErr.File != fullPath {
					t.Errorf("Expected File='%s', got File='%s'", fullPath, lintErr.File)
				}
			}
		})
	}
}

func TestLint_StdinFilename(t *testing.T) {
	// When linting from stdin, the filename should be "stdin"
	code := "x = +"
	result := Lint(code, &Options{Filename: "stdin"})

	if !result.HasIssues() {
		t.Fatal("Expected issues for invalid code")
	}

	for _, lintErr := range result.Errors {
		if lintErr.File != "stdin" {
			t.Errorf("Expected File='stdin', got File='%s'", lintErr.File)
		}
	}
}

func TestLintFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple test files
	files := []struct {
		name    string
		content string
		valid   bool
	}{
		{"valid1.py", "x = 1", true},
		{"valid2.py", "y = 2\nz = x + y", true},
		{"invalid.py", "x = +", false},
	}

	var filenames []string
	for _, f := range files {
		path := filepath.Join(tmpDir, f.name)
		err := os.WriteFile(path, []byte(f.content), 0644)
		if err != nil {
			t.Fatalf("Failed to write temp file: %v", err)
		}
		filenames = append(filenames, path)
	}

	result, err := LintFiles(filenames)
	if err != nil {
		t.Fatalf("LintFiles failed: %v", err)
	}

	if result.FilesChecked != 3 {
		t.Errorf("Expected FilesChecked=3, got %d", result.FilesChecked)
	}

	if !result.HasErrors {
		t.Error("Expected HasErrors to be true (at least one invalid file)")
	}

	// Should have errors from invalid.py
	if len(result.Errors) == 0 {
		t.Error("Expected at least one error")
	}
}

func TestLintFiles_CorrectFileAttribution(t *testing.T) {
	// This test verifies that when linting multiple files with errors,
	// each error is correctly attributed to the right source file.
	tmpDir := t.TempDir()

	// Create files with errors on different lines
	file1 := filepath.Join(tmpDir, "first.py")
	file2 := filepath.Join(tmpDir, "second.py")
	file3 := filepath.Join(tmpDir, "third.py")

	// first.py has error on line 1
	err := os.WriteFile(file1, []byte("x = +"), 0644)
	if err != nil {
		t.Fatalf("Failed to write file1: %v", err)
	}

	// second.py is valid
	err = os.WriteFile(file2, []byte("x = 1\ny = 2"), 0644)
	if err != nil {
		t.Fatalf("Failed to write file2: %v", err)
	}

	// third.py has error on line 3
	err = os.WriteFile(file3, []byte("a = 1\nb = 2\nc = +"), 0644)
	if err != nil {
		t.Fatalf("Failed to write file3: %v", err)
	}

	result, err := LintFiles([]string{file1, file2, file3})
	if err != nil {
		t.Fatalf("LintFiles failed: %v", err)
	}

	if result.FilesChecked != 3 {
		t.Errorf("Expected FilesChecked=3, got %d", result.FilesChecked)
	}

	// Should have 2 errors total (one from first.py, one from third.py)
	if len(result.Errors) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(result.Errors))
	}

	// Verify each error is attributed to the correct file
	for _, lintErr := range result.Errors {
		switch lintErr.File {
		case file1:
			// Error from first.py should be on line 1
			if lintErr.Line != 1 {
				t.Errorf("Error from first.py expected on line 1, got line %d", lintErr.Line)
			}
		case file3:
			// Error from third.py should be on line 3
			if lintErr.Line != 3 {
				t.Errorf("Error from third.py expected on line 3, got line %d", lintErr.Line)
			}
		default:
			t.Errorf("Unexpected error from file: %s", lintErr.File)
		}
	}

	// Verify no errors are attributed to second.py (the valid file)
	for _, lintErr := range result.Errors {
		if lintErr.File == file2 {
			t.Error("Unexpected error attributed to valid file second.py")
		}
	}
}

func TestLintFiles_ErrorSorting(t *testing.T) {
	// Test that errors are sorted by file, then line number
	tmpDir := t.TempDir()

	files := []struct {
		name    string
		content string
	}{
		{"zebra.py", "line1 = 1\nline2 = +"},
		{"alpha.py", "line1 = +"},
		{"beta.py", "line1 = 1\nline2 = 2\nline3 = +"},
	}

	var filenames []string
	for _, f := range files {
		path := filepath.Join(tmpDir, f.name)
		err := os.WriteFile(path, []byte(f.content), 0644)
		if err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
		filenames = append(filenames, path)
	}

	result, err := LintFiles(filenames)
	if err != nil {
		t.Fatalf("LintFiles failed: %v", err)
	}

	// Errors should be sorted by filename, then line
	// alpha.py comes before beta.py comes before zebra.py
	expectedOrder := []string{"alpha.py", "beta.py", "zebra.py"}

	for i, expected := range expectedOrder {
		if i >= len(result.Errors) {
			t.Fatalf("Not enough errors, expected at least %d", len(expectedOrder))
		}
		actual := filepath.Base(result.Errors[i].File)
		if actual != expected {
			t.Errorf("Error %d: expected file %s, got %s", i, expected, actual)
		}
	}
}

func TestLintFiles_AllValid(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple valid files
	for i := range 3 {
		path := filepath.Join(tmpDir, "valid"+string(rune('0'+i))+".py")
		err := os.WriteFile(path, []byte("x = 1"), 0644)
		if err != nil {
			t.Fatalf("Failed to write temp file: %v", err)
		}
	}

	// Test with all valid files
	filenames := []string{
		filepath.Join(tmpDir, "valid0.py"),
		filepath.Join(tmpDir, "valid1.py"),
		filepath.Join(tmpDir, "valid2.py"),
	}

	result, err := LintFiles(filenames)
	if err != nil {
		t.Fatalf("LintFiles failed: %v", err)
	}

	if result.HasIssues() {
		t.Errorf("Expected no issues, got: %v", result.Errors)
	}

	if result.FilesChecked != 3 {
		t.Errorf("Expected FilesChecked=3, got %d", result.FilesChecked)
	}
}

func TestLintFiles_MixedValidAndMissing(t *testing.T) {
	tmpDir := t.TempDir()

	// Create one valid file
	validPath := filepath.Join(tmpDir, "valid.py")
	err := os.WriteFile(validPath, []byte("x = 1"), 0644)
	if err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	// Try to lint valid file and non-existent file
	filenames := []string{
		validPath,
		filepath.Join(tmpDir, "nonexistent.py"),
	}

	result, err := LintFiles(filenames)

	// Should get partial results with read error
	if err == nil {
		t.Error("Expected error due to missing file")
	}

	if result == nil {
		t.Fatal("Expected partial result even with read error")
	}

	// Should have checked at least the valid file
	if result.FilesChecked < 1 {
		t.Errorf("Expected at least 1 file checked, got %d", result.FilesChecked)
	}
}

func TestLintError_String(t *testing.T) {
	tests := []struct {
		name     string
		err      LintError
		expected string
	}{
		{
			name: "with file and line",
			err: LintError{
				File:     "test.py",
				Line:     10,
				Message:  "syntax error",
				Severity: SeverityError,
			},
			expected: "test.py:10: syntax error (error)",
		},
		{
			name: "with file, line, and column",
			err: LintError{
				File:     "test.py",
				Line:     10,
				Column:   5,
				Message:  "syntax error",
				Severity: SeverityError,
			},
			expected: "test.py:10:5: syntax error (error)",
		},
		{
			name: "line only",
			err: LintError{
				Line:     5,
				Message:  "warning message",
				Severity: SeverityWarning,
			},
			expected: "5: warning message (warning)",
		},
		{
			name: "message only",
			err: LintError{
				Message:  "info message",
				Severity: SeverityInfo,
			},
			expected: "info message (info)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.String()
			if got != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestResult_String(t *testing.T) {
	t.Run("no issues", func(t *testing.T) {
		result := &Result{Errors: []LintError{}}
		if result.String() != "No issues found" {
			t.Errorf("Expected 'No issues found', got %q", result.String())
		}
	})

	t.Run("with issues", func(t *testing.T) {
		result := &Result{
			Errors: []LintError{
				{File: "test.py", Line: 1, Message: "error1", Severity: SeverityError},
				{File: "test.py", Line: 5, Message: "error2", Severity: SeverityWarning},
			},
		}
		got := result.String()
		if !strings.Contains(got, "error1") {
			t.Errorf("Expected output to contain 'error1', got %q", got)
		}
		if !strings.Contains(got, "error2") {
			t.Errorf("Expected output to contain 'error2', got %q", got)
		}
	})
}

func TestResult_HasIssues(t *testing.T) {
	t.Run("no issues", func(t *testing.T) {
		result := &Result{Errors: []LintError{}}
		if result.HasIssues() {
			t.Error("Expected HasIssues to be false")
		}
	})

	t.Run("with issues", func(t *testing.T) {
		result := &Result{
			Errors: []LintError{
				{Message: "error", Severity: SeverityError},
			},
		}
		if !result.HasIssues() {
			t.Error("Expected HasIssues to be true")
		}
	})
}

func TestParseParserError(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedLine  int
		expectedMsg   string
	}{
		{
			name:          "with line number",
			input:         "line 5: expected token COLON",
			expectedLine:  5,
			expectedMsg:   "expected token COLON",
		},
		{
			name:          "without line number",
			input:         "unexpected token",
			expectedLine:  1, // defaults to 1
			expectedMsg:   "unexpected token",
		},
		{
			name:          "line 1",
			input:         "line 1: some error",
			expectedLine:  1,
			expectedMsg:   "some error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line, _, msg := parseParserError(tt.input)
			if line != tt.expectedLine {
				t.Errorf("Expected line %d, got %d", tt.expectedLine, line)
			}
			if msg != tt.expectedMsg {
				t.Errorf("Expected message %q, got %q", tt.expectedMsg, msg)
			}
		})
	}
}
