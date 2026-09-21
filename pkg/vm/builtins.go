package vm

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

func GetBuiltins(machine *VM) []*BuiltinFunction {
	superFn := func(args ...Value) (Value, error) {
		return machine.builtinSuper(args...)
	}
	printFn := func(args ...Value) (Value, error) {
		return machine.builtinPrint(args...)
	}
	exitFn := func(args ...Value) (Value, error) {
		return machine.builtinExit(args...)
	}
	helpFn := func(args ...Value) (Value, error) {
		return machine.builtinHelp(args...)
	}
	breakpointFn := func(args ...Value) (Value, error) {
		return machine.builtinBreakpoint(args...)
	}
	mapFn := func(args ...Value) (Value, error) {
		return machine.builtinMap(args...)
	}
	filterFn := func(args ...Value) (Value, error) {
		return machine.builtinFilter(args...)
	}

	return []*BuiltinFunction{
		{Name: "print", Fn: printFn},
		{Name: "len", Fn: builtinLen},
		{Name: "range", Fn: builtinRange},
		{Name: "type", Fn: builtinType},
		{Name: "str", Fn: builtinStr},
		{Name: "int", Fn: builtinInt},
		{Name: "float", Fn: builtinFloat},
		{Name: "bool", Fn: builtinBool},
		{Name: "list", Fn: builtinList},
		{Name: "dict", Fn: builtinDict},
		{Name: "set", Fn: builtinSet},
		{Name: "tuple", Fn: builtinTuple},
		{Name: "sum", Fn: builtinSum},
		{Name: "min", Fn: builtinMin},
		{Name: "max", Fn: builtinMax},
		{Name: "abs", Fn: builtinAbs},
		{Name: "round", Fn: builtinRound},
		{Name: "map", Fn: mapFn},
		{Name: "filter", Fn: filterFn},
		{Name: "zip", Fn: builtinZip},
		{Name: "enumerate", Fn: builtinEnumerate},
		{Name: "sorted", Fn: builtinSorted},
		{Name: "reversed", Fn: builtinReversed},
		{Name: "any", Fn: builtinAny},
		{Name: "all", Fn: builtinAll},
		{Name: "isinstance", Fn: builtinIsInstance},
		{Name: "issubclass", Fn: builtinIsSubclass},
		{Name: "repr", Fn: builtinRepr},
		{Name: "ord", Fn: builtinOrd},
		{Name: "chr", Fn: builtinChr},
		{Name: "hex", Fn: builtinHex},
		{Name: "bin", Fn: builtinBin},
		{Name: "oct", Fn: builtinOct},
		{Name: "pow", Fn: builtinPow},
		{Name: "id", Fn: builtinId},
		{Name: "hash", Fn: builtinHash},
		{Name: "input", Fn: builtinInput},
		{Name: "exit", Fn: exitFn},
		{Name: "open", Fn: builtinOpen},
		{Name: "chan", Fn: builtinChan},
		{Name: "spawn", Fn: builtinSpawn},
		{Name: "time", Fn: builtinTime},
		{Name: "sleep", Fn: builtinSleep},
		{Name: "super", Fn: superFn},
		{Name: "next", Fn: builtinNext},
		{Name: "complex", Fn: builtinComplex},
		{Name: "bytes", Fn: builtinBytes},
		{Name: "bytearray", Fn: builtinByteArray},
		{Name: "frozenset", Fn: builtinFrozenset},
		{Name: "memoryview", Fn: builtinMemoryView},
		{Name: "help", Fn: helpFn},
		{Name: "breakpoint", Fn: breakpointFn},
		{Name: "classmethod", Fn: builtinClassMethod},
		{Name: "staticmethod", Fn: builtinStaticMethod},
		{Name: "property", Fn: builtinProperty},
		{Name: "Exception", Fn: builtinExceptionConstructor("Exception")},
		{Name: "ValueError", Fn: builtinExceptionConstructor("ValueError")},
		{Name: "TypeError", Fn: builtinExceptionConstructor("TypeError")},
		{Name: "KeyError", Fn: builtinExceptionConstructor("KeyError")},
		{Name: "IndexError", Fn: builtinExceptionConstructor("IndexError")},
		{Name: "AttributeError", Fn: builtinExceptionConstructor("AttributeError")},
		{Name: "ZeroDivisionError", Fn: builtinExceptionConstructor("ZeroDivisionError")},
		{Name: "RuntimeError", Fn: builtinExceptionConstructor("RuntimeError")},
		{Name: "StopIteration", Fn: builtinExceptionConstructor("StopIteration")},
		{Name: "AssertionError", Fn: builtinExceptionConstructor("AssertionError")},
		{Name: "ExceptionGroup", Fn: builtinExceptionGroupConstructor()},
	}
}

func builtinExceptionConstructor(name string) func(args ...Value) (Value, error) {
	return func(args ...Value) (Value, error) {
		msg := ""
		if len(args) > 0 {
			if sv, ok := args[0].(StringValue); ok {
				msg = string(sv)
			} else {
				msg = args[0].Inspect()
			}
		}
		return &ExceptionObject{TypeStr: name, Message: msg}, nil
	}
}

func builtinExceptionGroupConstructor() func(args ...Value) (Value, error) {
	return func(args ...Value) (Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("TypeError: ExceptionGroup() takes message and list of exceptions")
		}
		msg := ""
		if sv, ok := args[0].(StringValue); ok {
			msg = string(sv)
		} else {
			msg = args[0].Inspect()
		}
		var exList []Value
		if list, ok := args[1].(*ListObject); ok {
			exList = list.Elements
		} else {
			exList = ExtractIterable(args[1])
		}
		return NewExceptionGroup(msg, exList), nil
	}
}

func (vm *VM) builtinPrint(args ...Value) (Value, error) {
	var parts []string
	for _, a := range args {
		parts = append(parts, a.Inspect())
	}
	fmt.Fprintln(vm.stdout, strings.Join(parts, " "))
	return None, nil
}

func (vm *VM) builtinExit(args ...Value) (Value, error) {
	code := 0
	if len(args) > 0 {
		if c, ok := ToInt(args[0]); ok {
			code = int(c)
		}
	}
	return None, &ExitError{Code: code}
}

