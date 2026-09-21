package parser

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"aethium/pkg/ast"
	"aethium/pkg/lexer"
	"aethium/pkg/token"
)

const (
	_ int = iota
	LOWEST
	WALRUS
	IF_ELSE
	LOGICAL_OR
	LOGICAL_AND
	LOGICAL_NOT
	COMPARISON
	BIT_OR
	BIT_XOR
	BIT_AND
	SHIFT
	SUM
	PRODUCT
	UNARY
	POWER
	CALL
	INDEX
)

var precedences = map[token.TokenType]int{
	token.WALRUS:    WALRUS,
	token.OR:        LOGICAL_OR,
	token.AND:       LOGICAL_AND,
	token.EQ:        COMPARISON,
	token.NOT_EQ:    COMPARISON,
	token.LT:        COMPARISON,
	token.GT:        COMPARISON,
	token.LT_EQ:     COMPARISON,
	token.GT_EQ:     COMPARISON,
	token.IN:        COMPARISON,
	token.NOT_IN:    COMPARISON,
	token.IS:        COMPARISON,
	token.IS_NOT:    COMPARISON,
	token.PIPE:      BIT_OR,
	token.CARET:     BIT_XOR,
	token.AMPERSAND: BIT_AND,
	token.LSHIFT:    SHIFT,
	token.RSHIFT:    SHIFT,
	token.PLUS:      SUM,
	token.MINUS:     SUM,
	token.SLASH:     PRODUCT,
	token.FLOORDIV:  PRODUCT,
	token.ASTERISK:  PRODUCT,
	token.PERCENT:   PRODUCT,
	token.POW:       POWER,
	token.LPAREN:    CALL,
	token.LBRACKET:  INDEX,
	token.DOT:       INDEX,
	token.IF:        IF_ELSE,
}

type (
	prefixParseFn func() ast.Expression
	infixParseFn  func(ast.Expression) ast.Expression
)

type Parser struct {
	l      *lexer.Lexer
	errors []string

	curToken  token.Token
	peekToken token.Token

	prefixParseFns map[token.TokenType]prefixParseFn
	infixParseFns  map[token.TokenType]infixParseFn
}

func New(l *lexer.Lexer) *Parser {
	p := &Parser{
		l:              l,
		errors:         []string{},
		prefixParseFns: make(map[token.TokenType]prefixParseFn),
		infixParseFns:  make(map[token.TokenType]infixParseFn),
	}

	p.registerPrefix(token.IDENT, p.parseIdentifier)
	p.registerPrefix(token.CHAN, p.parseIdentifier)
	p.registerPrefix(token.SPAWN, p.parseIdentifier)
	p.registerPrefix(token.INT, p.parseIntegerLiteral)
	p.registerPrefix(token.FLOAT, p.parseFloatLiteral)
	p.registerPrefix(token.COMPLEX, p.parseComplexLiteral)
	p.registerPrefix(token.STRING, p.parseStringLiteral)
	p.registerPrefix(token.BYTES, p.parseBytesLiteral)
	p.registerPrefix(token.ELLIPSIS, p.parseEllipsisLiteral)
	p.registerPrefix(token.FSTRING, p.parseFStringLiteral)
	p.registerPrefix(token.BOOL, p.parseBoolLiteral)
	p.registerPrefix(token.NONE, p.parseNoneLiteral)
	p.registerPrefix(token.MINUS, p.parsePrefixExpression)
	p.registerPrefix(token.PLUS, p.parsePrefixExpression)
	p.registerPrefix(token.TILDE, p.parsePrefixExpression)
	p.registerPrefix(token.NOT, p.parsePrefixExpression)
	p.registerPrefix(token.ASTERISK, p.parseStarredExpression)
	p.registerPrefix(token.YIELD, p.parseYieldExpression)
	p.registerPrefix(token.AWAIT, p.parseAwaitExpression)
	p.registerPrefix(token.LPAREN, p.parseGroupedOrTupleOrGenerator)
	p.registerPrefix(token.LBRACKET, p.parseListLiteralOrComp)
	p.registerPrefix(token.LBRACE, p.parseDictOrSetOrComp)
	p.registerPrefix(token.LAMBDA, p.parseLambdaExpression)

	p.registerInfix(token.WALRUS, p.parseWalrusExpression)
	p.registerInfix(token.PLUS, p.parseInfixExpression)
	p.registerInfix(token.MINUS, p.parseInfixExpression)
	p.registerInfix(token.SLASH, p.parseInfixExpression)
	p.registerInfix(token.FLOORDIV, p.parseInfixExpression)
	p.registerInfix(token.ASTERISK, p.parseInfixExpression)
	p.registerInfix(token.PERCENT, p.parseInfixExpression)
	p.registerInfix(token.POW, p.parsePowerExpression)
	p.registerInfix(token.EQ, p.parseInfixExpression)
	p.registerInfix(token.NOT_EQ, p.parseInfixExpression)
	p.registerInfix(token.LT, p.parseInfixExpression)
	p.registerInfix(token.GT, p.parseInfixExpression)
	p.registerInfix(token.LT_EQ, p.parseInfixExpression)
	p.registerInfix(token.GT_EQ, p.parseInfixExpression)
	p.registerInfix(token.AND, p.parseInfixExpression)
	p.registerInfix(token.OR, p.parseInfixExpression)
	p.registerInfix(token.IN, p.parseInfixExpression)
	p.registerInfix(token.NOT_IN, p.parseInfixExpression)
	p.registerInfix(token.IS, p.parseInfixExpression)
	p.registerInfix(token.IS_NOT, p.parseInfixExpression)
	p.registerInfix(token.PIPE, p.parseInfixExpression)
	p.registerInfix(token.CARET, p.parseInfixExpression)
	p.registerInfix(token.AMPERSAND, p.parseInfixExpression)
	p.registerInfix(token.LSHIFT, p.parseInfixExpression)
	p.registerInfix(token.RSHIFT, p.parseInfixExpression)
	p.registerInfix(token.LPAREN, p.parseCallExpression)
	p.registerInfix(token.LBRACKET, p.parseIndexOrSliceExpression)
	p.registerInfix(token.DOT, p.parseAttributeExpression)
	p.registerInfix(token.IF, p.parseTernaryExpression)

	p.nextToken()
	p.nextToken()

	return p
}

