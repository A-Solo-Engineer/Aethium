package compiler

import (
	"fmt"
	"strings"

	"aethium/pkg/ast"
	"aethium/pkg/bytecode"
)

type EmittedInstruction struct {
	Opcode   bytecode.Opcode
	Position int
}

type CompilationScope struct {
	instructions        bytecode.Instructions
	lastInstruction     EmittedInstruction
	previousInstruction EmittedInstruction
	breakJumps          []int
	continueJumps       []int
}

type Compiler struct {
	constants   []interface{}
	names       []string
	nameIndex   map[string]int
	symbolTable *SymbolTable
	scopes      []CompilationScope
	scopeIndex  int
	// filename is threaded into CompiledFunction for error messages.
	filename string
}

func NewCompiler() *Compiler {
	return NewCompilerWithFilename("<module>")
}

func NewCompilerWithFilename(filename string) *Compiler {
	mainScope := CompilationScope{
		instructions:        bytecode.Instructions{},
		lastInstruction:     EmittedInstruction{},
		previousInstruction: EmittedInstruction{},
	}

	symbolTable := NewSymbolTable()
	for i, v := range BuiltinNames {
		symbolTable.DefineBuiltin(i, v)
	}

	return &Compiler{
		constants:   []interface{}{},
		names:       []string{},
		nameIndex:   make(map[string]int),
		symbolTable: symbolTable,
		scopes:      []CompilationScope{mainScope},
		scopeIndex:  0,
		filename:    filename,
	}
}

// BuiltinNames is the ordered list of built-in function names.
// The index in this slice matches the GetBuiltin operand used at runtime.
var BuiltinNames = []string{
	"print", "len", "range", "type", "str", "int", "float", "bool",
	"list", "dict", "set", "tuple", "sum", "min", "max", "abs",
	"round", "map", "filter", "zip", "enumerate", "sorted", "reversed",
	"any", "all", "isinstance", "issubclass", "repr", "ord", "chr", "hex", "bin",
	"oct", "pow", "id", "hash", "input", "exit", "open", "chan", "spawn", "time", "sleep",
	"super", "next", "complex", "bytes", "bytearray", "frozenset", "memoryview",
	"help", "breakpoint", "classmethod", "staticmethod", "property",
}

func (c *Compiler) currentInstructions() bytecode.Instructions {
	return c.scopes[c.scopeIndex].instructions
}

func (c *Compiler) addConstant(obj interface{}) int {
	c.constants = append(c.constants, obj)
	return len(c.constants) - 1
}

func (c *Compiler) addName(name string) int {
	if idx, ok := c.nameIndex[name]; ok {
		return idx
	}
	c.names = append(c.names, name)
	idx := len(c.names) - 1
	c.nameIndex[name] = idx
	return idx
}

func (c *Compiler) emit(op bytecode.Opcode, operands ...int) int {
	ins := bytecode.Make(op, operands...)
	pos := c.addInstruction(ins)

	c.setLastInstruction(op, pos)
	return pos
}

func (c *Compiler) addInstruction(ins []byte) int {
	posNewInstruction := len(c.currentInstructions())
	c.scopes[c.scopeIndex].instructions = append(c.currentInstructions(), ins...)
	return posNewInstruction
}

func (c *Compiler) setLastInstruction(op bytecode.Opcode, pos int) {
	previous := c.scopes[c.scopeIndex].lastInstruction
	last := EmittedInstruction{Opcode: op, Position: pos}

	c.scopes[c.scopeIndex].previousInstruction = previous
	c.scopes[c.scopeIndex].lastInstruction = last
}

func (c *Compiler) changeOperand(opPos int, operand int) {
	op := bytecode.Opcode(c.currentInstructions()[opPos])
	newInstruction := bytecode.Make(op, operand)
	c.replaceInstruction(opPos, newInstruction)
}

func (c *Compiler) replaceInstruction(pos int, newInstruction []byte) {
	ins := c.currentInstructions()
	for i := 0; i < len(newInstruction); i++ {
		ins[pos+i] = newInstruction[i]
	}
}

func (c *Compiler) enterScope() {
	scope := CompilationScope{
		instructions:        bytecode.Instructions{},
		lastInstruction:     EmittedInstruction{},
		previousInstruction: EmittedInstruction{},
	}
	c.scopes = append(c.scopes, scope)
	c.scopeIndex++
	c.symbolTable = NewEnclosedSymbolTable(c.symbolTable)
}

func (c *Compiler) leaveScope() bytecode.Instructions {
	instructions := c.currentInstructions()
	c.scopes = c.scopes[:len(c.scopes)-1]
	c.scopeIndex--
	c.symbolTable = c.symbolTable.Outer
	return instructions
}

type Bytecode struct {
	Instructions bytecode.Instructions
	Constants    []interface{}
	Names        []string
}

func (c *Compiler) Bytecode() *Bytecode {
	return &Bytecode{
		Instructions: c.currentInstructions(),
		Constants:    c.constants,
		Names:        c.names,
	}
}