func (vm *VM) builtinSuper(args ...Value) (Value, error) {
	if len(args) == 2 {
		cls, ok1 := args[0].(*ClassObject)
		inst, ok2 := args[1].(*InstanceObject)
		if !ok1 || !ok2 {
			return nil, vm.runtimeError("TypeError", "super() requires a class and an instance")
		}
		return &SuperObject{Instance: inst, StartFrom: cls}, nil
	}

	if vm.frameIndex >= 2 {
		frame := &vm.frames[vm.frameIndex-1]

		if frame.basePointer < vm.sp {
			self := vm.stack[frame.basePointer]
			if inst, ok := self.(*InstanceObject); ok {

				fnName := frame.cl.Fn.Name
				for _, cls := range inst.Class.MRO {
					if _, has := cls.Methods[fnName]; has {
						return &SuperObject{Instance: inst, StartFrom: cls}, nil
					}
				}
			}
		}
	}
	return nil, vm.runtimeError("RuntimeError", "super() called outside a method")
}

func builtinLen(args ...Value) (Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("TypeError: len() takes exactly one argument (%d given)", len(args))
	}
	switch obj := args[0].(type) {
	case StringValue:
		return IntValue(len([]rune(string(obj)))), nil
	case *ListObject:
		return IntValue(len(obj.Elements)), nil
	case *TupleObject:
		return IntValue(len(obj.Elements)), nil
	case *DictObject:
		return IntValue(len(obj.Pairs)), nil
	case *SetObject:
		return IntValue(len(obj.Elements)), nil
	case *FrozensetObject:
		return IntValue(len(obj.Elements)), nil
	case *MemoryViewObject:
		return IntValue(len(obj.ToBytes())), nil
	case *ByteArrayObject:
		return IntValue(len(obj.Bytes)), nil
	case BytesValue:
		return IntValue(len(obj)), nil
	case *RangeObject:
		if obj.Step == 0 {
			return IntValue(0), nil
		}
		n := (obj.Stop - obj.Start + obj.Step - 1) / obj.Step
		if n < 0 {
			n = 0
		}
		return IntValue(n), nil
	}
	return nil, fmt.Errorf("TypeError: object of type '%s' has no len()", args[0].Type())
}

func builtinRange(args ...Value) (Value, error) {
	var start, stop, step int64 = 0, 0, 1
	switch len(args) {
	case 1:
		s, ok := ToInt(args[0])
		if !ok {
			return nil, fmt.Errorf("TypeError: range() argument must be an integer")
		}
		stop = s
	case 2:
		s, _ := ToInt(args[0])
		e, _ := ToInt(args[1])
		start, stop = s, e
	case 3:
		s, _ := ToInt(args[0])
		e, _ := ToInt(args[1])
		st, _ := ToInt(args[2])
		if st == 0 {
			return nil, fmt.Errorf("ValueError: range() arg 3 must not be zero")
		}
		start, stop, step = s, e, st
	default:
		return nil, fmt.Errorf("TypeError: range expected at most 3 arguments, got %d", len(args))
	}
	return &RangeObject{Start: start, Stop: stop, Step: step}, nil
}

func builtinType(args ...Value) (Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("TypeError: type() takes 1 argument")
	}
	switch v := args[0].(type) {
	case *InstanceObject:
		return StringValue(fmt.Sprintf("<class '%s'>", v.Class.Name)), nil
	case *ClassObject:
		return StringValue(fmt.Sprintf("<class '%s'>", v.Name)), nil
	}
	return StringValue(fmt.Sprintf("<class '%s'>", args[0].Type())), nil
}

func builtinStr(args ...Value) (Value, error) {
	if len(args) == 0 {
		return StringValue(""), nil
	}
	if s, ok := args[0].(StringValue); ok {
		return s, nil
	}
	return StringValue(args[0].Inspect()), nil
}

func builtinInt(args ...Value) (Value, error) {
	if len(args) == 0 {
		return IntValue(0), nil
	}
	switch v := args[0].(type) {
	case IntValue:
		return v, nil
	case FloatValue:
		return IntValue(int64(v)), nil
	case BoolValue:
		if v {
			return IntValue(1), nil
		}
		return IntValue(0), nil
	case StringValue:
		base := 10
		if len(args) > 1 {
			if b, ok := ToInt(args[1]); ok {
				base = int(b)
			}
		}
		s := strings.TrimSpace(string(v))
		i, err := strconv.ParseInt(s, base, 64)
		if err != nil {
			f, errF := strconv.ParseFloat(s, 64)
			if errF == nil {
				return IntValue(int64(f)), nil
			}
			return nil, fmt.Errorf("ValueError: invalid literal for int() with base %d: %q", base, s)
		}
		return IntValue(i), nil
	}
	return nil, fmt.Errorf("TypeError: int() argument must be a string or a number")
}

func builtinFloat(args ...Value) (Value, error) {
	if len(args) == 0 {
		return FloatValue(0.0), nil
	}
	switch v := args[0].(type) {
	case FloatValue:
		return v, nil
	case IntValue:
		return FloatValue(float64(v)), nil
	case BoolValue:
		if v {
			return FloatValue(1.0), nil
		}
		return FloatValue(0.0), nil
	case StringValue:
		f, err := strconv.ParseFloat(strings.TrimSpace(string(v)), 64)
		if err != nil {
			return nil, fmt.Errorf("ValueError: could not convert string to float: %q", string(v))
		}
		return FloatValue(f), nil
	}
	return nil, fmt.Errorf("TypeError: float() argument must be a string or a real number")
}

func builtinBool(args ...Value) (Value, error) {
	if len(args) == 0 {
		return BoolValue(false), nil
	}
	return BoolValue(args[0].Truthy()), nil
}

func builtinList(args ...Value) (Value, error) {
	if len(args) == 0 {
		return NewList(), nil
	}
	return NewList(ExtractIterable(args[0])...), nil
}

func builtinTuple(args ...Value) (Value, error) {
	if len(args) == 0 {
		return NewTuple(), nil
	}
	return NewTuple(ExtractIterable(args[0])...), nil
}