func (p *Parser) registerPrefix(tokenType token.TokenType, fn prefixParseFn) {
	p.prefixParseFns[tokenType] = fn
}

func (p *Parser) registerInfix(tokenType token.TokenType, fn infixParseFn) {
	p.infixParseFns[tokenType] = fn
}

func (p *Parser) Errors() []string {
	return p.errors
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

func (p *Parser) curTokenIs(t token.TokenType) bool {
	return p.curToken.Type == t
}

func (p *Parser) peekTokenIs(t token.TokenType) bool {
	return p.peekToken.Type == t
}

func (p *Parser) expectPeek(t token.TokenType) bool {
	if p.peekTokenIs(t) {
		p.nextToken()
		return true
	}
	p.peekError(t)
	return false
}

func (p *Parser) peekError(t token.TokenType) {
	msg := fmt.Sprintf("%s: syntax error: expected next token to be %s, got %s ('%s')",
		p.peekToken.Pos, t, p.peekToken.Type, p.peekToken.Literal)
	p.errors = append(p.errors, msg)
}

func (p *Parser) peekPrecedence() int {
	if p, ok := precedences[p.peekToken.Type]; ok {
		return p
	}
	return LOWEST
}

func (p *Parser) curPrecedence() int {
	if p, ok := precedences[p.curToken.Type]; ok {
		return p
	}
	return LOWEST
}

func (p *Parser) skipNewlines() {
	for p.curTokenIs(token.NEWLINE) || p.curTokenIs(token.SEMICOLON) {
		p.nextToken()
	}
}

func (p *Parser) ParseProgram() *ast.Program {
	program := &ast.Program{
		Statements: []ast.Statement{},
	}

	p.skipNewlines()

	for !p.curTokenIs(token.EOF) {
		startToken := p.curToken
		stmt := p.parseStatement()
		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
		}
		p.skipNewlines()

		if p.curToken == startToken && !p.curTokenIs(token.EOF) {
			p.nextToken()
			p.skipNewlines()
		}
	}

	return program
}

