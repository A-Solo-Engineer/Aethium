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

const Version = "1.3.0"

type Config struct {
	Stdout          io.Writer
	Stderr          io.Writer
	Context         context.Context
	Globals         map[string]vm.Value
	MaxRecursion    int
	NoExit          bool
	AllowSubprocess bool
	AllowNetwork    bool
}

type ExitError = vm.ExitError

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

func NewEngine() *Engine {
	return NewEngineWithConfig(Config{})
}

func NewEngineWithConfig(cfg Config) *Engine {
	if cfg.Stdout == nil {
		cfg.Stdout = os.Stdout
	}
	if cfg.Stderr == nil {
		cfg.Stderr = os.Stderr
	}
	if cfg.Context == nil {
		cfg.Context = context.Background()
	}

	stdlib.SetSecurityOptions(cfg.AllowSubprocess, cfg.AllowNetwork)

	globals := cfg.Globals
	if globals == nil {
		globals = make(map[string]vm.Value)
	}

	for name, fn := range ffi.DefaultRegistry.AllFunctions() {
		if _, exists := globals[name]; !exists {
			globals[name] = &vm.BuiltinFunction{
				Name: name,
				Fn:   vm.BuiltinFn(fn),
			}
		}
	}

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

	for name, mod := range stdMods {
		eng.modules[name] = mod
		if _, exists := eng.globals[name]; !exists {
			eng.globals[name] = mod
		}
	}

	return eng
}

func (e *Engine) Globals() map[string]vm.Value {
	e.mu.RLock()
	defer e.mu.RUnlock()
	res := make(map[string]vm.Value, len(e.globals))
	for k, v := range e.globals {
		res[k] = v
	}
	return res
}

func (e *Engine) SetGlobal(name string, val vm.Value) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.globals[name] = val
}

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

func (e *Engine) RegisterModule(modName string, funcs map[string]ffi.GoFunction) {
	ffi.DefaultRegistry.RegisterModule(modName, funcs)
	mod := vm.NewModule(modName)
	for fnName, fn := range funcs {
		mod.Exports[fnName] = &vm.BuiltinFunction{
			Name: fnName,
			Fn:   vm.BuiltinFn(fn),
		}
	}

	e.modulesMu.Lock()
	e.modules[modName] = mod
	e.modulesMu.Unlock()

	e.mu.Lock()
	e.globals[modName] = mod
	e.mu.Unlock()
}

func (e *Engine) LoadModule(name string, callingVM *vm.VM) (vm.Value, error) {
	e.modulesMu.RLock()
	if mod, ok := e.modules[name]; ok {
		e.modulesMu.RUnlock()
		return mod, nil
	}
	e.modulesMu.RUnlock()

	if mod, ok := e.stdlibMods[name]; ok {
		e.modulesMu.Lock()
		e.modules[name] = mod
		e.modulesMu.Unlock()
		return mod, nil
	}

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

func (e *Engine) WaitGoroutines() {
	e.mu.RLock()
	machine := e.lastVM
	e.mu.RUnlock()
	if machine != nil {
		machine.WaitGoroutines()
	}
}

func (e *Engine) DrainGoroutineErrors() []error {
	e.mu.RLock()
	machine := e.lastVM
	e.mu.RUnlock()
	if machine != nil {
		return machine.DrainGoroutineErrors()
	}
	return nil
}

func (e *Engine) WaitAndDrainGoroutineErrors() []error {
	e.WaitGoroutines()
	return e.DrainGoroutineErrors()
}

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

func (e *Engine) ExecuteWithContext(ctx context.Context, filename, source string) (vm.Value, error) {
	old := e.cfg.Context
	e.cfg.Context = ctx
	defer func() { e.cfg.Context = old }()
	return e.Execute(filename, source)
}
