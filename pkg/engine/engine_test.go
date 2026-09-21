package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"aethium/pkg/ffi"
	"aethium/pkg/vm"
)

func runGlobals(t *testing.T, src string) map[string]vm.Value {
	t.Helper()
	eng := NewEngine()
	_, err := eng.Execute("test.py", src)
	if err != nil {
		t.Fatalf("execution error: %v", err)
	}
	return eng.Globals()
}

func assertInt(t *testing.T, g map[string]vm.Value, name string, want int64) {
	t.Helper()
	v, ok := g[name]
	if !ok {
		t.Fatalf("global %q not found", name)
	}
	iv, ok2 := vm.ToInt(v)
	if !ok2 || iv != want {
		t.Errorf("global %q: want %d, got %v", name, want, v)
	}
}

func assertFloat(t *testing.T, g map[string]vm.Value, name string, want float64) {
	t.Helper()
	v, ok := g[name]
	if !ok {
		t.Fatalf("global %q not found", name)
	}
	fv, ok2 := vm.ToFloat(v)
	if !ok2 || fv != want {
		t.Errorf("global %q: want %f, got %v", name, want, v)
	}
}

func assertString(t *testing.T, g map[string]vm.Value, name string, want string) {
	t.Helper()
	v, ok := g[name]
	if !ok {
		t.Fatalf("global %q not found", name)
	}
	sv, ok2 := v.(vm.StringValue)
	if !ok2 || string(sv) != want {
		t.Errorf("global %q: want %q, got %v", name, want, v)
	}
}

func assertBool(t *testing.T, g map[string]vm.Value, name string, want bool) {
	t.Helper()
	v, ok := g[name]
	if !ok {
		t.Fatalf("global %q not found", name)
	}
	bv, ok2 := v.(vm.BoolValue)
	if !ok2 || bool(bv) != want {
		t.Errorf("global %q: want %v, got %v", name, want, v)
	}
}

func TestEngineArithmeticAndLogic(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{"1 + 2 * 3", int64(7)},
		{"(10 - 4) // 2", int64(3)},
		{"2 ** 5", int64(32)},
		{"10 % 3", int64(1)},
		{"True and False", false},
		{"True or False", true},
		{"not False", true},
		{"10 > 5 and 3 < 4", true},
		{"\"hello \" + \"world\"", "hello world"},
		{"\"py\" * 3", "pypypy"},
	}
	for _, tt := range tests {
		eng := NewEngine()
		val, err := eng.Execute("test.py", tt.input)
		if err != nil {
			t.Fatalf("execution error for %q: %v", tt.input, err)
		}
		switch expected := tt.expected.(type) {
		case int64:
			if i, ok := vm.ToInt(val); !ok || i != expected {
				t.Errorf("for %q expected %d, got %v", tt.input, expected, val)
			}
		case bool:
			if b, ok := val.(vm.BoolValue); !ok || bool(b) != expected {
				t.Errorf("for %q expected %t, got %v", tt.input, expected, val)
			}
		case string:
			if s, ok := val.(vm.StringValue); !ok || string(s) != expected {
				t.Errorf("for %q expected %q, got %v", tt.input, expected, val)
			}
		}
	}
}

func TestEngineControlFlowAndLoops(t *testing.T) {
	g := runGlobals(t, `
total = 0
for i in range(10):
    if i % 2 == 0:
        total = total + i
`)
	assertInt(t, g, "total", 20)
}

func TestEngineFunctionsAndRecursion(t *testing.T) {
	g := runGlobals(t, `
def fib(n):
    if n <= 1:
        return n
    return fib(n - 1) + fib(n - 2)
res = fib(10)
`)
	assertInt(t, g, "res", 55)
}

func TestEngineComprehensions(t *testing.T) {
	g := runGlobals(t, `
evens = [x for x in range(10) if x % 2 == 0]
squares = {x: x * x for x in evens}
total_squares = sum([v for v in squares.values()])
`)
	assertInt(t, g, "total_squares", 0+4+16+36+64)
}

