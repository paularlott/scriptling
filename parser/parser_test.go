package parser

import (
	"testing"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/lexer"
)

func TestAssignStatements(t *testing.T) {
	input := `x = 5
y = 10
z = x`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 3 {
		t.Fatalf("program.Statements does not contain 3 statements. got=%d",
			len(program.Statements))
	}

	tests := []string{"x", "y", "z"}
	for i, tt := range tests {
		stmt := program.Statements[i]
		if !testAssignStatement(t, stmt, tt) {
			return
		}
	}
}

func TestIntegerLiteralExpression(t *testing.T) {
	input := "5"

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program has not enough statements. got=%d",
			len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ExpressionStatement. got=%T",
			program.Statements[0])
	}

	literal, ok := stmt.Expression.(*ast.IntegerLiteral)
	if !ok {
		t.Fatalf("exp not *ast.IntegerLiteral. got=%T", stmt.Expression)
	}
	if literal.Value != 5 {
		t.Errorf("literal.Value not %d. got=%d", 5, literal.Value)
	}
}

func TestPrefixExpressions(t *testing.T) {
	prefixTests := []struct {
		input    string
		operator string
		value    interface{}
	}{
		{"-5", "-", 5},
		{"not True", "not", true},
		{"not False", "not", false},
	}

	for _, tt := range prefixTests {
		l := lexer.New(tt.input)
		p := New(l)
		program := p.ParseProgram()
		checkParserErrors(t, p)

		if len(program.Statements) != 1 {
			t.Fatalf("program.Statements does not contain %d statements. got=%d\n",
				1, len(program.Statements))
		}

		stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
		if !ok {
			t.Fatalf("program.Statements[0] is not ast.ExpressionStatement. got=%T",
				program.Statements[0])
		}

		exp, ok := stmt.Expression.(*ast.PrefixExpression)
		if !ok {
			t.Fatalf("stmt is not ast.PrefixExpression. got=%T", stmt.Expression)
		}
		if exp.Operator != tt.operator {
			t.Fatalf("exp.Operator is not '%s'. got=%s",
				tt.operator, exp.Operator)
		}
	}
}

func TestInfixExpressions(t *testing.T) {
	infixTests := []struct {
		input      string
		leftValue  interface{}
		operator   string
		rightValue interface{}
	}{
		{"5 + 5", 5, "+", 5},
		{"5 - 5", 5, "-", 5},
		{"5 * 5", 5, "*", 5},
		{"5 / 5", 5, "/", 5},
		{"5 > 5", 5, ">", 5},
		{"5 < 5", 5, "<", 5},
		{"5 == 5", 5, "==", 5},
		{"5 != 5", 5, "!=", 5},
	}

	for _, tt := range infixTests {
		l := lexer.New(tt.input)
		p := New(l)
		program := p.ParseProgram()
		checkParserErrors(t, p)

		if len(program.Statements) != 1 {
			t.Fatalf("program.Statements does not contain %d statements. got=%d\n",
				1, len(program.Statements))
		}

		stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
		if !ok {
			t.Fatalf("program.Statements[0] is not ast.ExpressionStatement. got=%T",
				program.Statements[0])
		}

		exp, ok := stmt.Expression.(*ast.InfixExpression)
		if !ok {
			t.Fatalf("exp is not ast.InfixExpression. got=%T", stmt.Expression)
		}

		if exp.Operator != tt.operator {
			t.Fatalf("exp.Operator is not '%s'. got=%s",
				tt.operator, exp.Operator)
		}
	}
}

func TestIfStatement(t *testing.T) {
	input := `if x < y:
    x = 1`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain %d statements. got=%d\n",
			1, len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.IfStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.IfStatement. got=%T",
			program.Statements[0])
	}

	if stmt.Consequence == nil {
		t.Errorf("stmt.Consequence is nil")
	}
}

func TestFunctionStatement(t *testing.T) {
	input := `def add(x, y):
    return x + y`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain %d statements. got=%d\n",
			1, len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.FunctionStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.FunctionStatement. got=%T",
			program.Statements[0])
	}

	if stmt.Name.Value != "add" {
		t.Fatalf("function name is not 'add'. got=%s", stmt.Name.Value)
	}

	if len(stmt.Function.Parameters) != 2 {
		t.Fatalf("function parameters wrong. want 2, got=%d",
			len(stmt.Function.Parameters))
	}
}