func builtinDict(args ...Value) (Value, error) {
	d := NewDict()
	if len(args) == 0 {
		return d, nil
	}
	if srcDict, ok := args[0].(*DictObject); ok {
		for _, p := range srcDict.Pairs {
			d.Set(p.Key, p.Value)
		}
		return d, nil
	}
	for _, it := range ExtractIterable(args[0]) {
		if t, ok := it.(*TupleObject); ok && len(t.Elements) == 2 {
			d.Set(t.Elements[0], t.Elements[1])
		}
		if l, ok := it.(*ListObject); ok && len(l.Elements) == 2 {
			d.Set(l.Elements[0], l.Elements[1])
		}
	}
	return d, nil
}

func builtinSet(args ...Value) (Value, error) {
	if len(args) == 0 {
		return NewSet(), nil
	}
	return NewSet(ExtractIterable(args[0])...), nil
}

func builtinSum(args ...Value) (Value, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("TypeError: sum() missing required argument")
	}
	items := ExtractIterable(args[0])
	var isFloat bool
	var intSum int64
	var floatSum float64
	if len(args) > 1 {
		if f, ok := args[1].(FloatValue); ok {
			isFloat = true
			floatSum = float64(f)
		} else if i, ok := args[1].(IntValue); ok {
			intSum = int64(i)
		}
	}
	for _, it := range items {
		switch v := it.(type) {
		case IntValue:
			if isFloat {
				floatSum += float64(v)
			} else {
				intSum += int64(v)
			}
		case FloatValue:
			if !isFloat {
				isFloat = true
				floatSum = float64(intSum) + float64(v)
			} else {
				floatSum += float64(v)
			}
		}
	}
	if isFloat {
		return FloatValue(floatSum), nil
	}
	return IntValue(intSum), nil
}

func builtinMin(args ...Value) (Value, error) {
	var items []Value
	if len(args) == 1 {
		items = ExtractIterable(args[0])
	} else {
		items = args
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("ValueError: min() arg is an empty sequence")
	}
	minVal := items[0]
	for _, it := range items[1:] {
		if LessThan(it, minVal) {
			minVal = it
		}
	}
	return minVal, nil
}

func builtinMax(args ...Value) (Value, error) {
	var items []Value
	if len(args) == 1 {
		items = ExtractIterable(args[0])
	} else {
		items = args
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("ValueError: max() arg is an empty sequence")
	}
	maxVal := items[0]
	for _, it := range items[1:] {
		if LessThan(maxVal, it) {
			maxVal = it
		}
	}
	return maxVal, nil
}

func builtinAbs(args ...Value) (Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("TypeError: abs() takes exactly one argument")
	}
	switch v := args[0].(type) {
	case IntValue:
		if v < 0 {
			return -v, nil
		}
		return v, nil
	case FloatValue:
		return FloatValue(math.Abs(float64(v))), nil
	}
	return nil, fmt.Errorf("TypeError: bad operand type for abs(): '%s'", args[0].Type())
}

func builtinRound(args ...Value) (Value, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("TypeError: round() missing required argument")
	}
	digits := 0
	if len(args) > 1 {
		if d, ok := ToInt(args[1]); ok {
			digits = int(d)
		}
	}
	f, _ := ToFloat(args[0])
	shift := math.Pow10(digits)
	rounded := math.Round(f*shift) / shift
	if digits == 0 {
		return IntValue(int64(rounded)), nil
	}
	return FloatValue(rounded), nil
}

func (vm *VM) builtinMap(args ...Value) (Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("TypeError: map() requires at least 2 arguments")
	}
	fn := args[0]
	items := ExtractIterable(args[1])
	results := make([]Value, 0, len(items))
	for _, item := range items {
		var result Value
		switch f := fn.(type) {
		case *BuiltinFunction:
			r, err := f.Fn(item)
			if err != nil {
				return nil, err
			}
			result = r
		case *ClosureObject:
			sub := vm.subVM()
			r, err := sub.callClosureDirect(f, []Value{item})
			if err != nil {
				return nil, err
			}
			result = r
		default:
			result = item
		}
		results = append(results, result)
	}
	return &IteratorObject{Items: results, Index: 0}, nil
}

func (vm *VM) builtinFilter(args ...Value) (Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("TypeError: filter() requires at least 2 arguments")
	}
	fn := args[0]
	items := ExtractIterable(args[1])
	var results []Value
	for _, item := range items {
		keep := item.Truthy()
		if fn != None {
			switch f := fn.(type) {
			case *BuiltinFunction:
				r, err := f.Fn(item)
				if err != nil {
					return nil, err
				}
				keep = r.Truthy()
			case *ClosureObject:
				sub := vm.subVM()
				r, err := sub.callClosureDirect(f, []Value{item})
				if err != nil {
					return nil, err
				}
				keep = r.Truthy()
			}
		}
		if keep {
			results = append(results, item)
		}
	}
	return &IteratorObject{Items: results, Index: 0}, nil
}

func builtinZip(args ...Value) (Value, error) {
	var iters [][]Value
	minLen := math.MaxInt32
	for _, a := range args {
		els := ExtractIterable(a)
		iters = append(iters, els)
		if len(els) < minLen {
			minLen = len(els)
		}
	}
	if len(args) == 0 {
		minLen = 0
	}
	var res []Value
	for i := 0; i < minLen; i++ {
		var tup []Value
		for _, it := range iters {
			tup = append(tup, it[i])
		}
		res = append(res, NewTuple(tup...))
	}
	return NewList(res...), nil
}

func builtinEnumerate(args ...Value) (Value, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("TypeError: enumerate() missing required argument")
	}
	var start int64
	if len(args) > 1 {
		start, _ = ToInt(args[1])
	}
	items := ExtractIterable(args[0])
	var res []Value
	for i, it := range items {
		res = append(res, NewTuple(IntValue(start+int64(i)), it))
	}
	return NewList(res...), nil
}

func builtinSorted(args ...Value) (Value, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("TypeError: sorted() missing required argument")
	}
	items := ExtractIterable(args[0])
	sortedList := make([]Value, len(items))
	copy(sortedList, items)

	reverse := false
	if len(args) > 1 {
		reverse = args[1].Truthy()
	}
	sort.SliceStable(sortedList, func(i, j int) bool {
		if reverse {
			return LessThan(sortedList[j], sortedList[i])
		}
		return LessThan(sortedList[i], sortedList[j])
	})
	return NewList(sortedList...), nil
}

