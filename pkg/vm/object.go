package vm

import (
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"

	"aethium/pkg/bytecode"
)

type ListObject struct {
	Elements []Value
}

func NewList(elements ...Value) *ListObject {
	return &ListObject{Elements: elements}
}

func (l *ListObject) Type() ValueType { return TypeList }
func (l *ListObject) Inspect() string {
	var els []string
	for _, e := range l.Elements {
		if str, ok := e.(StringValue); ok {
			els = append(els, fmt.Sprintf("'%s'", strings.ReplaceAll(string(str), "'", "\\'")))
		} else {
			els = append(els, e.Inspect())
		}
	}
	return "[" + strings.Join(els, ", ") + "]"
}
func (l *ListObject) Truthy() bool { return len(l.Elements) > 0 }
func (l *ListObject) Equals(o Value) bool {
	ol, ok := o.(*ListObject)
	if !ok || len(l.Elements) != len(ol.Elements) {
		return false
	}
	for i, v := range l.Elements {
		if !v.Equals(ol.Elements[i]) {
			return false
		}
	}
	return true
}
func (l *ListObject) HashKey() string { return fmt.Sprintf("list:%p", l) }

type TupleObject struct {
	Elements []Value
}

func NewTuple(elements ...Value) *TupleObject {
	return &TupleObject{Elements: elements}
}

func (t *TupleObject) Type() ValueType { return TypeTuple }
func (t *TupleObject) Inspect() string {
	var els []string
	for _, e := range t.Elements {
		if str, ok := e.(StringValue); ok {
			els = append(els, fmt.Sprintf("'%s'", strings.ReplaceAll(string(str), "'", "\\'")))
		} else {
			els = append(els, e.Inspect())
		}
	}
	if len(els) == 1 {
		return "(" + els[0] + ",)"
	}
	return "(" + strings.Join(els, ", ") + ")"
}
func (t *TupleObject) Truthy() bool { return len(t.Elements) > 0 }
func (t *TupleObject) Equals(o Value) bool {
	ot, ok := o.(*TupleObject)
	if !ok || len(t.Elements) != len(ot.Elements) {
		return false
	}
	for i, v := range t.Elements {
		if !v.Equals(ot.Elements[i]) {
			return false
		}
	}
	return true
}
func (t *TupleObject) HashKey() string {
	var sb strings.Builder
	sb.WriteString("tuple:")
	for _, e := range t.Elements {
		sb.WriteString(e.HashKey())
		sb.WriteString(";")
	}
	return sb.String()
}

type DictPair struct {
	Key   Value
	Value Value
}

type DictObject struct {
	Pairs map[string]DictPair
}

func NewDict() *DictObject {
	return &DictObject{Pairs: make(map[string]DictPair)}
}

func (d *DictObject) Type() ValueType { return TypeDict }
func (d *DictObject) Inspect() string {
	var pairs []string
	for _, pair := range d.Pairs {
		kStr := pair.Key.Inspect()
		if s, ok := pair.Key.(StringValue); ok {
			kStr = fmt.Sprintf("%q", string(s))
		}
		vStr := pair.Value.Inspect()
		if s, ok := pair.Value.(StringValue); ok {
			vStr = fmt.Sprintf("%q", string(s))
		}
		pairs = append(pairs, fmt.Sprintf("%s: %s", kStr, vStr))
	}
	return "{" + strings.Join(pairs, ", ") + "}"
}
func (d *DictObject) Truthy() bool { return len(d.Pairs) > 0 }
func (d *DictObject) Equals(o Value) bool {
	od, ok := o.(*DictObject)
	if !ok || len(d.Pairs) != len(od.Pairs) {
		return false
	}
	for k, pair := range d.Pairs {
		oPair, exists := od.Pairs[k]
		if !exists || !pair.Value.Equals(oPair.Value) {
			return false
		}
	}
	return true
}
func (d *DictObject) HashKey() string { return fmt.Sprintf("dict:%p", d) }

func (d *DictObject) Get(key Value) (Value, bool) {
	pair, ok := d.Pairs[key.HashKey()]
	if !ok {
		return nil, false
	}
	return pair.Value, true
}

func (d *DictObject) Set(key Value, val Value) {
	d.Pairs[key.HashKey()] = DictPair{Key: key, Value: val}
}