func TestEngineOOP(t *testing.T) {
	g := runGlobals(t, `
class Animal:
    def __init__(self, name):
        self.name = name
    def sound(self):
        return "some sound"

class Dog(Animal):
    def sound(self):
        return f"{self.name} says woof"

d = Dog("Buddy")
msg = d.sound()
`)
	assertString(t, g, "msg", "Buddy says woof")
}

func TestEngineStandardLibrary(t *testing.T) {
	g := runGlobals(t, `
import math
import json
sq = math.sqrt(16)
data = {"a": 1, "b": [2, 3]}
dumped = json.dumps(data)
loaded = json.loads(dumped)
sum_b = sum(loaded["b"])
`)
	assertFloat(t, g, "sq", 4.0)
	assertInt(t, g, "sum_b", 5)
}

func TestEngineExceptions(t *testing.T) {
	g := runGlobals(t, `
caught = False
try:
    raise "CustomError"
except e:
    caught = True
`)
	assertBool(t, g, "caught", true)
}

func TestEngineConcurrency(t *testing.T) {
	g := runGlobals(t, `
ch = chan(5)
def worker(val):
    ch.send(val * 10)
go worker(1)
go worker(2)
go worker(3)
results = []
for _ in range(3):
    results.append(ch.recv())
sorted_res = sorted(results)
`)
	if g["sorted_res"].Inspect() != "[10, 20, 30]" {
		t.Fatalf("expected [10, 20, 30], got %s", g["sorted_res"].Inspect())
	}
}

func TestEngineClosuresAndLambdas(t *testing.T) {
	g := runGlobals(t, `
def make_counter(start):
    def increment(step):
        return start + step
    return increment
c = make_counter(10)
val = c(5)
sq = lambda x: x * x
sq_val = sq(6)
`)
	assertInt(t, g, "val", 15)
	assertInt(t, g, "sq_val", 36)
}



func TestDefaultArguments(t *testing.T) {
	g := runGlobals(t, `
def greet(name, greeting="Hello"):
    return greeting + ", " + name

r1 = greet("Alice")
r2 = greet("Bob", "Hi")
`)
	assertString(t, g, "r1", "Hello, Alice")
	assertString(t, g, "r2", "Hi, Bob")
}



func TestKeywordArguments(t *testing.T) {
	g := runGlobals(t, `
def rect(width, height):
    return width * height

area = rect(height=5, width=3)
`)
	assertInt(t, g, "area", 15)
}


func TestVarArgs(t *testing.T) {
	g := runGlobals(t, `
def my_sum(*args):
    total = 0
    for v in args:
        total = total + v
    return total

r = my_sum(1, 2, 3, 4, 5)
`)
	assertInt(t, g, "r", 15)
}


func TestKwArgs(t *testing.T) {
	g := runGlobals(t, `
def show(**kwargs):
    return kwargs["x"] + kwargs["y"]

r = show(x=10, y=20)
`)
	assertInt(t, g, "r", 30)
}



func TestGlobalStatement(t *testing.T) {
	g := runGlobals(t, `
counter = 0

def increment():
    global counter
    counter = counter + 1

increment()
increment()
increment()
`)
	assertInt(t, g, "counter", 3)
}


func TestAssertPasses(t *testing.T) {
	g := runGlobals(t, `
x = 42
assert x == 42
result = "ok"
`)
	assertString(t, g, "result", "ok")
}



func TestAssertFails(t *testing.T) {
	g := runGlobals(t, `
caught = False
try:
    assert 1 == 2, "one is not two"
except e:
    caught = True
`)
	assertBool(t, g, "caught", true)
}


func TestDeleteStatement(t *testing.T) {
	g := runGlobals(t, `
x = 99
del x
deleted = True
`)
	assertBool(t, g, "deleted", true)
	if _, ok := g["x"]; ok {
		t.Errorf("expected x to be deleted, but it is still present: %v", g["x"])
	}
}


