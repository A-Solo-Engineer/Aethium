package stdlib

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"aethium/pkg/vm"
)

// Version is the Aethium standard library version.
const Version = "1.3.0"

// LoadModules returns all built-in stdlib modules.
func LoadModules() map[string]*vm.ModuleObject {
	modules := make(map[string]*vm.ModuleObject)
	modules["math"] = initMathModule()
	modules["time"] = initTimeModule()
	modules["os"] = initOSModule()
	modules["sys"] = initSysModule()
	modules["json"] = initJSONModule()
	modules["random"] = initRandomModule()
	modules["re"] = initReModule()
	modules["http"] = initHTTPModule()
	modules["sync"] = initSyncModule()
	modules["io"] = initIOModule()
	modules["string"] = initStringModule()
	modules["pathlib"] = initPathlibModule()
	modules["datetime"] = initDatetimeModule()
	modules["collections"] = initCollectionsModule()
	modules["itertools"] = initItertoolsModule()
	modules["functools"] = initFunctoolsModule()
	modules["subprocess"] = initSubprocessModule()
	modules["socket"] = initSocketModule()
	modules["csv"] = initCSVModule()
	modules["xml"] = initXMLModule()
	modules["argparse"] = initArgparseModule()
	modules["logging"] = initLoggingModule()
	modules["unittest"] = initUnittestModule()
	modules["cProfile"] = initCProfileModule()
	modules["profile"] = initCProfileModule()
	modules["weakref"] = initWeakRefModule()
	return modules
}

func initMathModule() *vm.ModuleObject {
	m := vm.NewModule("math")
	m.Exports["pi"] = vm.FloatValue(math.Pi)
	m.Exports["e"] = vm.FloatValue(math.E)
	m.Exports["tau"] = vm.FloatValue(2 * math.Pi)
	m.Exports["inf"] = vm.FloatValue(math.Inf(1))
	m.Exports["nan"] = vm.FloatValue(math.NaN())

	fn1 := func(name string, f func(float64) float64) *vm.BuiltinFunction {
		return &vm.BuiltinFunction{Name: name, Fn: func(args ...vm.Value) (vm.Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("TypeError: %s() missing argument", name)
			}
			v, _ := vm.ToFloat(args[0])
			return vm.FloatValue(f(v)), nil
		}}
	}
	m.Exports["sqrt"] = fn1("sqrt", math.Sqrt)
	m.Exports["sin"] = fn1("sin", math.Sin)
	m.Exports["cos"] = fn1("cos", math.Cos)
	m.Exports["tan"] = fn1("tan", math.Tan)
	m.Exports["asin"] = fn1("asin", math.Asin)
	m.Exports["acos"] = fn1("acos", math.Acos)
	m.Exports["atan"] = fn1("atan", math.Atan)
	m.Exports["log"] = &vm.BuiltinFunction{Name: "log", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("TypeError: log() missing argument")
		}
		v, _ := vm.ToFloat(args[0])
		if len(args) > 1 {
			base, _ := vm.ToFloat(args[1])
			return vm.FloatValue(math.Log(v) / math.Log(base)), nil
		}
		return vm.FloatValue(math.Log(v)), nil
	}}
	m.Exports["log2"] = fn1("log2", math.Log2)
	m.Exports["log10"] = fn1("log10", math.Log10)
	m.Exports["exp"] = fn1("exp", math.Exp)
	m.Exports["floor"] = &vm.BuiltinFunction{Name: "floor", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("TypeError: floor() missing argument")
		}
		f, _ := vm.ToFloat(args[0])
		return vm.IntValue(int64(math.Floor(f))), nil
	}}
	m.Exports["ceil"] = &vm.BuiltinFunction{Name: "ceil", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("TypeError: ceil() missing argument")
		}
		f, _ := vm.ToFloat(args[0])
		return vm.IntValue(int64(math.Ceil(f))), nil
	}}
	m.Exports["trunc"] = &vm.BuiltinFunction{Name: "trunc", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("TypeError: trunc() missing argument")
		}
		f, _ := vm.ToFloat(args[0])
		return vm.IntValue(int64(math.Trunc(f))), nil
	}}
	m.Exports["atan2"] = &vm.BuiltinFunction{Name: "atan2", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("TypeError: atan2() requires 2 arguments")
		}
		y, _ := vm.ToFloat(args[0])
		x, _ := vm.ToFloat(args[1])
		return vm.FloatValue(math.Atan2(y, x)), nil
	}}
	m.Exports["hypot"] = &vm.BuiltinFunction{Name: "hypot", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("TypeError: hypot() requires 2 arguments")
		}
		x, _ := vm.ToFloat(args[0])
		y, _ := vm.ToFloat(args[1])
		return vm.FloatValue(math.Hypot(x, y)), nil
	}}
	m.Exports["pow"] = &vm.BuiltinFunction{Name: "pow", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("TypeError: pow() requires 2 arguments")
		}
		base, _ := vm.ToFloat(args[0])
		exp, _ := vm.ToFloat(args[1])
		return vm.FloatValue(math.Pow(base, exp)), nil
	}}
	m.Exports["factorial"] = &vm.BuiltinFunction{Name: "factorial", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("TypeError: factorial() requires 1 argument")
		}
		n, _ := vm.ToInt(args[0])
		if n < 0 {
			return nil, fmt.Errorf("ValueError: factorial() not defined for negative values")
		}
		var res int64 = 1
		for i := int64(2); i <= n; i++ {
			res *= i
		}
		return vm.IntValue(res), nil
	}}
	m.Exports["gcd"] = &vm.BuiltinFunction{Name: "gcd", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("TypeError: gcd() requires 2 arguments")
		}
		a, _ := vm.ToInt(args[0])
		b, _ := vm.ToInt(args[1])
		if a < 0 {
			a = -a
		}
		if b < 0 {
			b = -b
		}
		for b != 0 {
			a, b = b, a%b
		}
		return vm.IntValue(a), nil
	}}
	m.Exports["isnan"] = &vm.BuiltinFunction{Name: "isnan", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("TypeError: isnan() requires 1 argument")
		}
		f, _ := vm.ToFloat(args[0])
		return vm.BoolValue(math.IsNaN(f)), nil
	}}
	m.Exports["isinf"] = &vm.BuiltinFunction{Name: "isinf", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("TypeError: isinf() requires 1 argument")
		}
		f, _ := vm.ToFloat(args[0])
		return vm.BoolValue(math.IsInf(f, 0)), nil
	}}
	m.Exports["degrees"] = &vm.BuiltinFunction{Name: "degrees", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("TypeError: degrees() requires 1 argument")
		}
		f, _ := vm.ToFloat(args[0])
		return vm.FloatValue(f * 180 / math.Pi), nil
	}}
	m.Exports["radians"] = &vm.BuiltinFunction{Name: "radians", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("TypeError: radians() requires 1 argument")
		}
		f, _ := vm.ToFloat(args[0])
		return vm.FloatValue(f * math.Pi / 180), nil
	}}
	return m
}