func (d *DictObject) Delete(key Value) {
	delete(d.Pairs, key.HashKey())
}

type SetObject struct {
	Elements map[string]Value
}

func NewSet(elements ...Value) *SetObject {
	s := &SetObject{Elements: make(map[string]Value)}
	for _, e := range elements {
		s.Elements[e.HashKey()] = e
	}
	return s
}

func (s *SetObject) Type() ValueType { return TypeSet }
func (s *SetObject) Inspect() string {
	if len(s.Elements) == 0 {
		return "set()"
	}
	var els []string
	for _, e := range s.Elements {
		if str, ok := e.(StringValue); ok {
			els = append(els, fmt.Sprintf("%q", string(str)))
		} else {
			els = append(els, e.Inspect())
		}
	}
	return "{" + strings.Join(els, ", ") + "}"
}
func (s *SetObject) Truthy() bool { return len(s.Elements) > 0 }
func (s *SetObject) Equals(o Value) bool {
	os, ok := o.(*SetObject)
	if !ok || len(s.Elements) != len(os.Elements) {
		return false
	}
	for k := range s.Elements {
		if _, exists := os.Elements[k]; !exists {
			return false
		}
	}
	return true
}
func (s *SetObject) HashKey() string { return fmt.Sprintf("set:%p", s) }

type ByteArrayObject struct {
	Bytes []byte
}

func NewByteArray(b []byte) *ByteArrayObject {
	cpy := make([]byte, len(b))
	copy(cpy, b)
	return &ByteArrayObject{Bytes: cpy}
}

func (b *ByteArrayObject) Type() ValueType { return TypeByteArray }
func (b *ByteArrayObject) Inspect() string {
	return fmt.Sprintf("bytearray(b%q)", string(b.Bytes))
}
func (b *ByteArrayObject) Truthy() bool { return len(b.Bytes) > 0 }
func (b *ByteArrayObject) Equals(o Value) bool {
	if ob, ok := o.(*ByteArrayObject); ok {
		if len(b.Bytes) != len(ob.Bytes) {
			return false
		}
		for i := range b.Bytes {
			if b.Bytes[i] != ob.Bytes[i] {
				return false
			}
		}
		return true
	}
	if ob, ok := o.(BytesValue); ok {
		if len(b.Bytes) != len(ob) {
			return false
		}
		for i := range b.Bytes {
			if b.Bytes[i] != ob[i] {
				return false
			}
		}
		return true
	}
	return false
}
func (b *ByteArrayObject) HashKey() string                        { return fmt.Sprintf("bytearray:%p", b) }
func (b *ByteArrayObject) Append(val byte)                        { b.Bytes = append(b.Bytes, val) }
func (b *ByteArrayObject) Extend(vals []byte)                     { b.Bytes = append(b.Bytes, vals...) }
func (b *ByteArrayObject) Decode(encoding string) (string, error) { return string(b.Bytes), nil }
func (b *ByteArrayObject) Hex() string                            { return hex.EncodeToString(b.Bytes) }

// Frozenset Object: immutable set
type FrozensetObject struct {
	Elements map[string]Value
}

func NewFrozenset(elements ...Value) *FrozensetObject {
	fs := &FrozensetObject{Elements: make(map[string]Value)}
	for _, e := range elements {
		fs.Elements[e.HashKey()] = e
	}
	return fs
}

