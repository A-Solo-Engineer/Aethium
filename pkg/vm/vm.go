package vm

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"sync"

	"aethium/pkg/bytecode"
)

const (
	StackSize = 4096
	MaxFrames = 512
)

type ExceptHandler struct {
	frameIndex int
	sp         int
	handlerIP  int
}

type WithBlock struct {
	exitFn Value
	sp     int
}

type VM struct {
	constants []Value
	names     []string
	globals   map[string]Value
	globalsMu *sync.RWMutex
	builtins  []*BuiltinFunction

	importLoader func(name string) (Value, error)

	stack      [StackSize]Value
	sp         int
	frames     [MaxFrames]Frame
	frameIndex int

	exceptStack []ExceptHandler
	withStack   []WithBlock
	lastPopped  Value

	goroutineWg     *sync.WaitGroup
	goroutineMu     *sync.Mutex
	goroutineErrors *[]error

	stdout   io.Writer
	stderr   io.Writer
	ctx      context.Context
	filename string
}

func New(bc *Bytecode) *VM {
	return NewWithGlobals(bc, make(map[string]Value))
}

func NewForFunction(fn *ClosureObject) *VM {
	bc := &Bytecode{
		Instructions: bytecode.Instructions{},
		Constants:    nil,
		Names:        nil,
	}
	return New(bc)
}

func NewWithGlobals(bc *Bytecode, globals map[string]Value) *VM {
	mainFn := &bytecode.CompiledFunction{
		Instructions: bc.Instructions,
		NumLocals:    0,
		Name:         "<module>",
	}
	mainClosure := &ClosureObject{Fn: mainFn}
	mainFrame := Frame{
		cl:          mainClosure,
		ip:          0,
		basePointer: 0,
		returnSlot:  0,
	}

	var gErrors []error
	machine := &VM{
		constants:       convertConstants(bc.Constants),
		names:           bc.Names,
		globals:         globals,
		globalsMu:       &sync.RWMutex{},
		goroutineWg:     &sync.WaitGroup{},
		goroutineMu:     &sync.Mutex{},
		goroutineErrors: &gErrors,
		stdout:          os.Stdout,
		stderr:          os.Stderr,
		ctx:             context.Background(),
		filename:        "<module>",
	}
	machine.frames[0] = mainFrame
	machine.frameIndex = 1
	machine.builtins = GetBuiltins(machine)
	return machine
}

type Bytecode struct {
	Instructions bytecode.Instructions
	Constants    []interface{}
	Names        []string
}

func convertConstants(constants []interface{}) []Value {
	out := make([]Value, len(constants))
	for i, c := range constants {
		switch val := c.(type) {
		case int64:
			out[i] = IntValue(val)
		case int:
			out[i] = IntValue(int64(val))
		case float64:
			out[i] = FloatValue(val)
		case bool:
			out[i] = BoolValue(val)
		case string:
			out[i] = StringValue(val)
		case complex128:
			out[i] = ComplexValue{Real: real(val), Imag: imag(val)}
		case ComplexValue:
			out[i] = val
		case []byte:
			out[i] = BytesValue(val)
		case BytesValue:
			out[i] = val
		case *bytecode.CompiledFunction:
			out[i] = &FunctionObject{CompiledFunction: val}
		default:
			if v, ok := c.(Value); ok {
				out[i] = v
			} else {
				out[i] = None
			}
		}
	}
	return out
}

func (vm *VM) SetStdout(w io.Writer)                               { vm.stdout = w }
func (vm *VM) SetStderr(w io.Writer)                               { vm.stderr = w }
func (vm *VM) SetContext(ctx context.Context)                      { vm.ctx = ctx }
func (vm *VM) SetFilename(f string)                                { vm.filename = f }
func (vm *VM) Filename() string                                    { return vm.filename }
func (vm *VM) SetGlobalsMu(mu *sync.RWMutex)                       { vm.globalsMu = mu }
func (vm *VM) SetImportLoader(fn func(name string) (Value, error)) { vm.importLoader = fn }

func (vm *VM) Globals() map[string]Value {
	if vm.globalsMu != nil {
		vm.globalsMu.RLock()
		defer vm.globalsMu.RUnlock()
	}
	res := make(map[string]Value, len(vm.globals))
	for k, v := range vm.globals {
		res[k] = v
	}
	return res
}

func (vm *VM) ImportModule(name string) (Value, error) {
	if vm.importLoader != nil {
		return vm.importLoader(name)
	}
	if vm.globalsMu != nil {
		vm.globalsMu.RLock()
	}
	val, ok := vm.globals[name]
	if vm.globalsMu != nil {
		vm.globalsMu.RUnlock()
	}
	if ok {
		return val, nil
	}
	return nil, vm.runtimeError("ImportError", "No module named '%s'", name)
}

func (vm *VM) recordGoroutineError(err error) {
	if err == nil {
		return
	}
	if vm.goroutineMu != nil && vm.goroutineErrors != nil {
		vm.goroutineMu.Lock()
		*vm.goroutineErrors = append(*vm.goroutineErrors, err)
		vm.goroutineMu.Unlock()
	}
}

func (vm *VM) WaitGoroutines() {
	if vm.goroutineWg != nil {
		vm.goroutineWg.Wait()
	}
}

func (vm *VM) DrainGoroutineErrors() []error {
	if vm.goroutineMu == nil || vm.goroutineErrors == nil {
		return nil
	}
	vm.goroutineMu.Lock()
	defer vm.goroutineMu.Unlock()
	if len(*vm.goroutineErrors) == 0 {
		return nil
	}
	errs := make([]error, len(*vm.goroutineErrors))
	copy(errs, *vm.goroutineErrors)
	*vm.goroutineErrors = (*vm.goroutineErrors)[:0]
	return errs
}

func (vm *VM) WaitAndDrainGoroutineErrors() []error {
	vm.WaitGoroutines()
	return vm.DrainGoroutineErrors()
}

func (vm *VM) LastPoppedStackElem() Value {
	return vm.lastPopped
}

func (vm *VM) currentFrame() *Frame {
	return &vm.frames[vm.frameIndex-1]
}

func (vm *VM) pushFrame(f Frame) error {
	if vm.frameIndex >= MaxFrames {
		return vm.runtimeError("RecursionError", "maximum recursion depth exceeded (%d)", MaxFrames)
	}
	vm.frames[vm.frameIndex] = f
	vm.frameIndex++
	return nil
}

func (vm *VM) popFrame() Frame {
	vm.frameIndex--
	return vm.frames[vm.frameIndex]
}

func (vm *VM) push(v Value) {
	if vm.sp >= StackSize {
		panic("RuntimeError: stack overflow")
	}
	vm.stack[vm.sp] = v
	vm.sp++
}

func (vm *VM) pop() Value {
	if vm.sp <= 0 {
		panic("RuntimeError: stack underflow")
	}
	vm.sp--
	vm.lastPopped = vm.stack[vm.sp]
	return vm.stack[vm.sp]
}

func (vm *VM) peek() Value {
	return vm.stack[vm.sp-1]
}

func (vm *VM) runtimeError(typStr, format string, args ...interface{}) *ExceptionObject {
	msg := fmt.Sprintf(format, args...)
	exc := NewException(typStr, msg)
	for i := vm.frameIndex - 1; i >= 0; i-- {
		f := &vm.frames[i]
		fn := f.cl.Fn
		fname := fn.Filename
		if fname == "" {
			fname = vm.filename
		}
		exc.Traceback = append(exc.Traceback, StackFrame{
			Filename: fname,
			FuncName: fn.Name,
			Line:     fn.FirstLine,
		})
	}
	return exc
}

func (vm *VM) handleException(exc *ExceptionObject) error {
	if len(vm.exceptStack) > 0 {
		handler := vm.exceptStack[len(vm.exceptStack)-1]
		vm.exceptStack = vm.exceptStack[:len(vm.exceptStack)-1]
		vm.frameIndex = handler.frameIndex
		vm.sp = handler.sp
		vm.currentFrame().ip = handler.handlerIP
		vm.push(exc)
		return nil
	}
	return exc
}

