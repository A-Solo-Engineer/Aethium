package lexer

import (
    "fmt"
    "strconv"
    "strings"
    "unicode"

    "aethium/pkg/token"
)

type Lexer struct {
    filename     string
    input        string
    position     int  // current position in input (points to current char)
    readPosition int  // current reading position in input (after current char)
    ch           byte // current char under examination
    line         int
    col          int

    indentStack      []int
    bracketDepth     int
    atLineStart      bool
    seenTokenOnLine  bool
    pendingTokens    []token.Token
    eofEmitted       bool
}

func New(filename, input string) *Lexer {
    l := &Lexer{
        filename:        filename,
        input:           input,
        line:            1,
        col:             0,
        indentStack:     []int{0},
        atLineStart:     true,
        seenTokenOnLine: false,
    }
    l.readChar()
    return l
}

func (l *Lexer) readChar() {
    if l.readPosition >= len(l.input) {
        l.ch = 0
    } else {
        l.ch = l.input[l.readPosition]
    }
    l.position = l.readPosition
    l.readPosition++
    l.col++
}

func (l *Lexer) peekChar() byte {
    if l.readPosition >= len(l.input) {
        return 0
    }
    return l.input[l.readPosition]
}

func (l *Lexer) peekCharN(n int) byte {
    pos := l.position + n
    if pos >= len(l.input) || pos < 0 {
        return 0
    }
    return l.input[pos]
}

func (l *Lexer) currentPos() token.Pos {
    return token.Pos{
        Line: l.line,
        Col:  l.col,
        File: l.filename,
    }
}

