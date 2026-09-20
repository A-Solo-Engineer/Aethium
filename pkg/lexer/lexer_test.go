package lexer

import (
	"testing"

	"aethium/pkg/token"
)

func TestLexerBasic(t *testing.T) {
	input := `
def add(a, b):
    return a + b

x = 10
y = 20.5
if x < y:
    print(f"Result: {add(x, int(y))}")
`
	l := New("test.py", input)

	expected := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.DEF, "def"},
		{token.IDENT, "add"},
		{token.LPAREN, "("},
		{token.IDENT, "a"},
		{token.COMMA, ","},
		{token.IDENT, "b"},
		{token.RPAREN, ")"},
		{token.COLON, ":"},
		{token.NEWLINE, "\n"},
		{token.INDENT, ""},
		{token.RETURN, "return"},
		{token.IDENT, "a"},
		{token.PLUS, "+"},
		{token.IDENT, "b"},
		{token.NEWLINE, "\n"},
		{token.DEDENT, ""},
		{token.IDENT, "x"},
		{token.ASSIGN, "="},
		{token.INT, "10"},
		{token.NEWLINE, "\n"},
		{token.IDENT, "y"},
		{token.ASSIGN, "="},
		{token.FLOAT, "20.5"},
		{token.NEWLINE, "\n"},
		{token.IF, "if"},
		{token.IDENT, "x"},
		{token.LT, "<"},
		{token.IDENT, "y"},
		{token.COLON, ":"},
		{token.NEWLINE, "\n"},
		{token.INDENT, ""},
		{token.IDENT, "print"},
		{token.LPAREN, "("},
		{token.FSTRING, "Result: {add(x, int(y))}"},
		{token.RPAREN, ")"},
		{token.NEWLINE, "\n"},
		{token.DEDENT, ""},
		{token.EOF, ""},
	}

	for i, tt := range expected {
		tok := l.NextToken()
		if tok.Type != tt.expectedType {
			t.Fatalf("test[%d] - tokentype wrong. expected=%q, got=%q (%s)",
				i, tt.expectedType, tok.Type, tok.Literal)
		}
	}
}
