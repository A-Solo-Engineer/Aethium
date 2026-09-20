package ast

import (
	"fmt"
	"strings"

	"aethium/pkg/token"
)

type Node interface {
	Pos() token.Pos
	String() string
}

type Statement interface {
	Node
	statementNode()
}

type Expression interface {
	Node
	expressionNode()
}

type Program struct {
	Statements []Statement
}

func (p *Program) Pos() token.Pos {
	if len(p.Statements) > 0 {
		return p.Statements[0].Pos()
	}
	return token.Pos{}
}

func (p *Program) String() string {
	var sb strings.Builder
	for _, s := range p.Statements {
		sb.WriteString(s.String())
		sb.WriteString("\n")
	}
	return sb.String()
}

type BlockStatement struct {
	PosInfo    token.Pos
	Statements []Statement
}

func (b *BlockStatement) Pos() token.Pos { return b.PosInfo }
func (b *BlockStatement) statementNode() {}
func (b *BlockStatement) String() string {
	var sb strings.Builder
	for _, s := range b.Statements {
		sb.WriteString("  ")
		sb.WriteString(s.String())
		sb.WriteString("\n")
	}
	return sb.String()
}

type ExpressionStatement struct {
	PosInfo    token.Pos
	Expression Expression
}

func (es *ExpressionStatement) Pos() token.Pos { return es.PosInfo }
func (es *ExpressionStatement) statementNode() {}
func (es *ExpressionStatement) String() string {
	if es.Expression != nil {
		return es.Expression.String()
	}
	return ""
}

type AssignStatement struct {
	PosInfo  token.Pos
	Operator string // "=", "+=", "-=", "*=", "/=", "//=", "%=", "**=", "&=", "|=", "^=", "<<=", ">>="
	Targets  []Expression
	Value    Expression
}

func (as *AssignStatement) Pos() token.Pos { return as.PosInfo }
func (as *AssignStatement) statementNode() {}
func (as *AssignStatement) String() string {
	var tgts []string
	for _, t := range as.Targets {
		tgts = append(tgts, t.String())
	}
	return fmt.Sprintf("%s %s %s", strings.Join(tgts, ", "), as.Operator, as.Value.String())
}

type ReturnStatement struct {
	PosInfo token.Pos
	Value   Expression
}

func (rs *ReturnStatement) Pos() token.Pos { return rs.PosInfo }
func (rs *ReturnStatement) statementNode() {}
func (rs *ReturnStatement) String() string {
	if rs.Value != nil {
		return fmt.Sprintf("return %s", rs.Value.String())
	}
	return "return"
}

type BreakStatement struct {
	PosInfo token.Pos
}

func (bs *BreakStatement) Pos() token.Pos { return bs.PosInfo }
func (bs *BreakStatement) statementNode() {}
func (bs *BreakStatement) String() string { return "break" }

type ContinueStatement struct {
	PosInfo token.Pos
}

func (cs *ContinueStatement) Pos() token.Pos { return cs.PosInfo }
func (cs *ContinueStatement) statementNode() {}
func (cs *ContinueStatement) String() string { return "continue" }

type PassStatement struct {
	PosInfo token.Pos
}

func (ps *PassStatement) Pos() token.Pos { return ps.PosInfo }
func (ps *PassStatement) statementNode() {}
func (ps *PassStatement) String() string { return "pass" }

type ElifClause struct {
	Condition   Expression
	Consequence *BlockStatement
}

type IfStatement struct {
	PosInfo     token.Pos
	Condition   Expression
	Consequence *BlockStatement
	Elifs       []ElifClause
	Alternative *BlockStatement
}

func (is *IfStatement) Pos() token.Pos { return is.PosInfo }
func (is *IfStatement) statementNode() {}
func (is *IfStatement) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("if %s:\n%s", is.Condition.String(), is.Consequence.String()))
	for _, el := range is.Elifs {
		sb.WriteString(fmt.Sprintf("elif %s:\n%s", el.Condition.String(), el.Consequence.String()))
	}
	if is.Alternative != nil {
		sb.WriteString(fmt.Sprintf("else:\n%s", is.Alternative.String()))
	}
	return sb.String()
}