func builtinReversed(args ...Value) (Value, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("TypeError: reversed() missing required argument")
	}
	items := ExtractIterable(args[0])
	rev := make([]Value, len(items))
	for i, v := range items {
		rev[len(items)-1-i] = v
	}
	return &IteratorObject{Items: rev, Index: 0}, nil
}

func builtinAny(args ...Value) (Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("TypeError: any() takes exactly one argument")
	}
	for _, it := range ExtractIterable(args[0]) {
		if it.Truthy() {
			return BoolValue(true), nil
		}
	}
	return BoolValue(false), nil
}

func builtinAll(args ...Value) (Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("TypeError: all() takes exactly one argument")
	}
	for _, it := range ExtractIterable(args[0]) {
		if !it.Truthy() {
			return BoolValue(false), nil
		}
	}
	return BoolValue(true), nil
}

func builtinIsInstance(args ...Value) (Value, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("TypeError: isinstance() expected 2 arguments")
	}
	obj := args[0]

	if typesTuple, ok := args[1].(*TupleObject); ok {
		for _, t := range typesTuple.Elements {
			ok, _ := builtinIsInstance(obj, t)
			if ok.(BoolValue) {
				return BoolValue(true), nil
			}
		}
		return BoolValue(false), nil
	}

	if cls, ok := args[1].(*ClassObject); ok {
		if inst, ok := obj.(*InstanceObject); ok {
			return BoolValue(inst.Class.IsSubclassOf(cls)), nil
		}
		return BoolValue(false), nil
	}

	if typeName, ok := args[1].(StringValue); ok {
		switch string(typeName) {
		case "int":
			return BoolValue(obj.Type() == TypeInt), nil
		case "float":
			return BoolValue(obj.Type() == TypeFloat), nil
		case "bool":
			return BoolValue(obj.Type() == TypeBool), nil
		case "str":
			return BoolValue(obj.Type() == TypeString), nil
		case "list":
			return BoolValue(obj.Type() == TypeList), nil
		case "tuple":
			return BoolValue(obj.Type() == TypeTuple), nil
		case "dict":
			return BoolValue(obj.Type() == TypeDict), nil
		case "set":
			return BoolValue(obj.Type() == TypeSet), nil
		case "NoneType":
			_, isNone := obj.(NoneVal)
			return BoolValue(isNone), nil
		}
	}
	return BoolValue(false), nil
}

func builtinIsSubclass(args ...Value) (Value, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("TypeError: issubclass() expected 2 arguments")
	}
	sub, ok1 := args[0].(*ClassObject)
	sup, ok2 := args[1].(*ClassObject)
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("TypeError: issubclass() arg must be a class")
	}
	return BoolValue(sub.IsSubclassOf(sup)), nil
}

func builtinRepr(args ...Value) (Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("TypeError: repr() takes exactly one argument")
	}
	if s, ok := args[0].(StringValue); ok {
		return StringValue(fmt.Sprintf("%q", string(s))), nil
	}
	return StringValue(args[0].Inspect()), nil
}

func builtinOrd(args ...Value) (Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("TypeError: ord() expected string of length 1")
	}
	s, ok := args[0].(StringValue)
	if !ok {
		return nil, fmt.Errorf("TypeError: ord() expected string of length 1, not '%s'", args[0].Type())
	}
	r := []rune(string(s))
	if len(r) != 1 {
		return nil, fmt.Errorf("TypeError: ord() expected a character, but string of length %d found", len(r))
	}
	return IntValue(int64(r[0])), nil
}

func builtinChr(args ...Value) (Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("TypeError: chr() takes exactly one argument")
	}
	i, ok := ToInt(args[0])
	if !ok {
		return nil, fmt.Errorf("TypeError: an integer is required")
	}
	if i < 0 || i > 0x10FFFF {
		return nil, fmt.Errorf("ValueError: chr() arg not in range(0x110000)")
	}
	return StringValue(string(rune(i))), nil
}

func builtinHex(args ...Value) (Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("TypeError: hex() takes exactly one argument")
	}
	i, ok := ToInt(args[0])
	if !ok {
		return nil, fmt.Errorf("TypeError: '%s' object cannot be interpreted as an integer", args[0].Type())
	}
	if i < 0 {
		return StringValue(fmt.Sprintf("-0x%x", -i)), nil
	}
	return StringValue(fmt.Sprintf("0x%x", i)), nil
}

func builtinBin(args ...Value) (Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("TypeError: bin() takes exactly one argument")
	}
	i, _ := ToInt(args[0])
	if i < 0 {
		return StringValue(fmt.Sprintf("-0b%b", -i)), nil
	}
	return StringValue(fmt.Sprintf("0b%b", i)), nil
}

func builtinOct(args ...Value) (Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("TypeError: oct() takes exactly one argument")
	}
	i, _ := ToInt(args[0])
	if i < 0 {
		return StringValue(fmt.Sprintf("-0o%o", -i)), nil
	}
	return StringValue(fmt.Sprintf("0o%o", i)), nil
}

func builtinPow(args ...Value) (Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("TypeError: pow() expected at least 2 arguments")
	}
	base, _ := ToFloat(args[0])
	exp, _ := ToFloat(args[1])
	res := math.Pow(base, exp)
	if len(args) == 3 {
		mod, _ := ToFloat(args[2])
		if mod == 0 {
			return nil, fmt.Errorf("ValueError: pow() 3rd argument cannot be 0")
		}
		return IntValue(int64(math.Mod(res, mod))), nil
	}
	if args[0].Type() == TypeInt && args[1].Type() == TypeInt && exp >= 0 {
		return IntValue(int64(res)), nil
	}
	return FloatValue(res), nil
}

func builtinId(args ...Value) (Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("TypeError: id() takes exactly one argument")
	}
	return StringValue(fmt.Sprintf("%p", &args[0])), nil
}

func builtinHash(args ...Value) (Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("TypeError: hash() takes exactly one argument")
	}

	switch args[0].(type) {
	case *ListObject, *DictObject, *SetObject:
		return nil, fmt.Errorf("TypeError: unhashable type: '%s'", args[0].Type())
	}
	key := args[0].HashKey()
	var h int64
	for i, c := range key {
		h = h*31 + int64(c)*(int64(i)+1)
	}
	return IntValue(h), nil
}

