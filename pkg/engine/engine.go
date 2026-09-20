package engine

import (
    "context"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "sync"

    "aethium/pkg/compiler"
    "aethium/pkg/ffi"
    "aethium/pkg/lexer"
    "aethium/pkg/parser"
    "aethium/pkg/stdlib"
    "aethium/pkg/vm"
)

// Version is the Aethium language version, centralised here and in CLI / REPL.
const Version = "1.3.0"

// Config allows embedding hosts to customise Engine behaviour.
type Config struct {
    // Stdout is where print() output goes. Defaults to os.Stdout.
    Stdout io.Writer
    // Stderr is where error output goes. Defaults to os.Stderr.
    Stderr io.Writer
    // Context can be used to cancel execution.
    Context context.Context
    // Globals are pre-populated global variables.
    Globals map[string]vm.Value
    // MaxRecursion sets the maximum call depth. 0 means use the VM default.
    MaxRecursion int
    // NoExit prevents Aethium sys.exit() from calling os.Exit().
    // When true, Execute() returns an *ExitError instead.
    NoExit bool
    // AllowSubprocess permits child process creation via the subprocess module (defaults to false for sandboxed execution).
    AllowSubprocess bool
    // AllowNetwork permits socket and network operations (defaults to false for sandboxed execution).
    AllowNetwork bool
}

// ExitError is returned by Execute when sys.exit() is called and Config.NoExit is true.
type ExitError = vm.ExitError

// Engine compiles and executes Aethium source code.
type Engine struct {
    cfg        Config
    globals    map[string]vm.Value
    mu         sync.RWMutex
    modules    map[string]vm.Value
    modulesMu  sync.RWMutex
    loading    map[string]bool
    loadingMu  sync.Mutex
    stdlibMods map[string]vm.Value
    lastVM     *vm.VM
}

// NewEngine creates an Engine with default configuration.
func NewEngine() *Engine {
    return NewEngineWithConfig(Config{})
}

// NewEngineWithConfig creates an Engine with the given configuration.
func NewEngineWithConfig(cfg Config) *Engine {
    if cfg.Stdout == nil { cfg.Stdout = os.Stdout }
    if cfg.Stderr == nil { cfg.Stderr = os.Stderr }
    if cfg.Context == nil { cfg.Context = context.Background() }

    // Configure standard library sandboxing permissions
    stdlib.SetSecurityOptions(cfg.AllowSubprocess, cfg.AllowNetwork)

    globals := cfg.Globals
    if globals == nil {
        globals = make(map[string]vm.Value)
    }

    // Load any native Go functions registered in the default FFI registry
    for name, fn := range ffi.DefaultRegistry.AllFunctions() {
        if _, exists := globals[name]; !exists {
            globals[name] = &vm.BuiltinFunction{
                Name: name,
                Fn:   vm.BuiltinFn(fn),
            }
        }
    }

    // Cache standard library modules once at engine initialization
    stdMods := stdlib.LoadModules()
    stdlibMap := make(map[string]vm.Value, len(stdMods))
    for k, v := range stdMods {
        stdlibMap[k] = v
    }

    eng := &Engine{
        cfg:        cfg,
        globals:    globals,
        modules:    make(map[string]vm.Value),
        loading:    make(map[string]bool),
        stdlibMods: stdlibMap,
    }

    // Load any native Go modules registered in the default FFI registry
    for modName, funcs := range ffi.DefaultRegistry.AllModules() {
        mod := vm.NewModule(modName)
        for fnName, fn := range funcs {
            mod.Exports[fnName] = &vm.BuiltinFunction{
                Name: fnName,
                Fn:   vm.BuiltinFn(fn),
            }
        }
        eng.modules[modName] = mod
        if _, exists := eng.globals[modName]; !exists {
            eng.globals[modName] = mod
        }
    }

    // Inject standard library modules into globals table and modules registry.
    for name, mod := range stdMods {
        eng.modules[name] = mod
        if _, exists := eng.globals[name]; !exists {
            eng.globals[name] = mod
        }
    }

    return eng
}

// Globals returns a thread-safe snapshot copy of the engine's global variable table.
func (e *Engine) Globals() map[string]vm.Value {
    e.mu.RLock()
    defer e.mu.RUnlock()
    res := make(map[string]vm.Value, len(e.globals))
    for k, v := range e.globals {
        res[k] = v
    }
    return res
}

// SetGlobal thread-safely injects a named value into the global namespace before or during execution.
func (e *Engine) SetGlobal(name string, val vm.Value) {
    e.mu.Lock()
    defer e.mu.Unlock()
    e.globals[name] = val
}