type WhileStatement struct {
	PosInfo   token.Pos
	Condition Expression
	Body      *BlockStatement
	ElseBody  *BlockStatement
}

func (ws *WhileStatement) Pos() token.Pos { return ws.PosInfo }
func (ws *WhileStatement) statementNode() {}
func (ws *WhileStatement) String() string {
	return fmt.Sprintf("while %s:\n%s", ws.Condition.String(), ws.Body.String())
}

type ForInStatement struct {
	PosInfo  token.Pos
	Target   Expression // identifier or tuple of identifiers
	Iterable Expression
	Body     *BlockStatement
	ElseBody *BlockStatement
}

func (fs *ForInStatement) Pos() token.Pos { return fs.PosInfo }
func (fs *ForInStatement) statementNode() {}
func (fs *ForInStatement) String() string {
	return fmt.Sprintf("for %s in %s:\n%s", fs.Target.String(), fs.Iterable.String(), fs.Body.String())
}

type Parameter struct {
	Name         string
	DefaultValue Expression
	TypeHint     Expression
	IsVarArg     bool // *args
	IsKwArg      bool // **kwargs
}

type FunctionDef struct {
	PosInfo    token.Pos
	Name       string
	Parameters []Parameter
	Returns    Expression
	Body       *BlockStatement
	Decorators []Expression
	IsAsync    bool
	Doc        string
}

func (fd *FunctionDef) Pos() token.Pos { return fd.PosInfo }
func (fd *FunctionDef) statementNode() {}
func (fd *FunctionDef) String() string {
	var params []string
	for _, p := range fd.Parameters {
		if p.IsVarArg {
			params = append(params, "*"+p.Name)
		} else if p.IsKwArg {
			params = append(params, "**"+p.Name)
		} else if p.DefaultValue != nil {
			params = append(params, fmt.Sprintf("%s=%s", p.Name, p.DefaultValue.String()))
		} else {
			params = append(params, p.Name)
		}
	}
	return fmt.Sprintf("def %s(%s):\n%s", fd.Name, strings.Join(params, ", "), fd.Body.String())
}

type ClassDef struct {
	PosInfo    token.Pos
	Name       string
	Bases      []Expression
	Body       *BlockStatement
	Decorators []Expression
	Doc        string
}

func (cd *ClassDef) Pos() token.Pos { return cd.PosInfo }
func (cd *ClassDef) statementNode() {}
func (cd *ClassDef) String() string {
	var bases []string
	for _, b := range cd.Bases {
		bases = append(bases, b.String())
	}
	baseStr := ""
	if len(bases) > 0 {
		baseStr = fmt.Sprintf("(%s)", strings.Join(bases, ", "))
	}
	return fmt.Sprintf("class %s%s:\n%s", cd.Name, baseStr, cd.Body.String())
}

// ImportStatement: import math, os as my_os
type ImportAlias struct {
	Name  string
	Alias string
}

type ImportStatement struct {
	PosInfo token.Pos
	Names   []ImportAlias
}

func (is *ImportStatement) Pos() token.Pos { return is.PosInfo }
func (is *ImportStatement) statementNode() {}
func (is *ImportStatement) String() string {
	var parts []string
	for _, n := range is.Names {
		if n.Alias != "" {
			parts = append(parts, fmt.Sprintf("%s as %s", n.Name, n.Alias))
		} else {
			parts = append(parts, n.Name)
		}
	}
	return fmt.Sprintf("import %s", strings.Join(parts, ", "))
}

// FromImportStatement: from math import sin, cos as c
type FromImportStatement struct {
	PosInfo token.Pos
	Module  string
	Names   []ImportAlias
}

func (fis *FromImportStatement) Pos() token.Pos { return fis.PosInfo }
func (fis *FromImportStatement) statementNode() {}
func (fis *FromImportStatement) String() string {
	var parts []string
	for _, n := range fis.Names {
		if n.Alias != "" {
			parts = append(parts, fmt.Sprintf("%s as %s", n.Name, n.Alias))
		} else {
			parts = append(parts, n.Name)
		}
	}
	return fmt.Sprintf("from %s import %s", fis.Module, strings.Join(parts, ", "))
}