func builtinInput(args ...Value) (Value, error) {
	if len(args) > 0 {
		fmt.Print(args[0].Inspect())
	}
	reader := bufio.NewReader(os.Stdin)
	text, _ := reader.ReadString('\n')
	return StringValue(strings.TrimRight(text, "\r\n")), nil
}

func builtinOpen(args ...Value) (Value, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("TypeError: open() missing file path")
	}
	path := args[0].Inspect()
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("IOError: %v", err)
	}
	return StringValue(string(content)), nil
}

func builtinChan(args ...Value) (Value, error) {
	bufSize := 0
	if len(args) > 0 {
		if b, ok := ToInt(args[0]); ok {
			bufSize = int(b)
		}
	}
	return NewChannel(bufSize), nil
}

func builtinSpawn(_ ...Value) (Value, error) { return None, nil }

func builtinTime(_ ...Value) (Value, error) {
	return FloatValue(float64(time.Now().UnixNano()) / 1e9), nil
}

func builtinSleep(args ...Value) (Value, error) {
	if len(args) == 0 {
		return None, nil
	}
	f, _ := ToFloat(args[0])
	time.Sleep(time.Duration(f * float64(time.Second)))
	return None, nil
}

func (vm *VM) builtinHelp(args ...Value) (Value, error) {
	if len(args) == 0 {
		fmt.Fprintln(vm.stdout, "Aethium Interactive Help Utility. Pass any object, function, or class to inspect its documentation.")
		return None, nil
	}
	target := args[0]
	switch obj := target.(type) {
	case *ClosureObject:
		doc := obj.Fn.Doc
		if doc == "" {
			doc = "No docstring provided."
		}
		fmt.Fprintf(vm.stdout, "Help on function %s:\n\n%s(%s)\n    %s\n", obj.Fn.Name, obj.Fn.Name, strings.Join(obj.Fn.ParamNames, ", "), doc)
	case *ClassObject:
		doc := obj.Doc
		if doc == "" {
			doc = "No docstring provided."
		}
		fmt.Fprintf(vm.stdout, "Help on class %s:\n\nclass %s\n    %s\n", obj.Name, obj.Name, doc)
	case *BuiltinFunction:
		fmt.Fprintf(vm.stdout, "Help on built-in function %s:\n\n%s(...)\n    Built-in system function.\n", obj.Name, obj.Name)
	default:
		fmt.Fprintf(vm.stdout, "Help on %s object:\n\nType: %s\nRepresentation: %s\n", target.Type(), target.Type(), target.Inspect())
	}
	return None, nil
}

func (vm *VM) builtinBreakpoint(args ...Value) (Value, error) {
	fmt.Fprintln(vm.stdout, ">>> [Aethium Breakpoint] Execution paused at breakpoint()")
	return None, nil
}

func builtinNext(args ...Value) (Value, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("TypeError: next() missing required argument")
	}
	switch it := args[0].(type) {
	case *IteratorObject:
		if it.Index >= len(it.Items) {
			if len(args) > 1 {
				return args[1], nil
			}
			return nil, fmt.Errorf("StopIteration")
		}
		val := it.Items[it.Index]
		it.Index++
		return val, nil
	case *RangeIterator:
		v, ok := it.Next()
		if !ok {
			if len(args) > 1 {
				return args[1], nil
			}
			return nil, fmt.Errorf("StopIteration")
		}
		return v, nil
	case *GeneratorObject:
		v, err := it.Resume(None)
		if err != nil {
			if len(args) > 1 {
				return args[1], nil
			}
			return nil, err
		}
		return v, nil
	}
	return nil, fmt.Errorf("TypeError: '%s' object is not an iterator", args[0].Type())
}

func builtinComplex(args ...Value) (Value, error) {
	if len(args) == 0 {
		return ComplexValue{Real: 0, Imag: 0}, nil
	}
	r, _ := ToFloat(args[0])
	var i float64 = 0
	if len(args) > 1 {
		i, _ = ToFloat(args[1])
	}
	return ComplexValue{Real: r, Imag: i}, nil
}

func builtinBytes(args ...Value) (Value, error) {
	if len(args) == 0 {
		return BytesValue([]byte{}), nil
	}
	switch v := args[0].(type) {
	case StringValue:
		return BytesValue([]byte(string(v))), nil
	case BytesValue:
		return v, nil
	case *ByteArrayObject:
		copied := make([]byte, len(v.Bytes))
		copy(copied, v.Bytes)
		return BytesValue(copied), nil
	case IntValue:
		return BytesValue(make([]byte, int(v))), nil
	case *ListObject:
		b := make([]byte, len(v.Elements))
		for i, el := range v.Elements {
			n, _ := ToInt(el)
			b[i] = byte(n)
		}
		return BytesValue(b), nil
	}
	return BytesValue([]byte{}), nil
}

func builtinByteArray(args ...Value) (Value, error) {
	b, err := builtinBytes(args...)
	if err != nil {
		return nil, err
	}
	bv := b.(BytesValue)
	data := make([]byte, len(bv))
	copy(data, bv)
	return &ByteArrayObject{Bytes: data}, nil
}

func builtinFrozenset(args ...Value) (Value, error) {
	if len(args) == 0 {
		return &FrozensetObject{Elements: make(map[string]Value)}, nil
	}
	els := ExtractIterable(args[0])
	m := make(map[string]Value, len(els))
	for _, el := range els {
		m[el.HashKey()] = el
	}
	return &FrozensetObject{Elements: m}, nil
}

func builtinMemoryView(args ...Value) (Value, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("TypeError: memoryview() requires an argument")
	}
	switch v := args[0].(type) {
	case BytesValue:
		return &MemoryViewObject{Obj: v}, nil
	case *ByteArrayObject:
		return &MemoryViewObject{Obj: v}, nil
	}
	return nil, fmt.Errorf("TypeError: a bytes-like object is required, not '%s'", args[0].Type())
}

func builtinClassMethod(args ...Value) (Value, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("TypeError: classmethod() takes 1 argument")
	}
	return &ClassMethodObject{Func: args[0]}, nil
}