func (p *Parser) parseStatement() ast.Statement {
	var stmt ast.Statement
	switch p.curToken.Type {
	case token.DEF:
		stmt = p.parseFunctionDef()
	case token.CLASS:
		stmt = p.parseClassDef()
	case token.MATCH:
		stmt = p.parseMatchStatement()
	case token.ASYNC:
		stmt = p.parseAsyncStatement()
	case token.IF:
		stmt = p.parseIfStatement()
	case token.WHILE:
		stmt = p.parseWhileStatement()
	case token.FOR:
		stmt = p.parseForInStatement()
	case token.RETURN:
		stmt = p.parseReturnStatement()
	case token.BREAK:
		stmt = &ast.BreakStatement{PosInfo: p.curToken.Pos}
		p.nextToken()
	case token.CONTINUE:
		stmt = &ast.ContinueStatement{PosInfo: p.curToken.Pos}
		p.nextToken()
	case token.PASS:
		stmt = &ast.PassStatement{PosInfo: p.curToken.Pos}
		p.nextToken()
	case token.IMPORT:
		stmt = p.parseImportStatement()
	case token.FROM:
		stmt = p.parseFromImportStatement()
	case token.TRY:
		stmt = p.parseTryExceptStatement()
	case token.RAISE:
		stmt = p.parseRaiseStatement()
	case token.ASSERT:
		stmt = p.parseAssertStatement()
	case token.GLOBAL:
		stmt = p.parseGlobalStatement()
	case token.DEL:
		stmt = p.parseDeleteStatement()
	case token.AT:
		stmt = p.parseDecoratedStatement()
	case token.GO, token.SPAWN:
		stmt = p.parseGoSpawnStatement()
	default:
		stmt = p.parseExpressionOrAssignStatement()
	}

	if p.peekTokenIs(token.NEWLINE) || p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

func (p *Parser) parseBlock() *ast.BlockStatement {
	block := &ast.BlockStatement{
		PosInfo:    p.curToken.Pos,
		Statements: []ast.Statement{},
	}

	if !p.expectPeek(token.COLON) {
		return nil
	}

	p.nextToken()
	if p.curTokenIs(token.NEWLINE) {
		p.nextToken()
		if !p.curTokenIs(token.INDENT) {
			p.errors = append(p.errors, fmt.Sprintf("%s: expected INDENT after block header, got %s", p.curToken.Pos, p.curToken.Type))
			return nil
		}
		p.nextToken()

		for !p.curTokenIs(token.DEDENT) && !p.curTokenIs(token.EOF) {
			p.skipNewlines()
			if p.curTokenIs(token.DEDENT) || p.curTokenIs(token.EOF) {
				break
			}
			startTok := p.curToken
			stmt := p.parseStatement()
			if stmt != nil {
				block.Statements = append(block.Statements, stmt)
			}
			p.skipNewlines()
			if p.curToken == startTok && !p.curTokenIs(token.DEDENT) && !p.curTokenIs(token.EOF) {
				p.nextToken()
				p.skipNewlines()
			}
		}

		if p.curTokenIs(token.DEDENT) {
			p.nextToken()
		}
	} else {
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
	}

	return block
}

func (p *Parser) parseFunctionDef() *ast.FunctionDef {
	stmt := &ast.FunctionDef{
		PosInfo: p.curToken.Pos,
	}

	if !p.expectPeek(token.IDENT) {
		return nil
	}
	stmt.Name = p.curToken.Literal

	if !p.expectPeek(token.LPAREN) {
		return nil
	}

	stmt.Parameters = p.parseParameters()

	if p.peekTokenIs(token.ARROW) {
		p.nextToken()
		p.nextToken()
		if p.curTokenIs(token.IDENT) {
			stmt.Returns = &ast.Identifier{PosInfo: p.curToken.Pos, Value: p.curToken.Literal}
		}
	}

	stmt.Body = p.parseBlock()
	if stmt.Body != nil && len(stmt.Body.Statements) > 0 {
		if exprStmt, ok := stmt.Body.Statements[0].(*ast.ExpressionStatement); ok {
			if strLit, ok := exprStmt.Expression.(*ast.StringLiteral); ok {
				stmt.Doc = strLit.Value
			}
		}
	}
	return stmt
}
func (p *Parser) parseParameters() []ast.Parameter {
	var params []ast.Parameter
	if p.peekTokenIs(token.RPAREN) {
		p.nextToken()
		return params
	}

	p.nextToken()

	for {
		param := ast.Parameter{}
		if p.curTokenIs(token.ASTERISK) {
			param.IsVarArg = true
			p.nextToken()
			if p.curTokenIs(token.IDENT) {
				param.Name = p.curToken.Literal
			}
		} else if p.curTokenIs(token.POW) {
			param.IsKwArg = true
			p.nextToken()
			if p.curTokenIs(token.IDENT) {
				param.Name = p.curToken.Literal
			}
		} else if p.curTokenIs(token.IDENT) {
			param.Name = p.curToken.Literal
		}

		if p.peekTokenIs(token.COLON) {
			p.nextToken()
			p.nextToken()
			if p.curTokenIs(token.IDENT) {
				param.TypeHint = &ast.Identifier{PosInfo: p.curToken.Pos, Value: p.curToken.Literal}
			}
		}

		if p.peekTokenIs(token.ASSIGN) {
			p.nextToken()
			p.nextToken()
			param.DefaultValue = p.parseExpression(LOWEST)
		}

		params = append(params, param)

		if p.peekTokenIs(token.COMMA) {
			p.nextToken()
			if p.peekTokenIs(token.RPAREN) {
				p.nextToken()
				break
			}
			p.nextToken()
		} else {
			break
		}
	}

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return params
}

func (p *Parser) parseAsyncStatement() ast.Statement {
	pos := p.curToken.Pos
	if !p.expectPeek(token.DEF) {
		return nil
	}
	fn := p.parseFunctionDef()
	if fn != nil {
		fn.IsAsync = true
		fn.PosInfo = pos
	}
	return fn
}

func (p *Parser) parseClassDef() *ast.ClassDef {
	stmt := &ast.ClassDef{
		PosInfo: p.curToken.Pos,
	}

	if !p.expectPeek(token.IDENT) {
		return nil
	}
	stmt.Name = p.curToken.Literal

	if p.peekTokenIs(token.LPAREN) {
		p.nextToken()
		stmt.Bases = p.parseExpressionList(token.RPAREN)
	}

	stmt.Body = p.parseBlock()
	if stmt.Body != nil && len(stmt.Body.Statements) > 0 {
		if exprStmt, ok := stmt.Body.Statements[0].(*ast.ExpressionStatement); ok {
			if strLit, ok := exprStmt.Expression.(*ast.StringLiteral); ok {
				stmt.Doc = strLit.Value
			}
		}
	}
	return stmt
}

func (p *Parser) parseMatchStatement() *ast.MatchStatement {
	stmt := &ast.MatchStatement{
		PosInfo: p.curToken.Pos,
	}
	p.nextToken()
	stmt.Subject = p.parseExpression(LOWEST)

	if !p.expectPeek(token.COLON) {
		return nil
	}

	p.nextToken()
	if p.curTokenIs(token.NEWLINE) {
		p.nextToken()
		if !p.curTokenIs(token.INDENT) {
			p.errors = append(p.errors, fmt.Sprintf("%s: expected INDENT after match header, got %s", p.curToken.Pos, p.curToken.Type))
			return nil
		}
		p.nextToken()

		for !p.curTokenIs(token.DEDENT) && !p.curTokenIs(token.EOF) {
			p.skipNewlines()
			if p.curTokenIs(token.DEDENT) || p.curTokenIs(token.EOF) {
				break
			}
			if p.curTokenIs(token.CASE) {
				caseItem := p.parseMatchCase()
				stmt.Cases = append(stmt.Cases, caseItem)
			} else {
				p.nextToken()
			}
			p.skipNewlines()
		}

		if p.curTokenIs(token.DEDENT) {
			p.nextToken()
		}
	}
	return stmt
}

func (p *Parser) parseMatchCase() ast.MatchCase {
	mc := ast.MatchCase{
		PosInfo: p.curToken.Pos,
	}
	p.nextToken()
	mc.Pattern = p.parseExpression(LOWEST)
	if p.peekTokenIs(token.IF) {
		p.nextToken()
		p.nextToken()
		mc.Guard = p.parseExpression(LOWEST)
	}
	mc.Body = p.parseBlock()
	return mc
}

func (p *Parser) parseIfStatement() *ast.IfStatement {
	stmt := &ast.IfStatement{
		PosInfo: p.curToken.Pos,
	}

	p.nextToken()
	stmt.Condition = p.parseExpression(LOWEST)
	stmt.Consequence = p.parseBlock()

	p.skipNewlines()

	for p.curTokenIs(token.ELIF) {
		elifPos := p.curToken.Pos
		p.nextToken()
		cond := p.parseExpression(LOWEST)
		conseq := p.parseBlock()
		stmt.Elifs = append(stmt.Elifs, ast.ElifClause{
			Condition:   cond,
			Consequence: conseq,
		})
		_ = elifPos
		p.skipNewlines()
	}

	if p.curTokenIs(token.ELSE) {
		stmt.Alternative = p.parseBlock()
	}

	return stmt
}

func (p *Parser) parseWhileStatement() *ast.WhileStatement {
	stmt := &ast.WhileStatement{
		PosInfo: p.curToken.Pos,
	}

	p.nextToken()
	stmt.Condition = p.parseExpression(LOWEST)
	stmt.Body = p.parseBlock()

	p.skipNewlines()
	if p.curTokenIs(token.ELSE) {
		stmt.ElseBody = p.parseBlock()
	}

	return stmt
}

func (p *Parser) parseForInStatement() *ast.ForInStatement {
	stmt := &ast.ForInStatement{
		PosInfo: p.curToken.Pos,
	}

	p.nextToken()
	stmt.Target = p.parseExpression(COMPARISON)

	if !p.expectPeek(token.IN) {
		return nil
	}

	p.nextToken()
	stmt.Iterable = p.parseExpression(LOWEST)
	stmt.Body = p.parseBlock()

	p.skipNewlines()
	if p.curTokenIs(token.ELSE) {
		stmt.ElseBody = p.parseBlock()
	}

	return stmt
}

func (p *Parser) parseReturnStatement() *ast.ReturnStatement {
	stmt := &ast.ReturnStatement{
		PosInfo: p.curToken.Pos,
	}

	if p.peekTokenIs(token.NEWLINE) || p.peekTokenIs(token.SEMICOLON) || p.peekTokenIs(token.EOF) || p.peekTokenIs(token.DEDENT) {
		p.nextToken()
		return stmt
	}

	p.nextToken()
	stmt.Value = p.parseExpression(LOWEST)
	return stmt
}

func (p *Parser) parseImportStatement() *ast.ImportStatement {
	stmt := &ast.ImportStatement{
		PosInfo: p.curToken.Pos,
	}

	p.nextToken()

	for {
		if !p.curTokenIs(token.IDENT) {
			p.errors = append(p.errors, fmt.Sprintf("%s: expected module name, got %s", p.curToken.Pos, p.curToken.Type))
			return nil
		}
		modName := p.curToken.Literal
		alias := ""

		if p.peekTokenIs(token.AS) {
			p.nextToken()
			if !p.expectPeek(token.IDENT) {
				return nil
			}
			alias = p.curToken.Literal
		}

		stmt.Names = append(stmt.Names, ast.ImportAlias{
			Name:  modName,
			Alias: alias,
		})

		if p.peekTokenIs(token.COMMA) {
			p.nextToken()
			p.nextToken()
		} else {
			break
		}
	}

	return stmt
}

func (p *Parser) parseFromImportStatement() *ast.FromImportStatement {
	stmt := &ast.FromImportStatement{
		PosInfo: p.curToken.Pos,
	}

	p.nextToken()
	if !p.curTokenIs(token.IDENT) {
		p.errors = append(p.errors, fmt.Sprintf("%s: expected module name, got %s", p.curToken.Pos, p.curToken.Type))
		return nil
	}
	stmt.Module = p.curToken.Literal

	if !p.expectPeek(token.IMPORT) {
		return nil
	}

	p.nextToken()

	for {
		if !p.curTokenIs(token.IDENT) && !p.curTokenIs(token.ASTERISK) {
			p.errors = append(p.errors, fmt.Sprintf("%s: expected imported name or '*', got %s", p.curToken.Pos, p.curToken.Type))
			return nil
		}
		name := p.curToken.Literal
		alias := ""

		if p.peekTokenIs(token.AS) {
			p.nextToken()
			if !p.expectPeek(token.IDENT) {
				return nil
			}
			alias = p.curToken.Literal
		}

		stmt.Names = append(stmt.Names, ast.ImportAlias{
			Name:  name,
			Alias: alias,
		})

		if p.peekTokenIs(token.COMMA) {
			p.nextToken()
			p.nextToken()
		} else {
			break
		}
	}

	return stmt
}

func (p *Parser) parseTryExceptStatement() *ast.TryExceptStatement {
	stmt := &ast.TryExceptStatement{
		PosInfo: p.curToken.Pos,
	}

	stmt.Body = p.parseBlock()
	p.skipNewlines()

	for p.curTokenIs(token.EXCEPT) {
		handler := ast.ExceptHandler{
			PosInfo: p.curToken.Pos,
		}
		if !p.peekTokenIs(token.COLON) {
			p.nextToken()
			handler.Exception = p.parseExpression(LOWEST)
			if p.peekTokenIs(token.AS) {
				p.nextToken()
				if p.expectPeek(token.IDENT) {
					handler.Alias = p.curToken.Literal
				}
			} else if ident, ok := handler.Exception.(*ast.Identifier); ok {
				if len(ident.Value) > 0 && unicode.IsLower(rune(ident.Value[0])) {
					handler.Alias = ident.Value
					handler.Exception = nil
				}
			}
		}
		handler.Body = p.parseBlock()
		stmt.Handlers = append(stmt.Handlers, handler)
		p.skipNewlines()
	}

	if p.curTokenIs(token.ELSE) {
		stmt.ElseBody = p.parseBlock()
		p.skipNewlines()
	}

	if p.curTokenIs(token.FINALLY) {
		stmt.FinallyBody = p.parseBlock()
	}

	return stmt
}

func (p *Parser) parseRaiseStatement() *ast.RaiseStatement {
	stmt := &ast.RaiseStatement{
		PosInfo: p.curToken.Pos,
	}

	if p.peekTokenIs(token.NEWLINE) || p.peekTokenIs(token.EOF) || p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
		return stmt
	}

	p.nextToken()
	stmt.Value = p.parseExpression(LOWEST)

	if p.peekTokenIs(token.FROM) {
		p.nextToken()
		p.nextToken()
		stmt.Cause = p.parseExpression(LOWEST)
	}

	return stmt
}

