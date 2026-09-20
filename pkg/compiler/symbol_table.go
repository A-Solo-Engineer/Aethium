package compiler

type SymbolScope string

const (
    GlobalScope   SymbolScope = "GLOBAL"
    LocalScope    SymbolScope = "LOCAL"
    BuiltinScope  SymbolScope = "BUILTIN"
    FreeScope     SymbolScope = "FREE"
    FunctionScope SymbolScope = "FUNCTION"
    // CellScope marks a local that is captured by an inner closure (nonlocal target).
    CellScope     SymbolScope = "CELL"
)

type Symbol struct {
    Name  string
    Scope SymbolScope
    Index int
}

type SymbolTable struct {
    Outer          *SymbolTable
    store          map[string]Symbol
    numDefinitions int
    FreeSymbols    []Symbol
    // globalForced holds names declared `global` inside this scope.
    globalForced map[string]bool
    // nonlocalForced holds names declared `nonlocal` inside this scope.
    nonlocalForced map[string]bool
}

func NewSymbolTable() *SymbolTable {
    return &SymbolTable{
        store:          make(map[string]Symbol),
        FreeSymbols:    []Symbol{},
        numDefinitions: 0,
        globalForced:   make(map[string]bool),
        nonlocalForced: make(map[string]bool),
    }
}

func NewEnclosedSymbolTable(outer *SymbolTable) *SymbolTable {
    s := NewSymbolTable()
    s.Outer = outer
    return s
}

// isGlobalScope returns true when there is no enclosing function scope.
func (s *SymbolTable) isGlobalScope() bool {
    return s.Outer == nil
}

// DefineGlobal pins name as global in this (non-global) scope.
// Subsequent Define/Resolve calls for this name will use GlobalScope.
func (s *SymbolTable) DefineGlobal(name string) {
    s.globalForced[name] = true
    // Remove any local definition so Resolve falls through correctly.
    delete(s.store, name)
}

// DefineNonlocal pins name as a free variable referring to an enclosing scope.
func (s *SymbolTable) DefineNonlocal(name string) {
    s.nonlocalForced[name] = true
    delete(s.store, name)
}

func (s *SymbolTable) Define(name string) Symbol {
    // Honour explicit `global` declaration.
    if s.globalForced[name] {
        return s.defineGlobalAlias(name)
    }
    symbol := Symbol{Name: name, Index: s.numDefinitions}
    if s.Outer == nil {
        symbol.Scope = GlobalScope
    } else {
        symbol.Scope = LocalScope
    }
    s.store[name] = symbol
    s.numDefinitions++
    return symbol
}

// defineGlobalAlias returns a GlobalScope symbol without allocating a local slot.
func (s *SymbolTable) defineGlobalAlias(name string) Symbol {
    sym := Symbol{Name: name, Scope: GlobalScope, Index: 0}
    s.store[name] = sym
    return sym
}

func (s *SymbolTable) DefineBuiltin(index int, name string) Symbol {
    symbol := Symbol{Name: name, Index: index, Scope: BuiltinScope}
    s.store[name] = symbol
    return symbol
}

func (s *SymbolTable) DefineFunctionName(name string) Symbol {
    symbol := Symbol{Name: name, Index: 0, Scope: FunctionScope}
    s.store[name] = symbol
    return symbol
}

func (s *SymbolTable) DefineFree(original Symbol) Symbol {
    s.FreeSymbols = append(s.FreeSymbols, original)
    symbol := Symbol{Name: original.Name, Index: len(s.FreeSymbols) - 1, Scope: FreeScope}
    s.store[original.Name] = symbol
    return symbol
}

func (s *SymbolTable) Resolve(name string) (Symbol, bool) {
    // If explicitly declared global, bypass local store.
    if s.globalForced[name] {
        // Walk up to the global (outermost) table to get the real index.
        if s.Outer != nil {
            root := s
            for root.Outer != nil {
                root = root.Outer
            }
            sym, ok := root.store[name]
            if !ok {
                sym = Symbol{Name: name, Scope: GlobalScope}
            }
            sym.Scope = GlobalScope
            return sym, true
        }
    }

    obj, ok := s.store[name]
    if !ok && s.Outer != nil {
        obj, ok = s.Outer.Resolve(name)
        if !ok {
            return obj, ok
        }
        // Global and builtin symbols are passed through as-is.
        if obj.Scope == GlobalScope || obj.Scope == BuiltinScope {
            return obj, ok
        }
        // nonlocal: treat as free variable.
        if s.nonlocalForced[name] {
            free := s.DefineFree(obj)
            return free, true
        }
        free := s.DefineFree(obj)
        return free, true
    }
    return obj, ok
}