func initTimeModule() *vm.ModuleObject {
	m := vm.NewModule("time")
	m.Exports["time"] = &vm.BuiltinFunction{Name: "time", Fn: func(args ...vm.Value) (vm.Value, error) {
		return vm.FloatValue(float64(time.Now().UnixNano()) / 1e9), nil
	}}
	m.Exports["sleep"] = &vm.BuiltinFunction{Name: "sleep", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) == 0 {
			return vm.None, nil
		}
		f, _ := vm.ToFloat(args[0])
		time.Sleep(time.Duration(f * float64(time.Second)))
		return vm.None, nil
	}}
	m.Exports["perf_counter"] = &vm.BuiltinFunction{Name: "perf_counter", Fn: func(args ...vm.Value) (vm.Value, error) {
		return vm.FloatValue(float64(time.Now().UnixNano()) / 1e9), nil
	}}
	m.Exports["monotonic"] = m.Exports["perf_counter"]
	m.Exports["ctime"] = &vm.BuiltinFunction{Name: "ctime", Fn: func(args ...vm.Value) (vm.Value, error) {
		return vm.StringValue(time.Now().Format(time.ANSIC)), nil
	}}
	m.Exports["strftime"] = &vm.BuiltinFunction{Name: "strftime", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("TypeError: strftime() requires a format string")
		}
		// Minimal Python → Go format translation.
		pyFmt := args[0].Inspect()
		goFmt := strings.NewReplacer(
			"%Y", "2006", "%m", "01", "%d", "02",
			"%H", "15", "%M", "04", "%S", "05",
			"%f", "000000", "%A", "Monday", "%B", "January",
		).Replace(pyFmt)
		return vm.StringValue(time.Now().Format(goFmt)), nil
	}}
	return m
}