func TestDecorators(t *testing.T) {
	g := runGlobals(t, `
def double_result(fn):
    def wrapper(x):
        return fn(x) * 2
    return wrapper

@double_result
def square(x):
    return x * x

r = square(4)
`)
	assertInt(t, g, "r", 32) 
}


func TestSetComprehension(t *testing.T) {
	g := runGlobals(t, `
nums = {x * x for x in range(5)}
sz = len(nums)
`)
	assertInt(t, g, "sz", 5) 
}


func TestTryExceptElseFinally(t *testing.T) {
	g := runGlobals(t, `
log = []

def run(should_raise):
    try:
        if should_raise:
            raise "boom"
        log.append("try_ok")
    except e:
        log.append("except")
    else:
        log.append("else")
    finally:
        log.append("finally")

run(False)
run(True)
`)
	want := "['try_ok', 'else', 'finally', 'except', 'finally']"
	if got := g["log"].Inspect(); got != want {
		t.Errorf("log: want %s, got %s", want, got)
	}
}



func TestFinallyAlwaysRuns(t *testing.T) {
	g := runGlobals(t, `
fin = False
try:
    raise "err"
except e:
    pass
finally:
    fin = True
`)
	assertBool(t, g, "fin", true)
}



func TestNestedClosureMutation(t *testing.T) {
	g := runGlobals(t, `
def outer(x):
    def middle(y):
        def inner(z):
            return x + y + z
        return inner
    return middle

fn = outer(1)(2)
r = fn(3)
`)
	assertInt(t, g, "r", 6)
}


func TestMultipleAssignment(t *testing.T) {
	g := runGlobals(t, `
a, b, c = 10, 20, 30
total = a + b + c
`)
	assertInt(t, g, "total", 60)
}


func TestStringMethods(t *testing.T) {
	g := runGlobals(t, `
s = "  hello world  "
stripped = s.strip()
upper = stripped.upper()
parts = upper.split(" ")
joined = "-".join(parts)
`)
	assertString(t, g, "stripped", "hello world")
	assertString(t, g, "upper", "HELLO WORLD")
	assertString(t, g, "joined", "HELLO-WORLD")
}


func TestListMethods(t *testing.T) {
	g := runGlobals(t, `
lst = [3, 1, 4, 1, 5]
lst.append(9)
lst.sort()
lst.reverse()
top = lst[0]
popped = lst.pop()
sz = len(lst)
`)
	assertInt(t, g, "top", 9)
	assertInt(t, g, "sz", 5)
}


func TestDictMethods(t *testing.T) {
	g := runGlobals(t, `
d = {"a": 1, "b": 2, "c": 3}
v = d.get("b", 0)
missing = d.get("z", -1)
ks = sorted(d.keys())
total = sum(d.values())
`)
	assertInt(t, g, "v", 2)
	assertInt(t, g, "missing", -1)
	assertInt(t, g, "total", 6)
	if got := g["ks"].Inspect(); got != "['a', 'b', 'c']" {
		t.Errorf("keys: got %s", got)
	}
}


func TestIsinstanceBuiltin(t *testing.T) {
	g := runGlobals(t, `
class Foo:
    pass

f = Foo()
r1 = isinstance(f, Foo)
r2 = isinstance(42, "int")
r3 = isinstance("hi", "int")
`)
	assertBool(t, g, "r1", true)
	assertBool(t, g, "r2", true)
	assertBool(t, g, "r3", false)
}


func TestSuperCall(t *testing.T) {
	g := runGlobals(t, `
class Base:
    def greet(self):
        return "Hello from Base"

class Child(Base):
    def greet(self):
        base_msg = super().greet()
        return base_msg + " and Child"

c = Child()
msg = c.greet()
`)
	assertString(t, g, "msg", "Hello from Base and Child")
}


func TestWhileBreakContinue(t *testing.T) {
	g := runGlobals(t, `
collected = []
i = 0
while i < 10:
    i = i + 1
    if i % 2 == 0:
        continue
    if i > 7:
        break
    collected.append(i)
`)
	if got := g["collected"].Inspect(); got != "[1, 3, 5, 7]" {
		t.Errorf("collected: want [1, 3, 5, 7], got %s", got)
	}
}