func (vm *VM) Run() (retErr error) {
	defer func() {
		if r := recover(); r != nil {
			retErr = vm.runtimeError("RuntimeError", "%v", r)
		}
	}()

	var insCount uint32
	for {
		insCount++

		if insCount&1023 == 0 {
			select {
			case <-vm.ctx.Done():
				return vm.runtimeError("RuntimeError", "execution cancelled: %v", vm.ctx.Err())
			default:
			}
		}

		frame := vm.currentFrame()
		ins := frame.Instructions()
		ip := frame.ip

		if ip >= len(ins) {
			if vm.frameIndex == 1 {
				return nil
			}
			f := vm.popFrame()
			if vm.frameIndex == 0 {
				return nil
			}
			vm.sp = f.returnSlot
			vm.push(None)
			continue
		}

		op := bytecode.Opcode(ins[ip])
		frame.ip++

		switch op {
		case bytecode.OpHalt:
			return nil

		case bytecode.OpConstant:
			constIndex := int(binary.BigEndian.Uint16(ins[ip+1 : ip+3]))
			frame.ip += 2
			if constIndex >= len(vm.constants) {
				return vm.runtimeError("RuntimeError", "constant index %d out of range", constIndex)
			}
			vm.push(vm.constants[constIndex])

		case bytecode.OpPop:
			vm.lastPopped = vm.pop()

		case bytecode.OpDup:
			vm.push(vm.peek())

		case bytecode.OpRot2:
			a, b := vm.pop(), vm.pop()
			vm.push(a)
			vm.push(b)

		case bytecode.OpRot3:
			a, b, c := vm.pop(), vm.pop(), vm.pop()
			vm.push(a)
			vm.push(c)
			vm.push(b)

		case bytecode.OpTrue:
			vm.push(BoolValue(true))

		case bytecode.OpFalse:
			vm.push(BoolValue(false))

		case bytecode.OpNone:
			vm.push(None)

		case bytecode.OpAdd:
			r, l := vm.pop(), vm.pop()
			res, err := vm.evalAdd(l, r)
			if err != nil {
				if e := vm.handleException(WrapError(err)); e != nil {
					return e
				}
				continue
			}
			vm.push(res)

		case bytecode.OpSub:
			r, l := vm.pop(), vm.pop()
			res, err := vm.evalSub(l, r)
			if err != nil {
				if e := vm.handleException(WrapError(err)); e != nil {
					return e
				}
				continue
			}
			vm.push(res)

		case bytecode.OpMul:
			r, l := vm.pop(), vm.pop()
			res, err := vm.evalMul(l, r)
			if err != nil {
				if e := vm.handleException(WrapError(err)); e != nil {
					return e
				}
				continue
			}
			vm.push(res)

		case bytecode.OpDiv:
			r, l := vm.pop(), vm.pop()
			res, err := vm.evalDiv(l, r)
			if err != nil {
				if e := vm.handleException(WrapError(err)); e != nil {
					return e
				}
				continue
			}
			vm.push(res)

		case bytecode.OpFloorDiv:
			r, l := vm.pop(), vm.pop()
			res, err := vm.evalFloorDiv(l, r)
			if err != nil {
				if e := vm.handleException(WrapError(err)); e != nil {
					return e
				}
				continue
			}
			vm.push(res)

		case bytecode.OpMod:
			r, l := vm.pop(), vm.pop()
			res, err := vm.evalMod(l, r)
			if err != nil {
				if e := vm.handleException(WrapError(err)); e != nil {
					return e
				}
				continue
			}
			vm.push(res)

		case bytecode.OpPow:
			r, l := vm.pop(), vm.pop()
			vm.push(vm.evalPow(l, r))

		case bytecode.OpBitAnd:
			r, l := vm.pop(), vm.pop()
			li, okL := ToInt(l)
			ri, okR := ToInt(r)
			if !okL || !okR {
				exc := vm.runtimeError("TypeError", "unsupported operand types for &")
				if e := vm.handleException(exc); e != nil {
					return e
				}
				continue
			}
			vm.push(IntValue(li & ri))

		case bytecode.OpBitOr:
			r, l := vm.pop(), vm.pop()
			li, okL := ToInt(l)
			ri, okR := ToInt(r)
			if !okL || !okR {
				exc := vm.runtimeError("TypeError", "unsupported operand types for |: '%s' and '%s'", l.Type(), r.Type())
				if e := vm.handleException(exc); e != nil {
					return e
				}
				continue
			}
			vm.push(IntValue(li | ri))

		case bytecode.OpBitXor:
			r, l := vm.pop(), vm.pop()
			li, okL := ToInt(l)
			ri, okR := ToInt(r)
			if !okL || !okR {
				exc := vm.runtimeError("TypeError", "unsupported operand types for ^: '%s' and '%s'", l.Type(), r.Type())
				if e := vm.handleException(exc); e != nil {
					return e
				}
				continue
			}
			vm.push(IntValue(li ^ ri))

		case bytecode.OpBitNot:
			v := vm.pop()
			vi, ok := ToInt(v)
			if !ok {
				exc := vm.runtimeError("TypeError", "bad operand type for unary ~: '%s'", v.Type())
				if e := vm.handleException(exc); e != nil {
					return e
				}
				continue
			}
			vm.push(IntValue(^vi))

		case bytecode.OpLShift:
			r, l := vm.pop(), vm.pop()
			li, okL := ToInt(l)
			ri, okR := ToInt(r)
			if !okL || !okR {
				exc := vm.runtimeError("TypeError", "unsupported operand types for <<: '%s' and '%s'", l.Type(), r.Type())
				if e := vm.handleException(exc); e != nil {
					return e
				}
				continue
			}
			if ri < 0 {
				exc := vm.runtimeError("ValueError", "negative shift count")
				if e := vm.handleException(exc); e != nil {
					return e
				}
				continue
			}
			vm.push(IntValue(li << uint(ri)))

		case bytecode.OpRShift:
			r, l := vm.pop(), vm.pop()
			li, okL := ToInt(l)
			ri, okR := ToInt(r)
			if !okL || !okR {
				exc := vm.runtimeError("TypeError", "unsupported operand types for >>: '%s' and '%s'", l.Type(), r.Type())
				if e := vm.handleException(exc); e != nil {
					return e
				}
				continue
			}
			if ri < 0 {
				exc := vm.runtimeError("ValueError", "negative shift count")
				if e := vm.handleException(exc); e != nil {
					return e
				}
				continue
			}
			vm.push(IntValue(li >> uint(ri)))

		case bytecode.OpMinus:
			v := vm.pop()
			switch val := v.(type) {
			case IntValue:
				vm.push(-val)
			case FloatValue:
				vm.push(-val)
			default:
				exc := vm.runtimeError("TypeError", "bad operand type for unary -: '%s'", v.Type())
				if e := vm.handleException(exc); e != nil {
					return e
				}
			}

		case bytecode.OpPlus:

		case bytecode.OpNot:
			v := vm.pop()
			vm.push(BoolValue(!v.Truthy()))

		case bytecode.OpEqual:
			r, l := vm.pop(), vm.pop()
			vm.push(BoolValue(l.Equals(r)))

		case bytecode.OpNotEqual:
			r, l := vm.pop(), vm.pop()
			vm.push(BoolValue(!l.Equals(r)))

		case bytecode.OpLessThan:
			r, l := vm.pop(), vm.pop()
			vm.push(BoolValue(LessThan(l, r)))

		case bytecode.OpGreaterThan:
			r, l := vm.pop(), vm.pop()
			vm.push(BoolValue(LessThan(r, l)))

		case bytecode.OpLessEqual:
			r, l := vm.pop(), vm.pop()
			vm.push(BoolValue(!LessThan(r, l)))

		case bytecode.OpGreaterEqual:
			r, l := vm.pop(), vm.pop()
			vm.push(BoolValue(!LessThan(l, r)))

		case bytecode.OpIn:
			container, item := vm.pop(), vm.pop()
			vm.push(BoolValue(vm.evalIn(item, container)))

		case bytecode.OpNotIn:
			container, item := vm.pop(), vm.pop()
			vm.push(BoolValue(!vm.evalIn(item, container)))

		case bytecode.OpIs:
			r, l := vm.pop(), vm.pop()
			vm.push(BoolValue(isIdentical(l, r)))

		case bytecode.OpIsNot:
			r, l := vm.pop(), vm.pop()
			vm.push(BoolValue(!isIdentical(l, r)))

		case bytecode.OpJump:
			pos := int(binary.BigEndian.Uint32(ins[ip+1 : ip+5]))
			if pos <= frame.ip {
				select {
				case <-vm.ctx.Done():
					return vm.runtimeError("RuntimeError", "execution cancelled: %v", vm.ctx.Err())
				default:
				}
			}
			frame.ip = pos

		case bytecode.OpJumpIfFalse:
			pos := int(binary.BigEndian.Uint32(ins[ip+1 : ip+5]))
			frame.ip += 4
			if !vm.pop().Truthy() {
				if pos <= frame.ip {
					select {
					case <-vm.ctx.Done():
						return vm.runtimeError("RuntimeError", "execution cancelled: %v", vm.ctx.Err())
					default:
					}
				}
				frame.ip = pos
			}

		case bytecode.OpJumpIfTrue:
			pos := int(binary.BigEndian.Uint32(ins[ip+1 : ip+5]))
			frame.ip += 4
			if vm.pop().Truthy() {
				if pos <= frame.ip {
					select {
					case <-vm.ctx.Done():
						return vm.runtimeError("RuntimeError", "execution cancelled: %v", vm.ctx.Err())
					default:
					}
				}
				frame.ip = pos
			}

		case bytecode.OpJumpIfFalseOrPop:
			pos := int(binary.BigEndian.Uint32(ins[ip+1 : ip+5]))
			frame.ip += 4
			if !vm.peek().Truthy() {
				if pos <= frame.ip {
					select {
					case <-vm.ctx.Done():
						return vm.runtimeError("RuntimeError", "execution cancelled: %v", vm.ctx.Err())
					default:
					}
				}
				frame.ip = pos
			} else {
				vm.pop()
			}

		case bytecode.OpJumpIfTrueOrPop:
			pos := int(binary.BigEndian.Uint32(ins[ip+1 : ip+5]))
			frame.ip += 4
			if vm.peek().Truthy() {
				if pos <= frame.ip {
					select {
					case <-vm.ctx.Done():
						return vm.runtimeError("RuntimeError", "execution cancelled: %v", vm.ctx.Err())
					default:
					}
				}
				frame.ip = pos
			} else {
				vm.pop()
			}

		case bytecode.OpGetGlobal:
			nameIdx := int(binary.BigEndian.Uint16(ins[ip+1 : ip+3]))
			frame.ip += 2
			name := vm.names[nameIdx]
			if vm.globalsMu != nil {
				vm.globalsMu.RLock()
			}
			val, ok := vm.globals[name]
			if vm.globalsMu != nil {
				vm.globalsMu.RUnlock()
			}
			if !ok {
				if bVal, okB := vm.findBuiltin(name); okB {
					vm.push(bVal)
					continue
				}
				exc := vm.runtimeError("NameError", "name '%s' is not defined", name)
				if e := vm.handleException(exc); e != nil {
					return e
				}
				continue
			}
			vm.push(val)

		case bytecode.OpSetGlobal:
			nameIdx := int(binary.BigEndian.Uint16(ins[ip+1 : ip+3]))
			frame.ip += 2
			name := vm.names[nameIdx]
			val := vm.pop()
			if vm.globalsMu != nil {
				vm.globalsMu.Lock()
			}
			vm.globals[name] = val
			if vm.globalsMu != nil {
				vm.globalsMu.Unlock()
			}

		case bytecode.OpGetLocal:
			localIndex := int(binary.BigEndian.Uint16(ins[ip+1 : ip+3]))
			frame.ip += 2
			vm.push(vm.stack[frame.basePointer+localIndex])

		case bytecode.OpSetLocal:
			localIndex := int(binary.BigEndian.Uint16(ins[ip+1 : ip+3]))
			frame.ip += 2
			vm.stack[frame.basePointer+localIndex] = vm.pop()

		case bytecode.OpGetFree:
			freeIndex := int(binary.BigEndian.Uint16(ins[ip+1 : ip+3]))
			frame.ip += 2
			vm.push(frame.cl.Free[freeIndex])

		case bytecode.OpSetFree:
			freeIndex := int(binary.BigEndian.Uint16(ins[ip+1 : ip+3]))
			frame.ip += 2
			frame.cl.Free[freeIndex] = vm.pop()

		case bytecode.OpGetBuiltin:
			builtinIndex := int(binary.BigEndian.Uint16(ins[ip+1 : ip+3]))
			frame.ip += 2
			if builtinIndex >= len(vm.builtins) {
				return vm.runtimeError("RuntimeError", "builtin index %d out of range", builtinIndex)
			}
			vm.push(vm.builtins[builtinIndex])

		case bytecode.OpBuildList:
			numElements := int(binary.BigEndian.Uint16(ins[ip+1 : ip+3]))
			frame.ip += 2
			elements := make([]Value, numElements)
			for i := numElements - 1; i >= 0; i-- {
				elements[i] = vm.pop()
			}
			vm.push(NewList(elements...))

		case bytecode.OpBuildTuple:
			numElements := int(binary.BigEndian.Uint16(ins[ip+1 : ip+3]))
			frame.ip += 2
			elements := make([]Value, numElements)
			for i := numElements - 1; i >= 0; i-- {
				elements[i] = vm.pop()
			}
			vm.push(NewTuple(elements...))

		case bytecode.OpBuildDict:
			numPairs := int(binary.BigEndian.Uint16(ins[ip+1 : ip+3]))
			frame.ip += 2
			dict := NewDict()
			pairs := make([]Value, numPairs*2)
			for i := numPairs*2 - 1; i >= 0; i-- {
				pairs[i] = vm.pop()
			}
			for i := 0; i < numPairs; i++ {
				dict.Set(pairs[i*2], pairs[i*2+1])
			}
			vm.push(dict)

		case bytecode.OpBuildSet:
			numElements := int(binary.BigEndian.Uint16(ins[ip+1 : ip+3]))
			frame.ip += 2
			set := NewSet()
			elems := make([]Value, numElements)
			for i := numElements - 1; i >= 0; i-- {
				elems[i] = vm.pop()
			}
			for _, elem := range elems {
				set.Elements[elem.HashKey()] = elem
			}
			vm.push(set)

		case bytecode.OpBuildString:
			count := int(binary.BigEndian.Uint16(ins[ip+1 : ip+3]))
			frame.ip += 2
			parts := make([]string, count)
			for i := count - 1; i >= 0; i-- {
				parts[i] = vm.pop().Inspect()
			}
			vm.push(StringValue(strings.Join(parts, "")))

		case bytecode.OpGetIndex:
			index, left := vm.pop(), vm.pop()
			val, err := vm.evalIndex(left, index)
			if err != nil {
				if e := vm.handleException(WrapError(err)); e != nil {
					return e
				}
				continue
			}
			vm.push(val)

		case bytecode.OpSetIndex:
			val, index, left := vm.pop(), vm.pop(), vm.pop()
			if err := vm.evalSetIndex(left, index, val); err != nil {
				if e := vm.handleException(WrapError(err)); e != nil {
					return e
				}
				continue
			}

		case bytecode.OpGetSlice:
			step, end, start, left := vm.pop(), vm.pop(), vm.pop(), vm.pop()
			val, err := vm.evalSlice(left, start, end, step)
			if err != nil {
				if e := vm.handleException(WrapError(err)); e != nil {
					return e
				}
				continue
			}
			vm.push(val)

		case bytecode.OpGetAttr:
			nameIdx := int(binary.BigEndian.Uint16(ins[ip+1 : ip+3]))
			frame.ip += 2
			name := vm.names[nameIdx]
			obj := vm.pop()
			val, err := vm.evalGetAttr(obj, name)
			if err != nil {
				if e := vm.handleException(WrapError(err)); e != nil {
					return e
				}
				continue
			}
			vm.push(val)

		case bytecode.OpSetAttr:
			nameIdx := int(binary.BigEndian.Uint16(ins[ip+1 : ip+3]))
			frame.ip += 2
			name := vm.names[nameIdx]
			val, obj := vm.pop(), vm.pop()
			if err := vm.evalSetAttr(obj, name, val); err != nil {
				if e := vm.handleException(WrapError(err)); e != nil {
					return e
				}
				continue
			}

		case bytecode.OpCall:
			numArgs := int(binary.BigEndian.Uint16(ins[ip+1 : ip+3]))
			numKwargs := int(binary.BigEndian.Uint16(ins[ip+3 : ip+5]))
			frame.ip += 4
			if err := vm.executeCall(numArgs, numKwargs); err != nil {
				if exit, ok := err.(*ExitError); ok {
					return exit
				}
				if e := vm.handleException(WrapError(err)); e != nil {
					return e
				}
				continue
			}

		case bytecode.OpCallMethod:
			nameIdx := int(binary.BigEndian.Uint16(ins[ip+1 : ip+3]))
			numArgs := int(binary.BigEndian.Uint16(ins[ip+3 : ip+5]))
			frame.ip += 4
			methodName := vm.names[nameIdx]
			if err := vm.executeMethodCall(methodName, numArgs); err != nil {
				if e := vm.handleException(WrapError(err)); e != nil {
					return e
				}
				continue
			}

		case bytecode.OpReturnValue:
			returnValue := vm.pop()
			f := vm.popFrame()
			vm.lastPopped = returnValue
			if vm.frameIndex == 0 {
				return nil
			}
			vm.sp = f.returnSlot
			if f.isInit {
				vm.push(f.instance)
			} else {
				vm.push(returnValue)
			}

		case bytecode.OpReturnNone:
			f := vm.popFrame()
			vm.lastPopped = None
			if vm.frameIndex == 0 {
				return nil
			}
			vm.sp = f.returnSlot
			if f.isInit {
				vm.push(f.instance)
			} else {
				vm.push(None)
			}

		case bytecode.OpClosure:
			constIndex := int(binary.BigEndian.Uint16(ins[ip+1 : ip+3]))
			numFree := int(binary.BigEndian.Uint16(ins[ip+3 : ip+5]))
			frame.ip += 4
			free := make([]Value, numFree)
			for i := numFree - 1; i >= 0; i-- {
				free[i] = vm.pop()
			}
			var cf *bytecode.CompiledFunction
			switch fn := vm.constants[constIndex].(type) {
			case *FunctionObject:
				cf = fn.CompiledFunction
			default:
				return vm.runtimeError("RuntimeError", "expected function constant at index %d", constIndex)
			}
			defaults := make([]Value, cf.DefaultCount)
			for i := cf.DefaultCount - 1; i >= 0; i-- {
				defaults[i] = vm.pop()
			}
			vm.push(&ClosureObject{Fn: cf, Free: free, Defaults: defaults})

		case bytecode.OpMakeKwargs:
			count := int(binary.BigEndian.Uint16(ins[ip+1 : ip+3]))
			frame.ip += 2
			d := NewDict()
			pairs := make([]DictPair, count)
			for i := count - 1; i >= 0; i-- {
				val := vm.pop()
				key := vm.pop()
				pairs[i] = DictPair{Key: key, Value: val}
			}
			for _, p := range pairs {
				d.Set(p.Key, p.Value)
			}
			vm.push(d)

		case bytecode.OpCallKw:
			numArgs := int(binary.BigEndian.Uint16(ins[ip+1 : ip+3]))
			numKwargs := int(binary.BigEndian.Uint16(ins[ip+3 : ip+5]))
			frame.ip += 4
			kwargsObj, _ := vm.pop().(*DictObject)
			if err := vm.executeCallWithKw(numArgs, numKwargs, kwargsObj); err != nil {
				if exit, ok := err.(*ExitError); ok {
					return exit
				}
				if e := vm.handleException(WrapError(err)); e != nil {
					return e
				}
				continue
			}

		case bytecode.OpMakeClass:
			nameIdx := int(binary.BigEndian.Uint16(ins[ip+1 : ip+3]))
			numBases := int(binary.BigEndian.Uint16(ins[ip+3 : ip+5]))
			numMethods := int(binary.BigEndian.Uint16(ins[ip+5 : ip+7]))
			frame.ip += 6
			className := vm.names[nameIdx]

			methods := make(map[string]Value)
			for i := 0; i < numMethods; i++ {
				methodVal := vm.pop()
				methodNameVal, _ := vm.pop().(StringValue)
				if methodVal != nil {
					methods[string(methodNameVal)] = methodVal
				}
			}
			basesRaw := make([]Value, numBases)
			for i := numBases - 1; i >= 0; i-- {
				basesRaw[i] = vm.pop()
			}
			var bases []*ClassObject
			for _, b := range basesRaw {
				if cls, ok := b.(*ClassObject); ok {
					bases = append(bases, cls)
				}
			}
			classObj := NewClass(className, bases)
			classObj.Methods = methods
			vm.push(classObj)

		case bytecode.OpGetIter:
			iterable := vm.pop()
			switch it := iterable.(type) {
			case *RangeObject:
				vm.push(NewRangeIterator(it))
			default:
				items := ExtractIterable(iterable)
				vm.push(&IteratorObject{Items: items, Index: 0})
			}

		case bytecode.OpForIter:
			jumpOffset := int(binary.BigEndian.Uint32(ins[ip+1 : ip+5]))
			frame.ip += 4
			iterVal := vm.peek()
			switch it := iterVal.(type) {
			case *IteratorObject:
				val, hasNext := it.Next()
				if hasNext {
					vm.push(val)
				} else {
					vm.pop()
					frame.ip = jumpOffset
				}
			case *RangeIterator:
				val, hasNext := it.Next()
				if hasNext {
					vm.push(val)
				} else {
					vm.pop()
					frame.ip = jumpOffset
				}
			default:
				exc := vm.runtimeError("TypeError", "'%s' object is not an iterator", iterVal.Type())
				if e := vm.handleException(exc); e != nil {
					return e
				}
				continue
			}

		case bytecode.OpListAppend:
			elem := vm.pop()
			acc := vm.stack[vm.sp-2]
			if list, ok := acc.(*ListObject); ok {
				list.Elements = append(list.Elements, elem)
			} else if set, ok := acc.(*SetObject); ok {
				set.Elements[elem.HashKey()] = elem
			}

		case bytecode.OpDictAdd:
			val, key := vm.pop(), vm.pop()
			dict := vm.stack[vm.sp-2].(*DictObject)
			dict.Set(key, val)

		case bytecode.OpUnpackSequence:
			count := int(binary.BigEndian.Uint16(ins[ip+1 : ip+3]))
			frame.ip += 2
			seq := vm.pop()
			items := ExtractIterable(seq)
			if len(items) != count {
				exc := vm.runtimeError("ValueError", "not enough values to unpack (expected %d, got %d)", count, len(items))
				if e := vm.handleException(exc); e != nil {
					return e
				}
				continue
			}
			for i := count - 1; i >= 0; i-- {
				vm.push(items[i])
			}

		case bytecode.OpSetupExcept:
			offset := int(binary.BigEndian.Uint32(ins[ip+1 : ip+5]))
			frame.ip += 4
			vm.exceptStack = append(vm.exceptStack, ExceptHandler{
				frameIndex: vm.frameIndex,
				sp:         vm.sp,
				handlerIP:  offset,
			})

		case bytecode.OpPopExcept:
			if len(vm.exceptStack) > 0 {
				vm.exceptStack = vm.exceptStack[:len(vm.exceptStack)-1]
			}

		case bytecode.OpRaise:
			val := vm.pop()
			var exc *ExceptionObject
			switch v := val.(type) {
			case *ExceptionObject:
				exc = v
			case StringValue:
				exc = vm.runtimeError("Exception", string(v))
			case *ClassObject:
				exc = vm.runtimeError(v.Name, "")
			default:
				exc = vm.runtimeError("Exception", val.Inspect())
			}

			if len(exc.Traceback) == 0 {
				for i := vm.frameIndex - 1; i >= 0; i-- {
					f := &vm.frames[i]
					fname := f.cl.Fn.Filename
					if fname == "" {
						fname = vm.filename
					}
					exc.Traceback = append(exc.Traceback, StackFrame{
						Filename: fname, FuncName: f.cl.Fn.Name, Line: f.cl.Fn.FirstLine,
					})
				}
			}
			if e := vm.handleException(exc); e != nil {
				return e
			}

		case bytecode.OpGoSpawn:
			numArgs := int(binary.BigEndian.Uint16(ins[ip+1 : ip+3]))
			frame.ip += 2
			args := make([]Value, numArgs)
			for i := numArgs - 1; i >= 0; i-- {
				args[i] = vm.pop()
			}
			fn := vm.pop()
			if vm.goroutineWg != nil {
				vm.goroutineWg.Add(1)
			}
			go func(callee Value, callArgs []Value) {
				if vm.goroutineWg != nil {
					defer vm.goroutineWg.Done()
				}
				defer func() {
					if r := recover(); r != nil {
						vm.recordGoroutineError(fmt.Errorf("goroutine panic: %v", r))
					}
				}()
				cl, ok := callee.(*ClosureObject)
				if !ok {
					err := fmt.Errorf("TypeError: '%s' object is not callable via go/spawn", callee.Type())
					fmt.Fprintf(vm.stderr, "goroutine error: %v\n", err)
					vm.recordGoroutineError(err)
					return
				}
				subVM := &VM{
					constants:       vm.constants,
					names:           vm.names,
					globals:         vm.globals,
					globalsMu:       vm.globalsMu,
					importLoader:    vm.importLoader,
					goroutineWg:     vm.goroutineWg,
					goroutineMu:     vm.goroutineMu,
					goroutineErrors: vm.goroutineErrors,
					stdout:          vm.stdout,
					stderr:          vm.stderr,
					ctx:             vm.ctx,
					filename:        vm.filename,
				}
				subVM.builtins = GetBuiltins(subVM)
				subVM.stack[0] = cl
				subVM.sp = 1
				if err := subVM.callClosure(cl, 0, callArgs, nil); err != nil {
					vm.recordGoroutineError(err)
					return
				}
				if err := subVM.Run(); err != nil {
					if _, ok := err.(*ExitError); !ok {
						fmt.Fprintf(vm.stderr, "goroutine error: %v\n", err)
						vm.recordGoroutineError(err)
					}
				}
			}(fn, args)
			vm.push(None)

		case bytecode.OpAssert:
			msgVal := vm.pop()
			cond := vm.pop()
			if !cond.Truthy() {
				msgStr := "assertion failed"
				if msgVal != nil && msgVal != None {
					msgStr = msgVal.Inspect()
					if s, ok := msgVal.(StringValue); ok {
						msgStr = string(s)
					}
				}
				exc := vm.runtimeError("AssertionError", "%s", msgStr)
				if e := vm.handleException(exc); e != nil {
					return e
				}
				continue
			}

		case bytecode.OpDeleteName:
			nameIdx := int(binary.BigEndian.Uint16(ins[ip+1 : ip+3]))
			frame.ip += 2
			name := vm.names[nameIdx]
			if vm.globalsMu != nil {
				vm.globalsMu.Lock()
			}
			delete(vm.globals, name)
			if vm.globalsMu != nil {
				vm.globalsMu.Unlock()
			}

		case bytecode.OpImport:
			nameIdx := int(binary.BigEndian.Uint16(ins[ip+1 : ip+3]))
			frame.ip += 2
			name := vm.names[nameIdx]
			mod, err := vm.ImportModule(name)
			if err != nil {
				if e := vm.handleException(WrapError(err)); e != nil {
					return e
				}
				continue
			}
			vm.push(mod)

		case bytecode.OpSetSlice:
			val := vm.pop()
			stepVal := vm.pop()
			endVal := vm.pop()
			startVal := vm.pop()
			obj := vm.pop()
			_ = stepVal

			replacement := ExtractIterable(val)
			switch target := obj.(type) {
			case *ListObject:
				s := int64(0)
				e := int64(len(target.Elements))
				if startVal != None {
					if si, ok := ToInt(startVal); ok {
						s = si
						if s < 0 {
							s += int64(len(target.Elements))
						}
					}
				}
				if endVal != None {
					if ei, ok := ToInt(endVal); ok {
						e = ei
						if e < 0 {
							e += int64(len(target.Elements))
						}
					}
				}
				if s < 0 {
					s = 0
				}
				if s > int64(len(target.Elements)) {
					s = int64(len(target.Elements))
				}
				if e < s {
					e = s
				}
				if e > int64(len(target.Elements)) {
					e = int64(len(target.Elements))
				}

				newElems := make([]Value, 0, int64(len(target.Elements))-(e-s)+int64(len(replacement)))
				newElems = append(newElems, target.Elements[:s]...)
				newElems = append(newElems, replacement...)
				newElems = append(newElems, target.Elements[e:]...)
				target.Elements = newElems

			case *ByteArrayObject:
				bRepl := make([]byte, len(replacement))
				for i, item := range replacement {
					n, _ := ToInt(item)
					bRepl[i] = byte(n)
				}
				s := int64(0)
				e := int64(len(target.Bytes))
				if startVal != None {
					if si, ok := ToInt(startVal); ok {
						s = si
						if s < 0 {
							s += int64(len(target.Bytes))
						}
					}
				}
				if endVal != None {
					if ei, ok := ToInt(endVal); ok {
						e = ei
						if e < 0 {
							e += int64(len(target.Bytes))
						}
					}
				}
				if s < 0 {
					s = 0
				}
				if s > int64(len(target.Bytes)) {
					s = int64(len(target.Bytes))
				}
				if e < s {
					e = s
				}
				if e > int64(len(target.Bytes)) {
					e = int64(len(target.Bytes))
				}

				newData := make([]byte, 0, int64(len(target.Bytes))-(e-s)+int64(len(bRepl)))
				newData = append(newData, target.Bytes[:s]...)
				newData = append(newData, bRepl...)
				newData = append(newData, target.Bytes[e:]...)
				target.Bytes = newData
			default:
				exc := vm.runtimeError("TypeError", "'%s' object does not support slice assignment", obj.Type())
				if e := vm.handleException(exc); e != nil {
					return e
				}
				continue
			}

		case bytecode.OpDeleteAttr:
			nameIndex := int(binary.BigEndian.Uint16(ins[ip+1 : ip+3]))
			frame.ip += 2
			name := vm.names[nameIndex]
			obj := vm.pop()
			if inst, ok := obj.(*InstanceObject); ok {
				delete(inst.Fields, name)
			} else {
				exc := vm.runtimeError("AttributeError", "'%s' object has no attribute '%s'", obj.Type(), name)
				if e := vm.handleException(exc); e != nil {
					return e
				}
				continue
			}

		case bytecode.OpDeleteIndex:
			key := vm.pop()
			obj := vm.pop()
			switch target := obj.(type) {
			case *DictObject:
				target.Delete(key)
			case *ListObject:
				idx, ok := ToInt(key)
				if !ok {
					exc := vm.runtimeError("TypeError", "list indices must be integers, not %s", key.Type())
					if e := vm.handleException(exc); e != nil {
						return e
					}
					continue
				}
				i := int(idx)
				if i < 0 {
					i += len(target.Elements)
				}
				if i < 0 || i >= len(target.Elements) {
					exc := vm.runtimeError("IndexError", "list assignment index out of range")
					if e := vm.handleException(exc); e != nil {
						return e
					}
					continue
				}
				target.Elements = append(target.Elements[:i], target.Elements[i+1:]...)
			default:
				exc := vm.runtimeError("TypeError", "'%s' object doesn't support item deletion", obj.Type())
				if e := vm.handleException(exc); e != nil {
					return e
				}
				continue
			}

		case bytecode.OpUnpackRest:
			beforeCount := int(ins[ip+1])
			afterCount := int(ins[ip+2])
			frame.ip += 2
			seq := vm.pop()
			items := ExtractIterable(seq)
			totalNeeded := beforeCount + afterCount
			if len(items) < totalNeeded {
				exc := vm.runtimeError("ValueError", "not enough values to unpack (expected at least %d, got %d)", totalNeeded, len(items))
				if e := vm.handleException(exc); e != nil {
					return e
				}
				continue
			}
			for i := len(items) - 1; i >= len(items)-afterCount; i-- {
				vm.push(items[i])
			}
			restItems := items[beforeCount : len(items)-afterCount]
			vm.push(NewList(restItems...))
			for i := beforeCount - 1; i >= 0; i-- {
				vm.push(items[i])
			}

		case bytecode.OpRaiseFrom:
			cause := vm.pop()
			val := vm.pop()
			var exc *ExceptionObject
			switch v := val.(type) {
			case *ExceptionObject:
				exc = v
			case *InstanceObject:
				msg := ""
				if mVal, ok := v.Fields["message"]; ok {
					if sv, ok := mVal.(StringValue); ok {
						msg = string(sv)
					} else {
						msg = mVal.Inspect()
					}
				}
				exc = vm.runtimeError(v.Class.Name, "%s", msg)
			case StringValue:
				exc = vm.runtimeError("Exception", string(v))
			case *ClassObject:
				exc = vm.runtimeError(v.Name, "")
			default:
				exc = vm.runtimeError("Exception", val.Inspect())
			}
			if cExc, ok := cause.(*ExceptionObject); ok {
				exc.Cause = cExc
			}
			if e := vm.handleException(exc); e != nil {
				return e
			}

		case bytecode.OpMakeClassMethod:
			fn := vm.pop()
			vm.push(&ClassMethodObject{Func: fn})

		case bytecode.OpMakeStaticMethod:
			fn := vm.pop()
			vm.push(&StaticMethodObject{Func: fn})

		case bytecode.OpMakeProperty:
			fn := vm.pop()
			vm.push(&PropertyObject{FGet: fn})

		case bytecode.OpEllipsis:
			vm.push(StringValue("..."))

		case bytecode.OpBreakpoint:
			vm.builtinBreakpoint()

		case bytecode.OpAwait:
			val := vm.pop()
			if ch, ok := val.(*ChannelObject); ok {
				res, ok2 := ch.Recv()
				if !ok2 {
					vm.push(None)
				} else {
					vm.push(res)
				}
			} else {
				vm.push(val)
			}

		default:

		}
	}
}

