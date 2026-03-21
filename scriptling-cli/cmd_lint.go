package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/paularlott/cli"
	"github.com/paularlott/scriptling/lint"
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
		return outputLintResult(result, format)
	}

	if !isStdinEmpty() {
		content, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
		result := lint.Lint(string(content), &lint.Options{Filename: "stdin"})
		return outputLintResult(result, format)
	}

	cmd.ShowHelp()
	return nil
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
		os.Exit(1)
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