func builtinStaticMethod(args ...Value) (Value, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("TypeError: staticmethod() takes 1 argument")
	}
	return &StaticMethodObject{Func: args[0]}, nil
}

func builtinProperty(args ...Value) (Value, error) {
	p := &PropertyObject{}
	if len(args) > 0 {
		p.FGet = args[0]
	}
	if len(args) > 1 {
		p.FSet = args[1]
	}
	if len(args) > 2 {
		p.FDel = args[2]
	}
	return p, nil
}

func ExtractIterable(v Value) []Value {
	switch obj := v.(type) {
	case *ListObject:
		return obj.Elements
	case *TupleObject:
		return obj.Elements
	case *SetObject:
		els := make([]Value, 0, len(obj.Elements))
		for _, e := range obj.Elements {
			els = append(els, e)
		}
		return els
	case *DictObject:
		keys := make([]Value, 0, len(obj.Pairs))
		for _, p := range obj.Pairs {
			keys = append(keys, p.Key)
		}
		return keys
	case StringValue:
		var chars []Value
		for _, ch := range []rune(string(obj)) {
			chars = append(chars, StringValue(string(ch)))
		}
		return chars
	case *RangeObject:
		var nums []Value
		if obj.Step > 0 {
			for i := obj.Start; i < obj.Stop; i += obj.Step {
				nums = append(nums, IntValue(i))
			}
		} else if obj.Step < 0 {
			for i := obj.Start; i > obj.Stop; i += obj.Step {
				nums = append(nums, IntValue(i))
			}
		}
		return nums
	case *IteratorObject:
		return obj.Items[obj.Index:]
	case *RangeIterator:

		var items []Value
		for {
			v, ok := obj.Next()
			if !ok {
				break
			}
			items = append(items, v)
		}
		return items
	}
	return []Value{}
}

func LessThan(a, b Value) bool {
	if a.Type() == TypeInt && b.Type() == TypeInt {
		return a.(IntValue) < b.(IntValue)
	}
	if a.Type() == TypeFloat && b.Type() == TypeFloat {
		return a.(FloatValue) < b.(FloatValue)
	}
	if IsNumber(a) && IsNumber(b) {
		fa, _ := ToFloat(a)
		fb, _ := ToFloat(b)
		return fa < fb
	}
	if a.Type() == TypeString && b.Type() == TypeString {
		return a.(StringValue) < b.(StringValue)
	}
	return false
}

func callListMethod(obj *ListObject, name string, args []Value) (Value, error) {
	switch name {
	case "append":
		if len(args) != 1 {
			return nil, fmt.Errorf("TypeError: append() takes exactly one argument")
		}
		obj.Elements = append(obj.Elements, args[0])
		return None, nil
	case "pop":
		if len(obj.Elements) == 0 {
			return nil, fmt.Errorf("IndexError: pop from empty list")
		}
		idx := len(obj.Elements) - 1
		if len(args) == 1 {
			i, _ := ToInt(args[0])
			idx = int(i)
			if idx < 0 {
				idx += len(obj.Elements)
			}
		}
		if idx < 0 || idx >= len(obj.Elements) {
			return nil, fmt.Errorf("IndexError: pop index out of range")
		}
		val := obj.Elements[idx]
		obj.Elements = append(obj.Elements[:idx], obj.Elements[idx+1:]...)
		return val, nil
	case "extend":
		if len(args) != 1 {
			return nil, fmt.Errorf("TypeError: extend() takes exactly one argument")
		}
		obj.Elements = append(obj.Elements, ExtractIterable(args[0])...)
		return None, nil
	case "insert":
		if len(args) != 2 {
			return nil, fmt.Errorf("TypeError: insert() takes exactly 2 arguments")
		}
		idx, _ := ToInt(args[0])
		i := int(idx)
		if i < 0 {
			i += len(obj.Elements)
		}
		if i < 0 {
			i = 0
		}
		if i > len(obj.Elements) {
			i = len(obj.Elements)
		}
		obj.Elements = append(obj.Elements[:i], append([]Value{args[1]}, obj.Elements[i:]...)...)
		return None, nil
	case "remove":
		if len(args) != 1 {
			return nil, fmt.Errorf("TypeError: remove() takes exactly one argument")
		}
		for i, e := range obj.Elements {
			if e.Equals(args[0]) {
				obj.Elements = append(obj.Elements[:i], obj.Elements[i+1:]...)
				return None, nil
			}
		}
		return nil, fmt.Errorf("ValueError: list.remove(x): x not in list")
	case "index":
		if len(args) == 0 {
			return nil, fmt.Errorf("TypeError: index() requires item argument")
		}
		for i, el := range obj.Elements {
			if el.Equals(args[0]) {
				return IntValue(i), nil
			}
		}
		return nil, fmt.Errorf("ValueError: '%s' is not in list", args[0].Inspect())
	case "count":
		count := 0
		if len(args) > 0 {
			for _, el := range obj.Elements {
				if el.Equals(args[0]) {
					count++
				}
			}
		}
		return IntValue(count), nil
	case "reverse":
		for i, j := 0, len(obj.Elements)-1; i < j; i, j = i+1, j-1 {
			obj.Elements[i], obj.Elements[j] = obj.Elements[j], obj.Elements[i]
		}
		return None, nil
	case "sort":
		sort.SliceStable(obj.Elements, func(i, j int) bool { return LessThan(obj.Elements[i], obj.Elements[j]) })
		return None, nil
	case "clear":
		obj.Elements = []Value{}
		return None, nil
	case "copy":
		cp := make([]Value, len(obj.Elements))
		copy(cp, obj.Elements)
		return NewList(cp...), nil
	}
	return nil, fmt.Errorf("AttributeError: 'list' object has no attribute '%s'", name)
}

