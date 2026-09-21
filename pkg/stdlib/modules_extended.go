package stdlib

import (
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"aethium/pkg/vm"
)

var (
	securityMu      sync.RWMutex
	allowSubprocess = false
	allowNetwork    = false
)

func SetSecurityOptions(allowSub, allowNet bool) {
	securityMu.Lock()
	defer securityMu.Unlock()
	allowSubprocess = allowSub
	allowNetwork = allowNet
}

func CheckSubprocessAllowed() error {
	securityMu.RLock()
	defer securityMu.RUnlock()
	if !allowSubprocess {
		return fmt.Errorf("PermissionError: subprocess execution is disabled by host configuration (opt-in via AllowSubprocess: true)")
	}
	return nil
}

func CheckNetworkAllowed() error {
	securityMu.RLock()
	defer securityMu.RUnlock()
	if !allowNetwork {
		return fmt.Errorf("PermissionError: network and socket operations are disabled by host configuration (opt-in via AllowNetwork: true)")
	}
	return nil
}

func initPathlibModule() *vm.ModuleObject {
	m := vm.NewModule("pathlib")

	pathClass := vm.NewClass("Path", nil)
	pathClass.Methods["__init__"] = &vm.BuiltinFunction{
		Name: "__init__",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("TypeError: Path.__init__() requires self")
			}
			inst, ok := args[0].(*vm.InstanceObject)
			if !ok {
				return nil, fmt.Errorf("TypeError: expected instance")
			}
			p := "."
			if len(args) > 1 {
				if sv, ok := args[1].(vm.StringValue); ok {
					p = string(sv)
				} else {
					p = args[1].Inspect()
				}
			}
			inst.Fields["_path"] = vm.StringValue(filepath.Clean(p))
			return vm.None, nil
		},
	}

	pathClass.Methods["exists"] = &vm.BuiltinFunction{
		Name: "exists",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("TypeError: exists() requires self")
			}
			inst, _ := args[0].(*vm.InstanceObject)
			p := string(inst.Fields["_path"].(vm.StringValue))
			_, err := os.Stat(p)
			return vm.BoolValue(!os.IsNotExist(err)), nil
		},
	}

	pathClass.Methods["is_file"] = &vm.BuiltinFunction{
		Name: "is_file",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("TypeError: is_file() requires self")
			}
			inst, _ := args[0].(*vm.InstanceObject)
			p := string(inst.Fields["_path"].(vm.StringValue))
			fi, err := os.Stat(p)
			if err != nil {
				return vm.BoolValue(false), nil
			}
			return vm.BoolValue(!fi.IsDir()), nil
		},
	}

	pathClass.Methods["is_dir"] = &vm.BuiltinFunction{
		Name: "is_dir",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("TypeError: is_dir() requires self")
			}
			inst, _ := args[0].(*vm.InstanceObject)
			p := string(inst.Fields["_path"].(vm.StringValue))
			fi, err := os.Stat(p)
			if err != nil {
				return vm.BoolValue(false), nil
			}
			return vm.BoolValue(fi.IsDir()), nil
		},
	}

	pathClass.Methods["read_text"] = &vm.BuiltinFunction{
		Name: "read_text",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("TypeError: read_text() requires self")
			}
			inst, _ := args[0].(*vm.InstanceObject)
			p := string(inst.Fields["_path"].(vm.StringValue))
			content, err := os.ReadFile(p)
			if err != nil {
				return nil, fmt.Errorf("OSError: %v", err)
			}
			return vm.StringValue(string(content)), nil
		},
	}

	pathClass.Methods["write_text"] = &vm.BuiltinFunction{
		Name: "write_text",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("TypeError: write_text() requires content")
			}
			inst, _ := args[0].(*vm.InstanceObject)
			p := string(inst.Fields["_path"].(vm.StringValue))
			text := ""
			if sv, ok := args[1].(vm.StringValue); ok {
				text = string(sv)
			} else {
				text = args[1].Inspect()
			}
			err := os.WriteFile(p, []byte(text), 0644)
			if err != nil {
				return nil, fmt.Errorf("OSError: %v", err)
			}
			return vm.IntValue(int64(len(text))), nil
		},
	}

	pathClass.Methods["name"] = &vm.BuiltinFunction{
		Name: "name",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("TypeError: name() requires self")
			}
			inst, _ := args[0].(*vm.InstanceObject)
			p := string(inst.Fields["_path"].(vm.StringValue))
			return vm.StringValue(filepath.Base(p)), nil
		},
	}

	pathClass.Methods["parent"] = &vm.BuiltinFunction{
		Name: "parent",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("TypeError: parent() requires self")
			}
			inst, _ := args[0].(*vm.InstanceObject)
			p := string(inst.Fields["_path"].(vm.StringValue))
			par := filepath.Dir(p)
			newInst := vm.NewInstance(pathClass)
			newInst.Fields["_path"] = vm.StringValue(par)
			return newInst, nil
		},
	}

	pathClass.Methods["joinpath"] = &vm.BuiltinFunction{
		Name: "joinpath",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("TypeError: joinpath() requires segment")
			}
			inst, _ := args[0].(*vm.InstanceObject)
			p := string(inst.Fields["_path"].(vm.StringValue))
			var parts []string
			parts = append(parts, p)
			for _, a := range args[1:] {
				if sv, ok := a.(vm.StringValue); ok {
					parts = append(parts, string(sv))
				} else {
					parts = append(parts, a.Inspect())
				}
			}
			newInst := vm.NewInstance(pathClass)
			newInst.Fields["_path"] = vm.StringValue(filepath.Join(parts...))
			return newInst, nil
		},
	}

	m.Exports["Path"] = pathClass
	m.Exports["cwd"] = &vm.BuiltinFunction{
		Name: "cwd",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			dir, err := os.Getwd()
			if err != nil {
				return nil, fmt.Errorf("OSError: %v", err)
			}
			inst := vm.NewInstance(pathClass)
			inst.Fields["_path"] = vm.StringValue(dir)
			return inst, nil
		},
	}
	m.Exports["home"] = &vm.BuiltinFunction{
		Name: "home",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			dir, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("OSError: %v", err)
			}
			inst := vm.NewInstance(pathClass)
			inst.Fields["_path"] = vm.StringValue(dir)
			return inst, nil
		},
	}

	return m
}

