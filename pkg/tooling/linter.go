package tooling

import (
	"bufio"
	"fmt"
	"strings"
)

type LintDiagnostic struct {
	File     string
	Line     int
	Column   int
	Rule     string
	Message  string
	Severity string
}

func LintSource(filename string, src string) []LintDiagnostic {
	var diags []LintDiagnostic
	scanner := bufio.NewScanner(strings.NewReader(src))
	lineNo := 1

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasSuffix(line, " ") || strings.HasSuffix(line, "\t") {
			diags = append(diags, LintDiagnostic{
				File:     filename,
				Line:     lineNo,
				Column:   len(line),
				Rule:     "W291",
				Message:  "trailing whitespace",
				Severity: "warning",
			})
		}

		if len(line) > 120 {
			diags = append(diags, LintDiagnostic{
				File:     filename,
				Line:     lineNo,
				Column:   120,
				Rule:     "E501",
				Message:  fmt.Sprintf("line too long (%d > 120 characters)", len(line)),
				Severity: "warning",
			})
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "except:" {
			diags = append(diags, LintDiagnostic{
				File:     filename,
				Line:     lineNo,
				Column:   1,
				Rule:     "E722",
				Message:  "do not use bare 'except', specify exception class or variable",
				Severity: "warning",
			})
		}

		if strings.Contains(line, "== None") {
			diags = append(diags, LintDiagnostic{
				File:     filename,
				Line:     lineNo,
				Column:   strings.Index(line, "== None") + 1,
				Rule:     "E711",
				Message:  "comparison to None should be 'if cond is None:'",
				Severity: "warning",
			})
		}

		if strings.Contains(line, "== True") || strings.Contains(line, "== False") {
			diags = append(diags, LintDiagnostic{
				File:     filename,
				Line:     lineNo,
				Column:   1,
				Rule:     "E712",
				Message:  "comparison to True/False should be 'if cond:' or 'if not cond:'",
				Severity: "warning",
			})
		}

		lineNo++
	}

	return diags
}

func FormatSource(src string) string {
	lines := strings.Split(src, "\n")
	var formatted []string

	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t\r")
		formatted = append(formatted, trimmed)
	}

	result := strings.Join(formatted, "\n")
	if !strings.HasSuffix(result, "\n") && len(result) > 0 {
		result += "\n"
	}
	return result
}