// RegisterGoFunction registers a native Go function into the engine's globals and the FFI registry.
func (e *Engine) RegisterGoFunction(name string, fn ffi.GoFunction) {
    ffi.DefaultRegistry.RegisterFunction(name, fn)
    bf := &vm.BuiltinFunction{
        Name: name,
        Fn:   vm.BuiltinFn(fn),
    }
    e.mu.Lock()
    e.globals[name] = bf
    e.mu.Unlock()
}

// RegisterModule registers a native Go module with multiple functions into the engine's module cache and FFI registry.
// Locks are acquired sequentially (never nested) to prevent lock contention and deadlock hazards.
func (e *Engine) RegisterModule(modName string, funcs map[string]ffi.GoFunction) {
    ffi.DefaultRegistry.RegisterModule(modName, funcs)
    mod := vm.NewModule(modName)
    for fnName, fn := range funcs {
        mod.Exports[fnName] = &vm.BuiltinFunction{
            Name: fnName,
            Fn:   vm.BuiltinFn(fn),
        }
    }

    // 1. Update modules cache under modulesMu
    e.modulesMu.Lock()
    e.modules[modName] = mod
    e.modulesMu.Unlock()

    // 2. Update globals under mu (disjoint lock acquisition, zero nested locking)
    e.mu.Lock()
    e.globals[modName] = mod
    e.mu.Unlock()
}

// LoadModule loads a module by name: first checking memory cache, then cached stdlib, then .aeth user files.
func (e *Engine) LoadModule(name string, callingVM *vm.VM) (vm.Value, error) {
    e.modulesMu.RLock()
    if mod, ok := e.modules[name]; ok {
        e.modulesMu.RUnlock()
        return mod, nil
    }
    e.modulesMu.RUnlock()

    // 1. Built-in stdlib check (from cached stdlib modules)
    if mod, ok := e.stdlibMods[name]; ok {
        e.modulesMu.Lock()
        e.modules[name] = mod
        e.modulesMu.Unlock()
        return mod, nil
    }

    // Circular import detection
    e.loadingMu.Lock()
    if e.loading[name] {
        e.loadingMu.Unlock()
        return nil, fmt.Errorf("ImportError: circular import detected for module '%s'", name)
    }
    e.loading[name] = true
    e.loadingMu.Unlock()

    defer func() {
        e.loadingMu.Lock()
        delete(e.loading, name)
        e.loadingMu.Unlock()
    }()

    // 2. User module discovery on filesystem
    var currentDir string
    if callingVM != nil && callingVM.Filename() != "" && callingVM.Filename() != "<module>" && callingVM.Filename() != "<stdin>" && callingVM.Filename() != "<string>" {
        currentDir = filepath.Dir(callingVM.Filename())
    }

    searchDirs := []string{}
    if currentDir != "" {
        searchDirs = append(searchDirs, currentDir)
    }
    searchDirs = append(searchDirs, ".", "examples", "pkg")

    var foundPath string
    for _, dir := range searchDirs {
        candidates := []string{
            filepath.Join(dir, name+".aeth"),
            filepath.Join(dir, name, "__init__.aeth"),
            filepath.Join(dir, name+".py"),
        }
        for _, cand := range candidates {
            if info, err := os.Stat(cand); err == nil && !info.IsDir() {
                foundPath = cand
                break
            }
        }
        if foundPath != "" {
            break
        }
    }

    if foundPath == "" {
        return nil, fmt.Errorf("ImportError: No module named '%s'", name)
    }

    srcBytes, err := os.ReadFile(foundPath)
    if err != nil {
        return nil, fmt.Errorf("ImportError: failed to read module file '%s': %w", foundPath, err)
    }

    l := lexer.New(foundPath, string(srcBytes))
    p := parser.New(l)
    prog := p.ParseProgram()
    if len(p.Errors()) > 0 {
        return nil, fmt.Errorf("SyntaxError in module %s (%s): %s", name, foundPath, p.Errors()[0])
    }

    c := compiler.NewCompilerWithFilename(foundPath)
    if err := c.Compile(prog); err != nil {
        return nil, fmt.Errorf("CompileError in module %s (%s): %v", name, foundPath, err)
    }

    bc := c.Bytecode()
    vmBC := &vm.Bytecode{
        Instructions: bc.Instructions,
        Constants:    bc.Constants,
        Names:        bc.Names,
    }

    modGlobals := make(map[string]vm.Value)
    modGlobalsMu := &sync.RWMutex{}
    modGlobals["__name__"] = vm.StringValue(name)
    modGlobals["__file__"] = vm.StringValue(foundPath)

    // Pre-populate standard library modules in module namespace using cached stdlibMods
    for stdName, stdMod := range e.stdlibMods {
        modGlobals[stdName] = stdMod
    }

    subMachine := vm.NewWithGlobals(vmBC, modGlobals)
    subMachine.SetGlobalsMu(modGlobalsMu)
    subMachine.SetStdout(e.cfg.Stdout)
    subMachine.SetStderr(e.cfg.Stderr)
    subMachine.SetContext(e.cfg.Context)
    subMachine.SetFilename(foundPath)
    subMachine.SetImportLoader(func(mName string) (vm.Value, error) {
        return e.LoadModule(mName, subMachine)
    })

    if err := subMachine.Run(); err != nil {
        return nil, err
    }

    modObj := vm.NewModule(name)
    modGlobalsMu.RLock()
    for k, v := range modGlobals {
        modObj.Exports[k] = v
    }
    modGlobalsMu.RUnlock()

    e.modulesMu.Lock()
    e.modules[name] = modObj
    e.modulesMu.Unlock()

    return modObj, nil
}