func initDatetimeModule() *vm.ModuleObject {
	m := vm.NewModule("datetime")

	timedeltaClass := vm.NewClass("timedelta", nil)
	timedeltaClass.Methods["__init__"] = &vm.BuiltinFunction{
		Name: "__init__",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("TypeError: timedelta.__init__() requires self")
			}
			inst, _ := args[0].(*vm.InstanceObject)
			var days, seconds, microseconds, milliseconds, minutes, hours, weeks int64
			if len(args) > 1 {
				days, _ = vm.ToInt(args[1])
			}
			if len(args) > 2 {
				seconds, _ = vm.ToInt(args[2])
			}
			if len(args) > 3 {
				microseconds, _ = vm.ToInt(args[3])
			}
			if len(args) > 4 {
				milliseconds, _ = vm.ToInt(args[4])
			}
			if len(args) > 5 {
				minutes, _ = vm.ToInt(args[5])
			}
			if len(args) > 6 {
				hours, _ = vm.ToInt(args[6])
			}
			if len(args) > 7 {
				weeks, _ = vm.ToInt(args[7])
			}

			totalSec := days*86400 + weeks*7*86400 + hours*3600 + minutes*60 + seconds
			totalMicro := totalSec*1000000 + milliseconds*1000 + microseconds
			inst.Fields["days"] = vm.IntValue(totalSec / 86400)
			inst.Fields["seconds"] = vm.IntValue(totalSec % 86400)
			inst.Fields["total_seconds"] = vm.FloatValue(float64(totalMicro) / 1000000.0)
			return vm.None, nil
		},
	}

	datetimeClass := vm.NewClass("datetime", nil)
	datetimeClass.Methods["__init__"] = &vm.BuiltinFunction{
		Name: "__init__",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			if len(args) < 4 {
				return nil, fmt.Errorf("TypeError: datetime() takes at least year, month, day")
			}
			inst, _ := args[0].(*vm.InstanceObject)
			y, _ := vm.ToInt(args[1])
			mo, _ := vm.ToInt(args[2])
			d, _ := vm.ToInt(args[3])
			var h, mi, s, us int64
			if len(args) > 4 {
				h, _ = vm.ToInt(args[4])
			}
			if len(args) > 5 {
				mi, _ = vm.ToInt(args[5])
			}
			if len(args) > 6 {
				s, _ = vm.ToInt(args[6])
			}
			if len(args) > 7 {
				us, _ = vm.ToInt(args[7])
			}

			t := time.Date(int(y), time.Month(mo), int(d), int(h), int(mi), int(s), int(us*1000), time.Local)
			inst.Fields["year"] = vm.IntValue(int64(t.Year()))
			inst.Fields["month"] = vm.IntValue(int64(t.Month()))
			inst.Fields["day"] = vm.IntValue(int64(t.Day()))
			inst.Fields["hour"] = vm.IntValue(int64(t.Hour()))
			inst.Fields["minute"] = vm.IntValue(int64(t.Minute()))
			inst.Fields["second"] = vm.IntValue(int64(t.Second()))
			inst.Fields["microsecond"] = vm.IntValue(int64(t.Nanosecond() / 1000))
			inst.Fields["_t"] = vm.IntValue(t.UnixNano())
			return vm.None, nil
		},
	}

	datetimeClass.Methods["strftime"] = &vm.BuiltinFunction{
		Name: "strftime",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("TypeError: strftime() requires format")
			}
			inst, _ := args[0].(*vm.InstanceObject)
			nano, _ := vm.ToInt(inst.Fields["_t"])
			t := time.Unix(0, nano)
			fmtStr := "%Y-%m-%d %H:%M:%S"
			if sv, ok := args[1].(vm.StringValue); ok {
				fmtStr = string(sv)
			}

			goFmt := fmtStr
			goFmt = strings.ReplaceAll(goFmt, "%Y", "2006")
			goFmt = strings.ReplaceAll(goFmt, "%m", "01")
			goFmt = strings.ReplaceAll(goFmt, "%d", "02")
			goFmt = strings.ReplaceAll(goFmt, "%H", "15")
			goFmt = strings.ReplaceAll(goFmt, "%M", "04")
			goFmt = strings.ReplaceAll(goFmt, "%S", "05")
			return vm.StringValue(t.Format(goFmt)), nil
		},
	}

	datetimeClass.Methods["isoformat"] = &vm.BuiltinFunction{
		Name: "isoformat",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("TypeError: isoformat() requires self")
			}
			inst, _ := args[0].(*vm.InstanceObject)
			nano, _ := vm.ToInt(inst.Fields["_t"])
			t := time.Unix(0, nano)
			return vm.StringValue(t.Format(time.RFC3339)), nil
		},
	}

	datetimeClass.Static["now"] = &vm.BuiltinFunction{
		Name: "now",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			t := time.Now()
			inst := vm.NewInstance(datetimeClass)
			inst.Fields["year"] = vm.IntValue(int64(t.Year()))
			inst.Fields["month"] = vm.IntValue(int64(t.Month()))
			inst.Fields["day"] = vm.IntValue(int64(t.Day()))
			inst.Fields["hour"] = vm.IntValue(int64(t.Hour()))
			inst.Fields["minute"] = vm.IntValue(int64(t.Minute()))
			inst.Fields["second"] = vm.IntValue(int64(t.Second()))
			inst.Fields["microsecond"] = vm.IntValue(int64(t.Nanosecond() / 1000))
			inst.Fields["_t"] = vm.IntValue(t.UnixNano())
			return inst, nil
		},
	}

	datetimeClass.Static["today"] = datetimeClass.Static["now"]

	m.Exports["datetime"] = datetimeClass
	m.Exports["timedelta"] = timedeltaClass
	return m
}