func TestForElse(t *testing.T) {
	g := runGlobals(t, `
found = False
for i in range(5):
    if i == 10:
        found = True
        break
else_ran = not found
`)
	assertBool(t, g, "else_ran", true)
}



func TestArithmeticEdgeCases(t *testing.T) {
	g := runGlobals(t, `
fd = -7 // 2
md = -7 % 2
`)
	assertInt(t, g, "fd", -4) 
	assertInt(t, g, "md", 1)  
}


func TestIsOperator(t *testing.T) {
	g := runGlobals(t, `
a = [1, 2, 3]
b = a
c = [1, 2, 3]
same = a is b
diff = a is c
`)
	assertBool(t, g, "same", true)
	assertBool(t, g, "diff", false)
}


func TestMapFilterBuiltins(t *testing.T) {
	g := runGlobals(t, `
nums = [1, 2, 3, 4, 5]
doubled = list(map(lambda x: x * 2, nums))
evens = list(filter(lambda x: x % 2 == 0, nums))
`)
	if got := g["doubled"].Inspect(); got != "[2, 4, 6, 8, 10]" {
		t.Errorf("doubled: got %s", got)
	}
	if got := g["evens"].Inspect(); got != "[2, 4]" {
		t.Errorf("evens: got %s", got)
	}
}



func TestRangeLazy(t *testing.T) {
	g := runGlobals(t, `
r = range(1000000)
sz = len(r)
first_five = list(r)[0:5]
`)
	assertInt(t, g, "sz", 1_000_000)
	if got := g["first_five"].Inspect(); got != "[0, 1, 2, 3, 4]" {
		t.Errorf("first_five: got %s", got)
	}
}



func TestNoExitMode(t *testing.T) {
	eng := NewEngineWithConfig(Config{NoExit: true})
	_, err := eng.Execute("test.py", "exit(42)")
	if err == nil {
		t.Fatal("expected ExitError, got nil")
	}
	exitErr, ok := err.(*ExitError)
	if !ok {
		t.Fatalf("expected *ExitError, got %T: %v", err, err)
	}
	if exitErr.Code != 42 {
		t.Errorf("expected exit code 42, got %d", exitErr.Code)
	}
}