func isIdentical(l, r Value) bool {
	switch lv := l.(type) {
	case NoneVal:
		_, ok := r.(NoneVal)
		return ok
	case BoolValue:
		rv, ok := r.(BoolValue)
		return ok && lv == rv
	case IntValue:
		rv, ok := r.(IntValue)
		return ok && lv == rv
	case FloatValue:
		rv, ok := r.(FloatValue)
		return ok && lv == rv
	case StringValue:
		rv, ok := r.(StringValue)
		return ok && lv == rv
	default:
		return l == r
	}
}

func (vm *VM) executeCall(numArgs, numKwargs int) error {
	calleeIdx := vm.sp - 1 - numArgs
	callee := vm.stack[calleeIdx]

	switch fn := callee.(type) {
	case *ClosureObject:

		args := make([]Value, numArgs)
		for i := numArgs - 1; i >= 0; i-- {
			args[i] = vm.pop()
		}
		vm.pop()
		return vm.callClosure(fn, calleeIdx, args, nil)

	case *BuiltinFunction:
		args := make([]Value, numArgs)
		for i := numArgs - 1; i >= 0; i-- {
			args[i] = vm.pop()
		}
		vm.pop()
		res, err := fn.Fn(args...)
		if err != nil {
			return err
		}
		vm.push(res)
		return nil

	case *ClassObject:
		inst := NewInstance(fn)
		if initMethod, ok := fn.LookupMethod("__init__"); ok {
			args := make([]Value, numArgs)
			for i := numArgs - 1; i >= 0; i-- {
				args[i] = vm.pop()
			}
			vm.pop()
			allArgs := make([]Value, 1+len(args))
			allArgs[0] = inst
			copy(allArgs[1:], args)
			if closure, ok := initMethod.(*ClosureObject); ok {

				vm.stack[calleeIdx] = inst
				for i, a := range args {
					vm.stack[calleeIdx+1+i] = a
				}
				vm.sp = calleeIdx + 1 + len(args)
				err := vm.callClosure(closure, calleeIdx, allArgs, nil)
				if err != nil {
					return err
				}

				vm.frames[vm.frameIndex-1].isInit = true
				vm.frames[vm.frameIndex-1].instance = inst
				return nil
			}
			if b, ok := initMethod.(*BuiltinFunction); ok {
				_, err := b.Fn(allArgs...)
				if err != nil {
					return err
				}
				vm.sp = calleeIdx
				vm.push(inst)
				return nil
			}
		}
		for i := 0; i < numArgs; i++ {
			vm.pop()
		}
		vm.pop()
		vm.sp = calleeIdx
		vm.push(inst)
		return nil

	case *BoundMethodObject:
		args := make([]Value, numArgs)
		for i := numArgs - 1; i >= 0; i-- {
			args[i] = vm.pop()
		}
		vm.pop()
		allArgs := make([]Value, 1+len(args))
		if fn.IsClassMethod {
			allArgs[0] = fn.Class
		} else {
			allArgs[0] = fn.Instance
		}
		copy(allArgs[1:], args)
		if closure, ok := fn.Method.(*ClosureObject); ok {
			return vm.callClosure(closure, calleeIdx, allArgs, nil)
		}
		if b, ok := fn.Method.(*BuiltinFunction); ok {
			res, err := b.Fn(allArgs...)
			if err != nil {
				return err
			}
			vm.push(res)
			return nil
		}
		return vm.runtimeError("TypeError", "method is not callable")

	case *WeakRefObject:
		for i := 0; i < numArgs; i++ {
			vm.pop()
		}
		vm.pop()
		if fn.Referent != nil {
			vm.push(fn.Referent)
		} else {
			vm.push(None)
		}
		return nil

	default:
		return vm.runtimeError("TypeError", "'%s' object is not callable", callee.Type())
	}
}