// ExceptHandler
type ExceptHandler struct {
	PosInfo   token.Pos
	Exception Expression
	Alias     string
	Body      *BlockStatement
}

// TryExceptStatement
type TryExceptStatement struct {
	PosInfo     token.Pos
	Body        *BlockStatement
	Handlers    []ExceptHandler
	ElseBody    *BlockStatement
	FinallyBody *BlockStatement
}

func (ts *TryExceptStatement) Pos() token.Pos { return ts.PosInfo }
func (ts *TryExceptStatement) statementNode() {}
func (ts *TryExceptStatement) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("try:\n%s", ts.Body.String()))
	for _, h := range ts.Handlers {
		excStr := ""
		if h.Exception != nil {
			excStr = " " + h.Exception.String()
			if h.Alias != "" {
				excStr += " as " + h.Alias
			}
		}
		sb.WriteString(fmt.Sprintf("except%s:\n%s", excStr, h.Body.String()))
	}
	if ts.FinallyBody != nil {
		sb.WriteString(fmt.Sprintf("finally:\n%s", ts.FinallyBody.String()))
	}
	return sb.String()
}

// RaiseStatement
type RaiseStatement struct {
	PosInfo token.Pos
	Value   Expression
	Cause   Expression // for "raise X from Y"
}

func (rs *RaiseStatement) Pos() token.Pos { return rs.PosInfo }
func (rs *RaiseStatement) statementNode() {}
func (rs *RaiseStatement) String() string {
	if rs.Value != nil {
		return fmt.Sprintf("raise %s", rs.Value.String())
	}
	return "raise"
}

// AssertStatement
type AssertStatement struct {
	PosInfo   token.Pos
	Condition Expression
	Message   Expression
}

func (as *AssertStatement) Pos() token.Pos { return as.PosInfo }
func (as *AssertStatement) statementNode() {}
func (as *AssertStatement) String() string {
	if as.Message != nil {
		return fmt.Sprintf("assert %s, %s", as.Condition.String(), as.Message.String())
	}
	return fmt.Sprintf("assert %s", as.Condition.String())
}

// GlobalStatement
type GlobalStatement struct {
	PosInfo token.Pos
	Names   []string
}

func (gs *GlobalStatement) Pos() token.Pos { return gs.PosInfo }
func (gs *GlobalStatement) statementNode() {}
func (gs *GlobalStatement) String() string {
	return fmt.Sprintf("global %s", strings.Join(gs.Names, ", "))
}

// DeleteStatement
type DeleteStatement struct {
	PosInfo token.Pos
	Targets []Expression
}

func (ds *DeleteStatement) Pos() token.Pos { return ds.PosInfo }
func (ds *DeleteStatement) statementNode() {}
func (ds *DeleteStatement) String() string {
	var parts []string
	for _, t := range ds.Targets {
		parts = append(parts, t.String())
	}
	return fmt.Sprintf("del %s", strings.Join(parts, ", "))
}

// GoSpawnStatement: go func(...) or spawn func(...)
type GoSpawnStatement struct {
	PosInfo token.Pos
	Call    *CallExpression
}

func (gss *GoSpawnStatement) Pos() token.Pos { return gss.PosInfo }
func (gss *GoSpawnStatement) statementNode() {}
func (gss *GoSpawnStatement) String() string { return fmt.Sprintf("go %s", gss.Call.String()) }

// Expressions

type IntegerLiteral struct {
	PosInfo token.Pos
	Value   int64
}

func (il *IntegerLiteral) Pos() token.Pos  { return il.PosInfo }
func (il *IntegerLiteral) expressionNode() {}
func (il *IntegerLiteral) String() string  { return fmt.Sprintf("%d", il.Value) }

type FloatLiteral struct {
	PosInfo token.Pos
	Value   float64
}

func (fl *FloatLiteral) Pos() token.Pos  { return fl.PosInfo }
func (fl *FloatLiteral) expressionNode() {}
func (fl *FloatLiteral) String() string  { return fmt.Sprintf("%g", fl.Value) }

type StringLiteral struct {
	PosInfo token.Pos
	Value   string
}

func (sl *StringLiteral) Pos() token.Pos  { return sl.PosInfo }
func (sl *StringLiteral) expressionNode() {}
func (sl *StringLiteral) String() string  { return fmt.Sprintf("%q", sl.Value) }