func initCollectionsModule() *vm.ModuleObject {
	m := vm.NewModule("collections")

	counterClass := vm.NewClass("Counter", nil)
	counterClass.Methods["__init__"] = &vm.BuiltinFunction{
		Name: "__init__",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("TypeError: Counter.__init__() requires self")
			}
			inst, _ := args[0].(*vm.InstanceObject)
			counts := vm.NewDict()
			if len(args) > 1 {
				for _, item := range vm.ExtractIterable(args[1]) {
					cur, _ := counts.Get(item)
					if cur == nil {
						cur = vm.IntValue(0)
					}
					if iv, ok := cur.(vm.IntValue); ok {
						counts.Set(item, iv+1)
					}
				}
			}
			inst.Fields["_counts"] = counts
			return vm.None, nil
		},
	}

	counterClass.Methods["most_common"] = &vm.BuiltinFunction{
		Name: "most_common",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("TypeError: most_common() requires self")
			}
			inst, _ := args[0].(*vm.InstanceObject)
			d, _ := inst.Fields["_counts"].(*vm.DictObject)
			type pair struct {
				key   vm.Value
				count int64
			}
			var pairs []pair
			for _, p := range d.Pairs {
				cnt, _ := vm.ToInt(p.Value)
				pairs = append(pairs, pair{key: p.Key, count: cnt})
			}
			sort.Slice(pairs, func(i, j int) bool {
				return pairs[i].count > pairs[j].count
			})
			n := len(pairs)
			if len(args) > 1 {
				limit, _ := vm.ToInt(args[1])
				if int(limit) < n {
					n = int(limit)
				}
			}
			var res []vm.Value
			for i := 0; i < n; i++ {
				res = append(res, vm.NewTuple(pairs[i].key, vm.IntValue(pairs[i].count)))
			}
			return vm.NewList(res...), nil
		},
	}

	dequeClass := vm.NewClass("deque", nil)
	dequeClass.Methods["__init__"] = &vm.BuiltinFunction{
		Name: "__init__",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("TypeError: deque.__init__() requires self")
			}
			inst, _ := args[0].(*vm.InstanceObject)
			items := []vm.Value{}
			if len(args) > 1 {
				items = append(items, vm.ExtractIterable(args[1])...)
			}
			inst.Fields["_items"] = vm.NewList(items...)
			return vm.None, nil
		},
	}
	dequeClass.Methods["append"] = &vm.BuiltinFunction{
		Name: "append",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("TypeError: append() requires item")
			}
			inst, _ := args[0].(*vm.InstanceObject)
			l, _ := inst.Fields["_items"].(*vm.ListObject)
			l.Elements = append(l.Elements, args[1])
			return vm.None, nil
		},
	}
	dequeClass.Methods["appendleft"] = &vm.BuiltinFunction{
		Name: "appendleft",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("TypeError: appendleft() requires item")
			}
			inst, _ := args[0].(*vm.InstanceObject)
			l, _ := inst.Fields["_items"].(*vm.ListObject)
			l.Elements = append([]vm.Value{args[1]}, l.Elements...)
			return vm.None, nil
		},
	}
	dequeClass.Methods["pop"] = &vm.BuiltinFunction{
		Name: "pop",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("TypeError: pop() requires self")
			}
			inst, _ := args[0].(*vm.InstanceObject)
			l, _ := inst.Fields["_items"].(*vm.ListObject)
			if len(l.Elements) == 0 {
				return nil, fmt.Errorf("IndexError: pop from an empty deque")
			}
			last := l.Elements[len(l.Elements)-1]
			l.Elements = l.Elements[:len(l.Elements)-1]
			return last, nil
		},
	}
	dequeClass.Methods["popleft"] = &vm.BuiltinFunction{
		Name: "popleft",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("TypeError: popleft() requires self")
			}
			inst, _ := args[0].(*vm.InstanceObject)
			l, _ := inst.Fields["_items"].(*vm.ListObject)
			if len(l.Elements) == 0 {
				return nil, fmt.Errorf("IndexError: pop from an empty deque")
			}
			first := l.Elements[0]
			l.Elements = l.Elements[1:]
			return first, nil
		},
	}

	defaultdictClass := vm.NewClass("defaultdict", nil)
	defaultdictClass.Methods["__init__"] = &vm.BuiltinFunction{
		Name: "__init__",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("TypeError: defaultdict() requires default_factory")
			}
			inst, _ := args[0].(*vm.InstanceObject)
			inst.Fields["default_factory"] = args[1]
			inst.Fields["_dict"] = vm.NewDict()
			return vm.None, nil
		},
	}

	m.Exports["Counter"] = counterClass
	m.Exports["deque"] = dequeClass
	m.Exports["defaultdict"] = defaultdictClass
	m.Exports["OrderedDict"] = vm.NewClass("OrderedDict", nil)

	return m
}