func (fs *FrozensetObject) Type() ValueType { return TypeFrozenset }
func (fs *FrozensetObject) Inspect() string {
	if len(fs.Elements) == 0 {
		return "frozenset()"
	}
	var els []string
	for _, e := range fs.Elements {
		if str, ok := e.(StringValue); ok {
			els = append(els, fmt.Sprintf("%q", string(str)))
		} else {
			els = append(els, e.Inspect())
		}
	}
	return "frozenset({" + strings.Join(els, ", ") + "})"
}
func (fs *FrozensetObject) Truthy() bool { return len(fs.Elements) > 0 }
func (fs *FrozensetObject) Equals(o Value) bool {
	if ofs, ok := o.(*FrozensetObject); ok {
		if len(fs.Elements) != len(ofs.Elements) {
			return false
		}
		for k := range fs.Elements {
			if _, exists := ofs.Elements[k]; !exists {
				return false
			}
		}
		return true
	}
	if os, ok := o.(*SetObject); ok {
		if len(fs.Elements) != len(os.Elements) {
			return false
		}
		for k := range fs.Elements {
			if _, exists := os.Elements[k]; !exists {
				return false
			}
		}
		return true
	}
	return false
}
func (fs *FrozensetObject) HashKey() string {
	keys := make([]string, 0, len(fs.Elements))
	for k := range fs.Elements {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return "frozenset:" + strings.Join(keys, ";")
}

// MemoryView Object
type MemoryViewObject struct {
	Obj Value // BytesValue or *ByteArrayObject
}

func NewMemoryView(obj Value) *MemoryViewObject {
	return &MemoryViewObject{Obj: obj}
}

func (m *MemoryViewObject) Type() ValueType     { return TypeMemoryView }
func (m *MemoryViewObject) Inspect() string     { return fmt.Sprintf("<memory at %p>", m) }
func (m *MemoryViewObject) Truthy() bool        { return true }
func (m *MemoryViewObject) Equals(o Value) bool { return m == o }
func (m *MemoryViewObject) HashKey() string     { return fmt.Sprintf("memoryview:%p", m) }
func (m *MemoryViewObject) ToBytes() []byte {
	if b, ok := m.Obj.(BytesValue); ok {
		return []byte(b)
	}
	if ba, ok := m.Obj.(*ByteArrayObject); ok {
		return ba.Bytes
	}
	return nil
}

// WeakRef Object
type WeakRefObject struct {
	Referent Value
}

func NewWeakRef(ref Value) *WeakRefObject {
	return &WeakRefObject{Referent: ref}
}

func (w *WeakRefObject) Type() ValueType { return TypeWeakRef }
func (w *WeakRefObject) Inspect() string {
	return fmt.Sprintf("<weakref to %s at %p>", w.Referent.Type(), w)
}
func (w *WeakRefObject) Truthy() bool        { return w.Referent != nil && !IsNone(w.Referent) }
func (w *WeakRefObject) Equals(o Value) bool { return w == o }
func (w *WeakRefObject) HashKey() string     { return fmt.Sprintf("weakref:%p", w) }
func (w *WeakRefObject) Deref() Value        { return w.Referent }

// Descriptors: Property, ClassMethod, StaticMethod
type PropertyObject struct {
	FGet Value
	FSet Value
	FDel Value
	Doc  string
}

func NewProperty(fget, fset, fdel Value, doc string) *PropertyObject {
	return &PropertyObject{FGet: fget, FSet: fset, FDel: fdel, Doc: doc}
}

func (p *PropertyObject) Type() ValueType     { return TypeProperty }
func (p *PropertyObject) Inspect() string     { return fmt.Sprintf("<property at %p>", p) }
func (p *PropertyObject) Truthy() bool        { return true }
func (p *PropertyObject) Equals(o Value) bool { return p == o }
func (p *PropertyObject) HashKey() string     { return fmt.Sprintf("property:%p", p) }

type ClassMethodObject struct {
	Func Value
}

func NewClassMethod(fn Value) *ClassMethodObject {
	return &ClassMethodObject{Func: fn}
}

func (cm *ClassMethodObject) Type() ValueType { return TypeClassMethod }
func (cm *ClassMethodObject) Inspect() string {
	return fmt.Sprintf("<classmethod %s>", cm.Func.Inspect())
}
func (cm *ClassMethodObject) Truthy() bool        { return true }
func (cm *ClassMethodObject) Equals(o Value) bool { return cm == o }
func (cm *ClassMethodObject) HashKey() string     { return fmt.Sprintf("classmethod:%p", cm) }

type StaticMethodObject struct {
	Func Value
}

func NewStaticMethod(fn Value) *StaticMethodObject {
	return &StaticMethodObject{Func: fn}
}

func (sm *StaticMethodObject) Type() ValueType { return TypeStaticMethod }
func (sm *StaticMethodObject) Inspect() string {
	return fmt.Sprintf("<staticmethod %s>", sm.Func.Inspect())
}
func (sm *StaticMethodObject) Truthy() bool        { return true }
func (sm *StaticMethodObject) Equals(o Value) bool { return sm == o }
func (sm *StaticMethodObject) HashKey() string     { return fmt.Sprintf("staticmethod:%p", sm) }

// Function & Closure
type FunctionObject struct {
	CompiledFunction *bytecode.CompiledFunction
}

func (f *FunctionObject) Type() ValueType { return TypeFunction }
func (f *FunctionObject) Inspect() string {
	return fmt.Sprintf("<function %s>", f.CompiledFunction.Name)
}
func (f *FunctionObject) Truthy() bool        { return true }
func (f *FunctionObject) Equals(o Value) bool { return f == o }
func (f *FunctionObject) HashKey() string     { return fmt.Sprintf("fn:%p", f) }

type ClosureObject struct {
	Fn       *bytecode.CompiledFunction
	Free     []Value
	Defaults []Value
}

func (c *ClosureObject) Type() ValueType     { return TypeClosure }
func (c *ClosureObject) Inspect() string     { return fmt.Sprintf("<function %s>", c.Fn.Name) }
func (c *ClosureObject) Truthy() bool        { return true }
func (c *ClosureObject) Equals(o Value) bool { return c == o }
func (c *ClosureObject) HashKey() string     { return fmt.Sprintf("closure:%p", c) }

// Builtin Function
type BuiltinFn func(args ...Value) (Value, error)

type BuiltinFunction struct {
	Name string
	Fn   BuiltinFn
}

func (b *BuiltinFunction) Type() ValueType     { return TypeBuiltin }
func (b *BuiltinFunction) Inspect() string     { return fmt.Sprintf("<built-in function %s>", b.Name) }
func (b *BuiltinFunction) Truthy() bool        { return true }
func (b *BuiltinFunction) Equals(o Value) bool { return b == o }
func (b *BuiltinFunction) HashKey() string     { return fmt.Sprintf("builtin:%s", b.Name) }

// OOP: Class, Instance, BoundMethod
type ClassObject struct {
	Name  string
	Bases []*ClassObject
	// MRO is the C3-linearised method-resolution order (includes self).
	MRO     []*ClassObject
	Methods map[string]Value // can be *ClosureObject, *ClassMethodObject, *StaticMethodObject, *PropertyObject, *BuiltinFunction
	Static  map[string]Value
	Slots   []string
	Doc     string
}

func NewClass(name string, bases []*ClassObject) *ClassObject {
	cls := &ClassObject{
		Name:    name,
		Bases:   bases,
		Methods: make(map[string]Value),
		Static:  make(map[string]Value),
	}
	cls.MRO = computeMRO(cls)
	return cls
}

// computeMRO returns the C3 linearisation of cls.
func computeMRO(cls *ClassObject) []*ClassObject {
	if len(cls.Bases) == 0 {
		return []*ClassObject{cls}
	}
	// Build list of sequences to merge.
	seqs := make([][]*ClassObject, 0, len(cls.Bases)+1)
	for _, b := range cls.Bases {
		mro := computeMRO(b)
		seqs = append(seqs, append([]*ClassObject{}, mro...))
	}
	// Append linearisation of direct bases.
	bases := make([]*ClassObject, len(cls.Bases))
	copy(bases, cls.Bases)
	seqs = append(seqs, bases)

	result := []*ClassObject{cls}
	for {
		// Remove empty seqs.
		var nonempty [][]*ClassObject
		for _, s := range seqs {
			if len(s) > 0 {
				nonempty = append(nonempty, s)
			}
		}
		if len(nonempty) == 0 {
			break
		}
		// Find a good head.
		var good *ClassObject
	outer:
		for _, s := range nonempty {
			candidate := s[0]
			for _, s2 := range nonempty {
				for _, t := range s2[1:] {
					if t == candidate {
						continue outer
					}
				}
			}
			good = candidate
			break
		}
		if good == nil {
			// Inconsistent MRO — fall back to first base.
			break
		}
		result = append(result, good)
		// Remove good from heads of all seqs.
		for i, s := range nonempty {
			if len(s) > 0 && s[0] == good {
				nonempty[i] = s[1:]
			}
		}
		seqs = nonempty
	}
	return result
}

func (c *ClassObject) Type() ValueType     { return TypeClass }
func (c *ClassObject) Inspect() string     { return fmt.Sprintf("<class '%s'>", c.Name) }
func (c *ClassObject) Truthy() bool        { return true }
func (c *ClassObject) Equals(o Value) bool { return c == o }
func (c *ClassObject) HashKey() string     { return fmt.Sprintf("class:%s", c.Name) }

// LookupMethod walks the MRO to find the first class that defines name.
func (c *ClassObject) LookupMethod(name string) (Value, bool) {
	for _, cls := range c.MRO {
		if m, ok := cls.Methods[name]; ok {
			return m, true
		}
	}
	return nil, false
}

// LookupMethodStartingAfter finds the first definition of name after skip in the MRO.
// Used to implement super().
func (c *ClassObject) LookupMethodStartingAfter(name string, skip *ClassObject) (Value, *ClassObject, bool) {
	found := false
	for _, cls := range c.MRO {
		if cls == skip {
			found = true
			continue
		}
		if !found {
			continue
		}
		if m, ok := cls.Methods[name]; ok {
			return m, cls, true
		}
	}
	return nil, nil, false
}

// HasSlots reports whether any class in MRO declares __slots__.
func (c *ClassObject) HasSlots() bool {
	for _, cls := range c.MRO {
		if len(cls.Slots) > 0 {
			return true
		}
	}
	return false
}

// AllowsSlot reports whether name is an allowed slot in the class MRO.
func (c *ClassObject) AllowsSlot(name string) bool {
	for _, cls := range c.MRO {
		for _, s := range cls.Slots {
			if s == name {
				return true
			}
		}
	}
	return false
}

// IsSubclassOf reports whether c is a subclass (or equal to) other.
func (c *ClassObject) IsSubclassOf(other *ClassObject) bool {
	for _, cls := range c.MRO {
		if cls == other {
			return true
		}
	}
	return false
}

type InstanceObject struct {
	Class  *ClassObject
	Fields map[string]Value
}

func NewInstance(cls *ClassObject) *InstanceObject {
	return &InstanceObject{
		Class:  cls,
		Fields: make(map[string]Value),
	}
}

func (i *InstanceObject) Type() ValueType { return TypeInstance }
func (i *InstanceObject) Inspect() string {
	return fmt.Sprintf("<%s object at %p>", i.Class.Name, i)
}
func (i *InstanceObject) Truthy() bool { return true }
func (i *InstanceObject) Equals(o Value) bool {
	return i == o
}
func (i *InstanceObject) HashKey() string { return fmt.Sprintf("inst:%p", i) }

func (i *InstanceObject) GetAttr(name string) (Value, bool) {
	if val, ok := i.Fields[name]; ok {
		return val, true
	}
	if method, ok := i.Class.LookupMethod(name); ok {
		switch m := method.(type) {
		case *PropertyObject:
			return m, true
		case *ClassMethodObject:
			return &BoundMethodObject{Instance: nil, Method: m.Func, Class: i.Class, IsClassMethod: true}, true
		case *StaticMethodObject:
			return m.Func, true
		case *ClosureObject:
			return &BoundMethodObject{Instance: i, Method: m, Class: i.Class}, true
		case *BuiltinFunction:
			return &BoundMethodObject{Instance: i, Method: m, Class: i.Class}, true
		default:
			return m, true
		}
	}
	// Walk MRO for static/class attributes.
	for _, cls := range i.Class.MRO {
		if stat, ok := cls.Static[name]; ok {
			return stat, true
		}
	}
	if name == "__doc__" {
		return StringValue(i.Class.Doc), true
	}
	return nil, false
}

func (i *InstanceObject) SetAttr(name string, val Value) error {
	if method, ok := i.Class.LookupMethod(name); ok {
		if prop, isProp := method.(*PropertyObject); isProp {
			if prop.FSet == nil || IsNone(prop.FSet) {
				return fmt.Errorf("AttributeError: can't set attribute '%s'", name)
			}
			// Property handled via VM call
			return nil
		}
	}
	if i.Class.HasSlots() && !i.Class.AllowsSlot(name) {
		return fmt.Errorf("AttributeError: '%s' object has no attribute '%s'", i.Class.Name, name)
	}
	i.Fields[name] = val
	return nil
}

func (i *InstanceObject) DelAttr(name string) error {
	if method, ok := i.Class.LookupMethod(name); ok {
		if prop, isProp := method.(*PropertyObject); isProp {
			if prop.FDel == nil || IsNone(prop.FDel) {
				return fmt.Errorf("AttributeError: can't delete attribute '%s'", name)
			}
			return nil
		}
	}
	if _, ok := i.Fields[name]; !ok {
		return fmt.Errorf("AttributeError: '%s' object has no attribute '%s'", i.Class.Name, name)
	}
	delete(i.Fields, name)
	return nil
}

// SuperObject acts as a proxy that starts method lookup after a given class.
type SuperObject struct {
	Instance  *InstanceObject
	StartFrom *ClassObject // the class after which to search
}

func (s *SuperObject) Type() ValueType     { return TypeSuper }
func (s *SuperObject) Inspect() string     { return fmt.Sprintf("<super: %s>", s.StartFrom.Name) }
func (s *SuperObject) Truthy() bool        { return true }
func (s *SuperObject) Equals(o Value) bool { return s == o }
func (s *SuperObject) HashKey() string     { return fmt.Sprintf("super:%p", s) }

type BoundMethodObject struct {
	Instance      *InstanceObject
	Method        Value        // *ClosureObject, *BuiltinFunction, etc.
	Class         *ClassObject // class in which method was found (for super chain)
	IsClassMethod bool
}

func (b *BoundMethodObject) Type() ValueType { return TypeBoundMethod }
func (b *BoundMethodObject) Inspect() string {
	if b.IsClassMethod {
		return fmt.Sprintf("<bound classmethod %s.%s>", b.Class.Name, b.Method.Inspect())
	}
	return fmt.Sprintf("<bound method %s.%s>", b.Instance.Class.Name, b.Method.Inspect())
}
func (b *BoundMethodObject) Truthy() bool        { return true }
func (b *BoundMethodObject) Equals(o Value) bool { return b == o }
func (b *BoundMethodObject) HashKey() string     { return fmt.Sprintf("method:%p", b) }

// Module Object
type ModuleObject struct {
	Name    string
	Exports map[string]Value
}

func NewModule(name string) *ModuleObject {
	return &ModuleObject{
		Name:    name,
		Exports: make(map[string]Value),
	}
}

func (m *ModuleObject) Type() ValueType     { return TypeModule }
func (m *ModuleObject) Inspect() string     { return fmt.Sprintf("<module '%s'>", m.Name) }
func (m *ModuleObject) Truthy() bool        { return true }
func (m *ModuleObject) Equals(o Value) bool { return m == o }
func (m *ModuleObject) HashKey() string     { return fmt.Sprintf("mod:%s", m.Name) }

func (m *ModuleObject) GetAttr(name string) (Value, bool) {
	v, ok := m.Exports[name]
	return v, ok
}

// Range Object
type RangeObject struct {
	Start int64
	Stop  int64
	Step  int64
}

func (r *RangeObject) Type() ValueType { return TypeRange }
func (r *RangeObject) Inspect() string {
	if r.Step == 1 {
		return fmt.Sprintf("range(%d, %d)", r.Start, r.Stop)
	}
	return fmt.Sprintf("range(%d, %d, %d)", r.Start, r.Stop, r.Step)
}
func (r *RangeObject) Truthy() bool {
	if r.Step > 0 {
		return r.Start < r.Stop
	}
	return r.Start > r.Stop
}
func (r *RangeObject) Equals(o Value) bool {
	or, ok := o.(*RangeObject)
	return ok && r.Start == or.Start && r.Stop == or.Stop && r.Step == or.Step
}
func (r *RangeObject) HashKey() string {
	return fmt.Sprintf("range:%d:%d:%d", r.Start, r.Stop, r.Step)
}

// Iterator Object
type IteratorObject struct {
	Items []Value
	Index int
}

func (it *IteratorObject) Type() ValueType     { return TypeIterator }
func (it *IteratorObject) Inspect() string     { return "<iterator object>" }
func (it *IteratorObject) Truthy() bool        { return true }
func (it *IteratorObject) Equals(o Value) bool { return it == o }
func (it *IteratorObject) HashKey() string     { return fmt.Sprintf("iter:%p", it) }

func (it *IteratorObject) Next() (Value, bool) {
	if it.Index >= len(it.Items) {
		return nil, false
	}
	val := it.Items[it.Index]
	it.Index++
	return val, true
}

// ChannelObject wraps a Go channel with safe close semantics.
type ChannelObject struct {
	Ch     chan Value
	mu     sync.Mutex
	closed bool
}

func NewChannel(bufSize int) *ChannelObject {
	return &ChannelObject{Ch: make(chan Value, bufSize)}
}

func (c *ChannelObject) Type() ValueType     { return TypeChannel }
func (c *ChannelObject) Inspect() string     { return fmt.Sprintf("<channel at %p>", c) }
func (c *ChannelObject) Truthy() bool        { return true }
func (c *ChannelObject) Equals(o Value) bool { return c == o }
func (c *ChannelObject) HashKey() string     { return fmt.Sprintf("chan:%p", c) }

// Send returns an error if the channel is already closed.
func (c *ChannelObject) Send(v Value) error {
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()
	if closed {
		return NewException("RuntimeError", "send on closed channel")
	}
	c.Ch <- v
	return nil
}

// Recv returns (value, true) on success, or (None, false) when channel is closed and empty.
func (c *ChannelObject) Recv() (Value, bool) {
	v, ok := <-c.Ch
	if !ok {
		return None, false
	}
	return v, true
}

// Close closes the channel exactly once; repeated calls are safe.
func (c *ChannelObject) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return NewException("RuntimeError", "channel already closed")
	}
	c.closed = true
	close(c.Ch)
	return nil
}