type FStringPart struct {
	IsExpr bool
	Expr   Expression
	Text   string
}

type FStringLiteral struct {
	PosInfo token.Pos
	Parts   []FStringPart
}

func (fsl *FStringLiteral) Pos() token.Pos  { return fsl.PosInfo }
func (fsl *FStringLiteral) expressionNode() {}
func (fsl *FStringLiteral) String() string  { return "f\"...\"" }

type BoolLiteral struct {
	PosInfo token.Pos
	Value   bool
}

func (bl *BoolLiteral) Pos() token.Pos  { return bl.PosInfo }
func (bl *BoolLiteral) expressionNode() {}
func (bl *BoolLiteral) String() string {
	if bl.Value {
		return "True"
	}
	return "False"
}

type NoneLiteral struct {
	PosInfo token.Pos
}

func (nl *NoneLiteral) Pos() token.Pos  { return nl.PosInfo }
func (nl *NoneLiteral) expressionNode() {}
func (nl *NoneLiteral) String() string  { return "None" }

type Identifier struct {
	PosInfo token.Pos
	Value   string
}

func (i *Identifier) Pos() token.Pos  { return i.PosInfo }
func (i *Identifier) expressionNode() {}
func (i *Identifier) String() string  { return i.Value }

type UnaryExpression struct {
	PosInfo  token.Pos
	Operator string
	Right    Expression
}

func (ue *UnaryExpression) Pos() token.Pos  { return ue.PosInfo }
func (ue *UnaryExpression) expressionNode() {}
func (ue *UnaryExpression) String() string {
	return fmt.Sprintf("(%s%s)", ue.Operator, ue.Right.String())
}

type BinaryExpression struct {
	PosInfo  token.Pos
	Left     Expression
	Operator string
	Right    Expression
}

func (be *BinaryExpression) Pos() token.Pos  { return be.PosInfo }
func (be *BinaryExpression) expressionNode() {}
func (be *BinaryExpression) String() string {
	return fmt.Sprintf("(%s %s %s)", be.Left.String(), be.Operator, be.Right.String())
}

type KeywordArgument struct {
	Key   string
	Value Expression
}

type CallExpression struct {
	PosInfo    token.Pos
	Function   Expression
	Arguments  []Expression
	Keywords   []KeywordArgument
	IsChanRecv bool
}

func (ce *CallExpression) Pos() token.Pos  { return ce.PosInfo }
func (ce *CallExpression) expressionNode() {}
func (ce *CallExpression) String() string {
	var args []string
	for _, a := range ce.Arguments {
		args = append(args, a.String())
	}
	for _, kw := range ce.Keywords {
		args = append(args, fmt.Sprintf("%s=%s", kw.Key, kw.Value.String()))
	}
	return fmt.Sprintf("%s(%s)", ce.Function.String(), strings.Join(args, ", "))
}

type IndexExpression struct {
	PosInfo token.Pos
	Left    Expression
	Index   Expression
}

func (ie *IndexExpression) Pos() token.Pos  { return ie.PosInfo }
func (ie *IndexExpression) expressionNode() {}
func (ie *IndexExpression) String() string {
	return fmt.Sprintf("%s[%s]", ie.Left.String(), ie.Index.String())
}

type SliceExpression struct {
	PosInfo token.Pos
	Left    Expression
	Start   Expression
	End     Expression
	Step    Expression
}

func (se *SliceExpression) Pos() token.Pos  { return se.PosInfo }
func (se *SliceExpression) expressionNode() {}
func (se *SliceExpression) String() string {
	s, e, st := "", "", ""
	if se.Start != nil {
		s = se.Start.String()
	}
	if se.End != nil {
		e = se.End.String()
	}
	if se.Step != nil {
		st = ":" + se.Step.String()
	}
	return fmt.Sprintf("%s[%s:%s%s]", se.Left.String(), s, e, st)
}

type AttributeExpression struct {
	PosInfo   token.Pos
	Object    Expression
	Attribute string
}

func (ae *AttributeExpression) Pos() token.Pos  { return ae.PosInfo }
func (ae *AttributeExpression) expressionNode() {}
func (ae *AttributeExpression) String() string {
	return fmt.Sprintf("%s.%s", ae.Object.String(), ae.Attribute)
}