func initOSModule() *vm.ModuleObject {
	m := vm.NewModule("os")
	m.Exports["name"] = vm.StringValue("posix")
	m.Exports["sep"] = vm.StringValue(string(os.PathSeparator))
	m.Exports["linesep"] = vm.StringValue("\n")

	m.Exports["getcwd"] = &vm.BuiltinFunction{Name: "getcwd", Fn: func(args ...vm.Value) (vm.Value, error) {
		dir, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		return vm.StringValue(dir), nil
	}}
	m.Exports["chdir"] = &vm.BuiltinFunction{Name: "chdir", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("TypeError: chdir() requires a path")
		}
		if err := os.Chdir(args[0].Inspect()); err != nil {
			return nil, err
		}
		return vm.None, nil
	}}
	m.Exports["listdir"] = &vm.BuiltinFunction{Name: "listdir", Fn: func(args ...vm.Value) (vm.Value, error) {
		path := "."
		if len(args) > 0 {
			path = args[0].Inspect()
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, fmt.Errorf("OSError: %v", err)
		}
		var list []vm.Value
		for _, e := range entries {
			list = append(list, vm.StringValue(e.Name()))
		}
		return vm.NewList(list...), nil
	}}
	m.Exports["path"] = initOSPathModule()
	m.Exports["getenv"] = &vm.BuiltinFunction{Name: "getenv", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) == 0 {
			return vm.None, nil
		}
		val := os.Getenv(args[0].Inspect())
		if val == "" && len(args) > 1 {
			return args[1], nil
		}
		return vm.StringValue(val), nil
	}}
	m.Exports["environ"] = func() vm.Value {
		d := vm.NewDict()
		for _, kv := range os.Environ() {
			parts := strings.SplitN(kv, "=", 2)
			if len(parts) == 2 {
				d.Set(vm.StringValue(parts[0]), vm.StringValue(parts[1]))
			}
		}
		return d
	}()
	m.Exports["makedirs"] = &vm.BuiltinFunction{Name: "makedirs", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("TypeError: makedirs() requires a path")
		}
		if err := os.MkdirAll(args[0].Inspect(), 0755); err != nil {
			return nil, err
		}
		return vm.None, nil
	}}
	m.Exports["remove"] = &vm.BuiltinFunction{Name: "remove", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("TypeError: remove() requires a path")
		}
		if err := os.Remove(args[0].Inspect()); err != nil {
			return nil, err
		}
		return vm.None, nil
	}}
	m.Exports["rename"] = &vm.BuiltinFunction{Name: "rename", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("TypeError: rename() requires 2 arguments")
		}
		if err := os.Rename(args[0].Inspect(), args[1].Inspect()); err != nil {
			return nil, err
		}
		return vm.None, nil
	}}
	return m
}

func initOSPathModule() *vm.ModuleObject {
	m := vm.NewModule("os.path")
	m.Exports["join"] = &vm.BuiltinFunction{Name: "join", Fn: func(args ...vm.Value) (vm.Value, error) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = a.Inspect()
		}
		return vm.StringValue(filepath.Join(parts...)), nil
	}}
	m.Exports["exists"] = &vm.BuiltinFunction{Name: "exists", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) == 0 {
			return vm.BoolValue(false), nil
		}
		_, err := os.Stat(args[0].Inspect())
		return vm.BoolValue(!os.IsNotExist(err)), nil
	}}
	m.Exports["dirname"] = &vm.BuiltinFunction{Name: "dirname", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) == 0 {
			return vm.StringValue(""), nil
		}
		return vm.StringValue(filepath.Dir(args[0].Inspect())), nil
	}}
	m.Exports["basename"] = &vm.BuiltinFunction{Name: "basename", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) == 0 {
			return vm.StringValue(""), nil
		}
		return vm.StringValue(filepath.Base(args[0].Inspect())), nil
	}}
	m.Exports["abspath"] = &vm.BuiltinFunction{Name: "abspath", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) == 0 {
			return vm.StringValue(""), nil
		}
		p, err := filepath.Abs(args[0].Inspect())
		if err != nil {
			return nil, err
		}
		return vm.StringValue(p), nil
	}}
	m.Exports["isfile"] = &vm.BuiltinFunction{Name: "isfile", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) == 0 {
			return vm.BoolValue(false), nil
		}
		info, err := os.Stat(args[0].Inspect())
		return vm.BoolValue(err == nil && !info.IsDir()), nil
	}}
	m.Exports["isdir"] = &vm.BuiltinFunction{Name: "isdir", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) == 0 {
			return vm.BoolValue(false), nil
		}
		info, err := os.Stat(args[0].Inspect())
		return vm.BoolValue(err == nil && info.IsDir()), nil
	}}
	return m
}