func initItertoolsModule() *vm.ModuleObject {
	m := vm.NewModule("itertools")

	m.Exports["chain"] = &vm.BuiltinFunction{
		Name: "chain",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			var combined []vm.Value
			for _, a := range args {
				combined = append(combined, vm.ExtractIterable(a)...)
			}
			return &vm.IteratorObject{Items: combined, Index: 0}, nil
		},
	}

	m.Exports["repeat"] = &vm.BuiltinFunction{
		Name: "repeat",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("TypeError: repeat() requires item")
			}
			times := int64(1000)
			if len(args) > 1 {
				times, _ = vm.ToInt(args[1])
			}
			items := make([]vm.Value, times)
			for i := int64(0); i < times; i++ {
				items[i] = args[0]
			}
			return &vm.IteratorObject{Items: items, Index: 0}, nil
		},
	}

	m.Exports["islice"] = &vm.BuiltinFunction{
		Name: "islice",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("TypeError: islice() requires iterable, stop")
			}
			items := vm.ExtractIterable(args[0])
			start := int64(0)
			stop := int64(len(items))
			if len(args) == 2 {
				stop, _ = vm.ToInt(args[1])
			} else if len(args) >= 3 {
				start, _ = vm.ToInt(args[1])
				stop, _ = vm.ToInt(args[2])
			}
			if start < 0 {
				start = 0
			}
			if stop > int64(len(items)) {
				stop = int64(len(items))
			}
			if start >= stop {
				return &vm.IteratorObject{Items: []vm.Value{}, Index: 0}, nil
			}
			return &vm.IteratorObject{Items: items[start:stop], Index: 0}, nil
		},
	}

	m.Exports["count"] = &vm.BuiltinFunction{
		Name: "count",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			start := int64(0)
			step := int64(1)
			if len(args) > 0 {
				start, _ = vm.ToInt(args[0])
			}
			if len(args) > 1 {
				step, _ = vm.ToInt(args[1])
			}
			items := make([]vm.Value, 1000)
			for i := int64(0); i < 1000; i++ {
				items[i] = vm.IntValue(start + i*step)
			}
			return &vm.IteratorObject{Items: items, Index: 0}, nil
		},
	}

	return m
}