func (p *Parser) parseAssertStatement() *ast.AssertStatement {
	stmt := &ast.AssertStatement{
		PosInfo: p.curToken.Pos,
	}

	p.nextToken()
	stmt.Condition = p.parseExpression(LOWEST)

	if p.peekTokenIs(token.COMMA) {
		p.nextToken()
		p.nextToken()
		stmt.Message = p.parseExpression(LOWEST)
	}

	return stmt
}

func (p *Parser) parseGlobalStatement() *ast.GlobalStatement {
	stmt := &ast.GlobalStatement{
		PosInfo: p.curToken.Pos,
	}

	for {
		if !p.expectPeek(token.IDENT) {
			return nil
		}
		stmt.Names = append(stmt.Names, p.curToken.Literal)
		if p.peekTokenIs(token.COMMA) {
			p.nextToken()
		} else {
			break
		}
	}

	return stmt
}

func (p *Parser) parseDeleteStatement() *ast.DeleteStatement {
	stmt := &ast.DeleteStatement{
		PosInfo: p.curToken.Pos,
	}

	p.nextToken()
	for {
		expr := p.parseExpression(LOWEST)
		if expr != nil {
			stmt.Targets = append(stmt.Targets, expr)
		}
		if p.peekTokenIs(token.COMMA) {
			p.nextToken()
			p.nextToken()
		} else {
			break
		}
	}

	return stmt
}