func TestCallExpression(t *testing.T) {
	input := "add(1, 2 + 3)"

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain %d statements. got=%d\n",
			1, len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("stmt is not ast.ExpressionStatement. got=%T",
			program.Statements[0])
	}

	exp, ok := stmt.Expression.(*ast.CallExpression)
	if !ok {
		t.Fatalf("stmt.Expression is not ast.CallExpression. got=%T",
			stmt.Expression)
	}

	if len(exp.Arguments) != 2 {
		t.Fatalf("wrong length of arguments. got=%d", len(exp.Arguments))
	}
}

func checkParserErrors(t *testing.T, p *Parser) {
	errors := p.Errors()
	if len(errors) == 0 {
		return
	}

	t.Errorf("parser has %d errors", len(errors))
	for _, msg := range errors {
		t.Errorf("parser error: %q", msg)
	}
	t.FailNow()
}

func testAssignStatement(t *testing.T, s ast.Statement, name string) bool {
	if s.TokenLiteral() != name {
		t.Errorf("s.TokenLiteral not '%s'. got=%s", name, s.TokenLiteral())
		return false
	}

	assignStmt, ok := s.(*ast.AssignStatement)
	if !ok {
		t.Errorf("s not *ast.AssignStatement. got=%T", s)
		return false
	}

	ident, ok := assignStmt.Left.(*ast.Identifier)
	if !ok {
		t.Errorf("assignStmt.Left not *ast.Identifier. got=%T", assignStmt.Left)
		return false
	}

	if ident.Value != name {
		t.Errorf("assignStmt.Left.Value not '%s'. got=%s", name, ident.Value)
		return false
	}

	return true
}

func TestFloatLiteralExpression(t *testing.T) {
	input := "3.14"

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program has not enough statements. got=%d",
			len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ExpressionStatement. got=%T",
			program.Statements[0])
	}

	literal, ok := stmt.Expression.(*ast.FloatLiteral)
	if !ok {
		t.Fatalf("exp not *ast.FloatLiteral. got=%T", stmt.Expression)
	}
	if literal.Value != 3.14 {
		t.Errorf("literal.Value not %f. got=%f", 3.14, literal.Value)
	}
}

func TestStringLiteralExpression(t *testing.T) {
	input := `"hello"`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program has not enough statements. got=%d",
			len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ExpressionStatement. got=%T",
			program.Statements[0])
	}

	literal, ok := stmt.Expression.(*ast.StringLiteral)
	if !ok {
		t.Fatalf("exp not *ast.StringLiteral. got=%T", stmt.Expression)
	}
	if literal.Value != "hello" {
		t.Errorf("literal.Value not 'hello'. got=%s", literal.Value)
	}
}

func TestBooleanExpressions(t *testing.T) {
	tests := []struct {
		input  string
		value  bool
	}{
		{"True", true},
		{"False", false},
	}

	for _, tt := range tests {
		l := lexer.New(tt.input)
		p := New(l)
		program := p.ParseProgram()
		checkParserErrors(t, p)

		stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
		if !ok {
			t.Errorf("program.Statements[0] is not ast.ExpressionStatement")
			continue
		}

		exp, ok := stmt.Expression.(*ast.Boolean)
		if !ok {
			t.Errorf("stmt.Expression is not ast.Boolean")
			continue
		}

		if exp.Value != tt.value {
			t.Errorf("Boolean value not %v. got=%v", tt.value, exp.Value)
		}
	}
}

func TestListLiteral(t *testing.T) {
	input := "[1, 2, 3]"

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program has not enough statements. got=%d",
			len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ExpressionStatement")
	}

	list, ok := stmt.Expression.(*ast.ListLiteral)
	if !ok {
		t.Fatalf("stmt.Expression is not ast.ListLiteral")
	}

	if len(list.Elements) != 3 {
		t.Errorf("list.Elements length not 3. got=%d", len(list.Elements))
	}
}

func TestDictLiteral(t *testing.T) {
	input := `{"key": "value"}`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program has not enough statements. got=%d",
			len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ExpressionStatement")
	}

	dict, ok := stmt.Expression.(*ast.DictLiteral)
	if !ok {
		t.Fatalf("stmt.Expression is not ast.DictLiteral")
	}

	if len(dict.Pairs) != 1 {
		t.Errorf("dict.Pairs length not 1. got=%d", len(dict.Pairs))
	}
}

func TestWhileStatement(t *testing.T) {
	input := `while True:
    pass`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain %d statements. got=%d",
			1, len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.WhileStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.WhileStatement. got=%T",
			program.Statements[0])
	}

	if stmt.Body == nil {
		t.Error("stmt.Body is nil")
	}
}