func (vm *VM) callClosure(fn *ClosureObject, calleeIdx int, args []Value, kwargs *DictObject) error {
	cf := fn.Fn
	locals := make([]Value, cf.NumLocals)

	numParams := cf.NumParameters
	varArg := cf.VarArg
	kwArg := cf.KwArg
	defaultCount := cf.DefaultCount

	for i := 0; i < numParams; i++ {
		if i < len(args) {
			locals[i] = args[i]
		} else if kwargs != nil {
			name := ""
			if i < len(cf.ParamNames) {
				name = cf.ParamNames[i]
			}
			if val, ok := kwargs.Get(StringValue(name)); ok {
				locals[i] = val
				continue
			}
			defIdx := i - (numParams - defaultCount)
			if defIdx >= 0 && defIdx < len(fn.Defaults) {
				locals[i] = fn.Defaults[defIdx]
			} else {
				return vm.runtimeError("TypeError", "%s() missing required argument: '%s'", cf.Name, name)
			}
		} else {
			defIdx := i - (numParams - defaultCount)
			if defIdx >= 0 && defIdx < len(fn.Defaults) {
				locals[i] = fn.Defaults[defIdx]
			} else {
				name := ""
				if i < len(cf.ParamNames) {
					name = cf.ParamNames[i]
				}
				return vm.runtimeError("TypeError", "%s() missing required argument: '%s'", cf.Name, name)
			}
		}
	}

	varArgSlot := numParams
	if varArg != "" {
		extra := []Value{}
		if len(args) > numParams {
			extra = args[numParams:]
		}
		locals[varArgSlot] = NewTuple(extra...)
		varArgSlot++
	}

	if kwArg != "" {
		if kwargs != nil {
			leftover := NewDict()
			for _, pair := range kwargs.Pairs {
				isPositional := false
				for _, pname := range cf.ParamNames[:numParams] {
					if StringValue(pname).Equals(pair.Key) {
						isPositional = true
						break
					}
				}
				if !isPositional {
					leftover.Set(pair.Key, pair.Value)
				}
			}
			locals[varArgSlot] = leftover
		} else {
			locals[varArgSlot] = NewDict()
		}
	} else if kwargs != nil {

		for _, pair := range kwargs.Pairs {
			name := pair.Key.Inspect()
			for i, pname := range cf.ParamNames {
				if pname == name {
					locals[i] = pair.Value
					break
				}
			}
		}
	}

	for i, loc := range locals {
		if loc == nil {
			loc = None
		}
		vm.stack[calleeIdx+i] = loc
	}
	if vm.sp < calleeIdx+cf.NumLocals {
		vm.sp = calleeIdx + cf.NumLocals
	}

	return vm.pushFrame(Frame{
		cl:          fn,
		ip:          0,
		basePointer: calleeIdx,
		returnSlot:  calleeIdx,
	})
}