func (p *Parser) parseGoSpawnStatement() *ast.GoSpawnStatement {
	stmt := &ast.GoSpawnStatement{
		PosInfo: p.curToken.Pos,
	}
	p.nextToken()
	expr := p.parseExpression(LOWEST)
	if call, ok := expr.(*ast.CallExpression); ok {
		stmt.Call = call
	} else {
		p.errors = append(p.errors, fmt.Sprintf("%s: expected function call after go/spawn", p.curToken.Pos))
	}
	return stmt
}

func (p *Parser) parseDecoratedStatement() ast.Statement {
	var decorators []ast.Expression
	for p.curTokenIs(token.AT) {
		p.nextToken()
		dec := p.parseExpression(LOWEST)
		if dec != nil {
			decorators = append(decorators, dec)
		}
		if p.peekTokenIs(token.NEWLINE) || p.peekTokenIs(token.SEMICOLON) {
			p.nextToken()
		}
		p.skipNewlines()
	}
	if p.curTokenIs(token.DEF) {
		fn := p.parseFunctionDef()
		if fn != nil {
			fn.Decorators = decorators
		}
		return fn
	} else if p.curTokenIs(token.CLASS) {
		cls := p.parseClassDef()
		if cls != nil {
			cls.Decorators = decorators
		}
		return cls
	} else if p.curTokenIs(token.ASYNC) {
		stmt := p.parseAsyncStatement()
		if fn, ok := stmt.(*ast.FunctionDef); ok {
			fn.Decorators = decorators
		}
		return stmt
	}
	p.errors = append(p.errors, fmt.Sprintf("%s: expected def or class after decorator, got %s", p.curToken.Pos, p.curToken.Type))
	return nil
}

func (p *Parser) parseExpressionOrAssignStatement() ast.Statement {
	pos := p.curToken.Pos
	expr := p.parseExpression(LOWEST)
	if expr == nil {
		return nil
	}

	var targets []ast.Expression
	if p.peekTokenIs(token.COMMA) {
		targets = append(targets, expr)
		for p.peekTokenIs(token.COMMA) {
			p.nextToken()
			p.nextToken()
			nextExpr := p.parseExpression(LOWEST)
			targets = append(targets, nextExpr)
		}
	}

	isAssignOp := false
	op := ""
	switch p.peekToken.Type {
	case token.ASSIGN, token.PLUS_ASSIGN, token.MINUS_ASSIGN, token.MUL_ASSIGN,
		token.DIV_ASSIGN, token.FLOORDIV_ASSIGN, token.MOD_ASSIGN, token.POW_ASSIGN,
		token.AND_ASSIGN, token.OR_ASSIGN, token.XOR_ASSIGN, token.LSHIFT_ASSIGN, token.RSHIFT_ASSIGN:
		isAssignOp = true
		op = string(p.peekToken.Literal)
	}

	if isAssignOp {
		p.nextToken()
		p.nextToken()
		val := p.parseExpression(LOWEST)
		if p.peekTokenIs(token.COMMA) {
			elements := []ast.Expression{val}
			for p.peekTokenIs(token.COMMA) {
				p.nextToken()
				if p.peekTokenIs(token.NEWLINE) || p.peekTokenIs(token.EOF) || p.peekTokenIs(token.SEMICOLON) {
					break
				}
				p.nextToken()
				nextElem := p.parseExpression(LOWEST)
				if nextElem != nil {
					elements = append(elements, nextElem)
				}
			}
			val = &ast.TupleLiteral{
				PosInfo:  val.Pos(),
				Elements: elements,
			}
		}
		if len(targets) == 0 {
			targets = []ast.Expression{expr}
		}
		return &ast.AssignStatement{
			PosInfo:  pos,
			Operator: op,
			Targets:  targets,
			Value:    val,
		}
	}

	if len(targets) > 0 {

		return &ast.ExpressionStatement{
			PosInfo: pos,
			Expression: &ast.TupleLiteral{
				PosInfo:  pos,
				Elements: targets,
			},
		}
	}

	return &ast.ExpressionStatement{
		PosInfo:    pos,
		Expression: expr,
	}
}

func (p *Parser) parseExpression(precedence int) ast.Expression {
	prefix := p.prefixParseFns[p.curToken.Type]
	if prefix == nil {
		p.noPrefixParseFnError(p.curToken.Type)
		return nil
	}
	leftExp := prefix()

	for !p.peekTokenIs(token.NEWLINE) && !p.peekTokenIs(token.SEMICOLON) && !p.peekTokenIs(token.EOF) && precedence < p.peekPrecedence() {
		infix := p.infixParseFns[p.peekToken.Type]
		if infix == nil {
			return leftExp
		}
		p.nextToken()
		leftExp = infix(leftExp)
	}

	return leftExp
}

func (p *Parser) noPrefixParseFnError(t token.TokenType) {
	msg := fmt.Sprintf("%s: no prefix parse function for %s ('%s')", p.curToken.Pos, t, p.curToken.Literal)
	p.errors = append(p.errors, msg)
}