func (c *Compiler) Compile(node ast.Node) error {
	switch node := node.(type) {
	case *ast.Program:
		for _, s := range node.Statements {
			err := c.Compile(s)
			if err != nil {
				return err
			}
		}

	case *ast.BlockStatement:
		for _, s := range node.Statements {
			err := c.Compile(s)
			if err != nil {
				return err
			}
		}

	case *ast.ExpressionStatement:
		err := c.Compile(node.Expression)
		if err != nil {
			return err
		}
		c.emit(bytecode.OpPop)

	case *ast.IntegerLiteral:
		idx := c.addConstant(node.Value)
		c.emit(bytecode.OpConstant, idx)

	case *ast.FloatLiteral:
		idx := c.addConstant(node.Value)
		c.emit(bytecode.OpConstant, idx)

	case *ast.ComplexLiteral:
		idx := c.addConstant(complex(node.Real, node.Imag))
		c.emit(bytecode.OpConstant, idx)

	case *ast.StringLiteral:
		idx := c.addConstant(node.Value)
		c.emit(bytecode.OpConstant, idx)

	case *ast.BytesLiteral:
		idx := c.addConstant(node.Value)
		c.emit(bytecode.OpConstant, idx)

	case *ast.EllipsisLiteral:
		c.emit(bytecode.OpEllipsis)

	case *ast.BoolLiteral:
		if node.Value {
			c.emit(bytecode.OpTrue)
		} else {
			c.emit(bytecode.OpFalse)
		}

	case *ast.NoneLiteral:
		c.emit(bytecode.OpNone)

	case *ast.WalrusExpression:
		err := c.Compile(node.Value)
		if err != nil {
			return err
		}
		c.emit(bytecode.OpDup)
		if ident, ok := node.Target.(*ast.Identifier); ok {
			symbol, ok := c.symbolTable.Resolve(ident.Value)
			if !ok {
				symbol = c.symbolTable.Define(ident.Value)
			}
			c.storeSymbol(symbol, ident.Value)
		}

	case *ast.YieldExpression:
		if node.Value != nil {
			err := c.Compile(node.Value)
			if err != nil {
				return err
			}
		} else {
			c.emit(bytecode.OpNone)
		}
		c.emit(bytecode.OpYield)

	case *ast.AwaitExpression:
		err := c.Compile(node.Value)
		if err != nil {
			return err
		}
		c.emit(bytecode.OpAwait)

	case *ast.FStringLiteral:
		count := 0
		for _, part := range node.Parts {
			if part.IsExpr {
				err := c.Compile(part.Expr)
				if err != nil {
					return err
				}
				count++
			} else {
				idx := c.addConstant(part.Text)
				c.emit(bytecode.OpConstant, idx)
				count++
			}
		}
		c.emit(bytecode.OpBuildString, count)

	case *ast.Identifier:
		symbol, ok := c.symbolTable.Resolve(node.Value)
		if !ok {
			// If not found, treat as global/name lookup
			nameIdx := c.addName(node.Value)
			c.emit(bytecode.OpGetGlobal, nameIdx)
		} else {
			c.loadSymbol(symbol)
		}

	case *ast.UnaryExpression:
		err := c.Compile(node.Right)
		if err != nil {
			return err
		}
		switch node.Operator {
		case "-":
			c.emit(bytecode.OpMinus)
		case "+":
			c.emit(bytecode.OpPlus)
		case "not", "!":
			c.emit(bytecode.OpNot)
		case "~":
			c.emit(bytecode.OpBitNot)
		default:
			return fmt.Errorf("unknown unary operator %s", node.Operator)
		}

	case *ast.BinaryExpression:
		if node.Operator == "and" {
			err := c.Compile(node.Left)
			if err != nil {
				return err
			}
			jumpPos := c.emit(bytecode.OpJumpIfFalseOrPop, 9999)
			err = c.Compile(node.Right)
			if err != nil {
				return err
			}
			afterRightPos := len(c.currentInstructions())
			c.changeOperand(jumpPos, afterRightPos)
			return nil
		}

		if node.Operator == "or" {
			err := c.Compile(node.Left)
			if err != nil {
				return err
			}
			jumpPos := c.emit(bytecode.OpJumpIfTrueOrPop, 9999)
			err = c.Compile(node.Right)
			if err != nil {
				return err
			}
			afterRightPos := len(c.currentInstructions())
			c.changeOperand(jumpPos, afterRightPos)
			return nil
		}

		err := c.Compile(node.Left)
		if err != nil {
			return err
		}
		err = c.Compile(node.Right)
		if err != nil {
			return err
		}

		switch node.Operator {
		case "+":
			c.emit(bytecode.OpAdd)
		case "-":
			c.emit(bytecode.OpSub)
		case "*":
			c.emit(bytecode.OpMul)
		case "/":
			c.emit(bytecode.OpDiv)
		case "//":
			c.emit(bytecode.OpFloorDiv)
		case "%":
			c.emit(bytecode.OpMod)
		case "**":
			c.emit(bytecode.OpPow)
		case "&":
			c.emit(bytecode.OpBitAnd)
		case "|":
			c.emit(bytecode.OpBitOr)
		case "^":
			c.emit(bytecode.OpBitXor)
		case "<<":
			c.emit(bytecode.OpLShift)
		case ">>":
			c.emit(bytecode.OpRShift)
		case "==":
			c.emit(bytecode.OpEqual)
		case "!=":
			c.emit(bytecode.OpNotEqual)
		case "<":
			c.emit(bytecode.OpLessThan)
		case ">":
			c.emit(bytecode.OpGreaterThan)
		case "<=":
			c.emit(bytecode.OpLessEqual)
		case ">=":
			c.emit(bytecode.OpGreaterEqual)
		case "in":
			c.emit(bytecode.OpIn)
		case "not in":
			c.emit(bytecode.OpNotIn)
		case "is":
			c.emit(bytecode.OpIs)
		case "is not":
			c.emit(bytecode.OpIsNot)
		default:
			return fmt.Errorf("unknown binary operator %s", node.Operator)
		}

	case *ast.TernaryExpression:
		err := c.Compile(node.Condition)
		if err != nil {
			return err
		}
		jumpFalsePos := c.emit(bytecode.OpJumpIfFalse, 9999)
		err = c.Compile(node.Consequence)
		if err != nil {
			return err
		}
		jumpEndPos := c.emit(bytecode.OpJump, 9999)
		afterConseqPos := len(c.currentInstructions())
		c.changeOperand(jumpFalsePos, afterConseqPos)

		err = c.Compile(node.Alternative)
		if err != nil {
			return err
		}
		afterAltPos := len(c.currentInstructions())
		c.changeOperand(jumpEndPos, afterAltPos)

	case *ast.ListLiteral:
		for _, el := range node.Elements {
			err := c.Compile(el)
			if err != nil {
				return err
			}
		}
		c.emit(bytecode.OpBuildList, len(node.Elements))

	case *ast.TupleLiteral:
		for _, el := range node.Elements {
			err := c.Compile(el)
			if err != nil {
				return err
			}
		}
		c.emit(bytecode.OpBuildTuple, len(node.Elements))

	case *ast.DictLiteral:
		for _, entry := range node.Entries {
			err := c.Compile(entry.Key)
			if err != nil {
				return err
			}
			err = c.Compile(entry.Value)
			if err != nil {
				return err
			}
		}
		c.emit(bytecode.OpBuildDict, len(node.Entries))

	case *ast.SetLiteral:
		for _, el := range node.Elements {
			err := c.Compile(el)
			if err != nil {
				return err
			}
		}
		c.emit(bytecode.OpBuildSet, len(node.Elements))

	case *ast.IndexExpression:
		err := c.Compile(node.Left)
		if err != nil {
			return err
		}
		err = c.Compile(node.Index)
		if err != nil {
			return err
		}
		c.emit(bytecode.OpGetIndex)

	case *ast.SliceExpression:
		err := c.Compile(node.Left)
		if err != nil {
			return err
		}
		if node.Start != nil {
			err = c.Compile(node.Start)
			if err != nil {
				return err
			}
		} else {
			c.emit(bytecode.OpNone)
		}
		if node.End != nil {
			err = c.Compile(node.End)
			if err != nil {
				return err
			}
		} else {
			c.emit(bytecode.OpNone)
		}
		if node.Step != nil {
			err = c.Compile(node.Step)
			if err != nil {
				return err
			}
		} else {
			c.emit(bytecode.OpNone)
		}
		c.emit(bytecode.OpGetSlice)

	case *ast.AttributeExpression:
		err := c.Compile(node.Object)
		if err != nil {
			return err
		}
		nameIdx := c.addName(node.Attribute)
		c.emit(bytecode.OpGetAttr, nameIdx)

	case *ast.AssignStatement:
		// Check for target type
		if len(node.Targets) == 1 {
			target := node.Targets[0]
			switch tgt := target.(type) {
			case *ast.Identifier:
				if node.Operator == "=" {
					err := c.Compile(node.Value)
					if err != nil {
						return err
					}
				} else {
					// Augmented assignment: e.g. x += 1
					symbol, ok := c.symbolTable.Resolve(tgt.Value)
					if !ok {
						nameIdx := c.addName(tgt.Value)
						c.emit(bytecode.OpGetGlobal, nameIdx)
					} else {
						c.loadSymbol(symbol)
					}
					err := c.Compile(node.Value)
					if err != nil {
						return err
					}
					baseOp := strings.TrimSuffix(node.Operator, "=")
					c.emitAugmentedOp(baseOp)
				}
				symbol, ok := c.symbolTable.Resolve(tgt.Value)
				if !ok {
					symbol = c.symbolTable.Define(tgt.Value)
				}
				c.storeSymbol(symbol, tgt.Value)

			case *ast.AttributeExpression:
				err := c.Compile(tgt.Object)
				if err != nil {
					return err
				}
				if node.Operator == "=" {
					err = c.Compile(node.Value)
					if err != nil {
						return err
					}
				} else {
					c.emit(bytecode.OpDup)
					nameIdx := c.addName(tgt.Attribute)
					c.emit(bytecode.OpGetAttr, nameIdx)
					err = c.Compile(node.Value)
					if err != nil {
						return err
					}
					baseOp := strings.TrimSuffix(node.Operator, "=")
					c.emitAugmentedOp(baseOp)
				}
				nameIdx := c.addName(tgt.Attribute)
				c.emit(bytecode.OpSetAttr, nameIdx)

			case *ast.SliceExpression:
				err := c.Compile(tgt.Left)
				if err != nil {
					return err
				}
				if tgt.Start != nil {
					err = c.Compile(tgt.Start)
					if err != nil {
						return err
					}
				} else {
					c.emit(bytecode.OpNone)
				}
				if tgt.End != nil {
					err = c.Compile(tgt.End)
					if err != nil {
						return err
					}
				} else {
					c.emit(bytecode.OpNone)
				}
				if tgt.Step != nil {
					err = c.Compile(tgt.Step)
					if err != nil {
						return err
					}
				} else {
					c.emit(bytecode.OpNone)
				}
				err = c.Compile(node.Value)
				if err != nil {
					return err
				}
				c.emit(bytecode.OpSetSlice)
			case *ast.IndexExpression:
				err := c.Compile(tgt.Left)
				if err != nil {
					return err
				}
				err = c.Compile(tgt.Index)
				if err != nil {
					return err
				}
				if node.Operator == "=" {
					err = c.Compile(node.Value)
					if err != nil {
						return err
					}
				} else {
					c.emit(bytecode.OpDup)
					// Need to get index item
					err = c.Compile(node.Value)
					if err != nil {
						return err
					}
					baseOp := strings.TrimSuffix(node.Operator, "=")
					c.emitAugmentedOp(baseOp)
				}
				c.emit(bytecode.OpSetIndex)
			}
		} else {
			// Unpacking: a, b = c or a, *rest, b = c
			starredIdx := -1
			for i, tgt := range node.Targets {
				if _, ok := tgt.(*ast.StarredExpression); ok {
					starredIdx = i
					break
				}
			}
			err := c.Compile(node.Value)
			if err != nil {
				return err
			}
			if starredIdx >= 0 {
				beforeCount := starredIdx
				afterCount := len(node.Targets) - 1 - starredIdx
				c.emit(bytecode.OpUnpackRest, beforeCount, afterCount)
				for _, tgt := range node.Targets {
					if star, ok := tgt.(*ast.StarredExpression); ok {
						c.compileAssignTarget(star.Value)
					} else {
						c.compileAssignTarget(tgt)
					}
				}
			} else {
				c.emit(bytecode.OpUnpackSequence, len(node.Targets))
				for _, tgt := range node.Targets {
					c.compileAssignTarget(tgt)
				}
			}
		}

	case *ast.IfStatement:
		err := c.Compile(node.Condition)
		if err != nil {
			return err
		}
		jumpFalsePos := c.emit(bytecode.OpJumpIfFalse, 9999)

		err = c.Compile(node.Consequence)
		if err != nil {
			return err
		}

		var jumpEndPositions []int
		jumpEndPos := c.emit(bytecode.OpJump, 9999)
		jumpEndPositions = append(jumpEndPositions, jumpEndPos)

		afterConseqPos := len(c.currentInstructions())
		c.changeOperand(jumpFalsePos, afterConseqPos)

		for _, elif := range node.Elifs {
			err = c.Compile(elif.Condition)
			if err != nil {
				return err
			}
			elifFalsePos := c.emit(bytecode.OpJumpIfFalse, 9999)
			err = c.Compile(elif.Consequence)
			if err != nil {
				return err
			}
			jEnd := c.emit(bytecode.OpJump, 9999)
			jumpEndPositions = append(jumpEndPositions, jEnd)
			c.changeOperand(elifFalsePos, len(c.currentInstructions()))
		}

		if node.Alternative != nil {
			err = c.Compile(node.Alternative)
			if err != nil {
				return err
			}
		}

		endPos := len(c.currentInstructions())
		for _, pos := range jumpEndPositions {
			c.changeOperand(pos, endPos)
		}

	case *ast.WhileStatement:
		loopStart := len(c.currentInstructions())
		savedBreaks := c.scopes[c.scopeIndex].breakJumps
		savedContinues := c.scopes[c.scopeIndex].continueJumps
		c.scopes[c.scopeIndex].breakJumps = []int{}
		c.scopes[c.scopeIndex].continueJumps = []int{}

		err := c.Compile(node.Condition)
		if err != nil {
			return err
		}
		jumpFalsePos := c.emit(bytecode.OpJumpIfFalse, 9999)

		err = c.Compile(node.Body)
		if err != nil {
			return err
		}

		c.emit(bytecode.OpJump, loopStart)
		loopEnd := len(c.currentInstructions())
		c.changeOperand(jumpFalsePos, loopEnd)

		// Patch break and continue
		for _, pos := range c.scopes[c.scopeIndex].breakJumps {
			c.changeOperand(pos, loopEnd)
		}
		for _, pos := range c.scopes[c.scopeIndex].continueJumps {
			c.changeOperand(pos, loopStart)
		}

		c.scopes[c.scopeIndex].breakJumps = savedBreaks
		c.scopes[c.scopeIndex].continueJumps = savedContinues

	case *ast.ForInStatement:
		savedBreaks := c.scopes[c.scopeIndex].breakJumps
		savedContinues := c.scopes[c.scopeIndex].continueJumps
		c.scopes[c.scopeIndex].breakJumps = []int{}
		c.scopes[c.scopeIndex].continueJumps = []int{}

		err := c.Compile(node.Iterable)
		if err != nil {
			return err
		}
		c.emit(bytecode.OpGetIter)

		loopStart := len(c.currentInstructions())
		forIterPos := c.emit(bytecode.OpForIter, 9999)

		// Assign iter item to target (supports tuple unpacking).
		c.compileAssignTarget(node.Target)

		err = c.Compile(node.Body)
		if err != nil {
			return err
		}

		c.emit(bytecode.OpJump, loopStart)
		loopEnd := len(c.currentInstructions())
		c.changeOperand(forIterPos, loopEnd)

		for _, pos := range c.scopes[c.scopeIndex].breakJumps {
			c.changeOperand(pos, loopEnd)
		}
		for _, pos := range c.scopes[c.scopeIndex].continueJumps {
			c.changeOperand(pos, loopStart)
		}

		c.scopes[c.scopeIndex].breakJumps = savedBreaks
		c.scopes[c.scopeIndex].continueJumps = savedContinues

	case *ast.BreakStatement:
		pos := c.emit(bytecode.OpJump, 9999)
		c.scopes[c.scopeIndex].breakJumps = append(c.scopes[c.scopeIndex].breakJumps, pos)

	case *ast.ContinueStatement:
		pos := c.emit(bytecode.OpJump, 9999)
		c.scopes[c.scopeIndex].continueJumps = append(c.scopes[c.scopeIndex].continueJumps, pos)

	case *ast.PassStatement:

	case *ast.GlobalStatement:
		for _, name := range node.Names {
			c.symbolTable.DefineGlobal(name)
		}

	// NonlocalStatement shares the same AST node shape as GlobalStatement in
	// most parsers; if the parser produces a separate type add a case for it.
	// We handle it via DefineNonlocal in the symbol table.

	case *ast.AssertStatement:
		err := c.Compile(node.Condition)
		if err != nil {
			return err
		}
		if node.Message != nil {
			err = c.Compile(node.Message)
			if err != nil {
				return err
			}
		} else {
			c.emit(bytecode.OpNone)
		}
		c.emit(bytecode.OpAssert)

	case *ast.DeleteStatement:
		for _, tgt := range node.Targets {
			switch t := tgt.(type) {
			case *ast.Identifier:
				nameIdx := c.addName(t.Value)
				c.emit(bytecode.OpDeleteName, nameIdx)
			case *ast.IndexExpression:
				if err := c.Compile(t.Left); err != nil {
					return err
				}
				if err := c.Compile(t.Index); err != nil {
					return err
				}
				c.emit(bytecode.OpDeleteIndex)
			case *ast.AttributeExpression:
				if err := c.Compile(t.Object); err != nil {
					return err
				}
				nameIdx := c.addName(t.Attribute)
				c.emit(bytecode.OpDeleteAttr, nameIdx)
			}
		}

	case *ast.MatchStatement:
		err := c.Compile(node.Subject)
		if err != nil {
			return err
		}
		var endJumps []int
		for _, caseClause := range node.Cases {
			var nextCaseJump int = -1
			if ident, ok := caseClause.Pattern.(*ast.Identifier); ok && (ident.Value == "_" || ident.Value == "case") {
				// Wildcard matches anything
				if caseClause.Guard != nil {
					if err := c.Compile(caseClause.Guard); err != nil {
						return err
					}
					nextCaseJump = c.emit(bytecode.OpJumpIfFalse, 9999)
				}
			} else {
				c.emit(bytecode.OpDup)
				if err := c.Compile(caseClause.Pattern); err != nil {
					return err
				}
				c.emit(bytecode.OpEqual)
				nextCaseJump = c.emit(bytecode.OpJumpIfFalse, 9999)
				if caseClause.Guard != nil {
					if err := c.Compile(caseClause.Guard); err != nil {
						return err
					}
					guardJump := c.emit(bytecode.OpJumpIfFalse, 9999)
					c.changeOperand(guardJump, nextCaseJump)
				}
			}
			if err := c.Compile(caseClause.Body); err != nil {
				return err
			}
			endJumps = append(endJumps, c.emit(bytecode.OpJump, 9999))
			if nextCaseJump >= 0 {
				c.changeOperand(nextCaseJump, len(c.currentInstructions()))
			}
		}
		c.emit(bytecode.OpPop) // Pop subject off stack
		endPos := len(c.currentInstructions())
		for _, pos := range endJumps {
			c.changeOperand(pos, endPos)
		}

	case *ast.FunctionDef:
		// Compile default argument expressions *before* entering the new scope
		// so they evaluate in the enclosing environment.
		defaultCount := 0
		for _, p := range node.Parameters {
			if p.DefaultValue != nil && !p.IsVarArg && !p.IsKwArg {
				if err := c.Compile(p.DefaultValue); err != nil {
					return err
				}
				defaultCount++
			}
		}

		c.enterScope()
		c.symbolTable.DefineFunctionName(node.Name)

		var paramNames []string
		varArgName := ""
		kwArgName := ""
		positionalCount := 0

		for _, p := range node.Parameters {
			c.symbolTable.Define(p.Name)
			paramNames = append(paramNames, p.Name)
			switch {
			case p.IsVarArg:
				varArgName = p.Name
			case p.IsKwArg:
				kwArgName = p.Name
			default:
				positionalCount++
			}
		}

		if err := c.Compile(node.Body); err != nil {
			return err
		}
		if !c.lastInstructionIs(bytecode.OpReturnValue) && !c.lastInstructionIs(bytecode.OpReturnNone) {
			c.emit(bytecode.OpReturnNone)
		}

		freeSymbols := c.symbolTable.FreeSymbols
		numLocals := c.symbolTable.numDefinitions
		instructions := c.leaveScope()

		for _, s := range freeSymbols {
			c.loadSymbol(s)
		}

		hasYield := containsYield(node.Body)
		compiledFn := &bytecode.CompiledFunction{
			Instructions:  instructions,
			NumLocals:     numLocals,
			NumParameters: positionalCount,
			ParamNames:    paramNames,
			DefaultCount:  defaultCount,
			VarArg:        varArgName,
			KwArg:         kwArgName,
			IsGenerator:   hasYield,
			Name:          node.Name,
			Doc:           node.Doc,
			Filename:      c.filename,
		}

		fnIndex := c.addConstant(compiledFn)
		c.emit(bytecode.OpClosure, fnIndex, len(freeSymbols))

		for i := len(node.Decorators) - 1; i >= 0; i-- {
			if err := c.Compile(node.Decorators[i]); err != nil {
				return err
			}
			c.emit(bytecode.OpRot2) // swap: decorator, fn → fn, decorator
			c.emit(bytecode.OpCall, 1, 0)
		}

		symbol, ok := c.symbolTable.Resolve(node.Name)
		if !ok {
			symbol = c.symbolTable.Define(node.Name)
		}
		c.storeSymbol(symbol, node.Name)

	case *ast.LambdaExpression:
		defaultCount := 0
		for _, p := range node.Parameters {
			if p.DefaultValue != nil && !p.IsVarArg && !p.IsKwArg {
				if err := c.Compile(p.DefaultValue); err != nil {
					return err
				}
				defaultCount++
			}
		}

		c.enterScope()
		var paramNames []string
		varArgName := ""
		kwArgName := ""
		positionalCount := 0
		for _, p := range node.Parameters {
			c.symbolTable.Define(p.Name)
			paramNames = append(paramNames, p.Name)
			switch {
			case p.IsVarArg:
				varArgName = p.Name
			case p.IsKwArg:
				kwArgName = p.Name
			default:
				positionalCount++
			}
		}
		if err := c.Compile(node.Body); err != nil {
			return err
		}
		c.emit(bytecode.OpReturnValue)

		freeSymbols := c.symbolTable.FreeSymbols
		numLocals := c.symbolTable.numDefinitions
		instructions := c.leaveScope()

		for _, s := range freeSymbols {
			c.loadSymbol(s)
		}

		compiledFn := &bytecode.CompiledFunction{
			Instructions:  instructions,
			NumLocals:     numLocals,
			NumParameters: positionalCount,
			ParamNames:    paramNames,
			DefaultCount:  defaultCount,
			VarArg:        varArgName,
			KwArg:         kwArgName,
			Name:          "<lambda>",
			Filename:      c.filename,
		}
		fnIndex := c.addConstant(compiledFn)
		c.emit(bytecode.OpClosure, fnIndex, len(freeSymbols))

	case *ast.ClassDef:
		for _, base := range node.Bases {
			if err := c.Compile(base); err != nil {
				return err
			}
		}

		methodCount := 0
		for _, s := range node.Body.Statements {
			if as, isAssign := s.(*ast.AssignStatement); isAssign {
				if len(as.Targets) > 0 {
					if ident, isIdent := as.Targets[0].(*ast.Identifier); isIdent {
						idx := c.addConstant(ident.Value)
						c.emit(bytecode.OpConstant, idx)
						if err := c.Compile(as.Value); err != nil {
							return err
						}
						methodCount++
						continue
					}
				}
			}
			fn, ok := s.(*ast.FunctionDef)
			if !ok {
				continue
			}
			idx := c.addConstant(fn.Name)
			c.emit(bytecode.OpConstant, idx)

			// Compile method with full parameter support.
			defaultCount := 0
			for _, p := range fn.Parameters {
				if p.DefaultValue != nil && !p.IsVarArg && !p.IsKwArg {
					if err := c.Compile(p.DefaultValue); err != nil {
						return err
					}
					defaultCount++
				}
			}

			c.enterScope()
			c.symbolTable.DefineFunctionName(fn.Name)
			var paramNames []string
			varArgName := ""
			kwArgName := ""
			positionalCount := 0
			for _, p := range fn.Parameters {
				c.symbolTable.Define(p.Name)
				paramNames = append(paramNames, p.Name)
				switch {
				case p.IsVarArg:
					varArgName = p.Name
				case p.IsKwArg:
					kwArgName = p.Name
				default:
					positionalCount++
				}
			}
			if err := c.Compile(fn.Body); err != nil {
				return err
			}
			if !c.lastInstructionIs(bytecode.OpReturnValue) && !c.lastInstructionIs(bytecode.OpReturnNone) {
				c.emit(bytecode.OpReturnNone)
			}
			freeSymbols := c.symbolTable.FreeSymbols
			numLocals := c.symbolTable.numDefinitions
			instructions := c.leaveScope()

			for _, s2 := range freeSymbols {
				c.loadSymbol(s2)
			}
			compiledFn := &bytecode.CompiledFunction{
				Instructions:  instructions,
				NumLocals:     numLocals,
				NumParameters: positionalCount,
				ParamNames:    paramNames,
				DefaultCount:  defaultCount,
				VarArg:        varArgName,
				KwArg:         kwArgName,
				Name:          fn.Name,
				Filename:      c.filename,
			}
			fnIndex := c.addConstant(compiledFn)
			c.emit(bytecode.OpClosure, fnIndex, len(freeSymbols))

			// Apply method decorators
			for i := len(fn.Decorators) - 1; i >= 0; i-- {
				if err := c.Compile(fn.Decorators[i]); err != nil {
					return err
				}
				c.emit(bytecode.OpRot2)
				c.emit(bytecode.OpCall, 1, 0)
			}

			methodCount++
		}

		nameIdx := c.addName(node.Name)
		c.emit(bytecode.OpMakeClass, nameIdx, len(node.Bases), methodCount)

		// Apply class decorators.
		for i := len(node.Decorators) - 1; i >= 0; i-- {
			if err := c.Compile(node.Decorators[i]); err != nil {
				return err
			}
			c.emit(bytecode.OpRot2)
			c.emit(bytecode.OpCall, 1, 0)
		}

		symbol, ok := c.symbolTable.Resolve(node.Name)
		if !ok {
			symbol = c.symbolTable.Define(node.Name)
		}
		c.storeSymbol(symbol, node.Name)

	case *ast.CallExpression:
		if err := c.Compile(node.Function); err != nil {
			return err
		}
		for _, a := range node.Arguments {
			if err := c.Compile(a); err != nil {
				return err
			}
		}
		if len(node.Keywords) > 0 {
			// Push keyword pairs then build a kwargs dict.
			for _, kw := range node.Keywords {
				keyIdx := c.addConstant(kw.Key)
				c.emit(bytecode.OpConstant, keyIdx)
				if err := c.Compile(kw.Value); err != nil {
					return err
				}
			}
			c.emit(bytecode.OpMakeKwargs, len(node.Keywords))
			c.emit(bytecode.OpCallKw, len(node.Arguments), len(node.Keywords))
		} else {
			c.emit(bytecode.OpCall, len(node.Arguments), 0)
		}

	case *ast.ReturnStatement:
		if node.Value != nil {
			if err := c.Compile(node.Value); err != nil {
				return err
			}
			c.emit(bytecode.OpReturnValue)
		} else {
			c.emit(bytecode.OpReturnNone)
		}

	case *ast.ListComp:
		c.emit(bytecode.OpBuildList, 0)
		if err := c.Compile(node.Iterable); err != nil {
			return err
		}
		c.emit(bytecode.OpGetIter)
		loopStart := len(c.currentInstructions())
		forIterPos := c.emit(bytecode.OpForIter, 9999)

		c.compileAssignTarget(node.Target)

		jumpCondFalse := -1
		if node.Condition != nil {
			if err := c.Compile(node.Condition); err != nil {
				return err
			}
			jumpCondFalse = c.emit(bytecode.OpJumpIfFalse, 9999)
		}
		if err := c.Compile(node.Element); err != nil {
			return err
		}
		c.emit(bytecode.OpListAppend)
		if jumpCondFalse >= 0 {
			c.changeOperand(jumpCondFalse, len(c.currentInstructions()))
		}
		c.emit(bytecode.OpJump, loopStart)
		c.changeOperand(forIterPos, len(c.currentInstructions()))

	case *ast.DictComp:
		c.emit(bytecode.OpBuildDict, 0)
		if err := c.Compile(node.Iterable); err != nil {
			return err
		}
		c.emit(bytecode.OpGetIter)
		loopStart := len(c.currentInstructions())
		forIterPos := c.emit(bytecode.OpForIter, 9999)

		c.compileAssignTarget(node.Target)

		jumpCondFalse := -1
		if node.Condition != nil {
			if err := c.Compile(node.Condition); err != nil {
				return err
			}
			jumpCondFalse = c.emit(bytecode.OpJumpIfFalse, 9999)
		}
		if err := c.Compile(node.Key); err != nil {
			return err
		}
		if err := c.Compile(node.Value); err != nil {
			return err
		}
		c.emit(bytecode.OpDictAdd)
		if jumpCondFalse >= 0 {
			c.changeOperand(jumpCondFalse, len(c.currentInstructions()))
		}
		c.emit(bytecode.OpJump, loopStart)
		c.changeOperand(forIterPos, len(c.currentInstructions()))

	case *ast.SetComp:
		c.emit(bytecode.OpBuildSet, 0)
		if err := c.Compile(node.Iterable); err != nil {
			return err
		}
		c.emit(bytecode.OpGetIter)
		loopStart := len(c.currentInstructions())
		forIterPos := c.emit(bytecode.OpForIter, 9999)

		c.compileAssignTarget(node.Target)

		jumpCondFalse := -1
		if node.Condition != nil {
			if err := c.Compile(node.Condition); err != nil {
				return err
			}
			jumpCondFalse = c.emit(bytecode.OpJumpIfFalse, 9999)
		}
		if err := c.Compile(node.Element); err != nil {
			return err
		}
		// Reuse ListAppend; the VM checks the accumulator type and dispatches.
		c.emit(bytecode.OpListAppend)
		if jumpCondFalse >= 0 {
			c.changeOperand(jumpCondFalse, len(c.currentInstructions()))
		}
		c.emit(bytecode.OpJump, loopStart)
		c.changeOperand(forIterPos, len(c.currentInstructions()))

	case *ast.TryExceptStatement:
		// Layout:
		//   OpSetupExcept → handler_start
		//   <try body>
		//   OpPopExcept
		//   <else body>      (if any)
		//   OpJump → finally_start
		// handler_start:
		//   for each handler: type-check, store alias, body, jump → finally_start
		// finally_start:
		//   <finally body>   (if any)

		exceptJumpPos := c.emit(bytecode.OpSetupExcept, 9999)
		if err := c.Compile(node.Body); err != nil {
			return err
		}
		c.emit(bytecode.OpPopExcept)

		if node.ElseBody != nil {
			if err := c.Compile(node.ElseBody); err != nil {
				return err
			}
		}

		// Collect all jumps that need to land at finally_start.
		var jumpToFinallyPositions []int
		jumpToFinallyPositions = append(jumpToFinallyPositions, c.emit(bytecode.OpJump, 9999))

		handlerStartPos := len(c.currentInstructions())
		c.changeOperand(exceptJumpPos, handlerStartPos)

		for i, handler := range node.Handlers {
			// The VM pushes the exception on the stack at handler entry.
			// If the handler has a type filter, check it first.
			var skipHandlerJump int = -1
			if handler.Exception != nil && i < len(node.Handlers)-1 {
				// Dup exception, compile expected type, check isinstance.
				c.emit(bytecode.OpDup)
				if err := c.Compile(handler.Exception); err != nil {
					return err
				}
				// OpIs used as isinstance check sentinel; VM handles it.
				c.emit(bytecode.OpIs)
				skipHandlerJump = c.emit(bytecode.OpJumpIfFalse, 9999)
			}

			// Bind exception alias.
			if handler.Alias != "" {
				symbol, ok := c.symbolTable.Resolve(handler.Alias)
				if !ok {
					symbol = c.symbolTable.Define(handler.Alias)
				}
				c.storeSymbol(symbol, handler.Alias)
			} else {
				c.emit(bytecode.OpPop) // discard exception value
			}

			if err := c.Compile(handler.Body); err != nil {
				return err
			}

			jumpToFinallyPositions = append(jumpToFinallyPositions, c.emit(bytecode.OpJump, 9999))

			if skipHandlerJump >= 0 {
				c.changeOperand(skipHandlerJump, len(c.currentInstructions()))
			}
		}

		finallyStartPos := len(c.currentInstructions())
		for _, pos := range jumpToFinallyPositions {
			c.changeOperand(pos, finallyStartPos)
		}

		if node.FinallyBody != nil {
			if err := c.Compile(node.FinallyBody); err != nil {
				return err
			}
		}

	case *ast.RaiseStatement:
		if node.Value != nil {
			if err := c.Compile(node.Value); err != nil {
				return err
			}
		} else {
			c.emit(bytecode.OpNone)
		}
		if node.Cause != nil {
			if err := c.Compile(node.Cause); err != nil {
				return err
			}
			c.emit(bytecode.OpRaiseFrom)
		} else {
			c.emit(bytecode.OpRaise)
		}

	case *ast.ImportStatement:
		for _, item := range node.Names {
			modIdx := c.addName(item.Name)
			c.emit(bytecode.OpImport, modIdx)
			targetName := item.Name
			if item.Alias != "" {
				targetName = item.Alias
			}
			symbol, ok := c.symbolTable.Resolve(targetName)
			if !ok {
				symbol = c.symbolTable.Define(targetName)
			}
			c.storeSymbol(symbol, targetName)
		}

	case *ast.FromImportStatement:
		modIdx := c.addName(node.Module)
		c.emit(bytecode.OpImport, modIdx)
		for _, item := range node.Names {
			c.emit(bytecode.OpDup)
			attrIdx := c.addName(item.Name)
			c.emit(bytecode.OpGetAttr, attrIdx)
			targetName := item.Name
			if item.Alias != "" {
				targetName = item.Alias
			}
			symbol, ok := c.symbolTable.Resolve(targetName)
			if !ok {
				symbol = c.symbolTable.Define(targetName)
			}
			c.storeSymbol(symbol, targetName)
		}
		c.emit(bytecode.OpPop)

	case *ast.GoSpawnStatement:
		if err := c.Compile(node.Call.Function); err != nil {
			return err
		}
		for _, a := range node.Call.Arguments {
			if err := c.Compile(a); err != nil {
				return err
			}
		}
		c.emit(bytecode.OpGoSpawn, len(node.Call.Arguments))

	default:
		// Unsupported AST node: return an explicit error rather than silently
		// ignoring it so callers can detect incomplete compilation.
		return fmt.Errorf("compiler: unsupported AST node type %T at %v",
			node, posOf(node))
	}

	return nil
}