func (vm *VM) executeCallWithKw(numArgs, numKwargs int, kwargs *DictObject) error {
	calleeIdx := vm.sp - 1 - numArgs
	callee := vm.stack[calleeIdx]

	switch fn := callee.(type) {
	case *ClosureObject:
		args := make([]Value, numArgs)
		for i := numArgs - 1; i >= 0; i-- {
			args[i] = vm.pop()
		}
		vm.pop()
		return vm.callClosure(fn, calleeIdx, args, kwargs)

	case *BuiltinFunction:
		args := make([]Value, numArgs)
		for i := numArgs - 1; i >= 0; i-- {
			args[i] = vm.pop()
		}
		vm.pop()
		res, err := fn.Fn(args...)
		if err != nil {
			return err
		}
		vm.push(res)
		return nil

	default:
		return vm.runtimeError("TypeError", "'%s' object does not support keyword arguments", callee.Type())
	}
}

func (vm *VM) subVM() *VM {
	var gErrors []error
	sub := &VM{
		constants:       vm.constants,
		names:           vm.names,
		globals:         vm.globals,
		globalsMu:       vm.globalsMu,
		goroutineWg:     vm.goroutineWg,
		goroutineMu:     vm.goroutineMu,
		goroutineErrors: &gErrors,
		stdout:          vm.stdout,
		stderr:          vm.stderr,
		ctx:             vm.ctx,
		filename:        vm.filename,
	}
	sub.frameIndex = 0
	sub.builtins = GetBuiltins(sub)
	return sub
}