func TestForStatement(t *testing.T) {
	input := `for x in items:
    pass`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain %d statements. got=%d",
			1, len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ForStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ForStatement. got=%T",
			program.Statements[0])
	}

	if len(stmt.Variables) != 1 {
		t.Errorf("for loop variables length = %d, want 1", len(stmt.Variables))
	}

	if stmt.Body == nil {
		t.Error("for loop Body is nil")
	}
}

func TestMatchStatement(t *testing.T) {
	input := `match value:
    case 1:
        pass
    case 2:
        pass`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain %d statements. got=%d",
			1, len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.MatchStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.MatchStatement. got=%T",
			program.Statements[0])
	}

	if len(stmt.Cases) != 2 {
		t.Errorf("match statement cases length = %d, want 2", len(stmt.Cases))
	}

	if stmt.Subject == nil {
		t.Error("match statement Subject is nil")
	}
}

func TestLambdaExpression(t *testing.T) {
	input := "lambda x: x + 1"

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain %d statements. got=%d",
			1, len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ExpressionStatement. got=%T",
			program.Statements[0])
	}

	lambda, ok := stmt.Expression.(*ast.Lambda)
	if !ok {
		t.Fatalf("stmt.Expression is not ast.Lambda. got=%T", stmt.Expression)
	}

	if len(lambda.Parameters) != 1 {
		t.Errorf("lambda parameters length = %d, want 1", len(lambda.Parameters))
	}
}

func TestListComprehension(t *testing.T) {
	input := "[x for x in items]"

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain %d statements. got=%d",
			1, len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ExpressionStatement. got=%T",
			program.Statements[0])
	}

	lc, ok := stmt.Expression.(*ast.ListComprehension)
	if !ok {
		t.Fatalf("stmt.Expression is not ast.ListComprehension. got=%T", stmt.Expression)
	}

	if lc.Expression == nil {
		t.Error("list comprehension Expression is nil")
	}

	if lc.Iterable == nil {
		t.Error("list comprehension Iterable is nil")
	}
}

func TestAugmentedAssignStatement(t *testing.T) {
	input := "x += 1"

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain %d statements. got=%d",
			1, len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.AugmentedAssignStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.AugmentedAssignStatement. got=%T",
			program.Statements[0])
	}

	if stmt.Operator != "+=" {
		t.Errorf("augmented assign operator = %s, want +=", stmt.Operator)
	}
}

func TestIndexExpression(t *testing.T) {
	input := "arr[0]"

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain %d statements. got=%d",
			1, len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ExpressionStatement. got=%T",
			program.Statements[0])
	}

	exp, ok := stmt.Expression.(*ast.IndexExpression)
	if !ok {
		t.Fatalf("stmt.Expression is not ast.IndexExpression. got=%T", stmt.Expression)
	}

	if exp.Left == nil {
		t.Error("index expression Left is nil")
	}

	if exp.Index == nil {
		t.Error("index expression Index is nil")
	}
}

func TestSliceExpression(t *testing.T) {
	input := "arr[0:10]"

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain %d statements. got=%d",
			1, len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ExpressionStatement. got=%T",
			program.Statements[0])
	}

	exp, ok := stmt.Expression.(*ast.SliceExpression)
	if !ok {
		t.Fatalf("stmt.Expression is not ast.SliceExpression. got=%T", stmt.Expression)
	}

	if exp.Left == nil {
		t.Error("slice expression Left is nil")
	}
}

func TestMethodCallExpression(t *testing.T) {
	input := "obj.method(1, 2)"

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain %d statements. got=%d",
			1, len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ExpressionStatement. got=%T",
			program.Statements[0])
	}

	exp, ok := stmt.Expression.(*ast.MethodCallExpression)
	if !ok {
		t.Fatalf("stmt.Expression is not ast.MethodCallExpression. got=%T", stmt.Expression)
	}

	if exp.Object == nil {
		t.Error("method call Object is nil")
	}

	if exp.Method == nil {
		t.Error("method call Method is nil")
	}

	if len(exp.Arguments) != 2 {
		t.Errorf("method call arguments length = %d, want 2", len(exp.Arguments))
	}
}

func TestReturnStatement(t *testing.T) {
	input := "return 42"

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain %d statements. got=%d",
			1, len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ReturnStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ReturnStatement. got=%T",
			program.Statements[0])
	}

	if stmt.ReturnValue == nil {
		t.Error("return statement ReturnValue is nil")
	}
}