// posOf extracts the source position string from any ast.Node.
func posOf(n ast.Node) string {
	if n == nil {
		return "<nil>"
	}
	p := n.Pos()
	return fmt.Sprintf("line %d col %d", p.Line, p.Col)
}

// compileAssignTarget stores the top-of-stack value into the assignment target.
// Used by comprehension loops where the loop variable may be a plain identifier
// or a tuple pattern.
func (c *Compiler) compileAssignTarget(target ast.Expression) {
	switch t := target.(type) {
	case *ast.Identifier:
		symbol, ok := c.symbolTable.Resolve(t.Value)
		if !ok {
			symbol = c.symbolTable.Define(t.Value)
		}
		c.storeSymbol(symbol, t.Value)
	case *ast.TupleLiteral:
		c.emit(bytecode.OpUnpackSequence, len(t.Elements))
		for _, el := range t.Elements {
			c.compileAssignTarget(el)
		}
	default:
		// Fallback: treat as identifier if possible.
		if ident, ok := target.(*ast.Identifier); ok {
			symbol, found := c.symbolTable.Resolve(ident.Value)
			if !found {
				symbol = c.symbolTable.Define(ident.Value)
			}
			c.storeSymbol(symbol, ident.Value)
		}
	}
}

func (c *Compiler) emitAugmentedOp(op string) {
	switch op {
	case "+":
		c.emit(bytecode.OpAdd)
	case "-":
		c.emit(bytecode.OpSub)
	case "*":
		c.emit(bytecode.OpMul)
	case "/":
		c.emit(bytecode.OpDiv)
	case "//":
		c.emit(bytecode.OpFloorDiv)
	case "%":
		c.emit(bytecode.OpMod)
	case "**":
		c.emit(bytecode.OpPow)
	case "&":
		c.emit(bytecode.OpBitAnd)
	case "|":
		c.emit(bytecode.OpBitOr)
	case "^":
		c.emit(bytecode.OpBitXor)
	case "<<":
		c.emit(bytecode.OpLShift)
	case ">>":
		c.emit(bytecode.OpRShift)
	}
}

