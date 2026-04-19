package evaluator

import "testing"

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