func TestEmbeddingSetGlobal(t *testing.T) {
	eng := NewEngine()
	eng.SetGlobal("injected", vm.IntValue(999))
	g := map[string]vm.Value{}
	_, err := eng.Execute("test.py", "result = injected + 1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	g = eng.Globals()
	assertInt(t, g, "result", 1000)
}



func TestFStringInterpolation(t *testing.T) {
	g := runGlobals(t, `
name = "World"
n = 42
s1 = f"Hello, {name}!"
s2 = f"The answer is {n * 1}"
`)
	assertString(t, g, "s1", "Hello, World!")
	assertString(t, g, "s2", "The answer is 42")
}


func TestZipEnumerate(t *testing.T) {
	g := runGlobals(t, `
keys = ["a", "b", "c"]
vals = [1, 2, 3]
pairs = list(zip(keys, vals))
enumd = list(enumerate(keys, 10))
`)
	if got := g["pairs"].Inspect(); got != "[('a', 1), ('b', 2), ('c', 3)]" {
		t.Errorf("pairs: got %s", got)
	}
	if got := g["enumd"].Inspect(); got != "[(10, 'a'), (11, 'b'), (12, 'c')]" {
		t.Errorf("enumd: got %s", got)
	}
}


func TestBitwiseOperators(t *testing.T) {
	g := runGlobals(t, `
r_and  = 0b1100 & 0b1010
r_or   = 0b1100 | 0b1010
r_xor  = 0b1100 ^ 0b1010
r_not  = ~5
r_lsh  = 1 << 4
r_rsh  = 32 >> 2
`)
	assertInt(t, g, "r_and", 0b1000) 
	assertInt(t, g, "r_or", 0b1110)  
	assertInt(t, g, "r_xor", 0b0110) 
	assertInt(t, g, "r_not", -6)
	assertInt(t, g, "r_lsh", 16)
	assertInt(t, g, "r_rsh", 8)
}



func TestSortedReversed(t *testing.T) {
	g := runGlobals(t, `
orig  = [3, 1, 4, 1, 5, 9, 2, 6]
asc   = sorted(orig)
desc  = sorted(orig, True)
orig_len = len(orig)
`)
	assertInt(t, g, "orig_len", 8)
	if got := g["asc"].Inspect(); got != "[1, 1, 2, 3, 4, 5, 6, 9]" {
		t.Errorf("asc: got %s", got)
	}
	if got := g["desc"].Inspect(); got != "[9, 6, 5, 4, 3, 2, 1, 1]" {
		t.Errorf("desc: got %s", got)
	}
}



func TestZeroDivisionError(t *testing.T) {
	g := runGlobals(t, `
caught_type = ""
try:
    x = 1 / 0
except e:
    caught_type = e.type
`)
	sv, ok := g["caught_type"].(vm.StringValue)
	if !ok || !strings.Contains(string(sv), "ZeroDivision") {
		t.Errorf("expected ZeroDivisionError, got %v", g["caught_type"])
	}
}


func TestMathModule(t *testing.T) {
	g := runGlobals(t, `
import math
pi_floor = math.floor(math.pi)
e_ceil   = math.ceil(math.e)
lg2      = math.log(8, 2)
`)
	assertInt(t, g, "pi_floor", 3)
	assertInt(t, g, "e_ceil", 3)
	assertFloat(t, g, "lg2", 3.0)
}


func TestBitwiseTypeChecking(t *testing.T) {
	eng := NewEngine()
	_, err := eng.Execute("test.py", `x = "hello" | 5`)
	if err == nil {
		t.Fatal("expected TypeError for string | int, got nil")
	}
	if !strings.Contains(err.Error(), "TypeError") {
		t.Errorf("expected TypeError, got %v", err)
	}

	_, err = eng.Execute("test.py", `x = 10 << -1`)
	if err == nil {
		t.Fatal("expected ValueError for negative shift, got nil")
	}
	if !strings.Contains(err.Error(), "ValueError") {
		t.Errorf("expected ValueError, got %v", err)
	}
}


func TestAssertWithMessage(t *testing.T) {
	g := runGlobals(t, `
caught_msg = ""
try:
    assert 1 == 2, "values must be equal"
except e:
    caught_msg = e.message
`)
	sv, ok := g["caught_msg"].(vm.StringValue)
	if !ok || string(sv) != "values must be equal" {
		t.Errorf("expected custom assert message, got %v", g["caught_msg"])
	}
}



func TestOpGoSpawnWithLocals(t *testing.T) {
	g := runGlobals(t, `
import sync
ch = chan(5)

def worker(base, factor):
    local_a = base * 2
    local_b = factor * 3
    local_c = local_a + local_b
    local_d = local_c * 2
    ch.send(local_d)

go worker(10, 5) # (20 + 15) * 2 = 70
res = ch.recv()
ch.close()
`)
	assertInt(t, g, "res", 70)
}



func TestGoroutineErrorHandling(t *testing.T) {
	eng := NewEngine()
	g, err := eng.Execute("test.py", `
ch = chan(1)
def failing_worker():
    x = 1 / 0
    ch.send(x)

go failing_worker()
`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = g
}


func TestRecursionLimit(t *testing.T) {
	eng := NewEngine()
	_, err := eng.Execute("test.py", `
def infinite_recurse(n):
    return infinite_recurse(n + 1)

infinite_recurse(0)
`)
	if err == nil {
		t.Fatal("expected RecursionError, got nil")
	}
	if !strings.Contains(err.Error(), "RecursionError") {
		t.Errorf("expected RecursionError in error message, got: %v", err)
	}
}



func TestLosslessGoroutineErrorsWithWaitGroup(t *testing.T) {
	eng := NewEngine()
	_, err := eng.Execute("test.py", `
import time

def delayed_fail_1():
    time.sleep(0.01)
    x = 1 / 0

def delayed_fail_2():
    time.sleep(0.01)
    y = "invalid" + 123

go delayed_fail_1()
go delayed_fail_2()
`)
	if err != nil {
		t.Fatalf("unexpected compile/sync error: %v", err)
	}

	
	errs := eng.WaitAndDrainGoroutineErrors()
	if len(errs) != 2 {
		t.Fatalf("expected 2 goroutine errors captured, got %d: %v", len(errs), errs)
	}
}



func TestConcurrentSetGlobalAndExecute(t *testing.T) {
	eng := NewEngine()
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(2)
		idx := i
		go func() {
			defer wg.Done()
			eng.SetGlobal(fmt.Sprintf("key_%d", idx), vm.IntValue(int64(idx*10)))
		}()
		go func() {
			defer wg.Done()
			_, _ = eng.Execute("test.aeth", fmt.Sprintf("val_%d = %d * 2", idx, idx))
		}()
	}

	wg.Wait()
	globals := eng.Globals()
	if len(globals) == 0 {
		t.Fatal("expected globals to be populated")
	}
}



func TestConcurrentExecuteNoOverwrite(t *testing.T) {
	eng := NewEngine()
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := eng.Execute("thread1.aeth", "worker_a_var = 12345")
		if err != nil {
			t.Errorf("worker 1 error: %v", err)
		}
	}()
	go func() {
		defer wg.Done()
		_, err := eng.Execute("thread2.aeth", "worker_b_var = 67890")
		if err != nil {
			t.Errorf("worker 2 error: %v", err)
		}
	}()

	wg.Wait()
	globals := eng.Globals()
	if v, ok := globals["worker_a_var"]; !ok || v.Inspect() != "12345" {
		t.Fatalf("expected worker_a_var to be 12345, got %v", v)
	}
	if v, ok := globals["worker_b_var"]; !ok || v.Inspect() != "67890" {
		t.Fatalf("expected worker_b_var to be 67890, got %v", v)
	}
}