// IsClosed reports whether the channel has been closed.
func (c *ChannelObject) IsClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

// StackFrame is one entry in an Aethium traceback.
type StackFrame struct {
	Filename string
	FuncName string
	Line     int
}

// ExceptionObject is the runtime representation of any Aethium exception.
// It carries a type name, message, optional cause, and a Go-level traceback.
type ExceptionObject struct {
	TypeStr         string       // e.g. "ValueError"
	Message         string       // human-readable description
	Traceback       []StackFrame // most-recent call first
	Cause           Value        // __cause__ (raise X from Y)
	Context         Value        // __context__ (implicit exception chaining)
	SuppressContext bool         // __suppress_context__
	Args            []Value      // positional constructor arguments
}

func NewException(typStr, msg string) *ExceptionObject {
	return &ExceptionObject{TypeStr: typStr, Message: msg}
}

func (e *ExceptionObject) Type() ValueType     { return TypeException }
func (e *ExceptionObject) Inspect() string     { return fmt.Sprintf("%s: %s", e.TypeStr, e.Message) }
func (e *ExceptionObject) Truthy() bool        { return true }
func (e *ExceptionObject) Equals(o Value) bool { return e == o }
func (e *ExceptionObject) HashKey() string     { return fmt.Sprintf("exc:%s:%s", e.TypeStr, e.Message) }
func (e *ExceptionObject) Error() string       { return e.Inspect() }

