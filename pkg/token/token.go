package token

import "fmt"

type TokenType string

type Pos struct {
	Line int
	Col  int
	File string
}

func (p Pos) String() string {
	if p.File != "" {
		return fmt.Sprintf("%s:%d:%d", p.File, p.Line, p.Col)
	}
	return fmt.Sprintf("%d:%d", p.Line, p.Col)
}

type Token struct {
	Type    TokenType
	Literal string
	Pos     Pos
}

func (t Token) String() string {
	return fmt.Sprintf("Token(%s, %q, %s)", t.Type, t.Literal, t.Pos)
}

const (
	ILLEGAL = "ILLEGAL"
	EOF     = "EOF"
	NEWLINE = "NEWLINE"
	INDENT  = "INDENT"
	DEDENT  = "DEDENT"

	IDENT    = "IDENT"
	INT      = "INT"
	FLOAT    = "FLOAT"
	COMPLEX  = "COMPLEX"
	STRING   = "STRING"
	BYTES    = "BYTES"
	FSTRING  = "FSTRING" 
	BOOL     = "BOOL"
	NONE     = "NONE"
	ELLIPSIS = "..."

	PLUS      = "+"
	MINUS     = "-"
	ASTERISK  = "*"
	SLASH     = "/"
	FLOORDIV  = "//"
	PERCENT   = "%"
	POW       = "**"
	AMPERSAND = "&"
	PIPE      = "|"
	CARET     = "^"
	TILDE     = "~"
	LSHIFT    = "<<"
	RSHIFT    = ">>"

	ASSIGN          = "="
	PLUS_ASSIGN     = "+="
	MINUS_ASSIGN    = "-="
	MUL_ASSIGN      = "*="
	DIV_ASSIGN      = "/="
	FLOORDIV_ASSIGN = "//="
	MOD_ASSIGN      = "%="
	POW_ASSIGN      = "**="
	AND_ASSIGN      = "&="
	OR_ASSIGN       = "|="
	XOR_ASSIGN      = "^="
	LSHIFT_ASSIGN   = "<<="
	RSHIFT_ASSIGN   = ">>="
	WALRUS          = ":="

	EQ     = "=="
	NOT_EQ = "!="
	LT     = "<"
	GT     = ">"
	LT_EQ  = "<="
	GT_EQ  = ">="

	COMMA     = ","
	COLON     = ":"
	SEMICOLON = ";"
	DOT       = "."
	ARROW     = "->"
	AT        = "@"

	LPAREN   = "("
	RPAREN   = ")"
	LBRACE   = "{"
	RBRACE   = "}"
	LBRACKET = "["
	RBRACKET = "]"

	DEF      = "def"
	CLASS    = "class"
	RETURN   = "return"
	IF       = "if"
	ELIF     = "elif"
	ELSE     = "else"
	WHILE    = "while"
	FOR      = "for"
	IN       = "in"
	NOT_IN   = "not in"
	BREAK    = "break"
	CONTINUE = "continue"
	PASS     = "pass"
	IMPORT   = "import"
	FROM     = "from"
	AS       = "as"
	LAMBDA   = "lambda"
	TRY      = "try"
	EXCEPT   = "except"
	FINALLY  = "finally"
	RAISE    = "raise"
	AND      = "and"
	OR       = "or"
	NOT      = "not"
	IS       = "is"
	IS_NOT   = "is not"
	ASSERT   = "assert"
	GLOBAL   = "global"
	NONLOCAL = "nonlocal"
	WITH     = "with"
	YIELD    = "yield"
	DEL      = "del"
	MATCH    = "match"
	CASE     = "case"
	ASYNC    = "async"
	AWAIT    = "await"

	
	GO    = "go"
	SPAWN = "spawn"
	CHAN  = "chan"
)

var keywords = map[string]TokenType{
	"def":      DEF,
	"class":    CLASS,
	"return":   RETURN,
	"if":       IF,
	"elif":     ELIF,
	"else":     ELSE,
	"while":    WHILE,
	"for":      FOR,
	"in":       IN,
	"break":    BREAK,
	"continue": CONTINUE,
	"pass":     PASS,
	"import":   IMPORT,
	"from":     FROM,
	"as":       AS,
	"lambda":   LAMBDA,
	"try":      TRY,
	"except":   EXCEPT,
	"finally":  FINALLY,
	"raise":    RAISE,
	"and":      AND,
	"or":       OR,
	"not":      NOT,
	"is":       IS,
	"assert":   ASSERT,
	"global":   GLOBAL,
	"nonlocal": NONLOCAL,
	"with":     WITH,
	"yield":    YIELD,
	"del":      DEL,
	"match":    MATCH,
	"case":     CASE,
	"async":    ASYNC,
	"await":    AWAIT,
	"True":     BOOL,
	"False":    BOOL,
	"None":     NONE,
	"go":       GO,
	"spawn":    SPAWN,
	"chan":     CHAN,
}

func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}