func initSysModule() *vm.ModuleObject {
	m := vm.NewModule("sys")
	m.Exports["version"] = vm.StringValue("Aethium " + Version + " (Go Runtime)")
	m.Exports["version_info"] = vm.NewTuple(vm.IntValue(1), vm.IntValue(1), vm.IntValue(0))
	m.Exports["platform"] = vm.StringValue("linux")
	m.Exports["maxsize"] = vm.IntValue(math.MaxInt64)
	var argv []vm.Value
	for _, a := range os.Args {
		argv = append(argv, vm.StringValue(a))
	}
	m.Exports["argv"] = vm.NewList(argv...)

	// sys.exit: return ExitError so the engine can decide whether to os.Exit.
	m.Exports["exit"] = &vm.BuiltinFunction{Name: "exit", Fn: func(args ...vm.Value) (vm.Value, error) {
		code := 0
		if len(args) > 0 {
			if c, ok := vm.ToInt(args[0]); ok {
				code = int(c)
			}
		}
		return vm.None, &vm.ExitError{Code: code}
	}}
	m.Exports["stdout"] = vm.NewModule("stdout") // placeholder
	m.Exports["stderr"] = vm.NewModule("stderr")
	m.Exports["stdin"] = vm.NewModule("stdin")
	return m
}

func initJSONModule() *vm.ModuleObject {
	m := vm.NewModule("json")
	m.Exports["dumps"] = &vm.BuiltinFunction{Name: "dumps", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("TypeError: dumps() missing obj")
		}
		indent := 0
		if len(args) > 1 {
			if i, ok := vm.ToInt(args[1]); ok {
				indent = int(i)
			}
		}
		native := valueToNative(args[0])
		var b []byte
		var err error
		if indent > 0 {
			b, err = json.MarshalIndent(native, "", strings.Repeat(" ", indent))
		} else {
			b, err = json.Marshal(native)
		}
		if err != nil {
			return nil, fmt.Errorf("JSONEncodeError: %v", err)
		}
		return vm.StringValue(string(b)), nil
	}}
	m.Exports["loads"] = &vm.BuiltinFunction{Name: "loads", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("TypeError: loads() missing str")
		}
		var res interface{}
		if err := json.Unmarshal([]byte(args[0].Inspect()), &res); err != nil {
			return nil, fmt.Errorf("JSONDecodeError: %v", err)
		}
		return nativeToValue(res), nil
	}}
	return m
}

func initRandomModule() *vm.ModuleObject {
	m := vm.NewModule("random")
	src := rand.New(rand.NewSource(time.Now().UnixNano()))
	m.Exports["seed"] = &vm.BuiltinFunction{Name: "seed", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) > 0 {
			if s, ok := vm.ToInt(args[0]); ok {
				src = rand.New(rand.NewSource(s))
			}
		}
		return vm.None, nil
	}}
	m.Exports["random"] = &vm.BuiltinFunction{Name: "random", Fn: func(args ...vm.Value) (vm.Value, error) { return vm.FloatValue(src.Float64()), nil }}
	m.Exports["randint"] = &vm.BuiltinFunction{Name: "randint", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("TypeError: randint() requires 2 arguments")
		}
		a, _ := vm.ToInt(args[0])
		b, _ := vm.ToInt(args[1])
		if b < a {
			return nil, fmt.Errorf("ValueError: empty range for randint(%d, %d)", a, b)
		}
		return vm.IntValue(a + src.Int63n(b-a+1)), nil
	}}
	m.Exports["uniform"] = &vm.BuiltinFunction{Name: "uniform", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("TypeError: uniform() requires 2 arguments")
		}
		a, _ := vm.ToFloat(args[0])
		b, _ := vm.ToFloat(args[1])
		return vm.FloatValue(a + src.Float64()*(b-a)), nil
	}}
	m.Exports["choice"] = &vm.BuiltinFunction{Name: "choice", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("TypeError: choice() requires 1 argument")
		}
		items := vm.ExtractIterable(args[0])
		if len(items) == 0 {
			return nil, fmt.Errorf("IndexError: Cannot choose from an empty sequence")
		}
		return items[src.Intn(len(items))], nil
	}}
	m.Exports["shuffle"] = &vm.BuiltinFunction{Name: "shuffle", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) > 0 {
			if l, ok := args[0].(*vm.ListObject); ok {
				src.Shuffle(len(l.Elements), func(i, j int) { l.Elements[i], l.Elements[j] = l.Elements[j], l.Elements[i] })
			}
		}
		return vm.None, nil
	}}
	m.Exports["sample"] = &vm.BuiltinFunction{Name: "sample", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("TypeError: sample() requires 2 arguments")
		}
		items := vm.ExtractIterable(args[0])
		k, _ := vm.ToInt(args[1])
		if int(k) > len(items) {
			return nil, fmt.Errorf("ValueError: Sample larger than population")
		}
		perm := src.Perm(len(items))
		res := make([]vm.Value, k)
		for i := int64(0); i < k; i++ {
			res[i] = items[perm[i]]
		}
		return vm.NewList(res...), nil
	}}
	return m
}