func (vm *VM) callClosureDirect(fn *ClosureObject, args []Value) (Value, error) {
	calleeIdx := 0
	vm.stack[calleeIdx] = fn
	for i, a := range args {
		vm.stack[calleeIdx+1+i] = a
	}
	vm.sp = calleeIdx + 1 + len(args)
	err := vm.callClosure(fn, calleeIdx, args, nil)
	if err != nil {
		return nil, err
	}
	err = vm.Run()
	if err != nil {
		return nil, err
	}
	return vm.lastPopped, nil
}

func (vm *VM) executeMethodCall(methodName string, numArgs int) error {
	args := make([]Value, numArgs)
	for i := numArgs - 1; i >= 0; i-- {
		args[i] = vm.pop()
	}
	target := vm.pop()

	switch obj := target.(type) {
	case *ListObject:
		res, err := callListMethod(obj, methodName, args)
		if err != nil {
			return err
		}
		vm.push(res)
		return nil

	case *DictObject:
		res, err := callDictMethod(obj, methodName, args)
		if err != nil {
			return err
		}
		vm.push(res)
		return nil

	case StringValue:
		res, err := callStringMethod(obj, methodName, args)
		if err != nil {
			return err
		}
		vm.push(res)
		return nil

	case *SetObject:
		res, err := callSetMethod(obj, methodName, args)
		if err != nil {
			return err
		}
		vm.push(res)
		return nil

	case *ChannelObject:
		switch methodName {
		case "send":
			if len(args) != 1 {
				return vm.runtimeError("TypeError", "send() takes 1 argument")
			}
			if err := obj.Send(args[0]); err != nil {
				return err
			}
			vm.push(None)
			return nil
		case "recv":
			val, _ := obj.Recv()
			vm.push(val)
			return nil
		case "close":
			if err := obj.Close(); err != nil {
				return err
			}
			vm.push(None)
			return nil
		}

	case *InstanceObject:
		if method, ok := obj.Class.LookupMethod(methodName); ok {
			calleeIdx := vm.sp
			allArgs := make([]Value, 1+len(args))
			allArgs[0] = obj
			copy(allArgs[1:], args)
			if closure, okC := method.(*ClosureObject); okC {
				return vm.callClosure(closure, calleeIdx, allArgs, nil)
			}
			if cm, okCM := method.(*ClassMethodObject); okCM {
				if cl, okCl := cm.Func.(*ClosureObject); okCl {
					allArgs[0] = obj.Class
					return vm.callClosure(cl, calleeIdx, allArgs, nil)
				}
			}
			if sm, okSM := method.(*StaticMethodObject); okSM {
				if cl, okCl := sm.Func.(*ClosureObject); okCl {
					return vm.callClosure(cl, calleeIdx, args, nil)
				}
			}
			if b, okB := method.(*BuiltinFunction); okB {
				res, err := b.Fn(allArgs...)
				if err != nil {
					return err
				}
				vm.push(res)
				return nil
			}
		}

		if v, ok := obj.Fields[methodName]; ok {
			if b, ok := v.(*BuiltinFunction); ok {
				res, err := b.Fn(args...)
				if err != nil {
					return err
				}
				vm.push(res)
				return nil
			}
		}

	case *SuperObject:
		if method, _, ok := obj.Instance.Class.LookupMethodStartingAfter(methodName, obj.StartFrom); ok {
			calleeIdx := vm.sp
			allArgs := make([]Value, 1+len(args))
			allArgs[0] = obj.Instance
			copy(allArgs[1:], args)
			if closure, okC := method.(*ClosureObject); okC {
				return vm.callClosure(closure, calleeIdx, allArgs, nil)
			}
		}
	}

	return vm.runtimeError("AttributeError", "'%s' object has no method '%s'", target.Type(), methodName)
}

func (vm *VM) evalGetAttr(obj Value, name string) (Value, error) {
	switch o := obj.(type) {
	case *ExceptionObject:
		switch name {
		case "type", "Type":
			return StringValue(o.TypeStr), nil
		case "message", "Message":
			return StringValue(o.Message), nil
		case "cause", "Cause":
			if o.Cause != nil {
				return o.Cause, nil
			}
			return None, nil
		case "context", "Context":
			if o.Context != nil {
				return o.Context, nil
			}
			return None, nil
		}
	case *ExceptionGroupObject:
		switch name {
		case "type", "Type":
			return StringValue(o.TypeStr), nil
		case "message", "Message":
			return StringValue(o.Message), nil
		case "exceptions", "Exceptions":
			return NewList(o.Exceptions...), nil
		}
	case *InstanceObject:
		if val, ok := o.GetAttr(name); ok {
			return val, nil
		}
	case *SuperObject:
		if method, _, ok := o.Instance.Class.LookupMethodStartingAfter(name, o.StartFrom); ok {
			return &BoundMethodObject{Instance: o.Instance, Method: method, Class: o.Instance.Class}, nil
		}
		return nil, vm.runtimeError("AttributeError", "super() has no attribute '%s'", name)
	case *ModuleObject:
		if val, ok := o.GetAttr(name); ok {
			return val, nil
		}
		return nil, vm.runtimeError("AttributeError", "module '%s' has no attribute '%s'", o.Name, name)
	case *ClassObject:
		if method, ok := o.LookupMethod(name); ok {
			switch m := method.(type) {
			case *ClassMethodObject:
				return &BoundMethodObject{Instance: nil, Method: m.Func, Class: o, IsClassMethod: true}, nil
			case *StaticMethodObject:
				return m.Func, nil
			default:
				return m, nil
			}
		}
		if s, ok := o.Static[name]; ok {
			return s, nil
		}
	case ComplexValue:
		switch name {
		case "real":
			return FloatValue(o.Real), nil
		case "imag":
			return FloatValue(o.Imag), nil
		case "conjugate":
			return &BuiltinFunction{
				Name: "conjugate",
				Fn: func(args ...Value) (Value, error) {
					return ComplexValue{Real: o.Real, Imag: -o.Imag}, nil
				},
			}, nil
		}
	case *ByteArrayObject:
		switch name {
		case "append":
			return &BuiltinFunction{
				Name: "append",
				Fn: func(args ...Value) (Value, error) {
					if len(args) < 1 {
						return nil, fmt.Errorf("TypeError: append() requires integer")
					}
					iv, _ := ToInt(args[0])
					o.Bytes = append(o.Bytes, byte(iv))
					return None, nil
				},
			}, nil
		}
	case *DictObject:
		if fn := dictAttrFunc(o, name); fn != nil {
			return fn, nil
		}
	case *ListObject:
		if fn := listAttrFunc(o, name); fn != nil {
			return fn, nil
		}
	case StringValue:
		if fn := strAttrFunc(o, name); fn != nil {
			return fn, nil
		}
	case *SetObject:
		if fn := setAttrFunc(o, name); fn != nil {
			return fn, nil
		}
	case *ChannelObject:
		switch name {
		case "send":
			return &BuiltinFunction{Name: "send", Fn: func(a ...Value) (Value, error) {
				if len(a) != 1 {
					return nil, fmt.Errorf("send() takes 1 argument")
				}
				return None, o.Send(a[0])
			}}, nil
		case "recv":
			return &BuiltinFunction{Name: "recv", Fn: func(a ...Value) (Value, error) {
				v, _ := o.Recv()
				return v, nil
			}}, nil
		case "close":
			return &BuiltinFunction{Name: "close", Fn: func(a ...Value) (Value, error) {
				return None, o.Close()
			}}, nil
		}
	}
	return nil, vm.runtimeError("AttributeError", "'%s' object has no attribute '%s'", obj.Type(), name)
}