// WaitGoroutines blocks until all spawned goroutines from the last execution complete.
func (e *Engine) WaitGoroutines() {
    e.mu.RLock()
    machine := e.lastVM
    e.mu.RUnlock()
    if machine != nil {
        machine.WaitGoroutines()
    }
}

// DrainGoroutineErrors returns all errors captured from background goroutines.
func (e *Engine) DrainGoroutineErrors() []error {
    e.mu.RLock()
    machine := e.lastVM
    e.mu.RUnlock()
    if machine != nil {
        return machine.DrainGoroutineErrors()
    }
    return nil
}

// WaitAndDrainGoroutineErrors waits for active goroutines to finish and returns all errors.
func (e *Engine) WaitAndDrainGoroutineErrors() []error {
    e.WaitGoroutines()
    return e.DrainGoroutineErrors()
}

// Execute compiles and runs source, returning the last evaluated value.
// Syntax and compile errors are wrapped with position information.
// Runtime errors are returned as *vm.ExceptionObject where possible.
// If Config.NoExit is true, sys.exit() returns *ExitError instead of calling os.Exit().
func (e *Engine) Execute(filename, source string) (vm.Value, error) {
    l := lexer.New(filename, source)
    p := parser.New(l)
    prog := p.ParseProgram()

    if len(p.Errors()) > 0 {
        return nil, fmt.Errorf("SyntaxError in %s: %s", filename, p.Errors()[0])
    }

    c := compiler.NewCompilerWithFilename(filename)
    if err := c.Compile(prog); err != nil {
        return nil, fmt.Errorf("CompileError in %s: %v", filename, err)
    }

    bc := c.Bytecode()
    vmBC := &vm.Bytecode{
        Instructions: bc.Instructions,
        Constants:    bc.Constants,
        Names:        bc.Names,
    }

    // Pass shared globals map and mutex directly so concurrent executions and SetGlobal remain thread-safe without overwriting.
    machine := vm.NewWithGlobals(vmBC, e.globals)
    machine.SetGlobalsMu(&e.mu)
    machine.SetStdout(e.cfg.Stdout)
    machine.SetStderr(e.cfg.Stderr)
    machine.SetContext(e.cfg.Context)
    machine.SetFilename(filename)
    machine.SetImportLoader(func(modName string) (vm.Value, error) {
        return e.LoadModule(modName, machine)
    })

    e.mu.Lock()
    e.lastVM = machine
    e.mu.Unlock()

    runErr := machine.Run()

    if runErr != nil {
        // Propagate ExitError to caller when NoExit mode is active.
        if exitErr, ok := runErr.(*ExitError); ok {
            if e.cfg.NoExit {
                return nil, exitErr
            }
            os.Exit(exitErr.Code)
        }
        return nil, runErr
    }

    return machine.LastPoppedStackElem(), nil
}

// CompileToBytecode parses and compiles source without executing it.
func (e *Engine) CompileToBytecode(filename, source string) (*compiler.Bytecode, error) {
    l := lexer.New(filename, source)
    p := parser.New(l)
    prog := p.ParseProgram()

    if len(p.Errors()) > 0 {
        return nil, fmt.Errorf("SyntaxError in %s: %s", filename, p.Errors()[0])
    }

    c := compiler.NewCompilerWithFilename(filename)
    if err := c.Compile(prog); err != nil {
        return nil, fmt.Errorf("CompileError in %s: %v", filename, err)
    }

    return c.Bytecode(), nil
}

// ExecuteWithContext is a convenience wrapper that binds a context to execution.
func (e *Engine) ExecuteWithContext(ctx context.Context, filename, source string) (vm.Value, error) {
    old := e.cfg.Context
    e.cfg.Context = ctx
    defer func() { e.cfg.Context = old }()
    return e.Execute(filename, source)
}