type ListLiteral struct {
	PosInfo  token.Pos
	Elements []Expression
}

func (ll *ListLiteral) Pos() token.Pos  { return ll.PosInfo }
func (ll *ListLiteral) expressionNode() {}
func (ll *ListLiteral) String() string {
	var els []string
	for _, e := range ll.Elements {
		els = append(els, e.String())
	}
	return fmt.Sprintf("[%s]", strings.Join(els, ", "))
}

type TupleLiteral struct {
	PosInfo  token.Pos
	Elements []Expression
}

func (tl *TupleLiteral) Pos() token.Pos  { return tl.PosInfo }
func (tl *TupleLiteral) expressionNode() {}
func (tl *TupleLiteral) String() string {
	var els []string
	for _, e := range tl.Elements {
		els = append(els, e.String())
	}
	return fmt.Sprintf("(%s)", strings.Join(els, ", "))
}

type DictEntry struct {
	Key   Expression
	Value Expression
}

type DictLiteral struct {
	PosInfo token.Pos
	Entries []DictEntry
}

func (dl *DictLiteral) Pos() token.Pos  { return dl.PosInfo }
func (dl *DictLiteral) expressionNode() {}
func (dl *DictLiteral) String() string {
	var entries []string
	for _, e := range dl.Entries {
		entries = append(entries, fmt.Sprintf("%s: %s", e.Key.String(), e.Value.String()))
	}
	return fmt.Sprintf("{%s}", strings.Join(entries, ", "))
}

type SetLiteral struct {
	PosInfo  token.Pos
	Elements []Expression
}

func (sl *SetLiteral) Pos() token.Pos  { return sl.PosInfo }
func (sl *SetLiteral) expressionNode() {}
func (sl *SetLiteral) String() string {
	var els []string
	for _, e := range sl.Elements {
		els = append(els, e.String())
	}
	return fmt.Sprintf("{%s}", strings.Join(els, ", "))
}

type ListComp struct {
	PosInfo   token.Pos
	Element   Expression
	Target    Expression
	Iterable  Expression
	Condition Expression
}

func (lc *ListComp) Pos() token.Pos  { return lc.PosInfo }
func (lc *ListComp) expressionNode() {}
func (lc *ListComp) String() string {
	cond := ""
	if lc.Condition != nil {
		cond = " if " + lc.Condition.String()
	}
	return fmt.Sprintf("[%s for %s in %s%s]", lc.Element.String(), lc.Target.String(), lc.Iterable.String(), cond)
}

type DictComp struct {
	PosInfo   token.Pos
	Key       Expression
	Value     Expression
	Target    Expression
	Iterable  Expression
	Condition Expression
}

func (dc *DictComp) Pos() token.Pos  { return dc.PosInfo }
func (dc *DictComp) expressionNode() {}
func (dc *DictComp) String() string {
	cond := ""
	if dc.Condition != nil {
		cond = " if " + dc.Condition.String()
	}
	return fmt.Sprintf("{%s: %s for %s in %s%s}", dc.Key.String(), dc.Value.String(), dc.Target.String(), dc.Iterable.String(), cond)
}

type SetComp struct {
	PosInfo   token.Pos
	Element   Expression
	Target    Expression
	Iterable  Expression
	Condition Expression
}

func (sc *SetComp) Pos() token.Pos  { return sc.PosInfo }
func (sc *SetComp) expressionNode() {}
func (sc *SetComp) String() string {
	cond := ""
	if sc.Condition != nil {
		cond = " if " + sc.Condition.String()
	}
	return fmt.Sprintf("{%s for %s in %s%s}", sc.Element.String(), sc.Target.String(), sc.Iterable.String(), cond)
}

type LambdaExpression struct {
	PosInfo    token.Pos
	Parameters []Parameter
	Body       Expression
}

func (le *LambdaExpression) Pos() token.Pos  { return le.PosInfo }
func (le *LambdaExpression) expressionNode() {}
func (le *LambdaExpression) String() string {
	var params []string
	for _, p := range le.Parameters {
		params = append(params, p.Name)
	}
	return fmt.Sprintf("(lambda %s: %s)", strings.Join(params, ", "), le.Body.String())
}

