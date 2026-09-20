package bytecode

// Version is the bytecode format version; used for validation on load.
const Version uint16 = 2

// Magic bytes written at the start of serialised bytecode files.
var Magic = [4]byte{'A', 'E', 'T', 'H'}

// CompiledFunction holds the compiled bytecode and metadata for one function.
type CompiledFunction struct {
    Instructions  Instructions
    NumLocals     int
    NumParameters int
    // ParamNames are positional parameter names in declaration order.
    ParamNames []string
    // Defaults are compile-time constant defaults for the trailing N params.
    // Defaults[0] corresponds to ParamNames[NumParameters-len(Defaults)].
    Defaults []interface{}
    // DefaultCount is the number of parameters that have defaults.
    DefaultCount int
    // VarArg is the name of the *args parameter, or "" if none.
    VarArg string
    // KwArg is the name of the **kwargs parameter, or "" if none.
    KwArg string
    // IsGenerator marks that this function uses yield and should be wrapped.
    IsGenerator bool
    Name        string
    Doc         string
    // Filename and FirstLine are set by the compiler for error reporting.
    Filename  string
    FirstLine int
}

func (cf *CompiledFunction) String() string {
    return cf.Instructions.String()
}
