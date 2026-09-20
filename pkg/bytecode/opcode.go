package bytecode

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

type Opcode byte

const (
	OpHalt     Opcode = iota
	OpConstant        // [uint16 index]
	OpPop
	OpDup
	OpRot2
	OpRot3
	OpTrue
	OpFalse
	OpNone
	OpAdd
	OpSub
	OpMul
	OpDiv
	OpFloorDiv
	OpMod
	OpPow
	OpBitAnd
	OpBitOr
	OpBitXor
	OpBitNot
	OpLShift
	OpRShift
	OpMinus
	OpPlus
	OpEqual
	OpNotEqual
	OpLessThan
	OpGreaterThan
	OpLessEqual
	OpGreaterEqual
	OpIn
	OpNotIn
	OpIs
	OpIsNot
	OpNot
	OpJump             // [int32 offset]
	OpJumpIfFalse      // [int32 offset]
	OpJumpIfTrue       // [int32 offset]
	OpJumpIfFalseOrPop // [int32 offset]
	OpJumpIfTrueOrPop  // [int32 offset]
	OpGetGlobal        // [uint16 index]
	OpSetGlobal        // [uint16 index]
	OpGetLocal         // [uint16 index]
	OpSetLocal         // [uint16 index]
	OpGetFree          // [uint16 index]
	OpSetFree          // [uint16 index]
	OpGetBuiltin       // [uint16 index]
	OpBuildList        // [uint16 count]
	OpBuildTuple       // [uint16 count]
	OpBuildDict        // [uint16 count]
	OpBuildSet         // [uint16 count]
	OpBuildString      // [uint16 count]
	OpGetIndex
	OpSetIndex
	OpGetSlice
	OpGetAttr    // [uint16 nameIndex]
	OpSetAttr    // [uint16 nameIndex]
	OpCall       // [uint16 numArgs, uint16 numKwargs]
	OpCallMethod // [uint16 nameIndex, uint16 numArgs]
	OpReturnValue
	OpReturnNone
	OpClosure   // [uint16 constIndex, uint16 numFree]
	OpMakeClass // [uint16 nameIndex, uint16 numBases, uint16 numMethods]
	OpGetIter
	OpForIter // [int32 jumpOffset]
	OpListAppend
	OpDictAdd
	OpUnpackSequence // [uint16 count]
	OpSetupExcept    // [int32 offset]
	OpPopExcept
	OpRaise
	OpGoSpawn       // [uint16 numArgs]
	OpMakeKwargs    // [uint16 count] — build kwargs dict from stack (key,val pairs)
	OpCallKw        // [uint16 numArgs, uint16 numKwargs]
	OpYield         // yield top-of-stack from generator
	OpLoadClosure   // push current closure (for super())
	OpMakeGenerator // [uint16 constIndex, uint16 numFree] — wrap fn as generator
	OpAssert        // assert top-of-stack; uses next constant as message
	OpDeleteName    // [uint16 nameIndex]
	OpSetupWith     // [int32 offset]
	OpWithCleanup
	OpPopBlock
	OpImport           // [uint16 nameIndex] — import module (built-in or user .aeth file)
	OpSetSlice         // set slice: obj[start:end:step] = val
	OpDeleteAttr       // [uint16 nameIndex] — del obj.attr
	OpDeleteIndex      // del obj[idx]
	OpUnpackRest       // [uint16 beforeCount, uint16 afterCount] — for a, *rest, b = seq
	OpRaiseFrom        // raise exc from cause
	OpMakeClassMethod  // wrap top function as classmethod
	OpMakeStaticMethod // wrap top function as staticmethod
	OpMakeProperty     // [uint16 flags] — wrap getter/setter/deleter into Property
	OpEllipsis         // push EllipsisVal
	OpBreakpoint       // trigger debugger / breakpoint
	OpAwait            // await expression
	_OpcodeMax
)

type Definition struct {
	Name          string
	OperandWidths []int
}