type TernaryExpression struct {
	PosInfo     token.Pos
	Condition   Expression
	Consequence Expression
	Alternative Expression
}

func (te *TernaryExpression) Pos() token.Pos  { return te.PosInfo }
func (te *TernaryExpression) expressionNode() {}
func (te *TernaryExpression) String() string {
	return fmt.Sprintf("(%s if %s else %s)", te.Consequence.String(), te.Condition.String(), te.Alternative.String())
}

// ComplexLiteral: e.g. 1+2j or 3j
type ComplexLiteral struct {
	PosInfo token.Pos
	Real    float64
	Imag    float64
}

func (cl *ComplexLiteral) Pos() token.Pos  { return cl.PosInfo }
func (cl *ComplexLiteral) expressionNode() {}
func (cl *ComplexLiteral) String() string  { return fmt.Sprintf("(%g+%gj)", cl.Real, cl.Imag) }

// BytesLiteral: e.g. b"hello"
type BytesLiteral struct {
	PosInfo token.Pos
	Value   []byte
}

func (bl *BytesLiteral) Pos() token.Pos  { return bl.PosInfo }
func (bl *BytesLiteral) expressionNode() {}
func (bl *BytesLiteral) String() string  { return fmt.Sprintf("b%q", string(bl.Value)) }

// EllipsisLiteral: ...
type EllipsisLiteral struct {
	PosInfo token.Pos
}

func (el *EllipsisLiteral) Pos() token.Pos  { return el.PosInfo }
func (el *EllipsisLiteral) expressionNode() {}
func (el *EllipsisLiteral) String() string  { return "..." }

// WalrusExpression: x := expr
type WalrusExpression struct {
	PosInfo token.Pos
	Target  Expression
	Value   Expression
}

func (we *WalrusExpression) Pos() token.Pos  { return we.PosInfo }
func (we *WalrusExpression) expressionNode() {}
func (we *WalrusExpression) String() string {
	return fmt.Sprintf("(%s := %s)", we.Target.String(), we.Value.String())
}

// StarredExpression: *rest or *args
type StarredExpression struct {
	PosInfo token.Pos
	Value   Expression
}

func (se *StarredExpression) Pos() token.Pos  { return se.PosInfo }
func (se *StarredExpression) expressionNode() {}
func (se *StarredExpression) String() string  { return "*" + se.Value.String() }

// YieldExpression: yield expr, yield from expr
type YieldExpression struct {
	PosInfo     token.Pos
	Value       Expression
	IsYieldFrom bool
}

func (ye *YieldExpression) Pos() token.Pos  { return ye.PosInfo }
func (ye *YieldExpression) expressionNode() {}
func (ye *YieldExpression) String() string {
	if ye.IsYieldFrom {
		return fmt.Sprintf("yield from %s", ye.Value.String())
	}
	if ye.Value != nil {
		return fmt.Sprintf("yield %s", ye.Value.String())
	}
	return "yield"
}

// AwaitExpression: await coro()
type AwaitExpression struct {
	PosInfo token.Pos
	Value   Expression
}

func (ae *AwaitExpression) Pos() token.Pos  { return ae.PosInfo }
func (ae *AwaitExpression) expressionNode() {}
func (ae *AwaitExpression) String() string  { return fmt.Sprintf("await %s", ae.Value.String()) }

// Pattern Matching: match / case
type MatchCase struct {
	PosInfo token.Pos
	Pattern Expression
	Guard   Expression // if guard
	Body    *BlockStatement
}

type MatchStatement struct {
	PosInfo token.Pos
	Subject Expression
	Cases   []MatchCase
}

func (ms *MatchStatement) Pos() token.Pos { return ms.PosInfo }
func (ms *MatchStatement) statementNode() {}
func (ms *MatchStatement) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("match %s:\n", ms.Subject.String()))
	for _, c := range ms.Cases {
		guard := ""
		if c.Guard != nil {
			guard = " if " + c.Guard.String()
		}
		sb.WriteString(fmt.Sprintf("  case %s%s:\n%s", c.Pattern.String(), guard, c.Body.String()))
	}
	return sb.String()
}