func TestBareReturnStatement(t *testing.T) {
	// Bare return inside a function must not consume the DEDENT token,
	// which would cause subsequent top-level code to be absorbed into
	// the function body.
	input := `def f():
    return
print("after")`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 2 {
		t.Fatalf("expected 2 top-level statements (def + print), got=%d",
			len(program.Statements))
	}

	fnStmt, ok := program.Statements[0].(*ast.FunctionStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not FunctionStatement. got=%T",
			program.Statements[0])
	}
	if len(fnStmt.Function.Body.Statements) != 1 {
		t.Fatalf("function body should have 1 statement, got=%d",
			len(fnStmt.Function.Body.Statements))
	}
	retStmt, ok := fnStmt.Function.Body.Statements[0].(*ast.ReturnStatement)
	if !ok {
		t.Fatalf("function body statement is not ReturnStatement. got=%T",
			fnStmt.Function.Body.Statements[0])
	}
	if retStmt.ReturnValue != nil {
		t.Error("bare return should have nil ReturnValue")
	}

	_, ok = program.Statements[1].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("program.Statements[1] is not ExpressionStatement. got=%T",
			program.Statements[1])
	}
}

func TestBareReturnInIfBlock(t *testing.T) {
	// Bare return inside an if block within a function.
	input := `def f(x):
    if x:
        return
    print("not returned")
print("after")`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 2 {
		t.Fatalf("expected 2 top-level statements, got=%d",
			len(program.Statements))
	}

	fnStmt, ok := program.Statements[0].(*ast.FunctionStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not FunctionStatement. got=%T",
			program.Statements[0])
	}
	// Function body: if statement + print statement
	if len(fnStmt.Function.Body.Statements) != 2 {
		t.Fatalf("function body should have 2 statements, got=%d",
			len(fnStmt.Function.Body.Statements))
	}
}

func TestBareRaiseStatement(t *testing.T) {
	// Bare raise (re-raise) inside a function must not consume the DEDENT.
	input := `def f():
    raise
print("after")`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 2 {
		t.Fatalf("expected 2 top-level statements, got=%d",
			len(program.Statements))
	}

	fnStmt, ok := program.Statements[0].(*ast.FunctionStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not FunctionStatement. got=%T",
			program.Statements[0])
	}
	if len(fnStmt.Function.Body.Statements) != 1 {
		t.Fatalf("function body should have 1 statement, got=%d",
			len(fnStmt.Function.Body.Statements))
	}
	raiseStmt, ok := fnStmt.Function.Body.Statements[0].(*ast.RaiseStatement)
	if !ok {
		t.Fatalf("function body statement is not RaiseStatement. got=%T",
			fnStmt.Function.Body.Statements[0])
	}
	if raiseStmt.Message != nil {
		t.Error("bare raise should have nil Message")
	}
}

func TestBreakStatement(t *testing.T) {
	input := "break"

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain %d statements. got=%d",
			1, len(program.Statements))
	}

	_, ok := program.Statements[0].(*ast.BreakStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.BreakStatement. got=%T",
			program.Statements[0])
	}
}

func TestContinueStatement(t *testing.T) {
	input := "continue"

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain %d statements. got=%d",
			1, len(program.Statements))
	}

	_, ok := program.Statements[0].(*ast.ContinueStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ContinueStatement. got=%T",
			program.Statements[0])
	}
}

func TestImportStatement(t *testing.T) {
	input := "import os"

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain %d statements. got=%d",
			1, len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ImportStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ImportStatement. got=%T",
			program.Statements[0])
	}

	if stmt.Name == nil {
		t.Error("import statement Name is nil")
	}
}

func TestImportStatementWithAlias(t *testing.T) {
	input := "import os as operating_system"

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain %d statements. got=%d",
			1, len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ImportStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ImportStatement. got=%T",
			program.Statements[0])
	}

	if stmt.Name.Value != "os" {
		t.Errorf("stmt.Name.Value = %q, want %q", stmt.Name.Value, "os")
	}

	if stmt.Alias == nil {
		t.Fatal("stmt.Alias is nil")
	}

	if stmt.Alias.Value != "operating_system" {
		t.Errorf("stmt.Alias.Value = %q, want %q", stmt.Alias.Value, "operating_system")
	}
}