func (vm *VM) evalSetAttr(obj Value, name string, val Value) error {
	switch o := obj.(type) {
	case *InstanceObject:
		o.SetAttr(name, val)
		return nil
	case *ClassObject:
		o.Static[name] = val
		return nil
	case *ModuleObject:
		o.Exports[name] = val
		return nil
	}
	return vm.runtimeError("AttributeError", "cannot set attribute '%s' on '%s'", name, obj.Type())
}

func (vm *VM) evalAdd(l, r Value) (Value, error) {
	if IsComplex(l) || IsComplex(r) {
		cl, _ := ToComplex(l)
		cr, _ := ToComplex(r)
		return cl.Add(cr), nil
	}
	if l.Type() == TypeInt && r.Type() == TypeInt {
		return l.(IntValue) + r.(IntValue), nil
	}
	if IsNumber(l) && IsNumber(r) {
		fl, _ := ToFloat(l)
		fr, _ := ToFloat(r)
		return FloatValue(fl + fr), nil
	}
	if l.Type() == TypeString && r.Type() == TypeString {
		return l.(StringValue) + r.(StringValue), nil
	}
	if bL, ok := l.(BytesValue); ok {
		if bR, ok := r.(BytesValue); ok {
			comb := make([]byte, len(bL)+len(bR))
			copy(comb, bL)
			copy(comb[len(bL):], bR)
			return BytesValue(comb), nil
		}
	}
	if baL, ok := l.(*ByteArrayObject); ok {
		if baR, ok := r.(*ByteArrayObject); ok {
			comb := make([]byte, len(baL.Bytes)+len(baR.Bytes))
			copy(comb, baL.Bytes)
			copy(comb[len(baL.Bytes):], baR.Bytes)
			return &ByteArrayObject{Bytes: comb}, nil
		}
	}
	if lL, ok := l.(*ListObject); ok {
		if rL, ok := r.(*ListObject); ok {
			combined := make([]Value, len(lL.Elements)+len(rL.Elements))
			copy(combined, lL.Elements)
			copy(combined[len(lL.Elements):], rL.Elements)
			return NewList(combined...), nil
		}
	}
	return nil, vm.runtimeError("TypeError", "unsupported operand types for +: '%s' and '%s'", l.Type(), r.Type())
}

func (vm *VM) evalSub(l, r Value) (Value, error) {
	if IsComplex(l) || IsComplex(r) {
		cl, _ := ToComplex(l)
		cr, _ := ToComplex(r)
		return cl.Sub(cr), nil
	}
	if l.Type() == TypeInt && r.Type() == TypeInt {
		return l.(IntValue) - r.(IntValue), nil
	}
	if IsNumber(l) && IsNumber(r) {
		fl, _ := ToFloat(l)
		fr, _ := ToFloat(r)
		return FloatValue(fl - fr), nil
	}
	return nil, vm.runtimeError("TypeError", "unsupported operand types for -: '%s' and '%s'", l.Type(), r.Type())
}

func (vm *VM) evalMul(l, r Value) (Value, error) {
	if IsComplex(l) || IsComplex(r) {
		cl, _ := ToComplex(l)
		cr, _ := ToComplex(r)
		return cl.Mul(cr), nil
	}
	if l.Type() == TypeInt && r.Type() == TypeInt {
		return l.(IntValue) * r.(IntValue), nil
	}
	if IsNumber(l) && IsNumber(r) {
		fl, _ := ToFloat(l)
		fr, _ := ToFloat(r)
		return FloatValue(fl * fr), nil
	}
	if s, ok := l.(StringValue); ok {
		if count, ok2 := ToInt(r); ok2 {
			if count <= 0 {
				return StringValue(""), nil
			}
			return StringValue(strings.Repeat(string(s), int(count))), nil
		}
	}
	if s, ok := r.(StringValue); ok {
		if count, ok2 := ToInt(l); ok2 {
			if count <= 0 {
				return StringValue(""), nil
			}
			return StringValue(strings.Repeat(string(s), int(count))), nil
		}
	}
	if list, ok := l.(*ListObject); ok {
		if count, ok2 := ToInt(r); ok2 {
			if count <= 0 {
				return NewList(), nil
			}
			var res []Value
			for i := int64(0); i < count; i++ {
				res = append(res, list.Elements...)
			}
			return NewList(res...), nil
		}
	}
	return nil, vm.runtimeError("TypeError", "unsupported operand types for *: '%s' and '%s'", l.Type(), r.Type())
}

func (vm *VM) evalDiv(l, r Value) (Value, error) {
	if IsComplex(l) || IsComplex(r) {
		cl, _ := ToComplex(l)
		cr, _ := ToComplex(r)
		if cr.Real == 0 && cr.Imag == 0 {
			return nil, vm.runtimeError("ZeroDivisionError", "complex division by zero")
		}
		return cl.Div(cr), nil
	}
	fl, okL := ToFloat(l)
	fr, okR := ToFloat(r)
	if !okL || !okR {
		return nil, vm.runtimeError("TypeError", "unsupported operand types for /")
	}
	if fr == 0 {
		return nil, vm.runtimeError("ZeroDivisionError", "division by zero")
	}
	return FloatValue(fl / fr), nil
}

func (vm *VM) evalFloorDiv(l, r Value) (Value, error) {
	li, okL := ToInt(l)
	ri, okR := ToInt(r)
	if okL && okR {
		if ri == 0 {
			return nil, vm.runtimeError("ZeroDivisionError", "integer division or modulo by zero")
		}
		result := li / ri

		if (li^ri) < 0 && result*ri != li {
			result--
		}
		return IntValue(result), nil
	}
	fl, _ := ToFloat(l)
	fr, _ := ToFloat(r)
	if fr == 0 {
		return nil, vm.runtimeError("ZeroDivisionError", "float floor division by zero")
	}
	return FloatValue(math.Floor(fl / fr)), nil
}

func (vm *VM) evalMod(l, r Value) (Value, error) {
	if l.Type() == TypeInt && r.Type() == TypeInt {
		ri := r.(IntValue)
		if ri == 0 {
			return nil, vm.runtimeError("ZeroDivisionError", "integer modulo by zero")
		}
		result := l.(IntValue) % ri

		if result != 0 && (result < 0) != (ri < 0) {
			result += ri
		}
		return result, nil
	}
	fl, _ := ToFloat(l)
	fr, _ := ToFloat(r)
	if fr == 0 {
		return nil, vm.runtimeError("ZeroDivisionError", "float modulo by zero")
	}
	return FloatValue(math.Mod(fl, fr)), nil
}

func (vm *VM) evalPow(l, r Value) Value {
	fl, _ := ToFloat(l)
	fr, _ := ToFloat(r)
	res := math.Pow(fl, fr)
	if l.Type() == TypeInt && r.Type() == TypeInt && fr >= 0 {
		return IntValue(int64(res))
	}
	return FloatValue(res)
}

func (vm *VM) evalIn(item, container Value) bool {
	switch c := container.(type) {
	case *ListObject:
		for _, e := range c.Elements {
			if e.Equals(item) {
				return true
			}
		}
	case *TupleObject:
		for _, e := range c.Elements {
			if e.Equals(item) {
				return true
			}
		}
	case *DictObject:
		_, ok := c.Get(item)
		return ok
	case *SetObject:
		_, ok := c.Elements[item.HashKey()]
		return ok
	case *FrozensetObject:
		_, ok := c.Elements[item.HashKey()]
		return ok
	case BytesValue:
		iv, ok := ToInt(item)
		if ok {
			b := byte(iv)
			return bytes.Contains([]byte(c), []byte{b})
		}
	case *ByteArrayObject:
		iv, ok := ToInt(item)
		if ok {
			b := byte(iv)
			return bytes.Contains(c.Bytes, []byte{b})
		}
	case StringValue:
		return strings.Contains(string(c), item.Inspect())
	case *RangeObject:
		iv, ok := ToInt(item)
		if !ok {
			return false
		}
		if c.Step > 0 {
			return iv >= c.Start && iv < c.Stop && (iv-c.Start)%c.Step == 0
		}
		return iv <= c.Start && iv > c.Stop && (c.Start-iv)%(-c.Step) == 0
	}
	return false
}