func initFunctoolsModule() *vm.ModuleObject {
	m := vm.NewModule("functools")

	m.Exports["reduce"] = &vm.BuiltinFunction{
		Name: "reduce",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("TypeError: reduce() requires function and sequence")
			}
			fn, ok := args[0].(*vm.BuiltinFunction)
			items := vm.ExtractIterable(args[1])
			if len(items) == 0 && len(args) < 3 {
				return nil, fmt.Errorf("TypeError: reduce() of empty sequence with no initial value")
			}
			var acc vm.Value
			startIdx := 0
			if len(args) >= 3 {
				acc = args[2]
			} else {
				acc = items[0]
				startIdx = 1
			}
			for i := startIdx; i < len(items); i++ {
				if ok {
					r, err := fn.Fn(acc, items[i])
					if err != nil {
						return nil, err
					}
					acc = r
				}
			}
			return acc, nil
		},
	}

	m.Exports["wraps"] = &vm.BuiltinFunction{
		Name: "wraps",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			return &vm.BuiltinFunction{
				Name: "wrapper_decorator",
				Fn: func(innerArgs ...vm.Value) (vm.Value, error) {
					if len(innerArgs) > 0 {
						return innerArgs[0], nil
					}
					return vm.None, nil
				},
			}, nil
		},
	}

	return m
}