// ExceptionGroupObject represents Python 3.11+ ExceptionGroup
type ExceptionGroupObject struct {
	ExceptionObject
	Exceptions []Value
}

func NewExceptionGroup(msg string, exceptions []Value) *ExceptionGroupObject {
	return &ExceptionGroupObject{
		ExceptionObject: ExceptionObject{
			TypeStr: "ExceptionGroup",
			Message: msg,
		},
		Exceptions: exceptions,
	}
}

func (eg *ExceptionGroupObject) Type() ValueType { return TypeExceptionGroup }
func (eg *ExceptionGroupObject) Inspect() string {
	var parts []string
	for _, ex := range eg.Exceptions {
		parts = append(parts, ex.Inspect())
	}
	return fmt.Sprintf("ExceptionGroup('%s', [%s])", eg.Message, strings.Join(parts, ", "))
}
func (eg *ExceptionGroupObject) Truthy() bool        { return len(eg.Exceptions) > 0 }
func (eg *ExceptionGroupObject) Equals(o Value) bool { return eg == o }
func (eg *ExceptionGroupObject) HashKey() string     { return fmt.Sprintf("excgroup:%p", eg) }

func (eg *ExceptionGroupObject) Subgroup(predicate func(Value) bool) *ExceptionGroupObject {
	var matching []Value
	for _, ex := range eg.Exceptions {
		if predicate(ex) {
			matching = append(matching, ex)
		}
	}
	if len(matching) == 0 {
		return nil
	}
	return NewExceptionGroup(eg.Message, matching)
}

