// Package lint provides code analysis functionality for Scriptling scripts.
// It can detect syntax errors and potential issues without executing the code.
package lint

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/paularlott/scriptling/lexer"
	"github.com/paularlott/scriptling/parser"
)

// Severity represents the severity level of a lint error.
type Severity string

const (
	// SeverityError indicates a syntax error that prevents parsing.
	SeverityError Severity = "error"
	// SeverityWarning indicates a potential issue that doesn't prevent parsing.
	SeverityWarning Severity = "warning"
	// SeverityInfo indicates informational messages.
	SeverityInfo Severity = "info"
)

// LintError represents a single lint finding.
type LintError struct {
	// File is the source file name (may be empty for inline code).
	File string `json:"file,omitempty"`
	// Line is the 1-based line number where the issue was found.
	Line int `json:"line"`
	// Column is the 1-based column number (if available, 0 if not).
	Column int `json:"column,omitempty"`
	// Message is a human-readable description of the issue.
	Message string `json:"message"`
	// Severity indicates how serious the issue is.
	Severity Severity `json:"severity"`
	// Code is an optional error code for programmatic handling.
	Code string `json:"code,omitempty"`
}

// String returns a formatted string representation of the lint error.
func (e LintError) String() string {
	var location strings.Builder
	if e.File != "" {
		location.WriteString(e.File)
	}
	if e.Line > 0 {
		if location.Len() > 0 {
			location.WriteString(":")
		}
		fmt.Fprintf(&location, "%d", e.Line)
		if e.Column > 0 {
			fmt.Fprintf(&location, ":%d", e.Column)
		}
	}

	if location.Len() > 0 {
		return fmt.Sprintf("%s: %s (%s)", location.String(), e.Message, e.Severity)
	}
	return fmt.Sprintf("%s (%s)", e.Message, e.Severity)
}

// Result contains the results of linting one or more files.
type Result struct {
	// Errors is the list of lint errors found.
	Errors []LintError `json:"errors"`
	// FilesChecked is the number of files that were linted.
	FilesChecked int `json:"files_checked"`
	// HasErrors is true if any errors with severity "error" were found.
	HasErrors bool `json:"has_errors"`
}

// HasIssues returns true if any lint errors were found.
func (r Result) HasIssues() bool {
	return len(r.Errors) > 0
}

// String returns a formatted summary of all lint errors.
func (r Result) String() string {
	if len(r.Errors) == 0 {
		return "No issues found"
	}

	var lines []string
	for _, err := range r.Errors {
		lines = append(lines, err.String())
	}
	return strings.Join(lines, "\n")
}

// Options configures the behavior of the linter.
type Options struct {
	// Filename is used for error messages when linting inline code.
	Filename string
}

// Lint analyzes Scriptling source code and returns any issues found.
// The code is parsed but not executed, making this safe for untrusted input.
//
// Parameters:
//   - source: The Scriptling source code to analyze
//   - opts: Optional configuration (can be nil for defaults)
//
// Returns a Result containing any issues found.
//
// Example:
//
//	result := lint.Lint(`x = 1 + `, nil)
//	if result.HasIssues() {
//	    fmt.Println(result.String())
//	}
func Lint(source string, opts *Options) *Result {
	result := &Result{
		Errors:      []LintError{},
		FilesChecked: 1,
	}

	filename := ""
	if opts != nil {
		filename = opts.Filename
	}

	// Parse the source code
	l := lexer.New(source)
	p := parser.New(l)
	_ = p.ParseProgram()

	// Collect parser errors
	for _, errMsg := range p.Errors() {
		line, col, message := parseParserError(errMsg)
		result.Errors = append(result.Errors, LintError{
			File:     filename,
			Line:     line,
			Column:   col,
			Message:  message,
			Severity: SeverityError,
			Code:     "parse-error",
		})
	}

	// Sort errors by line number
	sort.Slice(result.Errors, func(i, j int) bool {
		if result.Errors[i].Line != result.Errors[j].Line {
			return result.Errors[i].Line < result.Errors[j].Line
		}
		return result.Errors[i].Column < result.Errors[j].Column
	})

	// Check if any errors are actual errors (not warnings)
	result.HasErrors = false
	for _, err := range result.Errors {
		if err.Severity == SeverityError {
			result.HasErrors = true
			break
		}
	}

	return result
}

// LintFile reads and analyzes a Scriptling file.
// This is a convenience wrapper around Lint that reads the file first.
//
// Parameters:
//   - filename: Path to the Scriptling file to analyze
//
// Returns a Result containing any issues found, or an error if the file
// cannot be read.
//
// Example:
//
//	result, err := lint.LintFile("script.py")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	if result.HasErrors {
//	    fmt.Println(result.String())
//	    os.Exit(1)
//	}
func LintFile(filename string) (*Result, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filename, err)
	}

	result := Lint(string(content), &Options{Filename: filename})
	return result, nil
}

// LintFiles analyzes multiple Scriptling files.
// All files are linted even if some have errors.
//
// Parameters:
//   - filenames: Paths to the Scriptling files to analyze
//
// Returns a combined Result containing all issues found.
//
// Example:
//
//	result, err := lint.LintFiles([]string{"a.py", "b.py"})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Checked %d files, found %d issues\n",
//	    result.FilesChecked, len(result.Errors))
func LintFiles(filenames []string) (*Result, error) {
	result := &Result{
		Errors:      []LintError{},
		FilesChecked: 0,
		HasErrors:   false,
	}

	var readErrors []error

	for _, filename := range filenames {
		fileResult, err := LintFile(filename)
		if err != nil {
			readErrors = append(readErrors, err)
			continue
		}

		result.Errors = append(result.Errors, fileResult.Errors...)
		result.FilesChecked++
		if fileResult.HasErrors {
			result.HasErrors = true
		}
	}

	// Sort all errors by file, then line
	sort.Slice(result.Errors, func(i, j int) bool {
		if result.Errors[i].File != result.Errors[j].File {
			return result.Errors[i].File < result.Errors[j].File
		}
		if result.Errors[i].Line != result.Errors[j].Line {
			return result.Errors[i].Line < result.Errors[j].Line
		}
		return result.Errors[i].Column < result.Errors[j].Column
	})

	if len(readErrors) > 0 {
		// Return partial results along with the read errors
		return result, fmt.Errorf("failed to read %d file(s): %v", len(readErrors), readErrors)
	}

	return result, nil
}

// parseParserError extracts line number, column, and message from a parser error string.
// Parser errors are formatted as: "line N: message" or just "message"
func parseParserError(errMsg string) (line int, column int, message string) {
	line = 1
	column = 0

	// Try to match "line N: message" format
	linePattern := regexp.MustCompile(`^line (\d+):\s*(.+)$`)
	if matches := linePattern.FindStringSubmatch(errMsg); len(matches) == 3 {
		fmt.Sscanf(matches[1], "%d", &line)
		message = matches[2]
		return line, column, message
	}

	// Fallback: use the entire message
	message = errMsg
	return line, column, message
}