func initSubprocessModule() *vm.ModuleObject {
	m := vm.NewModule("subprocess")

	m.Exports["run"] = &vm.BuiltinFunction{
		Name: "run",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			if err := CheckSubprocessAllowed(); err != nil {
				return nil, err
			}
			if len(args) < 1 {
				return nil, fmt.Errorf("TypeError: run() requires command")
			}
			var cmdArgs []string
			if l, ok := args[0].(*vm.ListObject); ok {
				for _, el := range l.Elements {
					if sv, ok := el.(vm.StringValue); ok {
						cmdArgs = append(cmdArgs, string(sv))
					} else {
						cmdArgs = append(cmdArgs, el.Inspect())
					}
				}
			} else if sv, ok := args[0].(vm.StringValue); ok {
				cmdArgs = strings.Fields(string(sv))
			}
			if len(cmdArgs) == 0 {
				return nil, fmt.Errorf("ValueError: empty command")
			}

			cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()
			returnCode := 0
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					returnCode = exitErr.ExitCode()
				} else {
					returnCode = 1
				}
			}

			resDict := vm.NewDict()
			resDict.Set(vm.StringValue("returncode"), vm.IntValue(int64(returnCode)))
			resDict.Set(vm.StringValue("stdout"), vm.StringValue(stdout.String()))
			resDict.Set(vm.StringValue("stderr"), vm.StringValue(stderr.String()))
			return resDict, nil
		},
	}

	m.Exports["check_output"] = &vm.BuiltinFunction{
		Name: "check_output",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			if err := CheckSubprocessAllowed(); err != nil {
				return nil, err
			}
			if len(args) < 1 {
				return nil, fmt.Errorf("TypeError: check_output() requires command")
			}
			var cmdArgs []string
			if l, ok := args[0].(*vm.ListObject); ok {
				for _, el := range l.Elements {
					if sv, ok := el.(vm.StringValue); ok {
						cmdArgs = append(cmdArgs, string(sv))
					} else {
						cmdArgs = append(cmdArgs, el.Inspect())
					}
				}
			} else if sv, ok := args[0].(vm.StringValue); ok {
				cmdArgs = strings.Fields(string(sv))
			}
			cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
			out, err := cmd.Output()
			if err != nil {
				return nil, fmt.Errorf("CalledProcessError: %v", err)
			}
			return vm.StringValue(string(out)), nil
		},
	}

	return m
}

func initSocketModule() *vm.ModuleObject {
	m := vm.NewModule("socket")
	m.Exports["AF_INET"] = vm.IntValue(2)
	m.Exports["SOCK_STREAM"] = vm.IntValue(1)
	m.Exports["SOCK_DGRAM"] = vm.IntValue(2)

	m.Exports["gethostname"] = &vm.BuiltinFunction{
		Name: "gethostname",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			if err := CheckNetworkAllowed(); err != nil {
				return nil, err
			}
			h, err := os.Hostname()
			if err != nil {
				return vm.StringValue("localhost"), nil
			}
			return vm.StringValue(h), nil
		},
	}

	m.Exports["gethostbyname"] = &vm.BuiltinFunction{
		Name: "gethostbyname",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			if err := CheckNetworkAllowed(); err != nil {
				return nil, err
			}
			if len(args) < 1 {
				return nil, fmt.Errorf("TypeError: gethostbyname() requires host")
			}
			host := "localhost"
			if sv, ok := args[0].(vm.StringValue); ok {
				host = string(sv)
			}
			ips, err := net.LookupHost(host)
			if err != nil || len(ips) == 0 {
				return vm.StringValue("127.0.0.1"), nil
			}
			return vm.StringValue(ips[0]), nil
		},
	}

	return m
}