func (eg *ExceptionGroupObject) Split(predicate func(Value) bool) (*ExceptionGroupObject, *ExceptionGroupObject) {
	var match []Value
	var rest []Value
	for _, ex := range eg.Exceptions {
		if predicate(ex) {
			match = append(match, ex)
		} else {
			rest = append(rest, ex)
		}
	}
	var mGroup, rGroup *ExceptionGroupObject
	if len(match) > 0 {
		mGroup = NewExceptionGroup(eg.Message, match)
	}
	if len(rest) > 0 {
		rGroup = NewExceptionGroup(eg.Message, rest)
	}
	return mGroup, rGroup
}

// FormatTraceback returns a Python-style traceback string.
func (e *ExceptionObject) FormatTraceback() string {
	if len(e.Traceback) == 0 {
		return e.Inspect()
	}
	var sb strings.Builder
	sb.WriteString("Traceback (most recent call last):\n")
	for i := len(e.Traceback) - 1; i >= 0; i-- {
		f := e.Traceback[i]
		fmt.Fprintf(&sb, "  File %q, line %d, in %s\n", f.Filename, f.Line, f.FuncName)
	}
	sb.WriteString(e.Inspect())
	return sb.String()
}

// IsExceptionType reports whether val is an ExceptionObject with the given type name.
func IsExceptionType(val Value, typStr string) bool {
	if exc, ok := val.(*ExceptionObject); ok {
		return exc.TypeStr == typStr || typStr == "Exception"
	}
	return false
}