func callDictMethod(obj *DictObject, name string, args []Value) (Value, error) {
	switch name {
	case "keys":
		var keys []Value
		for _, p := range obj.Pairs {
			keys = append(keys, p.Key)
		}
		return NewList(keys...), nil
	case "values":
		var vals []Value
		for _, p := range obj.Pairs {
			vals = append(vals, p.Value)
		}
		return NewList(vals...), nil
	case "items":
		var items []Value
		for _, p := range obj.Pairs {
			items = append(items, NewTuple(p.Key, p.Value))
		}
		return NewList(items...), nil
	case "get":
		if len(args) == 0 {
			return nil, fmt.Errorf("TypeError: get() requires key")
		}
		if val, ok := obj.Get(args[0]); ok {
			return val, nil
		}
		if len(args) > 1 {
			return args[1], nil
		}
		return None, nil
	case "pop":
		if len(args) == 0 {
			return nil, fmt.Errorf("TypeError: pop() requires key")
		}
		if val, ok := obj.Get(args[0]); ok {
			obj.Delete(args[0])
			return val, nil
		}
		if len(args) > 1 {
			return args[1], nil
		}
		return nil, fmt.Errorf("KeyError: %s", args[0].Inspect())
	case "update":
		if len(args) == 1 {
			if d, ok := args[0].(*DictObject); ok {
				for _, p := range d.Pairs {
					obj.Set(p.Key, p.Value)
				}
			}
		}
		return None, nil
	case "setdefault":
		if len(args) < 1 {
			return nil, fmt.Errorf("TypeError: setdefault() requires at least 1 argument")
		}
		if val, ok := obj.Get(args[0]); ok {
			return val, nil
		}
		var def Value = None
		if len(args) > 1 {
			def = args[1]
		}
		obj.Set(args[0], def)
		return def, nil
	case "clear":
		obj.Pairs = make(map[string]DictPair)
		return None, nil
	case "copy":
		nd := NewDict()
		for _, p := range obj.Pairs {
			nd.Set(p.Key, p.Value)
		}
		return nd, nil
	}
	return nil, fmt.Errorf("AttributeError: 'dict' object has no attribute '%s'", name)
}