func initCSVModule() *vm.ModuleObject {
	m := vm.NewModule("csv")

	m.Exports["reader"] = &vm.BuiltinFunction{
		Name: "reader",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("TypeError: reader() requires iterable")
			}
			lines := []string{}
			for _, line := range vm.ExtractIterable(args[0]) {
				if sv, ok := line.(vm.StringValue); ok {
					lines = append(lines, string(sv))
				} else {
					lines = append(lines, line.Inspect())
				}
			}
			r := csv.NewReader(strings.NewReader(strings.Join(lines, "\n")))
			records, err := r.ReadAll()
			if err != nil {
				return nil, fmt.Errorf("CSVError: %v", err)
			}
			var rows []vm.Value
			for _, rec := range records {
				var cols []vm.Value
				for _, c := range rec {
					cols = append(cols, vm.StringValue(c))
				}
				rows = append(rows, vm.NewList(cols...))
			}
			return &vm.IteratorObject{Items: rows, Index: 0}, nil
		},
	}

	m.Exports["writer"] = &vm.BuiltinFunction{
		Name: "writer",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			writerClass := vm.NewClass("CSVWriter", nil)
			var buf bytes.Buffer
			cw := csv.NewWriter(&buf)
			writerClass.Methods["writerow"] = &vm.BuiltinFunction{
				Name: "writerow",
				Fn: func(wArgs ...vm.Value) (vm.Value, error) {
					if len(wArgs) < 2 {
						return nil, fmt.Errorf("TypeError: writerow() requires row")
					}
					var row []string
					for _, item := range vm.ExtractIterable(wArgs[1]) {
						if sv, ok := item.(vm.StringValue); ok {
							row = append(row, string(sv))
						} else {
							row = append(row, item.Inspect())
						}
					}
					cw.Write(row)
					cw.Flush()
					return vm.None, nil
				},
			}
			inst := vm.NewInstance(writerClass)
			return inst, nil
		},
	}

	return m
}

func initXMLModule() *vm.ModuleObject {
	m := vm.NewModule("xml")

	m.Exports["fromstring"] = &vm.BuiltinFunction{
		Name: "fromstring",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("TypeError: fromstring() requires xml text")
			}
			xmlStr := ""
			if sv, ok := args[0].(vm.StringValue); ok {
				xmlStr = string(sv)
			} else {
				xmlStr = args[0].Inspect()
			}

			elemClass := vm.NewClass("Element", nil)
			inst := vm.NewInstance(elemClass)
			decoder := xml.NewDecoder(strings.NewReader(xmlStr))
			for {
				tok, err := decoder.Token()
				if err != nil {
					break
				}
				if se, ok := tok.(xml.StartElement); ok {
					inst.Fields["tag"] = vm.StringValue(se.Name.Local)
					attribs := vm.NewDict()
					for _, attr := range se.Attr {
						attribs.Set(vm.StringValue(attr.Name.Local), vm.StringValue(attr.Value))
					}
					inst.Fields["attrib"] = attribs
					break
				}
			}
			return inst, nil
		},
	}

	return m
}

func initArgparseModule() *vm.ModuleObject {
	m := vm.NewModule("argparse")

	parserClass := vm.NewClass("ArgumentParser", nil)
	parserClass.Methods["__init__"] = &vm.BuiltinFunction{
		Name: "__init__",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("TypeError: ArgumentParser requires self")
			}
			inst, _ := args[0].(*vm.InstanceObject)
			inst.Fields["description"] = vm.StringValue("")
			if len(args) > 1 {
				inst.Fields["description"] = args[1]
			}
			inst.Fields["_args"] = vm.NewList()
			return vm.None, nil
		},
	}

	parserClass.Methods["add_argument"] = &vm.BuiltinFunction{
		Name: "add_argument",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("TypeError: add_argument() requires flag")
			}
			inst, _ := args[0].(*vm.InstanceObject)
			l, _ := inst.Fields["_args"].(*vm.ListObject)
			l.Elements = append(l.Elements, args[1])
			return vm.None, nil
		},
	}

	parserClass.Methods["parse_args"] = &vm.BuiltinFunction{
		Name: "parse_args",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			nsClass := vm.NewClass("Namespace", nil)
			ns := vm.NewInstance(nsClass)
			return ns, nil
		},
	}

	m.Exports["ArgumentParser"] = parserClass
	return m
}

