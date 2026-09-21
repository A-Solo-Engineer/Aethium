package vm

import (
	"encoding/hex"
	"fmt"
	"math"
	"math/cmplx"
	"strconv"
	"strings"
)

type ValueType string

const (
	TypeInt            ValueType = "int"
	TypeFloat          ValueType = "float"
	TypeComplex        ValueType = "complex"
	TypeBool           ValueType = "bool"
	TypeString         ValueType = "str"
	TypeBytes          ValueType = "bytes"
	TypeByteArray      ValueType = "bytearray"
	TypeNone           ValueType = "NoneType"
	TypeEllipsis       ValueType = "ellipsis"
	TypeList           ValueType = "list"
	TypeTuple          ValueType = "tuple"
	TypeDict           ValueType = "dict"
	TypeSet            ValueType = "set"
	TypeFrozenset      ValueType = "frozenset"
	TypeFunction       ValueType = "function"
	TypeClosure        ValueType = "closure"
	TypeBuiltin        ValueType = "builtin_function_or_method"
	TypeClass          ValueType = "type"
	TypeInstance       ValueType = "instance"
	TypeBoundMethod    ValueType = "method"
	TypeProperty       ValueType = "property"
	TypeClassMethod    ValueType = "classmethod"
	TypeStaticMethod   ValueType = "staticmethod"
	TypeModule         ValueType = "module"
	TypeRange          ValueType = "range"
	TypeIterator       ValueType = "iterator"
	TypeChannel        ValueType = "channel"
	TypeException      ValueType = "exception"
	TypeExceptionGroup ValueType = "ExceptionGroup"
	TypeGenerator      ValueType = "generator"
	TypeSuper          ValueType = "super"
	TypeMemoryView     ValueType = "memoryview"
	TypeWeakRef        ValueType = "weakref"
)

type Value interface {
	Type() ValueType
	Inspect() string
	Truthy() bool
	Equals(other Value) bool
	HashKey() string
}

type IntValue int64

func (v IntValue) Type() ValueType { return TypeInt }
func (v IntValue) Inspect() string { return strconv.FormatInt(int64(v), 10) }
func (v IntValue) Truthy() bool    { return v != 0 }
func (v IntValue) Equals(o Value) bool {
	if ov, ok := o.(IntValue); ok {
		return v == ov
	}
	if ov, ok := o.(FloatValue); ok {
		return float64(v) == float64(ov)
	}
	if ov, ok := o.(BoolValue); ok {
		return (v != 0) == bool(ov)
	}
	if ov, ok := o.(ComplexValue); ok {
		return ov.Imag == 0 && float64(v) == ov.Real
	}
	return false
}
func (v IntValue) HashKey() string { return fmt.Sprintf("int:%d", v) }

type FloatValue float64

func (v FloatValue) Type() ValueType { return TypeFloat }
func (v FloatValue) Inspect() string {
	if math.IsInf(float64(v), 1) {
		return "inf"
	}
	if math.IsInf(float64(v), -1) {
		return "-inf"
	}
	if math.IsNaN(float64(v)) {
		return "nan"
	}
	s := strconv.FormatFloat(float64(v), 'g', -1, 64)
	if !strings.Contains(s, ".") && !strings.Contains(s, "e") {
		s += ".0"
	}
	return s
}
func (v FloatValue) Truthy() bool { return v != 0.0 && !math.IsNaN(float64(v)) }
func (v FloatValue) Equals(o Value) bool {
	if ov, ok := o.(FloatValue); ok {
		return v == ov
	}
	if ov, ok := o.(IntValue); ok {
		return float64(v) == float64(ov)
	}
	if ov, ok := o.(ComplexValue); ok {
		return ov.Imag == 0 && float64(v) == ov.Real
	}
	return false
}
func (v FloatValue) HashKey() string { return fmt.Sprintf("float:%g", v) }

type ComplexValue struct {
	Real float64
	Imag float64
}

func NewComplex(re, im float64) ComplexValue {
	return ComplexValue{Real: re, Imag: im}
}

func (v ComplexValue) Type() ValueType { return TypeComplex }
func (v ComplexValue) Inspect() string {
	if v.Real == 0 {
		return fmt.Sprintf("%gj", v.Imag)
	}
	if v.Imag >= 0 {
		return fmt.Sprintf("(%g+%gj)", v.Real, v.Imag)
	}
	return fmt.Sprintf("(%g-%gj)", v.Real, -v.Imag)
}
func (v ComplexValue) Truthy() bool {
	return v.Real != 0.0 || v.Imag != 0.0
}
func (v ComplexValue) Equals(o Value) bool {
	if ov, ok := o.(ComplexValue); ok {
		return v.Real == ov.Real && v.Imag == ov.Imag
	}
	if ov, ok := o.(IntValue); ok {
		return v.Imag == 0 && v.Real == float64(ov)
	}
	if ov, ok := o.(FloatValue); ok {
		return v.Imag == 0 && v.Real == float64(ov)
	}
	return false
}
func (v ComplexValue) HashKey() string {
	return fmt.Sprintf("complex:%g+%gj", v.Real, v.Imag)
}
func (v ComplexValue) Conjugate() ComplexValue {
	return ComplexValue{Real: v.Real, Imag: -v.Imag}
}