func (p *Parser) parseIdentifier() ast.Expression {
	return &ast.Identifier{PosInfo: p.curToken.Pos, Value: p.curToken.Literal}
}

func (p *Parser) parseIntegerLiteral() ast.Expression {
	lit := &ast.IntegerLiteral{PosInfo: p.curToken.Pos}
	val, err := strconv.ParseInt(p.curToken.Literal, 0, 64)
	if err != nil {
		p.errors = append(p.errors, fmt.Sprintf("%s: could not parse %q as integer", p.curToken.Pos, p.curToken.Literal))
		return nil
	}
	lit.Value = val
	return lit
}

func (p *Parser) parseFloatLiteral() ast.Expression {
	lit := &ast.FloatLiteral{PosInfo: p.curToken.Pos}
	val, err := strconv.ParseFloat(p.curToken.Literal, 64)
	if err != nil {
		p.errors = append(p.errors, fmt.Sprintf("%s: could not parse %q as float", p.curToken.Pos, p.curToken.Literal))
		return nil
	}
	lit.Value = val
	return lit
}

func (p *Parser) parseComplexLiteral() ast.Expression {
	lit := &ast.ComplexLiteral{PosInfo: p.curToken.Pos}
	s := p.curToken.Literal
	s = strings.TrimSuffix(s, "j")
	s = strings.TrimSuffix(s, "J")
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		p.errors = append(p.errors, fmt.Sprintf("%s: could not parse %q as complex", p.curToken.Pos, p.curToken.Literal))
		return nil
	}
	lit.Imag = val
	return lit
}

func (p *Parser) parseBytesLiteral() ast.Expression {
	return &ast.BytesLiteral{PosInfo: p.curToken.Pos, Value: []byte(p.curToken.Literal)}
}

func (p *Parser) parseEllipsisLiteral() ast.Expression {
	return &ast.EllipsisLiteral{PosInfo: p.curToken.Pos}
}

func (p *Parser) parseWalrusExpression(left ast.Expression) ast.Expression {
	exp := &ast.WalrusExpression{
		PosInfo: p.curToken.Pos,
		Target:  left,
	}
	p.nextToken()
	exp.Value = p.parseExpression(WALRUS)
	return exp
}

func (p *Parser) parseStarredExpression() ast.Expression {
	pos := p.curToken.Pos
	p.nextToken()
	expr := p.parseExpression(UNARY)
	return &ast.StarredExpression{PosInfo: pos, Value: expr}
}

func (p *Parser) parseYieldExpression() ast.Expression {
	pos := p.curToken.Pos
	isFrom := false
	if p.peekTokenIs(token.FROM) {
		isFrom = true
		p.nextToken()
	}
	var val ast.Expression
	if !p.peekTokenIs(token.NEWLINE) && !p.peekTokenIs(token.SEMICOLON) && !p.peekTokenIs(token.EOF) && !p.peekTokenIs(token.RPAREN) && !p.peekTokenIs(token.RBRACKET) && !p.peekTokenIs(token.RBRACE) && !p.peekTokenIs(token.COMMA) {
		p.nextToken()
		val = p.parseExpression(LOWEST)
	}
	return &ast.YieldExpression{PosInfo: pos, Value: val, IsYieldFrom: isFrom}
}

func (p *Parser) parseAwaitExpression() ast.Expression {
	pos := p.curToken.Pos
	p.nextToken()
	val := p.parseExpression(UNARY)
	return &ast.AwaitExpression{PosInfo: pos, Value: val}
}

func (p *Parser) parseStringLiteral() ast.Expression {
	return &ast.StringLiteral{PosInfo: p.curToken.Pos, Value: p.curToken.Literal}
}

func (p *Parser) parseFStringLiteral() ast.Expression {

	raw := p.curToken.Literal
	pos := p.curToken.Pos
	var parts []ast.FStringPart

	var buf strings.Builder
	i := 0
	for i < len(raw) {
		if raw[i] == '{' {
			if i+1 < len(raw) && raw[i+1] == '{' {
				buf.WriteByte('{')
				i += 2
				continue
			}
			if buf.Len() > 0 {
				parts = append(parts, ast.FStringPart{
					IsExpr: false,
					Text:   buf.String(),
				})
				buf.Reset()
			}

			i++
			exprStart := i
			braceDepth := 1
			for i < len(raw) && braceDepth > 0 {
				if raw[i] == '{' {
					braceDepth++
				} else if raw[i] == '}' {
					braceDepth--
				}
				i++
			}
			exprStr := raw[exprStart : i-1]

			subLexer := lexer.New(pos.File, exprStr)
			subParser := New(subLexer)
			subExpr := subParser.parseExpression(LOWEST)
			parts = append(parts, ast.FStringPart{
				IsExpr: true,
				Expr:   subExpr,
			})
		} else if raw[i] == '}' && i+1 < len(raw) && raw[i+1] == '}' {
			buf.WriteByte('}')
			i += 2
		} else {
			buf.WriteByte(raw[i])
			i++
		}
	}

	if buf.Len() > 0 {
		parts = append(parts, ast.FStringPart{
			IsExpr: false,
			Text:   buf.String(),
		})
	}

	return &ast.FStringLiteral{PosInfo: pos, Parts: parts}
}

func (p *Parser) parseBoolLiteral() ast.Expression {
	return &ast.BoolLiteral{PosInfo: p.curToken.Pos, Value: p.curToken.Literal == "True"}
}

func (p *Parser) parseNoneLiteral() ast.Expression {
	return &ast.NoneLiteral{PosInfo: p.curToken.Pos}
}

func (p *Parser) parsePrefixExpression() ast.Expression {
	expression := &ast.UnaryExpression{
		PosInfo:  p.curToken.Pos,
		Operator: p.curToken.Literal,
	}

	p.nextToken()
	expression.Right = p.parseExpression(UNARY)
	return expression
}