func TestImportStatementMultiple(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		expectedNames    []string
		expectedAliases  []string
	}{
		{
			name:          "multiple imports without aliases",
			input:         "import os, sys, json",
			expectedNames: []string{"os", "sys", "json"},
			expectedAliases: []string{"", "", ""},
		},
		{
			name:          "multiple imports with aliases",
			input:         "import os as op, sys as system, json",
			expectedNames: []string{"os", "sys", "json"},
			expectedAliases: []string{"op", "system", ""},
		},
		{
			name:          "mixed imports with and without aliases",
			input:         "import os, sys as system, json as j",
			expectedNames: []string{"os", "sys", "json"},
			expectedAliases: []string{"", "system", "j"},
		},
		{
			name:          "dotted imports with aliases",
			input:         "import scriptling.ai as ai, scriptling.console as console, glob, json",
			expectedNames: []string{"scriptling.ai", "scriptling.console", "glob", "json"},
			expectedAliases: []string{"ai", "console", "", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := New(l)
			program := p.ParseProgram()
			checkParserErrors(t, p)

			if len(program.Statements) != 1 {
				t.Fatalf("program.Statements does not contain %d statements. got=%d",
					1, len(program.Statements))
			}

			stmt, ok := program.Statements[0].(*ast.ImportStatement)
			if !ok {
				t.Fatalf("program.Statements[0] is not ast.ImportStatement. got=%T",
					program.Statements[0])
			}

			if stmt.Name.Value != tt.expectedNames[0] {
				t.Errorf("stmt.Name.Value = %q, want %q", stmt.Name.Value, tt.expectedNames[0])
			}

			if len(stmt.AdditionalNames) != len(tt.expectedNames)-1 {
				t.Fatalf("stmt.AdditionalNames length = %d, want %d",
					len(stmt.AdditionalNames), len(tt.expectedNames)-1)
			}

			for i, name := range stmt.AdditionalNames {
				expectedName := tt.expectedNames[i+1]
				if name.Value != expectedName {
					t.Errorf("stmt.AdditionalNames[%d].Value = %q, want %q",
						i, name.Value, expectedName)
				}
			}

			if len(stmt.AdditionalAliases) != len(tt.expectedAliases)-1 {
				t.Fatalf("stmt.AdditionalAliases length = %d, want %d",
					len(stmt.AdditionalAliases), len(tt.expectedAliases)-1)
			}

			for i, alias := range stmt.AdditionalAliases {
				expectedAlias := tt.expectedAliases[i+1]
				if expectedAlias == "" {
					if alias != nil {
						t.Errorf("stmt.AdditionalAliases[%d] should be nil, got %q",
							i, alias.Value)
					}
				} else {
					if alias == nil {
						t.Errorf("stmt.AdditionalAliases[%d] is nil, want %q",
							i, expectedAlias)
					} else if alias.Value != expectedAlias {
						t.Errorf("stmt.AdditionalAliases[%d].Value = %q, want %q",
							i, alias.Value, expectedAlias)
					}
				}
			}
		})
	}
}

func TestClassStatement(t *testing.T) {
	input := `class MyClass:
    pass`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain %d statements. got=%d",
			1, len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ClassStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ClassStatement. got=%T",
			program.Statements[0])
	}

	if stmt.Name == nil {
		t.Error("class statement Name is nil")
	}

	if stmt.Body == nil {
		t.Error("class statement Body is nil")
	}
}

func TestTryStatement(t *testing.T) {
	input := `try:
    pass
except:
    pass`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain %d statements. got=%d",
			1, len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.TryStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.TryStatement. got=%T",
			program.Statements[0])
	}

	if stmt.Body == nil {
		t.Error("try statement Body is nil")
	}

	if len(stmt.ExceptClauses) == 0 {
		t.Error("try statement ExceptClauses is empty")
	}
}

func TestFStringLiteral(t *testing.T) {
	input := `f"Hello {name}"`

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain %d statements. got=%d",
			1, len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ExpressionStatement. got=%T",
			program.Statements[0])
	}

	fstr, ok := stmt.Expression.(*ast.FStringLiteral)
	if !ok {
		t.Fatalf("stmt.Expression is not ast.FStringLiteral. got=%T", stmt.Expression)
	}

	if len(fstr.Expressions) == 0 {
		t.Error("f-string Expressions is empty")
	}

	if len(fstr.Parts) == 0 {
		t.Error("f-string Parts is empty")
	}
}

func TestTupleLiteral(t *testing.T) {
	input := "(1, 2, 3)"

	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)

	if len(program.Statements) != 1 {
		t.Fatalf("program.Statements does not contain %d statements. got=%d",
			1, len(program.Statements))
	}

	stmt, ok := program.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("program.Statements[0] is not ast.ExpressionStatement. got=%T",
			program.Statements[0])
	}

	tuple, ok := stmt.Expression.(*ast.TupleLiteral)
	if !ok {
		t.Fatalf("stmt.Expression is not ast.TupleLiteral. got=%T", stmt.Expression)
	}

	if len(tuple.Elements) != 3 {
		t.Errorf("tuple Elements length = %d, want 3", len(tuple.Elements))
	}
}