func (vm *VM) evalIndex(left, index Value) (Value, error) {
	switch l := left.(type) {
	case *ListObject:
		idx, ok := ToInt(index)
		if !ok {
			return nil, vm.runtimeError("TypeError", "list indices must be integers")
		}
		i := int(idx)
		if i < 0 {
			i += len(l.Elements)
		}
		if i < 0 || i >= len(l.Elements) {
			return nil, vm.runtimeError("IndexError", "list index out of range")
		}
		return l.Elements[i], nil
	case *TupleObject:
		idx, ok := ToInt(index)
		if !ok {
			return nil, vm.runtimeError("TypeError", "tuple indices must be integers")
		}
		i := int(idx)
		if i < 0 {
			i += len(l.Elements)
		}
		if i < 0 || i >= len(l.Elements) {
			return nil, vm.runtimeError("IndexError", "tuple index out of range")
		}
		return l.Elements[i], nil
	case *DictObject:
		if val, ok := l.Get(index); ok {
			return val, nil
		}
		return nil, vm.runtimeError("KeyError", "%s", index.Inspect())
	case StringValue:
		idx, ok := ToInt(index)
		if !ok {
			return nil, vm.runtimeError("TypeError", "string indices must be integers")
		}
		runes := []rune(string(l))
		i := int(idx)
		if i < 0 {
			i += len(runes)
		}
		if i < 0 || i >= len(runes) {
			return nil, vm.runtimeError("IndexError", "string index out of range")
		}
		return StringValue(string(runes[i])), nil
	case BytesValue:
		idx, ok := ToInt(index)
		if !ok {
			return nil, vm.runtimeError("TypeError", "byte indices must be integers")
		}
		i := int(idx)
		if i < 0 {
			i += len(l)
		}
		if i < 0 || i >= len(l) {
			return nil, vm.runtimeError("IndexError", "index out of range")
		}
		return IntValue(int64(l[i])), nil
	case *ByteArrayObject:
		idx, ok := ToInt(index)
		if !ok {
			return nil, vm.runtimeError("TypeError", "byte indices must be integers")
		}
		i := int(idx)
		if i < 0 {
			i += len(l.Bytes)
		}
		if i < 0 || i >= len(l.Bytes) {
			return nil, vm.runtimeError("IndexError", "index out of range")
		}
		return IntValue(int64(l.Bytes[i])), nil
	case *MemoryViewObject:
		b := l.ToBytes()
		idx, ok := ToInt(index)
		if !ok {
			return nil, vm.runtimeError("TypeError", "memoryview indices must be integers")
		}
		i := int(idx)
		if i < 0 {
			i += len(b)
		}
		if i < 0 || i >= len(b) {
			return nil, vm.runtimeError("IndexError", "index out of range")
		}
		return IntValue(int64(b[i])), nil
	}
	return nil, vm.runtimeError("TypeError", "'%s' object is not subscriptable", left.Type())
}

func (vm *VM) evalSetIndex(left, index, val Value) error {
	switch l := left.(type) {
	case *ListObject:
		idx, ok := ToInt(index)
		if !ok {
			return vm.runtimeError("TypeError", "list indices must be integers")
		}
		i := int(idx)
		if i < 0 {
			i += len(l.Elements)
		}
		if i < 0 || i >= len(l.Elements) {
			return vm.runtimeError("IndexError", "list assignment index out of range")
		}
		l.Elements[i] = val
		return nil
	case *DictObject:
		l.Set(index, val)
		return nil
	case *ByteArrayObject:
		idx, ok := ToInt(index)
		if !ok {
			return vm.runtimeError("TypeError", "byte indices must be integers")
		}
		i := int(idx)
		if i < 0 {
			i += len(l.Bytes)
		}
		if i < 0 || i >= len(l.Bytes) {
			return vm.runtimeError("IndexError", "index out of range")
		}
		v, _ := ToInt(val)
		l.Bytes[i] = byte(v)
		return nil
	}
	return vm.runtimeError("TypeError", "'%s' object does not support item assignment", left.Type())
}

func (vm *VM) evalSlice(left, start, end, step Value) (Value, error) {
	var elements []Value
	var isStr bool
	var runes []rune

	switch l := left.(type) {
	case *ListObject:
		elements = l.Elements
	case *TupleObject:
		elements = l.Elements
	case StringValue:
		isStr = true
		runes = []rune(string(l))
	case BytesValue:
		src := []byte(l)
		length := len(src)
		st := int64(1)
		if step != None && step != nil {
			if sv, ok := ToInt(step); ok && sv != 0 {
				st = sv
			}
		}
		var s, e int64
		if st > 0 {
			s = 0
			e = int64(length)
		} else {
			s = int64(length - 1)
			e = -1
		}
		if start != None && start != nil {
			sv, _ := ToInt(start)
			if sv < 0 {
				sv += int64(length)
			}
			s = sv
		}
		if end != None && end != nil {
			ev, _ := ToInt(end)
			if ev < 0 {
				ev += int64(length)
			}
			e = ev
		}
		var b []byte
		for idx := s; ; {
			if st > 0 && idx >= e {
				break
			}
			if st < 0 && idx <= e {
				break
			}
			if idx >= 0 && idx < int64(length) {
				b = append(b, src[idx])
			}
			idx += st
		}
		return BytesValue(b), nil
	case *ByteArrayObject:
		src := l.Bytes
		length := len(src)
		st := int64(1)
		if step != None && step != nil {
			if sv, ok := ToInt(step); ok && sv != 0 {
				st = sv
			}
		}
		var s, e int64
		if st > 0 {
			s = 0
			e = int64(length)
		} else {
			s = int64(length - 1)
			e = -1
		}
		if start != None && start != nil {
			sv, _ := ToInt(start)
			if sv < 0 {
				sv += int64(length)
			}
			s = sv
		}
		if end != None && end != nil {
			ev, _ := ToInt(end)
			if ev < 0 {
				ev += int64(length)
			}
			e = ev
		}
		var b []byte
		for idx := s; ; {
			if st > 0 && idx >= e {
				break
			}
			if st < 0 && idx <= e {
				break
			}
			if idx >= 0 && idx < int64(length) {
				b = append(b, src[idx])
			}
			idx += st
		}
		return &ByteArrayObject{Bytes: b}, nil
	case *MemoryViewObject:
		src := l.ToBytes()
		length := len(src)
		st := int64(1)
		if step != None && step != nil {
			if sv, ok := ToInt(step); ok && sv != 0 {
				st = sv
			}
		}
		var s, e int64
		if st > 0 {
			s = 0
			e = int64(length)
		} else {
			s = int64(length - 1)
			e = -1
		}
		if start != None && start != nil {
			sv, _ := ToInt(start)
			if sv < 0 {
				sv += int64(length)
			}
			s = sv
		}
		if end != None && end != nil {
			ev, _ := ToInt(end)
			if ev < 0 {
				ev += int64(length)
			}
			e = ev
		}
		var b []byte
		for idx := s; ; {
			if st > 0 && idx >= e {
				break
			}
			if st < 0 && idx <= e {
				break
			}
			if idx >= 0 && idx < int64(length) {
				b = append(b, src[idx])
			}
			idx += st
		}
		return &MemoryViewObject{Obj: BytesValue(b)}, nil
	default:
		return nil, vm.runtimeError("TypeError", "'%s' object is not sliceable", left.Type())
	}

	length := len(elements)
	if isStr {
		length = len(runes)
	}

	st := int64(1)
	if step != None && step != nil {
		if sv, ok := ToInt(step); ok && sv != 0 {
			st = sv
		}
	}
	var s, e int64
	if st > 0 {
		s = 0
		e = int64(length)
	} else {
		s = int64(length - 1)
		e = -1
	}
	if start != None && start != nil {
		sv, _ := ToInt(start)
		if sv < 0 {
			sv += int64(length)
		}
		s = sv
	}
	if end != None && end != nil {
		ev, _ := ToInt(end)
		if ev < 0 {
			ev += int64(length)
		}
		e = ev
	}

	if isStr {
		var sb strings.Builder
		if st > 0 {
			for i := s; i < e && i < int64(length); i += st {
				if i >= 0 {
					sb.WriteRune(runes[i])
				}
			}
		} else {
			for i := s; i > e && i >= 0; i += st {
				if i < int64(length) {
					sb.WriteRune(runes[i])
				}
			}
		}
		return StringValue(sb.String()), nil
	}

	var res []Value
	if st > 0 {
		for i := s; i < e && i < int64(length); i += st {
			if i >= 0 {
				res = append(res, elements[i])
			}
		}
	} else {
		for i := s; i > e && i >= 0; i += st {
			if i < int64(length) {
				res = append(res, elements[i])
			}
		}
	}
	if left.Type() == TypeTuple {
		return NewTuple(res...), nil
	}
	return NewList(res...), nil
}

func (vm *VM) findBuiltin(name string) (Value, bool) {
	for _, b := range vm.builtins {
		if b.Name == name {
			return b, true
		}
	}
	return nil, false
}