var definitions = map[Opcode]*Definition{
	OpHalt:             {"OpHalt", []int{}},
	OpConstant:         {"OpConstant", []int{2}},
	OpPop:              {"OpPop", []int{}},
	OpDup:              {"OpDup", []int{}},
	OpRot2:             {"OpRot2", []int{}},
	OpRot3:             {"OpRot3", []int{}},
	OpTrue:             {"OpTrue", []int{}},
	OpFalse:            {"OpFalse", []int{}},
	OpNone:             {"OpNone", []int{}},
	OpAdd:              {"OpAdd", []int{}},
	OpSub:              {"OpSub", []int{}},
	OpMul:              {"OpMul", []int{}},
	OpDiv:              {"OpDiv", []int{}},
	OpFloorDiv:         {"OpFloorDiv", []int{}},
	OpMod:              {"OpMod", []int{}},
	OpPow:              {"OpPow", []int{}},
	OpBitAnd:           {"OpBitAnd", []int{}},
	OpBitOr:            {"OpBitOr", []int{}},
	OpBitXor:           {"OpBitXor", []int{}},
	OpBitNot:           {"OpBitNot", []int{}},
	OpLShift:           {"OpLShift", []int{}},
	OpRShift:           {"OpRShift", []int{}},
	OpMinus:            {"OpMinus", []int{}},
	OpPlus:             {"OpPlus", []int{}},
	OpEqual:            {"OpEqual", []int{}},
	OpNotEqual:         {"OpNotEqual", []int{}},
	OpLessThan:         {"OpLessThan", []int{}},
	OpGreaterThan:      {"OpGreaterThan", []int{}},
	OpLessEqual:        {"OpLessEqual", []int{}},
	OpGreaterEqual:     {"OpGreaterEqual", []int{}},
	OpIn:               {"OpIn", []int{}},
	OpNotIn:            {"OpNotIn", []int{}},
	OpIs:               {"OpIs", []int{}},
	OpIsNot:            {"OpIsNot", []int{}},
	OpNot:              {"OpNot", []int{}},
	OpJump:             {"OpJump", []int{4}},
	OpJumpIfFalse:      {"OpJumpIfFalse", []int{4}},
	OpJumpIfTrue:       {"OpJumpIfTrue", []int{4}},
	OpJumpIfFalseOrPop: {"OpJumpIfFalseOrPop", []int{4}},
	OpJumpIfTrueOrPop:  {"OpJumpIfTrueOrPop", []int{4}},
	OpGetGlobal:        {"OpGetGlobal", []int{2}},
	OpSetGlobal:        {"OpSetGlobal", []int{2}},
	OpGetLocal:         {"OpGetLocal", []int{2}},
	OpSetLocal:         {"OpSetLocal", []int{2}},
	OpGetFree:          {"OpGetFree", []int{2}},
	OpSetFree:          {"OpSetFree", []int{2}},
	OpGetBuiltin:       {"OpGetBuiltin", []int{2}},
	OpBuildList:        {"OpBuildList", []int{2}},
	OpBuildTuple:       {"OpBuildTuple", []int{2}},
	OpBuildDict:        {"OpBuildDict", []int{2}},
	OpBuildSet:         {"OpBuildSet", []int{2}},
	OpBuildString:      {"OpBuildString", []int{2}},
	OpGetIndex:         {"OpGetIndex", []int{}},
	OpSetIndex:         {"OpSetIndex", []int{}},
	OpGetSlice:         {"OpGetSlice", []int{}},
	OpGetAttr:          {"OpGetAttr", []int{2}},
	OpSetAttr:          {"OpSetAttr", []int{2}},
	OpCall:             {"OpCall", []int{2, 2}},
	OpCallMethod:       {"OpCallMethod", []int{2, 2}},
	OpReturnValue:      {"OpReturnValue", []int{}},
	OpReturnNone:       {"OpReturnNone", []int{}},
	OpClosure:          {"OpClosure", []int{2, 2}},
	OpMakeClass:        {"OpMakeClass", []int{2, 2, 2}},
	OpGetIter:          {"OpGetIter", []int{}},
	OpForIter:          {"OpForIter", []int{4}},
	OpListAppend:       {"OpListAppend", []int{}},
	OpDictAdd:          {"OpDictAdd", []int{}},
	OpUnpackSequence:   {"OpUnpackSequence", []int{2}},
	OpSetupExcept:      {"OpSetupExcept", []int{4}},
	OpPopExcept:        {"OpPopExcept", []int{}},
	OpRaise:            {"OpRaise", []int{}},
	OpGoSpawn:          {"OpGoSpawn", []int{2}},
	OpMakeKwargs:       {"OpMakeKwargs", []int{2}},
	OpCallKw:           {"OpCallKw", []int{2, 2}},
	OpYield:            {"OpYield", []int{}},
	OpLoadClosure:      {"OpLoadClosure", []int{}},
	OpMakeGenerator:    {"OpMakeGenerator", []int{2, 2}},
	OpAssert:           {"OpAssert", []int{}},
	OpDeleteName:       {"OpDeleteName", []int{2}},
	OpSetupWith:        {"OpSetupWith", []int{4}},
	OpWithCleanup:      {"OpWithCleanup", []int{}},
	OpPopBlock:         {"OpPopBlock", []int{}},
	OpImport:           {"OpImport", []int{2}},
	OpSetSlice:         {"OpSetSlice", []int{}},
	OpDeleteAttr:       {"OpDeleteAttr", []int{2}},
	OpDeleteIndex:      {"OpDeleteIndex", []int{}},
	OpUnpackRest:       {"OpUnpackRest", []int{2, 2}},
	OpRaiseFrom:        {"OpRaiseFrom", []int{}},
	OpMakeClassMethod:  {"OpMakeClassMethod", []int{}},
	OpMakeStaticMethod: {"OpMakeStaticMethod", []int{}},
	OpMakeProperty:     {"OpMakeProperty", []int{2}},
	OpEllipsis:         {"OpEllipsis", []int{}},
	OpBreakpoint:       {"OpBreakpoint", []int{}},
	OpAwait:            {"OpAwait", []int{}},
}