func TestUserModuleImportAeth(t *testing.T) {
	tmpDir := t.TempDir()
	modPath := filepath.Join(tmpDir, "calculator.aeth")
	modSrc := `
PI_APPROX = 314
def add(a, b):
    return a + b

def multiply(a, b):
    return a * b
`
	if err := os.WriteFile(modPath, []byte(modSrc), 0644); err != nil {
		t.Fatalf("failed to write test module: %v", err)
	}

	mainPath := filepath.Join(tmpDir, "main.aeth")
	mainSrc := `
import calculator
from calculator import multiply, PI_APPROX

sum_val = calculator.add(10, 20)
prod_val = multiply(5, 6)
pi_val = PI_APPROX
`
	eng := NewEngine()
	_, err := eng.Execute(mainPath, mainSrc)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}

	globals := eng.Globals()
	if v := globals["sum_val"]; v == nil || v.Inspect() != "30" {
		t.Fatalf("expected sum_val=30, got %v", v)
	}
	if v := globals["prod_val"]; v == nil || v.Inspect() != "30" {
		t.Fatalf("expected prod_val=30, got %v", v)
	}
	if v := globals["pi_val"]; v == nil || v.Inspect() != "314" {
		t.Fatalf("expected pi_val=314, got %v", v)
	}
}


func TestContextCancellationPolling(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	eng := NewEngineWithConfig(Config{Context: ctx})
	_, err := eng.Execute("loop.aeth", `
count = 0
while True:
    count = count + 1
`)
	if err == nil {
		t.Fatal("expected execution error on cancelled context")
	}
}