func (l *Lexer) NextToken() token.Token {
    if len(l.pendingTokens) > 0 {
        tok := l.pendingTokens[0]
        l.pendingTokens = l.pendingTokens[1:]
        if tok.Type != token.NEWLINE && tok.Type != token.DEDENT && tok.Type != token.INDENT && tok.Type != token.EOF {
            l.seenTokenOnLine = true
        }
        return tok
    }

    for {
        if l.atLineStart {
            if l.handleLineStart() {
                if len(l.pendingTokens) > 0 {
                    tok := l.pendingTokens[0]
                    l.pendingTokens = l.pendingTokens[1:]
                    if tok.Type != token.NEWLINE && tok.Type != token.DEDENT && tok.Type != token.INDENT && tok.Type != token.EOF {
                        l.seenTokenOnLine = true
                    }
                    return tok
                }
                continue
            }
        }

        l.skipWhitespace()

        if l.ch == '\\' && (l.peekChar() == '\n' || (l.peekChar() == '\r' && l.peekCharN(2) == '\n')) {
            // Explicit line continuation
            l.readChar()
            if l.ch == '\r' {
                l.readChar()
            }
            l.readChar()
            l.line++
            l.col = 0
            continue
        }

        if l.ch == '#' {
            l.skipComment()
            continue
        }

        break
    }

    pos := l.currentPos()

    if l.ch == 0 {
        if !l.eofEmitted {
            if l.seenTokenOnLine {
                l.seenTokenOnLine = false
                l.pendingTokens = append(l.pendingTokens, token.Token{
                    Type:    token.NEWLINE,
                    Literal: "\n",
                    Pos:     pos,
                })
            }
            // Emit pending dedents
            for len(l.indentStack) > 1 {
                l.indentStack = l.indentStack[:len(l.indentStack)-1]
                l.pendingTokens = append(l.pendingTokens, token.Token{
                    Type:    token.DEDENT,
                    Literal: "",
                    Pos:     pos,
                })
            }
            l.eofEmitted = true
            if len(l.pendingTokens) > 0 {
                tok := l.pendingTokens[0]
                l.pendingTokens = l.pendingTokens[1:]
                return tok
            }
        }
        return token.Token{Type: token.EOF, Literal: "", Pos: pos}
    }

    if l.ch == '\n' || l.ch == '\r' {
        if l.ch == '\r' && l.peekChar() == '\n' {
            l.readChar()
        }
        l.line++
        l.col = 0
        l.readChar()
        if l.bracketDepth > 0 {
            // Inside parens/brackets/braces, newlines are ignored
            return l.NextToken()
        }
        l.atLineStart = true
        if !l.seenTokenOnLine {
            // Blank line: ignore without emitting NEWLINE
            return l.NextToken()
        }
        l.seenTokenOnLine = false
        return token.Token{Type: token.NEWLINE, Literal: "\n", Pos: pos}
    }

    l.seenTokenOnLine = true

    // Identifiers, keywords, f-strings, bytes, strings
    if l.ch == 'f' || l.ch == 'F' {
        next := l.peekChar()
        if next == '"' || next == '\'' {
            l.readChar() // consume 'f'
            return l.readString(true, false, pos)
        }
    }
    if l.ch == 'b' || l.ch == 'B' {
        next := l.peekChar()
        if next == '"' || next == '\'' {
            l.readChar() // consume 'b'
            return l.readString(false, true, pos)
        }
    }

    if isLetter(l.ch) {
        ident := l.readIdentifier()
        tokType := token.LookupIdent(ident)
        // Check for multi-word keywords like "not in", "is not"
        if tokType == token.NOT {
            savedPos := l.position
            savedReadPos := l.readPosition
            savedCol := l.col
            savedLine := l.line
            savedCh := l.ch
            l.skipWhitespace()
            if isLetter(l.ch) {
                nextIdent := l.readIdentifier()
                if nextIdent == "in" {
                    return token.Token{Type: token.NOT_IN, Literal: "not in", Pos: pos}
                }
            }
            // Backtrack
            l.position = savedPos
            l.readPosition = savedReadPos
            l.col = savedCol
            l.line = savedLine
            l.ch = savedCh
        } else if tokType == token.IS {
            savedPos := l.position
            savedReadPos := l.readPosition
            savedCol := l.col
            savedLine := l.line
            savedCh := l.ch
            l.skipWhitespace()
            if isLetter(l.ch) {
                nextIdent := l.readIdentifier()
                if nextIdent == "not" {
                    return token.Token{Type: token.IS_NOT, Literal: "is not", Pos: pos}
                }
            }
            // Backtrack
            l.position = savedPos
            l.readPosition = savedReadPos
            l.col = savedCol
            l.line = savedLine
            l.ch = savedCh
        }
        return token.Token{Type: tokType, Literal: ident, Pos: pos}
    }

    if isDigit(l.ch) {
        return l.readNumber(pos)
    }

    if l.ch == '"' || l.ch == '\'' {
        return l.readString(false, false, pos)
    }

    var tok token.Token
    switch l.ch {
    case '+':
        if l.peekChar() == '=' {
            l.readChar()
            tok = token.Token{Type: token.PLUS_ASSIGN, Literal: "+=", Pos: pos}
        } else {
            tok = token.Token{Type: token.PLUS, Literal: "+", Pos: pos}
        }
    case '-':
        if l.peekChar() == '=' {
            l.readChar()
            tok = token.Token{Type: token.MINUS_ASSIGN, Literal: "-=", Pos: pos}
        } else if l.peekChar() == '>' {
            l.readChar()
            tok = token.Token{Type: token.ARROW, Literal: "->", Pos: pos}
        } else {
            tok = token.Token{Type: token.MINUS, Literal: "-", Pos: pos}
        }
    case '*':
        if l.peekChar() == '*' {
            l.readChar()
            if l.peekChar() == '=' {
                l.readChar()
                tok = token.Token{Type: token.POW_ASSIGN, Literal: "**=", Pos: pos}
            } else {
                tok = token.Token{Type: token.POW, Literal: "**", Pos: pos}
            }
        } else if l.peekChar() == '=' {
            l.readChar()
            tok = token.Token{Type: token.MUL_ASSIGN, Literal: "*=", Pos: pos}
        } else {
            tok = token.Token{Type: token.ASTERISK, Literal: "*", Pos: pos}
        }
    case '/':
        if l.peekChar() == '/' {
            l.readChar()
            if l.peekChar() == '=' {
                l.readChar()
                tok = token.Token{Type: token.FLOORDIV_ASSIGN, Literal: "//=", Pos: pos}
            } else {
                tok = token.Token{Type: token.FLOORDIV, Literal: "//", Pos: pos}
            }
        } else if l.peekChar() == '=' {
            l.readChar()
            tok = token.Token{Type: token.DIV_ASSIGN, Literal: "/=", Pos: pos}
        } else {
            tok = token.Token{Type: token.SLASH, Literal: "/", Pos: pos}
        }
    case '%':
        if l.peekChar() == '=' {
            l.readChar()
            tok = token.Token{Type: token.MOD_ASSIGN, Literal: "%=", Pos: pos}
        } else {
            tok = token.Token{Type: token.PERCENT, Literal: "%", Pos: pos}
        }
    case '=':
        if l.peekChar() == '=' {
            l.readChar()
            tok = token.Token{Type: token.EQ, Literal: "==", Pos: pos}
        } else {
            tok = token.Token{Type: token.ASSIGN, Literal: "=", Pos: pos}
        }
    case '!':
        if l.peekChar() == '=' {
            l.readChar()
            tok = token.Token{Type: token.NOT_EQ, Literal: "!=", Pos: pos}
        } else {
            tok = token.Token{Type: token.ILLEGAL, Literal: string(l.ch), Pos: pos}
        }
    case '<':
        if l.peekChar() == '<' {
            l.readChar()
            if l.peekChar() == '=' {
                l.readChar()
                tok = token.Token{Type: token.LSHIFT_ASSIGN, Literal: "<<=", Pos: pos}
            } else {
                tok = token.Token{Type: token.LSHIFT, Literal: "<<", Pos: pos}
            }
        } else if l.peekChar() == '=' {
            l.readChar()
            tok = token.Token{Type: token.LT_EQ, Literal: "<=", Pos: pos}
        } else {
            tok = token.Token{Type: token.LT, Literal: "<", Pos: pos}
        }
    case '>':
        if l.peekChar() == '>' {
            l.readChar()
            if l.peekChar() == '=' {
                l.readChar()
                tok = token.Token{Type: token.RSHIFT_ASSIGN, Literal: ">>=", Pos: pos}
            } else {
                tok = token.Token{Type: token.RSHIFT, Literal: ">>", Pos: pos}
            }
        } else if l.peekChar() == '=' {
            l.readChar()
            tok = token.Token{Type: token.GT_EQ, Literal: ">=", Pos: pos}
        } else {
            tok = token.Token{Type: token.GT, Literal: ">", Pos: pos}
        }
    case '&':
        if l.peekChar() == '=' {
            l.readChar()
            tok = token.Token{Type: token.AND_ASSIGN, Literal: "&=", Pos: pos}
        } else {
            tok = token.Token{Type: token.AMPERSAND, Literal: "&", Pos: pos}
        }
    case '|':
        if l.peekChar() == '=' {
            l.readChar()
            tok = token.Token{Type: token.OR_ASSIGN, Literal: "|=", Pos: pos}
        } else {
            tok = token.Token{Type: token.PIPE, Literal: "|", Pos: pos}
        }
    case '^':
        if l.peekChar() == '=' {
            l.readChar()
            tok = token.Token{Type: token.XOR_ASSIGN, Literal: "^=", Pos: pos}
        } else {
            tok = token.Token{Type: token.CARET, Literal: "^", Pos: pos}
        }
    case '~':
        tok = token.Token{Type: token.TILDE, Literal: "~", Pos: pos}
    case ':':
        if l.peekChar() == '=' {
            l.readChar()
            tok = token.Token{Type: token.WALRUS, Literal: ":=", Pos: pos}
        } else {
            tok = token.Token{Type: token.COLON, Literal: ":", Pos: pos}
        }
    case ';':
        tok = token.Token{Type: token.SEMICOLON, Literal: ";", Pos: pos}
    case ',':
        tok = token.Token{Type: token.COMMA, Literal: ",", Pos: pos}
    case '.':
        if isDigit(l.peekChar()) {
            return l.readNumber(pos)
        }
        if l.peekChar() == '.' && l.peekCharN(2) == '.' {
            l.readChar()
            l.readChar()
            tok = token.Token{Type: token.ELLIPSIS, Literal: "...", Pos: pos}
        } else {
            tok = token.Token{Type: token.DOT, Literal: ".", Pos: pos}
        }
    case '@':
        tok = token.Token{Type: token.AT, Literal: "@", Pos: pos}
    case '(':
        l.bracketDepth++
        tok = token.Token{Type: token.LPAREN, Literal: "(", Pos: pos}
    case ')':
        if l.bracketDepth > 0 {
            l.bracketDepth--
        }
        tok = token.Token{Type: token.RPAREN, Literal: ")", Pos: pos}
    case '[':
        l.bracketDepth++
        tok = token.Token{Type: token.LBRACKET, Literal: "[", Pos: pos}
    case ']':
        if l.bracketDepth > 0 {
            l.bracketDepth--
        }
        tok = token.Token{Type: token.RBRACKET, Literal: "]", Pos: pos}
    case '{':
        l.bracketDepth++
        tok = token.Token{Type: token.LBRACE, Literal: "{", Pos: pos}
    case '}':
        if l.bracketDepth > 0 {
            l.bracketDepth--
        }
        tok = token.Token{Type: token.RBRACE, Literal: "}", Pos: pos}
    default:
        tok = token.Token{Type: token.ILLEGAL, Literal: string(l.ch), Pos: pos}
    }

    l.readChar()
    return tok
}

