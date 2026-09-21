package repl

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"aethium/pkg/engine"
	"aethium/pkg/lexer"
	"aethium/pkg/parser"
	"aethium/pkg/vm"
)

const (
	PromptPrimary   = "aethium>>> "
	PromptSecondary = "....... "
)

func Banner() string {
	return fmt.Sprintf("Aethium %s (Fast Python-like runtime in Go)\nType \"exit()\", \"quit()\", or Ctrl+D to exit.\n", engine.Version)
}

const (
	ColorReset  = "\033[0m"
	ColorCyan   = "\033[36m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorRed    = "\033[31m"
	ColorBlue   = "\033[34m"
	ColorGray   = "\033[90m"
)

func Start(in io.Reader, out io.Writer) {
	fmt.Fprint(out, ColorCyan+Banner()+ColorReset)
	scanner := bufio.NewScanner(in)
	eng := engine.NewEngine()

	var multilineBuffer []string
	inBlock := false

	for {
		if inBlock {
			fmt.Fprint(out, ColorGray+PromptSecondary+ColorReset)
		} else {
			fmt.Fprint(out, ColorGreen+PromptPrimary+ColorReset)
		}

		scanned := scanner.Scan()
		if !scanned {
			break
		}

		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if !inBlock && (trimmed == "exit()" || trimmed == "quit()") {
			break
		}

		if !inBlock {
			if trimmed == "" {
				continue
			}
			if isIncomplete(line) {
				inBlock = true
				multilineBuffer = append(multilineBuffer, line)
				continue
			}
			executeLine(eng, line, out)
		} else {
			if line == "" {
				inBlock = false
				fullSource := strings.Join(multilineBuffer, "\n")
				multilineBuffer = []string{}
				executeLine(eng, fullSource, out)
			} else {
				multilineBuffer = append(multilineBuffer, line)
				fullSource := strings.Join(multilineBuffer, "\n")
				if !isIncomplete(fullSource) && !strings.HasSuffix(trimmed, ":") {
					inBlock = false
					multilineBuffer = []string{}
					executeLine(eng, fullSource, out)
				}
			}
		}
	}
	fmt.Fprintln(out, "\nGoodbye!")
}

func isIncomplete(src string) bool {
	trimmed := strings.TrimSpace(src)
	if strings.HasSuffix(trimmed, ":") {
		return true
	}

	var paren, bracket, brace int
	inSingle := false
	inDouble := false
	escaped := false

	for i := 0; i < len(src); i++ {
		ch := src[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}
		if ch == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}
		if inSingle || inDouble {
			continue
		}
		switch ch {
		case '(':
			paren++
		case ')':
			paren--
		case '[':
			bracket++
		case ']':
			bracket--
		case '{':
			brace++
		case '}':
			brace--
		}
	}

	if paren > 0 || bracket > 0 || brace > 0 || inSingle || inDouble {
		return true
	}

	l := lexer.New("<repl>", src)
	p := parser.New(l)
	p.ParseProgram()
	for _, err := range p.Errors() {
		if strings.Contains(strings.ToLower(err), "expected") ||
			strings.Contains(strings.ToLower(err), "eof") ||
			strings.Contains(strings.ToLower(err), "indent") {
			return true
		}
	}

	return false
}

func executeLine(eng *engine.Engine, source string, out io.Writer) {
	val, err := eng.Execute("<repl>", source)
	if err != nil {
		fmt.Fprintf(out, "%s%v%s\n", ColorRed, err, ColorReset)
		return
	}
	if val != nil && val != vm.None {
		fmt.Fprintf(out, "%s%s%s\n", ColorYellow, val.Inspect(), ColorReset)
	}
}