func (p *Parser) parseInfixExpression(left ast.Expression) ast.Expression {
	expression := &ast.BinaryExpression{
		PosInfo:  p.curToken.Pos,
		Operator: p.curToken.Literal,
		Left:     left,
	}

	prec := p.curPrecedence()
	p.nextToken()
	expression.Right = p.parseExpression(prec)
	return expression
}

func (p *Parser) parsePowerExpression(left ast.Expression) ast.Expression {
	expression := &ast.BinaryExpression{
		PosInfo:  p.curToken.Pos,
		Operator: "**",
		Left:     left,
	}

	p.nextToken()
	expression.Right = p.parseExpression(POWER - 1)
	return expression
}

func (p *Parser) parseGroupedOrTupleOrGenerator() ast.Expression {
	pos := p.curToken.Pos

	if p.peekTokenIs(token.RPAREN) {
		p.nextToken()
		return &ast.TupleLiteral{PosInfo: pos, Elements: []ast.Expression{}}
	}

	p.nextToken()
	expr := p.parseExpression(LOWEST)

	if p.peekTokenIs(token.COMMA) {

		elements := []ast.Expression{expr}
		for p.peekTokenIs(token.COMMA) {
			p.nextToken()
			if p.peekTokenIs(token.RPAREN) {
				p.nextToken()
				return &ast.TupleLiteral{PosInfo: pos, Elements: elements}
			}
			p.nextToken()
			elements = append(elements, p.parseExpression(LOWEST))
		}
		if !p.expectPeek(token.RPAREN) {
			return nil
		}
		return &ast.TupleLiteral{PosInfo: pos, Elements: elements}
	}

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return expr
}

func (p *Parser) parseListLiteralOrComp() ast.Expression {
	pos := p.curToken.Pos
	p.nextToken()

	if p.curTokenIs(token.RBRACKET) {
		return &ast.ListLiteral{PosInfo: pos, Elements: []ast.Expression{}}
	}

	elem := p.parseExpression(LOWEST)

	if p.peekTokenIs(token.FOR) {
		p.nextToken()
		p.nextToken()
		target := p.parseExpression(COMPARISON)

		if !p.expectPeek(token.IN) {
			return nil
		}
		p.nextToken()
		iter := p.parseExpression(IF_ELSE)

		var cond ast.Expression
		if p.peekTokenIs(token.IF) {
			p.nextToken()
			p.nextToken()
			cond = p.parseExpression(LOWEST)
		}

		if !p.expectPeek(token.RBRACKET) {
			return nil
		}

		return &ast.ListComp{
			PosInfo:   pos,
			Element:   elem,
			Target:    target,
			Iterable:  iter,
			Condition: cond,
		}
	}

	elements := []ast.Expression{elem}
	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		if p.peekTokenIs(token.RBRACKET) {
			p.nextToken()
			return &ast.ListLiteral{PosInfo: pos, Elements: elements}
		}
		p.nextToken()
		elements = append(elements, p.parseExpression(LOWEST))
	}

	if !p.expectPeek(token.RBRACKET) {
		return nil
	}

	return &ast.ListLiteral{PosInfo: pos, Elements: elements}
}

func (p *Parser) parseDictOrSetOrComp() ast.Expression {
	pos := p.curToken.Pos
	p.nextToken()

	if p.curTokenIs(token.RBRACE) {

		return &ast.DictLiteral{PosInfo: pos, Entries: []ast.DictEntry{}}
	}

	firstExpr := p.parseExpression(LOWEST)

	if p.peekTokenIs(token.COLON) {
		p.nextToken()
		p.nextToken()
		valExpr := p.parseExpression(LOWEST)

		if p.peekTokenIs(token.FOR) {
			p.nextToken()
			p.nextToken()
			target := p.parseExpression(COMPARISON)
			if !p.expectPeek(token.IN) {
				return nil
			}
			p.nextToken()
			iter := p.parseExpression(IF_ELSE)
			var cond ast.Expression
			if p.peekTokenIs(token.IF) {
				p.nextToken()
				p.nextToken()
				cond = p.parseExpression(LOWEST)
			}
			if !p.expectPeek(token.RBRACE) {
				return nil
			}
			return &ast.DictComp{
				PosInfo:   pos,
				Key:       firstExpr,
				Value:     valExpr,
				Target:    target,
				Iterable:  iter,
				Condition: cond,
			}
		}

		entries := []ast.DictEntry{{Key: firstExpr, Value: valExpr}}
		for p.peekTokenIs(token.COMMA) {
			p.nextToken()
			if p.peekTokenIs(token.RBRACE) {
				p.nextToken()
				return &ast.DictLiteral{PosInfo: pos, Entries: entries}
			}
			p.nextToken()
			k := p.parseExpression(LOWEST)
			if !p.expectPeek(token.COLON) {
				return nil
			}
			p.nextToken()
			v := p.parseExpression(LOWEST)
			entries = append(entries, ast.DictEntry{Key: k, Value: v})
		}

		if !p.expectPeek(token.RBRACE) {
			return nil
		}
		return &ast.DictLiteral{PosInfo: pos, Entries: entries}
	}

	if p.peekTokenIs(token.FOR) {
		p.nextToken()
		p.nextToken()
		target := p.parseExpression(COMPARISON)
		if !p.expectPeek(token.IN) {
			return nil
		}
		p.nextToken()
		iter := p.parseExpression(IF_ELSE)
		var cond ast.Expression
		if p.peekTokenIs(token.IF) {
			p.nextToken()
			p.nextToken()
			cond = p.parseExpression(LOWEST)
		}
		if !p.expectPeek(token.RBRACE) {
			return nil
		}
		return &ast.SetComp{
			PosInfo:   pos,
			Element:   firstExpr,
			Target:    target,
			Iterable:  iter,
			Condition: cond,
		}
	}

	elements := []ast.Expression{firstExpr}
	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		if p.peekTokenIs(token.RBRACE) {
			p.nextToken()
			return &ast.SetLiteral{PosInfo: pos, Elements: elements}
		}
		p.nextToken()
		elements = append(elements, p.parseExpression(LOWEST))
	}

	if !p.expectPeek(token.RBRACE) {
		return nil
	}

	return &ast.SetLiteral{PosInfo: pos, Elements: elements}
}

