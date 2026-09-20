package ffi

import (
    "sync"

    "aethium/pkg/vm"
)

// GoFunction is a native Go function callable from Aethium.
type GoFunction func(args ...vm.Value) (vm.Value, error)

// Registry manages native Go extensions and plugins.
type Registry struct {
    mu        sync.RWMutex
    functions map[string]GoFunction
    modules   map[string]map[string]GoFunction
}

var DefaultRegistry = NewRegistry()

func NewRegistry() *Registry {
    return &Registry{
        functions: make(map[string]GoFunction),
        modules:   make(map[string]map[string]GoFunction),
    }
}

// RegisterFunction exposes a Go function globally to Aethium.
func (r *Registry) RegisterFunction(name string, fn GoFunction) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.functions[name] = fn
}

// RegisterModule exposes a named Go module with multiple functions to Aethium.
func (r *Registry) RegisterModule(modName string, funcs map[string]GoFunction) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.modules[modName] = funcs
}

// LookupFunction finds a registered native function.
func (r *Registry) LookupFunction(name string) (GoFunction, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    fn, ok := r.functions[name]
    return fn, ok
}

// AllFunctions returns a snapshot map of all registered native Go functions.
func (r *Registry) AllFunctions() map[string]GoFunction {
    r.mu.RLock()
    defer r.mu.RUnlock()
    res := make(map[string]GoFunction, len(r.functions))
    for k, v := range r.functions {
        res[k] = v
    }
    return res
}

// AllModules returns a snapshot map of all registered native Go modules.
func (r *Registry) AllModules() map[string]map[string]GoFunction {
    r.mu.RLock()
    defer r.mu.RUnlock()
    res := make(map[string]map[string]GoFunction, len(r.modules))
    for modName, funcs := range r.modules {
        modCopy := make(map[string]GoFunction, len(funcs))
        for k, v := range funcs {
            modCopy[k] = v
        }
        res[modName] = modCopy
    }
    return res
}

// ExportModule creates a *vm.ModuleObject containing all module functions.
func (r *Registry) ExportModule(modName string) (*vm.ModuleObject, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    funcs, ok := r.modules[modName]
    if !ok {
        return nil, false
    }
    mod := vm.NewModule(modName)
    for fnName, goFn := range funcs {
        captured := goFn
        mod.Exports[fnName] = &vm.BuiltinFunction{
            Name: fnName,
            Fn: func(args ...vm.Value) (vm.Value, error) {
                return captured(args...)
            },
        }
    }
    return mod, true
}