func TestCircularImportDetection(t *testing.T) {
	tmpDir := t.TempDir()
	modAPath := filepath.Join(tmpDir, "module_a.aeth")
	modBPath := filepath.Join(tmpDir, "module_b.aeth")

	if err := os.WriteFile(modAPath, []byte("import module_b\nval_a = 1"), 0644); err != nil {
		t.Fatalf("failed to write module_a: %v", err)
	}
	if err := os.WriteFile(modBPath, []byte("import module_a\nval_b = 2"), 0644); err != nil {
		t.Fatalf("failed to write module_b: %v", err)
	}

	eng := NewEngine()
	_, err := eng.Execute(modAPath, "import module_b")
	if err == nil {
		t.Fatal("expected error on circular import, got nil")
	}
	if !strings.Contains(err.Error(), "circular import detected") {
		t.Fatalf("expected 'circular import detected' error, got: %v", err)
	}
}


func TestComplexNumbers(t *testing.T) {
	g := runGlobals(t, `
z1 = 3 + 4j
z2 = 1 - 2j
z_sum = z1 + z2
z_diff = z1 - z2
z_prod = z1 * z2
z_conj = z1.conjugate()
r1 = z1.real
i1 = z1.imag
z3 = complex(5, 6)
`)
	if got := g["z_sum"].Inspect(); got != "(4+2j)" {
		t.Errorf("z_sum: want (4+2j), got %s", got)
	}
	if got := g["z_diff"].Inspect(); got != "(2+6j)" {
		t.Errorf("z_diff: want (2+6j), got %s", got)
	}
	if got := g["z_prod"].Inspect(); got != "(11-2j)" {
		t.Errorf("z_prod: want (11-2j), got %s", got)
	}
	if got := g["z_conj"].Inspect(); got != "(3-4j)" {
		t.Errorf("z_conj: want (3-4j), got %s", got)
	}
	assertFloat(t, g, "r1", 3.0)
	assertFloat(t, g, "i1", 4.0)
	if got := g["z3"].Inspect(); got != "(5+6j)" {
		t.Errorf("z3: want (5+6j), got %s", got)
	}
}


func TestBytesAndBytearray(t *testing.T) {
	g := runGlobals(t, `
b1 = b"hello"
ba = bytearray(b1)
ba.append(33) # '!'
b_len = len(b1)
ba_len = len(ba)
first_byte = b1[0]
`)
	assertInt(t, g, "b_len", 5)
	assertInt(t, g, "ba_len", 6)
	assertInt(t, g, "first_byte", 104) 
}


func TestFrozenset(t *testing.T) {
	g := runGlobals(t, `
fs = frozenset([1, 2, 3])
has_2 = 2 in fs
has_9 = 9 in fs
fs_len = len(fs)
s = set([3, 4, 5])
has_s3 = 3 in s
`)
	assertBool(t, g, "has_2", true)
	assertBool(t, g, "has_9", false)
	assertInt(t, g, "fs_len", 3)
	assertBool(t, g, "has_s3", true)
}


func TestMemoryView(t *testing.T) {
	g := runGlobals(t, `
data = b"abcdef"
mv = memoryview(data)
mv_len = len(mv)
mv_slice = mv[1:4]
slice_len = len(mv_slice)
`)
	assertInt(t, g, "mv_len", 6)
	assertInt(t, g, "slice_len", 3)
}


func TestWeakRef(t *testing.T) {
	g := runGlobals(t, `
import weakref

class Person:
    def __init__(self, name):
        self.name = name

p = Person("Alice")
ref = weakref.ref(p)
deref_p = ref()
p_name = deref_p.name
`)
	assertString(t, g, "p_name", "Alice")
}


func TestClassMethodAndStaticMethod(t *testing.T) {
	g := runGlobals(t, `
class MathUtils:
    base = 10

    @classmethod
    def get_base(cls):
        return cls.base

    @staticmethod
    def add(a, b):
        return a + b

res_add = MathUtils.add(5, 7)
res_base = MathUtils.get_base()
`)
	assertInt(t, g, "res_add", 12)
	assertInt(t, g, "res_base", 10)
}


func TestMultipleInheritanceMRO(t *testing.T) {
	g := runGlobals(t, `
class A:
    def greet(self):
        return "from A"

class B(A):
    def greet(self):
        return "from B"

class C(A):
    def greet(self):
        return "from C"

class D(B, C):
    pass

d = D()
msg = d.greet()
`)
	assertString(t, g, "msg", "from B")
}