func (l *Lexer) handleLineStart() bool {
    l.atLineStart = false
    indent := 0

    // Count leading whitespace
    for l.ch == ' ' || l.ch == '\t' {
        if l.ch == ' ' {
            indent++
        } else if l.ch == '\t' {
            indent += 4
        }
        l.readChar()
    }

    // Blank line or comment line: ignore indentation change
    if l.ch == '#' || l.ch == '\n' || l.ch == '\r' || l.ch == 0 {
        return false
    }

    currentIndent := l.indentStack[len(l.indentStack)-1]

    if indent > currentIndent {
        l.indentStack = append(l.indentStack, indent)
        l.pendingTokens = append(l.pendingTokens, token.Token{
            Type:    token.INDENT,
            Literal: "",
            Pos:     l.currentPos(),
        })
        return true
    } else if indent < currentIndent {
        for len(l.indentStack) > 1 && l.indentStack[len(l.indentStack)-1] > indent {
            l.indentStack = l.indentStack[:len(l.indentStack)-1]
            l.pendingTokens = append(l.pendingTokens, token.Token{
                Type:    token.DEDENT,
                Literal: "",
                Pos:     l.currentPos(),
            })
        }
        if l.indentStack[len(l.indentStack)-1] != indent {
            // Indentation error
            l.pendingTokens = append(l.pendingTokens, token.Token{
                Type:    token.ILLEGAL,
                Literal: fmt.Sprintf("IndentationError: unindent does not match any outer indentation level"),
                Pos:     l.currentPos(),
            })
        }
        return true
    }

    return false
}