func (c *Compiler) loadSymbol(s Symbol) {
	switch s.Scope {
	case GlobalScope:
		nameIdx := c.addName(s.Name)
		c.emit(bytecode.OpGetGlobal, nameIdx)
	case LocalScope:
		c.emit(bytecode.OpGetLocal, s.Index)
	case BuiltinScope:
		c.emit(bytecode.OpGetBuiltin, s.Index)
	case FreeScope:
		c.emit(bytecode.OpGetFree, s.Index)
	case FunctionScope:
		c.emit(bytecode.OpGetGlobal, c.addName(s.Name))
	}
}

func (c *Compiler) storeSymbol(s Symbol, name string) {
	switch s.Scope {
	case GlobalScope:
		nameIdx := c.addName(name)
		c.emit(bytecode.OpSetGlobal, nameIdx)
	case LocalScope:
		c.emit(bytecode.OpSetLocal, s.Index)
	case FreeScope:
		c.emit(bytecode.OpSetFree, s.Index)
	default:
		nameIdx := c.addName(name)
		c.emit(bytecode.OpSetGlobal, nameIdx)
	}
}

func containsYield(node ast.Node) bool {
	if node == nil {
		return false
	}
	switch n := node.(type) {
	case *ast.YieldExpression:
		return true
	case *ast.BlockStatement:
		if n == nil {
			return false
		}
		for _, s := range n.Statements {
			if containsYield(s) {
				return true
			}
		}
	case *ast.ExpressionStatement:
		if n == nil {
			return false
		}
		return containsYield(n.Expression)
	case *ast.IfStatement:
		if n == nil {
			return false
		}
		if containsYield(n.Condition) || containsYield(n.Consequence) || containsYield(n.Alternative) {
			return true
		}
		for _, elif := range n.Elifs {
			if containsYield(elif.Condition) || containsYield(elif.Consequence) {
				return true
			}
		}
	case *ast.WhileStatement:
		if n == nil {
			return false
		}
		return containsYield(n.Condition) || containsYield(n.Body) || containsYield(n.ElseBody)
	case *ast.ForInStatement:
		if n == nil {
			return false
		}
		return containsYield(n.Iterable) || containsYield(n.Body) || containsYield(n.ElseBody)
	case *ast.TryExceptStatement:
		if n == nil {
			return false
		}
		if containsYield(n.Body) || containsYield(n.ElseBody) || containsYield(n.FinallyBody) {
			return true
		}
		for _, h := range n.Handlers {
			if containsYield(h.Body) {
				return true
			}
		}
	case *ast.BinaryExpression:
		if n == nil {
			return false
		}
		return containsYield(n.Left) || containsYield(n.Right)
	case *ast.UnaryExpression:
		if n == nil {
			return false
		}
		return containsYield(n.Right)
	case *ast.CallExpression:
		if n == nil {
			return false
		}
		if containsYield(n.Function) {
			return true
		}
		for _, a := range n.Arguments {
			if containsYield(a) {
				return true
			}
		}
	case *ast.AssignStatement:
		if n == nil {
			return false
		}
		return containsYield(n.Value)
	case *ast.ReturnStatement:
		if n == nil {
			return false
		}
		return containsYield(n.Value)
	}
	return false
}

func (c *Compiler) lastInstructionIs(op bytecode.Opcode) bool {
	if len(c.currentInstructions()) == 0 {
		return false
	}
	return c.scopes[c.scopeIndex].lastInstruction.Opcode == op
}