func (v ComplexValue) Add(o ComplexValue) ComplexValue {
	return ComplexValue{Real: v.Real + o.Real, Imag: v.Imag + o.Imag}
}
func (v ComplexValue) Sub(o ComplexValue) ComplexValue {
	return ComplexValue{Real: v.Real - o.Real, Imag: v.Imag - o.Imag}
}
func (v ComplexValue) Mul(o ComplexValue) ComplexValue {
	c1 := complex(v.Real, v.Imag)
	c2 := complex(o.Real, o.Imag)
	res := c1 * c2
	return ComplexValue{Real: real(res), Imag: imag(res)}
}
func (v ComplexValue) Div(o ComplexValue) ComplexValue {
	c1 := complex(v.Real, v.Imag)
	c2 := complex(o.Real, o.Imag)
	res := c1 / c2
	return ComplexValue{Real: real(res), Imag: imag(res)}
}
func (v ComplexValue) Pow(o ComplexValue) ComplexValue {
	c1 := complex(v.Real, v.Imag)
	c2 := complex(o.Real, o.Imag)
	res := cmplx.Pow(c1, c2)
	return ComplexValue{Real: real(res), Imag: imag(res)}
}

type BoolValue bool

func (v BoolValue) Type() ValueType { return TypeBool }
func (v BoolValue) Inspect() string {
	if v {
		return "True"
	}
	return "False"
}
func (v BoolValue) Truthy() bool { return bool(v) }
func (v BoolValue) Equals(o Value) bool {
	if ov, ok := o.(BoolValue); ok {
		return v == ov
	}
	if ov, ok := o.(IntValue); ok {
		return (bool(v) && ov == 1) || (!bool(v) && ov == 0)
	}
	return false
}
func (v BoolValue) HashKey() string { return fmt.Sprintf("bool:%t", v) }

type StringValue string

func (v StringValue) Type() ValueType { return TypeString }
func (v StringValue) Inspect() string { return string(v) }
func (v StringValue) Repr() string    { return fmt.Sprintf("%q", string(v)) }
func (v StringValue) Truthy() bool    { return len(v) > 0 }
func (v StringValue) Equals(o Value) bool {
	if ov, ok := o.(StringValue); ok {
		return v == ov
	}
	return false
}
func (v StringValue) HashKey() string { return "str:" + string(v) }


type BytesValue []byte

func (b BytesValue) Type() ValueType { return TypeBytes }
func (b BytesValue) Inspect() string {
	return fmt.Sprintf("b%q", string(b))
}
func (b BytesValue) Truthy() bool { return len(b) > 0 }
func (b BytesValue) Equals(o Value) bool {
	if ob, ok := o.(BytesValue); ok {
		if len(b) != len(ob) {
			return false
		}
		for i := range b {
			if b[i] != ob[i] {
				return false
			}
		}
		return true
	}
	return false
}
func (b BytesValue) HashKey() string {
	return "bytes:" + hex.EncodeToString(b)
}
func (b BytesValue) Decode(encoding string) (string, error) {
	
	return string(b), nil
}
func (b BytesValue) Hex() string {
	return hex.EncodeToString(b)
}

type NoneVal struct{}


var None = NoneVal{}

func (v NoneVal) Type() ValueType { return TypeNone }
func (v NoneVal) Inspect() string { return "None" }
func (v NoneVal) Truthy() bool    { return false }
func (v NoneVal) Equals(o Value) bool {
	_, ok := o.(NoneVal)
	return ok
}
func (v NoneVal) HashKey() string { return "none:None" }


func IsNone(v Value) bool {
	if v == nil {
		return true
	}
	_, ok := v.(NoneVal)
	return ok
}

type EllipsisVal struct{}

var Ellipsis = EllipsisVal{}

func (v EllipsisVal) Type() ValueType { return TypeEllipsis }
func (v EllipsisVal) Inspect() string { return "..." }
func (v EllipsisVal) Truthy() bool    { return true }
func (v EllipsisVal) Equals(o Value) bool {
	_, ok := o.(EllipsisVal)
	return ok
}
func (v EllipsisVal) HashKey() string { return "ellipsis:..." }

func ToValue(v interface{}) Value {
	if v == nil {
		return None
	}
	switch val := v.(type) {
	case Value:
		return val
	case int64:
		return IntValue(val)
	case int:
		return IntValue(int64(val))
	case float64:
		return FloatValue(val)
	case complex128:
		return ComplexValue{Real: real(val), Imag: imag(val)}
	case bool:
		return BoolValue(val)
	case string:
		return StringValue(val)
	case []byte:
		return BytesValue(val)
	default:
		return StringValue(fmt.Sprintf("%v", val))
	}
}

func IsNumber(v Value) bool {
	return v.Type() == TypeInt || v.Type() == TypeFloat || v.Type() == TypeBool || v.Type() == TypeComplex
}

func ToFloat(v Value) (float64, bool) {
	switch val := v.(type) {
	case IntValue:
		return float64(val), true
	case FloatValue:
		return float64(val), true
	case BoolValue:
		if val {
			return 1.0, true
		}
		return 0.0, true
	case ComplexValue:
		if val.Imag == 0 {
			return val.Real, true
		}
	}
	return 0, false
}

func ToInt(v Value) (int64, bool) {
	switch val := v.(type) {
	case IntValue:
		return int64(val), true
	case FloatValue:
		return int64(val), true
	case BoolValue:
		if val {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

func ToComplex(v Value) (ComplexValue, bool) {
	switch val := v.(type) {
	case ComplexValue:
		return val, true
	case IntValue:
		return ComplexValue{Real: float64(val), Imag: 0}, true
	case FloatValue:
		return ComplexValue{Real: float64(val), Imag: 0}, true
	case BoolValue:
		if val {
			return ComplexValue{Real: 1, Imag: 0}, true
		}
		return ComplexValue{Real: 0, Imag: 0}, true
	}
	return ComplexValue{}, false
}
func IsComplex(v Value) bool {
	return v != nil && v.Type() == TypeComplex
}