func initReModule() *vm.ModuleObject {
	m := vm.NewModule("re")
	compile := func(pattern string) (*regexp.Regexp, error) {
		r, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("re.error: %v", err)
		}
		return r, nil
	}
	m.Exports["search"] = &vm.BuiltinFunction{Name: "search", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("TypeError: search() requires 2 arguments")
		}
		re, err := compile(args[0].Inspect())
		if err != nil {
			return nil, err
		}
		loc := re.FindStringIndex(args[1].Inspect())
		if loc == nil {
			return vm.None, nil
		}
		return vm.StringValue(args[1].Inspect()[loc[0]:loc[1]]), nil
	}}
	m.Exports["match"] = &vm.BuiltinFunction{Name: "match", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("TypeError: match() requires 2 arguments")
		}
		re, err := compile("^(?:" + args[0].Inspect() + ")")
		if err != nil {
			return nil, err
		}
		text := args[1].Inspect()
		loc := re.FindStringIndex(text)
		if loc == nil {
			return vm.None, nil
		}
		return vm.StringValue(text[loc[0]:loc[1]]), nil
	}}
	m.Exports["findall"] = &vm.BuiltinFunction{Name: "findall", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("TypeError: findall() requires 2 arguments")
		}
		re, err := compile(args[0].Inspect())
		if err != nil {
			return nil, err
		}
		matches := re.FindAllString(args[1].Inspect(), -1)
		var list []vm.Value
		for _, mm := range matches {
			list = append(list, vm.StringValue(mm))
		}
		return vm.NewList(list...), nil
	}}
	m.Exports["sub"] = &vm.BuiltinFunction{Name: "sub", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) < 3 {
			return nil, fmt.Errorf("TypeError: sub() requires 3 arguments")
		}
		re, err := compile(args[0].Inspect())
		if err != nil {
			return nil, err
		}
		n := -1
		if len(args) > 3 {
			if v, ok := vm.ToInt(args[3]); ok {
				n = int(v)
			}
		}
		return vm.StringValue(re.ReplaceAllStringFunc(args[2].Inspect(), func(s string) string { _ = n; return args[1].Inspect() })), nil
	}}
	m.Exports["split"] = &vm.BuiltinFunction{Name: "split", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("TypeError: split() requires 2 arguments")
		}
		re, err := compile(args[0].Inspect())
		if err != nil {
			return nil, err
		}
		parts := re.Split(args[1].Inspect(), -1)
		var list []vm.Value
		for _, p := range parts {
			list = append(list, vm.StringValue(p))
		}
		return vm.NewList(list...), nil
	}}
	m.Exports["compile"] = &vm.BuiltinFunction{Name: "compile", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("TypeError: compile() requires a pattern")
		}
		_, err := compile(args[0].Inspect())
		if err != nil {
			return nil, err
		}
		return args[0], nil // return pattern string for now
	}}
	return m
}