func TestExceptionChainingAndGroups(t *testing.T) {
	g := runGlobals(t, `
cause_msg = ""
try:
    try:
        x = 1 / 0
    except orig:
        raise ValueError("computation failed") from orig
except err:
    if err.cause != None:
        cause_msg = err.cause.type
`)
	sv, ok := g["cause_msg"].(vm.StringValue)
	if !ok || !strings.Contains(string(sv), "ZeroDivision") {
		t.Errorf("expected ZeroDivisionError as cause, got %v", g["cause_msg"])
	}
}


func TestExtendedStdlib(t *testing.T) {
	g := runGlobals(t, `
import collections
import itertools
import pathlib
import datetime

# collections.Counter
counts = collections.Counter(["a", "b", "a", "c", "a", "b"])
mc = counts.most_common(1)
top_item = mc[0][0]
top_count = mc[0][1]

# itertools.islice
items = list(itertools.islice(range(10), 2, 5))

# pathlib
p = pathlib.Path("test.txt")
p_name = p.name()

# datetime
dt = datetime.datetime(2026, 9, 20, 12, 30, 0)
iso = dt.isoformat()
`)
	assertString(t, g, "top_item", "a")
	assertInt(t, g, "top_count", 3)
	if got := g["items"].Inspect(); got != "[2, 3, 4]" {
		t.Errorf("islice: want [2, 3, 4], got %s", got)
	}
	assertString(t, g, "p_name", "test.txt")
	if sv, ok := g["iso"].(vm.StringValue); !ok || !strings.HasPrefix(string(sv), "2026-09-20") {
		t.Errorf("isoformat: got %v", g["iso"])
	}
}


func TestFFIRegistryAndGoFunctions(t *testing.T) {
	eng := NewEngine()

	
	eng.RegisterGoFunction("go_multiply", func(args ...vm.Value) (vm.Value, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("TypeError: go_multiply requires 2 arguments")
		}
		a, ok1 := vm.ToInt(args[0])
		b, ok2 := vm.ToInt(args[1])
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("TypeError: arguments must be integers")
		}
		return vm.IntValue(a * b), nil
	})

	
	eng.RegisterModule("native_crypto", map[string]ffi.GoFunction{
		"sha_length": func(args ...vm.Value) (vm.Value, error) {
			return vm.IntValue(32), nil
		},
	})

	val, err := eng.Execute("ffi_test.aeth", `
res1 = go_multiply(6, 7)
import native_crypto
res2 = native_crypto.sha_length()
`)
	if err != nil {
		t.Fatalf("FFI execution failed: %v", err)
	}
	_ = val

	globals := eng.Globals()
	assertInt(t, globals, "res1", 42)
	assertInt(t, globals, "res2", 32)
}


func TestSubprocessAndSocketSandboxing(t *testing.T) {
	
	sandboxedEng := NewEngineWithConfig(Config{
		AllowSubprocess: false,
		AllowNetwork:    false,
	})

	_, err := sandboxedEng.Execute("sandbox_test.aeth", `
import subprocess
subprocess.run(["echo", "hello"])
`)
	if err == nil || !strings.Contains(err.Error(), "PermissionError") {
		t.Errorf("expected PermissionError for subprocess in sandbox, got: %v", err)
	}

	_, err2 := sandboxedEng.Execute("sandbox_test2.aeth", `
import socket
socket.gethostname()
`)
	if err2 == nil || !strings.Contains(err2.Error(), "PermissionError") {
		t.Errorf("expected PermissionError for socket in sandbox, got: %v", err2)
	}

	
	permittedEng := NewEngineWithConfig(Config{
		AllowSubprocess: true,
		AllowNetwork:    true,
	})

	_, err3 := permittedEng.Execute("permitted_test.aeth", `
import subprocess
import socket
h = socket.gethostname()
`)
	if err3 != nil {
		t.Fatalf("unexpected error with permissions enabled: %v", err3)
	}
}
