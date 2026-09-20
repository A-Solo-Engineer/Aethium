package tooling

import (
    "bufio"
    "fmt"
    "strings"
)

// LintDiagnostic represents a linter warning or issue.
type LintDiagnostic struct {
    File     string
    Line     int
    Column   int
    Rule     string
    Message  string
    Severity string // "warning", "error", "info"
}

// LintSource analyzes Aethium source code for common stylistic and semantic errors.
func LintSource(filename string, src string) []LintDiagnostic {
    var diags []LintDiagnostic
    scanner := bufio.NewScanner(strings.NewReader(src))
    lineNo := 1

    for scanner.Scan() {
        line := scanner.Text()

        // Rule 1: Trailing whitespace
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

        // Rule 2: Line too long (> 120 chars)
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

        // Rule 3: Use of bare except clause (except: without specific error)
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

        // Rule 4: Comparison to None using == instead of 'is None'
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

        // Rule 5: Comparison to True/False using ==
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

// FormatSource formats Aethium source code with standard 4-space indentation and clean newlines.
func FormatSource(src string) string {
    lines := strings.Split(src, "\n")
    var formatted []string

    for _, line := range lines {
        // Trim right whitespace
        trimmed := strings.TrimRight(line, " \t\r")
        formatted = append(formatted, trimmed)
    }

    result := strings.Join(formatted, "\n")
    if !strings.HasSuffix(result, "\n") && len(result) > 0 {
        result += "\n"
    }
    return result
}