func initHTTPModule() *vm.ModuleObject {
	m := vm.NewModule("http")
	makeResponse := func(resp *http.Response) (vm.Value, error) {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		d := vm.NewDict()
		d.Set(vm.StringValue("status_code"), vm.IntValue(int64(resp.StatusCode)))
		d.Set(vm.StringValue("text"), vm.StringValue(string(body)))
		return d, nil
	}
	m.Exports["get"] = &vm.BuiltinFunction{Name: "get", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("TypeError: get() requires a URL")
		}
		resp, err := http.Get(args[0].Inspect())
		if err != nil {
			return nil, fmt.Errorf("HTTPError: %v", err)
		}
		return makeResponse(resp)
	}}
	m.Exports["post"] = &vm.BuiltinFunction{Name: "post", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("TypeError: post() requires a URL")
		}
		body := ""
		if len(args) > 1 {
			body = args[1].Inspect()
		}
		resp, err := http.Post(args[0].Inspect(), "application/json", strings.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("HTTPError: %v", err)
		}
		return makeResponse(resp)
	}}
	return m
}

func initSyncModule() *vm.ModuleObject {
	m := vm.NewModule("sync")
	m.Exports["WaitGroup"] = &vm.BuiltinFunction{Name: "WaitGroup", Fn: func(args ...vm.Value) (vm.Value, error) {
		wg := &sync.WaitGroup{}
		d := vm.NewDict()
		d.Set(vm.StringValue("add"), &vm.BuiltinFunction{Name: "add", Fn: func(a ...vm.Value) (vm.Value, error) {
			delta := 1
			if len(a) > 0 {
				if v, ok := vm.ToInt(a[0]); ok {
					delta = int(v)
				}
			}
			wg.Add(delta)
			return vm.None, nil
		}})
		d.Set(vm.StringValue("done"), &vm.BuiltinFunction{Name: "done", Fn: func(a ...vm.Value) (vm.Value, error) { wg.Done(); return vm.None, nil }})
		d.Set(vm.StringValue("wait"), &vm.BuiltinFunction{Name: "wait", Fn: func(a ...vm.Value) (vm.Value, error) { wg.Wait(); return vm.None, nil }})
		return d, nil
	}}
	m.Exports["Mutex"] = &vm.BuiltinFunction{Name: "Mutex", Fn: func(args ...vm.Value) (vm.Value, error) {
		mu := &sync.Mutex{}
		d := vm.NewDict()
		d.Set(vm.StringValue("lock"), &vm.BuiltinFunction{Name: "lock", Fn: func(a ...vm.Value) (vm.Value, error) { mu.Lock(); return vm.None, nil }})
		d.Set(vm.StringValue("unlock"), &vm.BuiltinFunction{Name: "unlock", Fn: func(a ...vm.Value) (vm.Value, error) { mu.Unlock(); return vm.None, nil }})
		return d, nil
	}}
	return m
}

func initIOModule() *vm.ModuleObject {
	m := vm.NewModule("io")
	m.Exports["StringIO"] = &vm.BuiltinFunction{Name: "StringIO", Fn: func(args ...vm.Value) (vm.Value, error) {
		initial := ""
		if len(args) > 0 {
			initial = args[0].Inspect()
		}
		buf := &strings.Builder{}
		buf.WriteString(initial)
		d := vm.NewDict()
		d.Set(vm.StringValue("write"), &vm.BuiltinFunction{Name: "write", Fn: func(a ...vm.Value) (vm.Value, error) {
			if len(a) > 0 {
				n, _ := buf.WriteString(a[0].Inspect())
				return vm.IntValue(int64(n)), nil
			}
			return vm.IntValue(0), nil
		}})
		d.Set(vm.StringValue("getvalue"), &vm.BuiltinFunction{Name: "getvalue", Fn: func(a ...vm.Value) (vm.Value, error) { return vm.StringValue(buf.String()), nil }})
		d.Set(vm.StringValue("truncate"), &vm.BuiltinFunction{Name: "truncate", Fn: func(a ...vm.Value) (vm.Value, error) {
			*buf = strings.Builder{}
			return vm.None, nil
		}})
		return d, nil
	}}
	m.Exports["open"] = &vm.BuiltinFunction{Name: "open", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("TypeError: open() requires a path")
		}
		path := args[0].Inspect()
		mode := "r"
		if len(args) > 1 {
			mode = args[1].Inspect()
		}
		d := vm.NewDict()
		if strings.Contains(mode, "w") {
			f, err := os.Create(path)
			if err != nil {
				return nil, fmt.Errorf("IOError: %v", err)
			}
			d.Set(vm.StringValue("write"), &vm.BuiltinFunction{Name: "write", Fn: func(a ...vm.Value) (vm.Value, error) {
				if len(a) > 0 {
					n, err := f.WriteString(a[0].Inspect())
					return vm.IntValue(int64(n)), err
				}
				return vm.IntValue(0), nil
			}})
			d.Set(vm.StringValue("close"), &vm.BuiltinFunction{Name: "close", Fn: func(a ...vm.Value) (vm.Value, error) { return vm.None, f.Close() }})
		} else {
			content, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("IOError: %v", err)
			}
			text := string(content)
			d.Set(vm.StringValue("read"), &vm.BuiltinFunction{Name: "read", Fn: func(a ...vm.Value) (vm.Value, error) { return vm.StringValue(text), nil }})
			d.Set(vm.StringValue("readlines"), &vm.BuiltinFunction{Name: "readlines", Fn: func(a ...vm.Value) (vm.Value, error) {
				lines := strings.Split(text, "\n")
				var vals []vm.Value
				for _, l := range lines {
					vals = append(vals, vm.StringValue(l+"\n"))
				}
				return vm.NewList(vals...), nil
			}})
			d.Set(vm.StringValue("close"), &vm.BuiltinFunction{Name: "close", Fn: func(a ...vm.Value) (vm.Value, error) { return vm.None, nil }})
		}
		return d, nil
	}}
	return m
}