func (l *Lexer) skipWhitespace() {
    for l.ch == ' ' || l.ch == '\t' || (l.bracketDepth > 0 && (l.ch == '\n' || l.ch == '\r')) {
        if l.ch == '\n' || (l.ch == '\r' && l.peekChar() != '\n') {
            l.line++
            l.col = 0
        } else if l.ch == '\r' && l.peekChar() == '\n' {
            l.readChar()
            l.line++
            l.col = 0
        }
        l.readChar()
    }
}

func (l *Lexer) skipComment() {
    for l.ch != '\n' && l.ch != '\r' && l.ch != 0 {
        l.readChar()
    }
}

func (l *Lexer) readIdentifier() string {
    position := l.position
    for isLetter(l.ch) || isDigit(l.ch) {
        l.readChar()
    }
    return l.input[position:l.position]
}

func (l *Lexer) readNumber(pos token.Pos) token.Token {
    startPos := l.position
    isFloat := false

    if l.ch == '0' {
        next := l.peekChar()
        if next == 'x' || next == 'X' {
            l.readChar() // 0
            l.readChar() // x
            for isHexDigit(l.ch) || l.ch == '_' {
                l.readChar()
            }
            lit := strings.ReplaceAll(l.input[startPos:l.position], "_", "")
            return token.Token{Type: token.INT, Literal: lit, Pos: pos}
        } else if next == 'o' || next == 'O' {
            l.readChar() // 0
            l.readChar() // o
            for (l.ch >= '0' && l.ch <= '7') || l.ch == '_' {
                l.readChar()
            }
            lit := strings.ReplaceAll(l.input[startPos:l.position], "_", "")
            return token.Token{Type: token.INT, Literal: lit, Pos: pos}
        } else if next == 'b' || next == 'B' {
            l.readChar() // 0
            l.readChar() // b
            for l.ch == '0' || l.ch == '1' || l.ch == '_' {
                l.readChar()
            }
            lit := strings.ReplaceAll(l.input[startPos:l.position], "_", "")
            return token.Token{Type: token.INT, Literal: lit, Pos: pos}
        }
    }

    for isDigit(l.ch) || l.ch == '_' {
        l.readChar()
    }

    if l.ch == '.' && isDigit(l.peekChar()) {
        isFloat = true
        l.readChar() // consume '.'
        for isDigit(l.ch) || l.ch == '_' {
            l.readChar()
        }
    }

    if l.ch == 'e' || l.ch == 'E' {
        isFloat = true
        l.readChar()
        if l.ch == '+' || l.ch == '-' {
            l.readChar()
        }
        for isDigit(l.ch) || l.ch == '_' {
            l.readChar()
        }
    }

    lit := strings.ReplaceAll(l.input[startPos:l.position], "_", "")
    if l.ch == 'j' || l.ch == 'J' {
        l.readChar()
        return token.Token{Type: token.COMPLEX, Literal: lit + "j", Pos: pos}
    }
    if isFloat {
        return token.Token{Type: token.FLOAT, Literal: lit, Pos: pos}
    }
    return token.Token{Type: token.INT, Literal: lit, Pos: pos}
}

