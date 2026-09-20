package parser

import (
	"testing"

	"aethium/pkg/ast"
	"aethium/pkg/lexer"
)

func TestParseFunctionAndIf(t *testing.T) {
	input := `
def fib(n):
    if n <= 1:
        return n
    return fib(n - 1) + fib(n - 2)

x = fib(10)
`
	l := lexer.New("test_fib.py", input)
	p := New(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		for _, err := range p.Errors() {
			t.Errorf("parser error: %s", err)
		}
		t.FailNow()
	}

	if len(prog.Statements) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(prog.Statements))
	}

	fnDef, ok := prog.Statements[0].(*ast.FunctionDef)
	if !ok {
		t.Fatalf("stmt[0] not FunctionDef, got %T", prog.Statements[0])
	}
	if fnDef.Name != "fib" {
		t.Errorf("fnDef.Name != fib, got %q", fnDef.Name)
	}
	if len(fnDef.Parameters) != 1 || fnDef.Parameters[0].Name != "n" {
		t.Errorf("fnDef params mismatch, got %+v", fnDef.Parameters)
	}
}

func TestParseComprehensionsAndOOP(t *testing.T) {
	input := `
class Person:
    def __init__(self, name, age=0):
        self.name = name
        self.age = age

    def greet(self):
        return f"Hello, I am {self.name}"

squares = [x * x for x in range(10) if x % 2 == 0]
lookup = {x: str(x) for x in squares}
`
	l := lexer.New("test_oop.py", input)
	p := New(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		for _, err := range p.Errors() {
			t.Errorf("parser error: %s", err)
		}
		t.FailNow()
	}

	if len(prog.Statements) != 3 {
		t.Fatalf("expected 3 statements, got %d", len(prog.Statements))
	}
}
