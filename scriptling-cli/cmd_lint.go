package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/paularlott/cli"
	"github.com/paularlott/scriptling/lint"
	"github.com/paularlott/scriptling/metadata"
)

func runLint(cmd *cli.Command) error {
	format := cmd.GetString("lint-format")
	if format != "text" && format != "json" {
		return fmt.Errorf("invalid value for --lint-format: %s (must be 'text' or 'json')", format)
	}

	file := cmd.GetStringArg("file")

	if file != "" {
		result, err := lint.LintFile(file)
		if err != nil {
			return err
		}
		if data, err := os.ReadFile(file); err == nil {
			lintMetadata(result, file, data)
		}
		return outputLintResult(result, format)
	}

	if !isStdinEmpty() {
		content, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
		result := lint.Lint(string(content), &lint.Options{Filename: "stdin"})
		lintMetadata(result, "stdin", content)
		return outputLintResult(result, format)
	}

	cmd.ShowHelp()
	return nil
}

// lintMetadata reports a malformed script metadata block as a lint error: a
// block that cannot be parsed would abort the run at the requirements
// pre-flight, so lint should say so too. A block that parses is trusted;
// whether its requirements are met is an environment question, not a
// source question.
func lintMetadata(result *lint.Result, filename string, source []byte) {
	if _, _, err := metadata.Parse(source); err != nil {
		result.Errors = append(result.Errors, lint.LintError{
			File:     filename,
			Line:     1,
			Message:  err.Error(),
			Severity: lint.SeverityError,
			Code:     "metadata",
		})
		result.HasErrors = true
	}
}

func outputLintResult(result *lint.Result, format string) error {
	if format == "json" {
		output, err := formatLintJSON(result)
		if err != nil {
			return fmt.Errorf("failed to format JSON output: %w", err)
		}
		fmt.Println(output)
	} else {
		if result.HasIssues() {
			fmt.Println(result.String())
		} else {
			fmt.Println("No issues found")
		}
	}
	if result.HasErrors {
		return exitCodeError{code: 1}
	}
	return nil
}

func formatLintJSON(result *lint.Result) (string, error) {
	type lintError struct {
		File     string `json:"file,omitempty"`
		Line     int    `json:"line"`
		Column   int    `json:"column,omitempty"`
		Message  string `json:"message"`
		Severity string `json:"severity"`
		Code     string `json:"code,omitempty"`
	}
	type lintOutput struct {
		FilesChecked int         `json:"files_checked"`
		HasErrors    bool        `json:"has_errors"`
		Errors       []lintError `json:"errors"`
	}

	out := lintOutput{
		FilesChecked: result.FilesChecked,
		HasErrors:    result.HasErrors,
		Errors:       make([]lintError, 0, len(result.Errors)),
	}
	for _, e := range result.Errors {
		out.Errors = append(out.Errors, lintError{
			File:     e.File,
			Line:     e.Line,
			Column:   e.Column,
			Message:  e.Message,
			Severity: string(e.Severity),
			Code:     e.Code,
		})
	}

	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}