func (l *Lexer) readString(isFString bool, isBytes bool, pos token.Pos) token.Token {
    quote := l.ch
    isTriple := false

    if l.peekChar() == quote && l.peekCharN(2) == quote {
        isTriple = true
        l.readChar()
        l.readChar()
    }
    l.readChar() // consume first quote

    var sb strings.Builder

    for {
        if l.ch == 0 {
            return token.Token{Type: token.ILLEGAL, Literal: "SyntaxError: EOL while scanning string literal", Pos: pos}
        }

        if isTriple {
            if l.ch == quote && l.peekChar() == quote && l.peekCharN(2) == quote {
                l.readChar()
                l.readChar()
                l.readChar() // consume end triple quote
                break
            }
        } else {
            if l.ch == quote {
                l.readChar()
                break
            }
            if l.ch == '\n' || l.ch == '\r' {
                return token.Token{Type: token.ILLEGAL, Literal: "SyntaxError: EOL while scanning string literal", Pos: pos}
            }
        }

        if l.ch == '\\' {
            l.readChar()
            switch l.ch {
            case 'n':
                sb.WriteByte('\n')
            case 'r':
                sb.WriteByte('\r')
            case 't':
                sb.WriteByte('\t')
            case '\\':
                sb.WriteByte('\\')
            case '\'':
                sb.WriteByte('\'')
            case '"':
                sb.WriteByte('"')
            case 'x': // \xHH
                l.readChar()
                h1 := l.ch
                l.readChar()
                h2 := l.ch
                val, err := strconv.ParseInt(string([]byte{h1, h2}), 16, 32)
                if err == nil {
                    sb.WriteByte(byte(val))
                }
            default:
                sb.WriteByte('\\')
                sb.WriteByte(l.ch)
            }
            l.readChar()
            continue
        }

        if l.ch == '\n' {
            l.line++
            l.col = 0
        }
        sb.WriteByte(l.ch)
        l.readChar()
    }

    var tokType token.TokenType = token.STRING
    if isFString {
        tokType = token.FSTRING
    } else if isBytes {
        tokType = token.BYTES
    }

    return token.Token{Type: tokType, Literal: sb.String(), Pos: pos}
}

func isLetter(ch byte) bool {
    return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_' || unicode.IsLetter(rune(ch))
}

func isDigit(ch byte) bool {
    return ch >= '0' && ch <= '9'
}

func isHexDigit(ch byte) bool {
    return isDigit(ch) || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}
