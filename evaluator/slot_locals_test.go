package evaluator

import (
	"testing"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/lexer"
	"github.com/paularlott/scriptling/parser"
)

func TestSlotLocalsSupportRecursiveFunctionName(t *testing.T) {
	input := `
def fact(n):
    if n <= 1:
        return 1
    return n * fact(n - 1)

fact(5)
`

	testIntegerObject(t, testEval(input), 120)
}

func TestSlotLocalsRespectGlobalAndNonlocal(t *testing.T) {
	input := `
x = 1

def outer():
    y = 2
    def inner():
        nonlocal y
        global x
        y = y + 3
        x = x + 4
        return y
    inner()
    return y

outer() * 100 + x
`

	testIntegerObject(t, testEval(input), 505)
}

func TestSlotLocalsCollectControlFlowBindings(t *testing.T) {
	input := `
def compute():
    total = 0
    for left, right in [(1, 2), (3, 4)]:
        total = total + left + right
    try:
        raise ValueError("bad")
    except ValueError as err:
        message = str(err)
    if len(message) > 0:
        alias = total
    return alias

compute()
`

	testIntegerObject(t, testEval(input), 10)
}

func TestSlotLocalsCollectMatchCaptureAndClassBindings(t *testing.T) {
	input := `
def classify(value):
    class Box:
        def get(self, n):
            return n

    result = 0
    match value:
        case 7 as matched:
            result = Box().get(matched)
    return result

classify(7)
`

	testIntegerObject(t, testEval(input), 7)
}

// Names bound by match patterns, by with targets (also several in one
// statement) and by a yield's assignment get local slots like assignments.
func TestSlotLocalsCollectPatternWithAndYieldBindings(t *testing.T) {
	src := `
def f(v):
    match v:
        case [a, *rest, (b, _)]:
            pass
        case Point(c, y=d) | {"k": c, **d}:
            pass
    with x() as e, y() as g:
        pass
    h = yield 1
    i, j = yield
    k = yield from v
`
	program := parser.New(lexer.New(src)).ParseProgram()
	fs := program.Statements[0].(*ast.FunctionStatement)
	for _, name := range []string{"v", "a", "rest", "b", "c", "d", "e", "g", "h", "i", "j", "k"} {
		if _, ok := fs.Function.LocalSlots[name]; !ok {
			t.Errorf("%s has no local slot (slots: %v)", name, fs.Function.LocalSlotNames)
		}
	}
	for _, name := range []string{"_", "Point", "x", "y"} {
		if _, ok := fs.Function.LocalSlots[name]; ok {
			t.Errorf("%s should not be a local slot", name)
		}
	}
}
