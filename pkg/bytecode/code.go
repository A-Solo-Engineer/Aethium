package bytecode

const Version uint16 = 2

var Magic = [4]byte{'A', 'E', 'T', 'H'}

type CompiledFunction struct {
	Instructions  Instructions
	NumLocals     int
	NumParameters int
	ParamNames    []string
	Defaults      []interface{}
	DefaultCount  int
	VarArg        string
	KwArg         string
	IsGenerator   bool
	Name          string
	Doc           string
	Filename      string
	FirstLine     int
}

func (cf *CompiledFunction) String() string {
	return cf.Instructions.String()
}