func callStringMethod(obj StringValue, name string, args []Value) (Value, error) {
	s := string(obj)
	switch name {
	case "upper":
		return StringValue(strings.ToUpper(s)), nil
	case "lower":
		return StringValue(strings.ToLower(s)), nil
	case "strip":
		if len(args) > 0 {
			cut := args[0].Inspect()
			if sv, ok := args[0].(StringValue); ok {
				cut = string(sv)
			}
			return StringValue(strings.Trim(s, cut)), nil
		}
		return StringValue(strings.TrimSpace(s)), nil
	case "lstrip":
		if len(args) > 0 {
			cut := args[0].Inspect()
			if sv, ok := args[0].(StringValue); ok {
				cut = string(sv)
			}
			return StringValue(strings.TrimLeft(s, cut)), nil
		}
		return StringValue(strings.TrimLeft(s, " \t\n\r")), nil
	case "rstrip":
		if len(args) > 0 {
			cut := args[0].Inspect()
			if sv, ok := args[0].(StringValue); ok {
				cut = string(sv)
			}
			return StringValue(strings.TrimRight(s, cut)), nil
		}
		return StringValue(strings.TrimRight(s, " \t\n\r")), nil
	case "split":
		sep := " "
		maxn := -1
		if len(args) > 0 && args[0] != None {
			if sv, ok := args[0].(StringValue); ok {
				sep = string(sv)
			} else {
				sep = args[0].Inspect()
			}
		}
		if len(args) > 1 {
			if n, ok := ToInt(args[1]); ok {
				maxn = int(n)
			}
		}
		var parts []string
		if sep == " " && len(args) == 0 {
			parts = strings.Fields(s)
		} else {
			n := -1
			if maxn >= 0 {
				n = maxn + 1
			}
			parts = strings.SplitN(s, sep, n)
		}
		var res []Value
		for _, p := range parts {
			res = append(res, StringValue(p))
		}
		return NewList(res...), nil
	case "join":
		if len(args) != 1 {
			return nil, fmt.Errorf("TypeError: join() takes exactly one argument")
		}
		parts := ExtractIterable(args[0])
		strs := make([]string, len(parts))
		for i, p := range parts {
			if sv, ok := p.(StringValue); ok {
				strs[i] = string(sv)
			} else {
				strs[i] = p.Inspect()
			}
		}
		return StringValue(strings.Join(strs, s)), nil
	case "replace":
		if len(args) < 2 {
			return nil, fmt.Errorf("TypeError: replace() takes at least 2 arguments")
		}
		oldStr := args[0].Inspect()
		if sv, ok := args[0].(StringValue); ok {
			oldStr = string(sv)
		}
		newStr := args[1].Inspect()
		if sv, ok := args[1].(StringValue); ok {
			newStr = string(sv)
		}
		n := -1
		if len(args) > 2 {
			if v, ok := ToInt(args[2]); ok {
				n = int(v)
			}
		}
		return StringValue(strings.Replace(s, oldStr, newStr, n)), nil
	case "startswith":
		if len(args) != 1 {
			return nil, fmt.Errorf("TypeError: startswith() takes one argument")
		}
		prefix := args[0].Inspect()
		if sv, ok := args[0].(StringValue); ok {
			prefix = string(sv)
		}
		return BoolValue(strings.HasPrefix(s, prefix)), nil
	case "endswith":
		if len(args) != 1 {
			return nil, fmt.Errorf("TypeError: endswith() takes one argument")
		}
		suffix := args[0].Inspect()
		if sv, ok := args[0].(StringValue); ok {
			suffix = string(sv)
		}
		return BoolValue(strings.HasSuffix(s, suffix)), nil
	case "find":
		if len(args) == 0 {
			return nil, fmt.Errorf("TypeError: find() takes at least 1 argument")
		}
		target := args[0].Inspect()
		if sv, ok := args[0].(StringValue); ok {
			target = string(sv)
		}
		return IntValue(int64(strings.Index(s, target))), nil
	case "index":
		if len(args) == 0 {
			return nil, fmt.Errorf("TypeError: index() takes at least 1 argument")
		}
		idx := strings.Index(s, args[0].Inspect())
		if idx == -1 {
			return nil, fmt.Errorf("ValueError: substring not found")
		}
		return IntValue(int64(idx)), nil
	case "count":
		if len(args) == 0 {
			return nil, fmt.Errorf("TypeError: count() takes at least 1 argument")
		}
		return IntValue(int64(strings.Count(s, args[0].Inspect()))), nil
	case "format":
		result := s
		for i, a := range args {
			result = strings.Replace(result, fmt.Sprintf("{%d}", i), a.Inspect(), 1)
		}
		return StringValue(result), nil
	case "isdigit":
		return BoolValue(len(s) > 0 && strings.TrimFunc(s, func(r rune) bool { return r >= '0' && r <= '9' }) == ""), nil
	case "isalpha":
		return BoolValue(len(s) > 0 && strings.TrimFunc(s, func(r rune) bool { return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') }) == ""), nil
	case "isalnum":
		return BoolValue(len(s) > 0 && strings.TrimFunc(s, func(r rune) bool { return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') }) == ""), nil
	case "isspace":
		return BoolValue(len(s) > 0 && strings.TrimSpace(s) == ""), nil
	case "isupper":
		return BoolValue(len(s) > 0 && strings.ToUpper(s) == s && strings.ToLower(s) != s), nil
	case "islower":
		return BoolValue(len(s) > 0 && strings.ToLower(s) == s && strings.ToUpper(s) != s), nil
	case "title":
		return StringValue(strings.Title(s)), nil
	case "capitalize":
		if len(s) == 0 {
			return StringValue(""), nil
		}
		return StringValue(strings.ToUpper(s[:1]) + strings.ToLower(s[1:])), nil
	case "zfill":
		if len(args) != 1 {
			return nil, fmt.Errorf("TypeError: zfill() takes one argument")
		}
		w, _ := ToInt(args[0])
		if int64(len(s)) >= w {
			return obj, nil
		}
		return StringValue(strings.Repeat("0", int(w)-len(s)) + s), nil
	case "center":
		if len(args) == 0 {
			return nil, fmt.Errorf("TypeError: center() requires width argument")
		}
		w, _ := ToInt(args[0])
		fill := " "
		if len(args) > 1 {
			fill = args[1].Inspect()
		}
		extra := int(w) - len(s)
		if extra <= 0 {
			return obj, nil
		}
		left := extra / 2
		right := extra - left
		return StringValue(strings.Repeat(fill, left) + s + strings.Repeat(fill, right)), nil
	case "encode":
		return StringValue(s), nil
	}
	return nil, fmt.Errorf("AttributeError: 'str' object has no attribute '%s'", name)
}

func callSetMethod(obj *SetObject, name string, args []Value) (Value, error) {
	switch name {
	case "add":
		if len(args) != 1 {
			return nil, fmt.Errorf("TypeError: add() takes exactly one argument")
		}
		obj.Elements[args[0].HashKey()] = args[0]
		return None, nil
	case "remove":
		if len(args) != 1 {
			return nil, fmt.Errorf("TypeError: remove() takes exactly one argument")
		}
		k := args[0].HashKey()
		if _, ok := obj.Elements[k]; !ok {
			return nil, fmt.Errorf("KeyError: %s", args[0].Inspect())
		}
		delete(obj.Elements, k)
		return None, nil
	case "discard":
		if len(args) != 1 {
			return nil, fmt.Errorf("TypeError: discard() takes exactly one argument")
		}
		delete(obj.Elements, args[0].HashKey())
		return None, nil
	case "pop":
		for k, v := range obj.Elements {
			delete(obj.Elements, k)
			return v, nil
		}
		return nil, fmt.Errorf("KeyError: 'pop from an empty set'")
	case "clear":
		obj.Elements = make(map[string]Value)
		return None, nil
	case "union":
		ns := NewSet()
		for k, v := range obj.Elements {
			ns.Elements[k] = v
		}
		if len(args) > 0 {
			for _, it := range ExtractIterable(args[0]) {
				ns.Elements[it.HashKey()] = it
			}
		}
		return ns, nil
	case "intersection":
		ns := NewSet()
		if len(args) > 0 {
			other := ExtractIterable(args[0])
			om := make(map[string]bool, len(other))
			for _, it := range other {
				om[it.HashKey()] = true
			}
			for k, v := range obj.Elements {
				if om[k] {
					ns.Elements[k] = v
				}
			}
		}
		return ns, nil
	case "difference":
		ns := NewSet()
		if len(args) > 0 {
			other := ExtractIterable(args[0])
			om := make(map[string]bool, len(other))
			for _, it := range other {
				om[it.HashKey()] = true
			}
			for k, v := range obj.Elements {
				if !om[k] {
					ns.Elements[k] = v
				}
			}
		} else {
			for k, v := range obj.Elements {
				ns.Elements[k] = v
			}
		}
		return ns, nil
	case "issubset":
		if len(args) != 1 {
			return nil, fmt.Errorf("TypeError: issubset() takes one argument")
		}
		other := ExtractIterable(args[0])
		om := make(map[string]bool, len(other))
		for _, it := range other {
			om[it.HashKey()] = true
		}
		for k := range obj.Elements {
			if !om[k] {
				return BoolValue(false), nil
			}
		}
		return BoolValue(true), nil
	case "issuperset":
		if len(args) != 1 {
			return nil, fmt.Errorf("TypeError: issuperset() takes one argument")
		}
		for _, it := range ExtractIterable(args[0]) {
			if _, ok := obj.Elements[it.HashKey()]; !ok {
				return BoolValue(false), nil
			}
		}
		return BoolValue(true), nil
	}
	return nil, fmt.Errorf("AttributeError: 'set' object has no attribute '%s'", name)
}

func listAttrFunc(obj *ListObject, name string) Value {
	return &BuiltinFunction{Name: name, Fn: func(args ...Value) (Value, error) { return callListMethod(obj, name, args) }}
}
func dictAttrFunc(obj *DictObject, name string) Value {
	return &BuiltinFunction{Name: name, Fn: func(args ...Value) (Value, error) { return callDictMethod(obj, name, args) }}
}
func strAttrFunc(obj StringValue, name string) Value {
	return &BuiltinFunction{Name: name, Fn: func(args ...Value) (Value, error) { return callStringMethod(obj, name, args) }}
}
func setAttrFunc(obj *SetObject, name string) Value {
	return &BuiltinFunction{Name: name, Fn: func(args ...Value) (Value, error) { return callSetMethod(obj, name, args) }}
}