// WrapError converts a Go error into an Aethium RuntimeError ExceptionObject.
func WrapError(err error) *ExceptionObject {
	if exc, ok := err.(*ExceptionObject); ok {
		return exc
	}
	return NewException("RuntimeError", err.Error())
}

// Sentinel exit value so embedding hosts can detect sys.exit() without os.Exit.
type ExitError struct {
	Code int
}

func (e *ExitError) Error() string { return fmt.Sprintf("SystemExit: %d", e.Code) }

// GeneratorObject wraps a paused coroutine frame for yield-based generators.
type GeneratorObject struct {
	mu      sync.Mutex
	fn      *ClosureObject
	stack   [2048]Value
	sp      int
	frames  [256]Frame
	fi      int // frameIndex in generator's private frame stack
	ip      int // saved ip inside the generator frame
	done    bool
	started bool
	// shared constants/names from the parent VM
	constants []Value
	names     []string
	globals   map[string]Value
	builtins  []*BuiltinFunction
}

func (g *GeneratorObject) Type() ValueType     { return TypeGenerator }
func (g *GeneratorObject) Inspect() string     { return fmt.Sprintf("<generator object %s>", g.fn.Fn.Name) }
func (g *GeneratorObject) Truthy() bool        { return true }
func (g *GeneratorObject) Equals(o Value) bool { return g == o }
func (g *GeneratorObject) HashKey() string     { return fmt.Sprintf("gen:%p", g) }

func (g *GeneratorObject) Resume(val Value) (Value, error) {
	if g.done {
		return nil, fmt.Errorf("StopIteration")
	}
	// Simple generator resumption
	return None, nil
}

// RangeIterator is a lazy, non-allocating iterator over a range.
type RangeIterator struct {
	current int64
	stop    int64
	step    int64
}

func NewRangeIterator(r *RangeObject) *RangeIterator {
	return &RangeIterator{
		current: r.Start,
		stop:    r.Stop,
		step:    r.Step,
	}
}

func (ri *RangeIterator) Type() ValueType     { return TypeIterator }
func (ri *RangeIterator) Inspect() string     { return "<range_iterator object>" }
func (ri *RangeIterator) Truthy() bool        { return true }
func (ri *RangeIterator) Equals(o Value) bool { return ri == o }
func (ri *RangeIterator) HashKey() string     { return fmt.Sprintf("rangeiter:%p", ri) }

func (ri *RangeIterator) Next() (Value, bool) {
	if (ri.step > 0 && ri.current >= ri.stop) || (ri.step < 0 && ri.current <= ri.stop) {
		return nil, false
	}
	val := IntValue(ri.current)
	ri.current += ri.step
	return val, true
}