func Lookup(op byte) (*Definition, error) {
	def, ok := definitions[Opcode(op)]
	if !ok {
		return nil, fmt.Errorf("opcode %d undefined", op)
	}
	return def, nil
}

func Make(op Opcode, operands ...int) []byte {
	def, ok := definitions[op]
	if !ok {
		return []byte{}
	}

	instructionLen := 1
	for _, w := range def.OperandWidths {
		instructionLen += w
	}

	instruction := make([]byte, instructionLen)
	instruction[0] = byte(op)

	offset := 1
	for i, o := range operands {
		width := def.OperandWidths[i]
		switch width {
		case 2:
			binary.BigEndian.PutUint16(instruction[offset:], uint16(o))
		case 4:
			binary.BigEndian.PutUint32(instruction[offset:], uint32(o))
		}
		offset += width
	}

	return instruction
}

func ReadOperands(def *Definition, ins []byte) ([]int, int) {
	operands := make([]int, len(def.OperandWidths))
	offset := 0

	for i, width := range def.OperandWidths {
		switch width {
		case 2:
			operands[i] = int(binary.BigEndian.Uint16(ins[offset:]))
		case 4:
			operands[i] = int(binary.BigEndian.Uint32(ins[offset:]))
		}
		offset += width
	}

	return operands, offset
}

type Instructions []byte

func (ins Instructions) String() string {
	var out bytes.Buffer

	i := 0
	for i < len(ins) {
		def, err := Lookup(ins[i])
		if err != nil {
			fmt.Fprintf(&out, "ERROR: %s\n", err)
			i++
			continue
		}

		operands, read := ReadOperands(def, ins[i+1:])
		fmt.Fprintf(&out, "%04d %s\n", i, ins.fmtInstruction(def, operands))
		i += 1 + read
	}

	return out.String()
}

func (ins Instructions) fmtInstruction(def *Definition, operands []int) string {
	operandCount := len(def.OperandWidths)
	switch operandCount {
	case 0:
		return def.Name
	case 1:
		return fmt.Sprintf("%s %d", def.Name, operands[0])
	case 2:
		return fmt.Sprintf("%s %d %d", def.Name, operands[0], operands[1])
	case 3:
		return fmt.Sprintf("%s %d %d %d", def.Name, operands[0], operands[1], operands[2])
	}
	return fmt.Sprintf("ERROR: unhandled operandCount for %s\n", def.Name)
}