func (p *Parser) parseLambdaExpression() ast.Expression {
	pos := p.curToken.Pos
	p.nextToken()

	var params []ast.Parameter

	if !p.curTokenIs(token.COLON) {
		for {
			if !p.curTokenIs(token.IDENT) {
				p.errors = append(p.errors, fmt.Sprintf("%s: expected parameter name in lambda, got %s", p.curToken.Pos, p.curToken.Type))
				return nil
			}
			params = append(params, ast.Parameter{Name: p.curToken.Literal})
			if p.peekTokenIs(token.COMMA) {
				p.nextToken()
				p.nextToken()
			} else if p.peekTokenIs(token.COLON) {
				p.nextToken()
				break
			} else {
				p.errors = append(p.errors, fmt.Sprintf("%s: expected ',' or ':' in lambda, got %s", p.peekToken.Pos, p.peekToken.Type))
				return nil
			}
		}
	}

	p.nextToken()
	body := p.parseExpression(LOWEST)
	return &ast.LambdaExpression{
		PosInfo:    pos,
		Parameters: params,
		Body:       body,
	}
}

func (p *Parser) parseCallExpression(fn ast.Expression) ast.Expression {
	exp := &ast.CallExpression{PosInfo: p.curToken.Pos, Function: fn}
	exp.Arguments, exp.Keywords = p.parseCallArguments()
	return exp
}

func (p *Parser) parseCallArguments() ([]ast.Expression, []ast.KeywordArgument) {
	var args []ast.Expression
	var kwargs []ast.KeywordArgument

	if p.peekTokenIs(token.RPAREN) {
		p.nextToken()
		return args, kwargs
	}

	p.nextToken()

	for {
		if p.curTokenIs(token.IDENT) && p.peekTokenIs(token.ASSIGN) {
			key := p.curToken.Literal
			p.nextToken()
			p.nextToken()
			val := p.parseExpression(LOWEST)
			kwargs = append(kwargs, ast.KeywordArgument{Key: key, Value: val})
		} else {
			args = append(args, p.parseExpression(LOWEST))
		}

		if p.peekTokenIs(token.COMMA) {
			p.nextToken()
			if p.peekTokenIs(token.RPAREN) {
				p.nextToken()
				break
			}
			p.nextToken()
		} else if p.peekTokenIs(token.RPAREN) {
			p.nextToken()
			break
		} else {
			p.errors = append(p.errors, fmt.Sprintf("%s: expected ',' or ')', got %s", p.peekToken.Pos, p.peekToken.Type))
			break
		}
	}

	return args, kwargs
}

func (p *Parser) parseIndexOrSliceExpression(left ast.Expression) ast.Expression {
	pos := p.curToken.Pos
	p.nextToken()

	var start ast.Expression
	isSlice := false

	if p.curTokenIs(token.COLON) {
		isSlice = true
	} else {
		start = p.parseExpression(LOWEST)
		if p.peekTokenIs(token.COLON) {
			isSlice = true
			p.nextToken()
		}
	}

	if isSlice {
		var end ast.Expression
		var step ast.Expression

		if !p.peekTokenIs(token.RBRACKET) && !p.peekTokenIs(token.COLON) && !p.curTokenIs(token.RBRACKET) {
			if !p.curTokenIs(token.COLON) {

			} else {
				p.nextToken()
			}
			if !p.curTokenIs(token.COLON) && !p.curTokenIs(token.RBRACKET) {
				end = p.parseExpression(LOWEST)
			}
		}

		if p.peekTokenIs(token.COLON) || p.curTokenIs(token.COLON) {
			if p.peekTokenIs(token.COLON) {
				p.nextToken()
			}
			if !p.peekTokenIs(token.RBRACKET) {
				p.nextToken()
				step = p.parseExpression(LOWEST)
			}
		}

		if !p.expectPeek(token.RBRACKET) {
			return nil
		}

		return &ast.SliceExpression{
			PosInfo: pos,
			Left:    left,
			Start:   start,
			End:     end,
			Step:    step,
		}
	}

	if !p.expectPeek(token.RBRACKET) {
		return nil
	}

	return &ast.IndexExpression{
		PosInfo: pos,
		Left:    left,
		Index:   start,
	}
}

func (p *Parser) parseAttributeExpression(left ast.Expression) ast.Expression {
	pos := p.curToken.Pos
	p.nextToken()

	if !p.curTokenIs(token.IDENT) {
		p.errors = append(p.errors, fmt.Sprintf("%s: expected identifier after '.', got %s", p.curToken.Pos, p.curToken.Type))
		return nil
	}

	return &ast.AttributeExpression{
		PosInfo:   pos,
		Object:    left,
		Attribute: p.curToken.Literal,
	}
}

func (p *Parser) parseTernaryExpression(consequence ast.Expression) ast.Expression {
	pos := p.curToken.Pos
	p.nextToken()
	condition := p.parseExpression(LOWEST)

	if !p.expectPeek(token.ELSE) {
		return nil
	}

	p.nextToken()
	alternative := p.parseExpression(IF_ELSE)

	return &ast.TernaryExpression{
		PosInfo:     pos,
		Condition:   condition,
		Consequence: consequence,
		Alternative: alternative,
	}
}

func (p *Parser) parseExpressionList(end token.TokenType) []ast.Expression {
	var list []ast.Expression

	if p.peekTokenIs(end) {
		p.nextToken()
		return list
	}

	p.nextToken()
	list = append(list, p.parseExpression(LOWEST))

	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		if p.peekTokenIs(end) {
			p.nextToken()
			return list
		}
		p.nextToken()
		list = append(list, p.parseExpression(LOWEST))
	}

	if !p.expectPeek(end) {
		return nil
	}

	return list
}