func initStringModule() *vm.ModuleObject {
	m := vm.NewModule("string")
	m.Exports["ascii_letters"] = vm.StringValue("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	m.Exports["ascii_lowercase"] = vm.StringValue("abcdefghijklmnopqrstuvwxyz")
	m.Exports["ascii_uppercase"] = vm.StringValue("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	m.Exports["digits"] = vm.StringValue("0123456789")
	m.Exports["hexdigits"] = vm.StringValue("0123456789abcdefABCDEF")
	m.Exports["punctuation"] = vm.StringValue("!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~")
	m.Exports["whitespace"] = vm.StringValue(" \t\n\r\x0b\x0c")
	m.Exports["printable"] = vm.StringValue(func() string {
		var sb strings.Builder
		for r := rune(32); r < 127; r++ {
			if unicode.IsPrint(r) {
				sb.WriteRune(r)
			}
		}
		return sb.String()
	}())
	m.Exports["capwords"] = &vm.BuiltinFunction{Name: "capwords", Fn: func(args ...vm.Value) (vm.Value, error) {
		if len(args) == 0 {
			return vm.StringValue(""), nil
		}
		words := strings.Fields(args[0].Inspect())
		for i, w := range words {
			if len(w) > 0 {
				words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
			}
		}
		return vm.StringValue(strings.Join(words, " ")), nil
	}}
	return m
}

func valueToNative(v vm.Value) interface{} {
	switch val := v.(type) {
	case vm.IntValue:
		return int64(val)
	case vm.FloatValue:
		return float64(val)
	case vm.BoolValue:
		return bool(val)
	case vm.StringValue:
		return string(val)
	case vm.NoneVal:
		return nil
	case *vm.ListObject:
		list := make([]interface{}, len(val.Elements))
		for i, el := range val.Elements {
			list[i] = valueToNative(el)
		}
		return list
	case *vm.TupleObject:
		list := make([]interface{}, len(val.Elements))
		for i, el := range val.Elements {
			list[i] = valueToNative(el)
		}
		return list
	case *vm.DictObject:
		mm := make(map[string]interface{}, len(val.Pairs))
		for _, p := range val.Pairs {
			mm[p.Key.Inspect()] = valueToNative(p.Value)
		}
		return mm
	default:
		return v.Inspect()
	}
}

func nativeToValue(n interface{}) vm.Value {
	if n == nil {
		return vm.None
	}
	switch v := n.(type) {
	case bool:
		return vm.BoolValue(v)
	case float64:
		if v == math.Floor(v) && v >= -1e15 && v <= 1e15 {
			return vm.IntValue(int64(v))
		}
		return vm.FloatValue(v)
	case int64:
		return vm.IntValue(v)
	case string:
		return vm.StringValue(v)
	case []interface{}:
		els := make([]vm.Value, len(v))
		for i, e := range v {
			els[i] = nativeToValue(e)
		}
		return vm.NewList(els...)
	case map[string]interface{}:
		d := vm.NewDict()
		for k, val := range v {
			d.Set(vm.StringValue(k), nativeToValue(val))
		}
		return d
	}
	return vm.StringValue(fmt.Sprintf("%v", n))
}