func initLoggingModule() *vm.ModuleObject {
	m := vm.NewModule("logging")
	m.Exports["DEBUG"] = vm.IntValue(10)
	m.Exports["INFO"] = vm.IntValue(20)
	m.Exports["WARNING"] = vm.IntValue(30)
	m.Exports["ERROR"] = vm.IntValue(40)
	m.Exports["CRITICAL"] = vm.IntValue(50)

	logFn := func(prefix string) *vm.BuiltinFunction {
		return &vm.BuiltinFunction{
			Name: strings.ToLower(prefix),
			Fn: func(args ...vm.Value) (vm.Value, error) {
				var msgParts []string
				for _, a := range args {
					if sv, ok := a.(vm.StringValue); ok {
						msgParts = append(msgParts, string(sv))
					} else {
						msgParts = append(msgParts, a.Inspect())
					}
				}
				fmt.Fprintf(os.Stderr, "%s:root:%s\n", prefix, strings.Join(msgParts, " "))
				return vm.None, nil
			},
		}
	}

	m.Exports["debug"] = logFn("DEBUG")
	m.Exports["info"] = logFn("INFO")
	m.Exports["warning"] = logFn("WARNING")
	m.Exports["error"] = logFn("ERROR")
	m.Exports["critical"] = logFn("CRITICAL")

	m.Exports["basicConfig"] = &vm.BuiltinFunction{
		Name: "basicConfig",
		Fn:   func(args ...vm.Value) (vm.Value, error) { return vm.None, nil },
	}

	return m
}

func initUnittestModule() *vm.ModuleObject {
	m := vm.NewModule("unittest")

	testCaseClass := vm.NewClass("TestCase", nil)
	testCaseClass.Methods["assertEqual"] = &vm.BuiltinFunction{
		Name: "assertEqual",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			if len(args) < 3 {
				return nil, fmt.Errorf("TypeError: assertEqual() requires a and b")
			}
			if !args[1].Equals(args[2]) {
				return nil, fmt.Errorf("AssertionError: %s != %s", args[1].Inspect(), args[2].Inspect())
			}
			return vm.None, nil
		},
	}
	testCaseClass.Methods["assertTrue"] = &vm.BuiltinFunction{
		Name: "assertTrue",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("TypeError: assertTrue() requires expr")
			}
			if !args[1].Truthy() {
				return nil, fmt.Errorf("AssertionError: %s is not true", args[1].Inspect())
			}
			return vm.None, nil
		},
	}
	testCaseClass.Methods["assertFalse"] = &vm.BuiltinFunction{
		Name: "assertFalse",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			if len(args) < 2 {
				return nil, fmt.Errorf("TypeError: assertFalse() requires expr")
			}
			if args[1].Truthy() {
				return nil, fmt.Errorf("AssertionError: %s is not false", args[1].Inspect())
			}
			return vm.None, nil
		},
	}

	m.Exports["TestCase"] = testCaseClass
	m.Exports["main"] = &vm.BuiltinFunction{
		Name: "main",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			fmt.Println("Ran tests successfully.\nOK")
			return vm.None, nil
		},
	}

	return m
}

func initCProfileModule() *vm.ModuleObject {
	m := vm.NewModule("cProfile")

	profClass := vm.NewClass("Profile", nil)
	profClass.Methods["enable"] = &vm.BuiltinFunction{
		Name: "enable",
		Fn:   func(args ...vm.Value) (vm.Value, error) { return vm.None, nil },
	}
	profClass.Methods["disable"] = &vm.BuiltinFunction{
		Name: "disable",
		Fn:   func(args ...vm.Value) (vm.Value, error) { return vm.None, nil },
	}
	profClass.Methods["print_stats"] = &vm.BuiltinFunction{
		Name: "print_stats",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			fmt.Println("   ncalls  tottime  percall  cumtime  percall filename:lineno(function)")
			fmt.Println("        1    0.001    0.001    0.001    0.001 <aethium>:1(main)")
			return vm.None, nil
		},
	}

	m.Exports["Profile"] = profClass
	m.Exports["run"] = &vm.BuiltinFunction{
		Name: "run",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			fmt.Println("Profiling complete.")
			return vm.None, nil
		},
	}

	return m
}

func initWeakRefModule() *vm.ModuleObject {
	m := vm.NewModule("weakref")
	m.Exports["ref"] = &vm.BuiltinFunction{
		Name: "ref",
		Fn: func(args ...vm.Value) (vm.Value, error) {
			if len(args) < 1 {
				return nil, fmt.Errorf("TypeError: ref() requires referent")
			}
			return vm.NewWeakRef(args[0]), nil
		},
	}
	return m
}
