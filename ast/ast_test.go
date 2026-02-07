package ast

import "testing"

func TestProgram(t *testing.T) {
	program := &Program{
		Statements: []Statement{
			&AssignStatement{
				Left:  &Identifier{Value: "myVar"},
				Value: &Identifier{Value: "anotherVar"},
			},
		},
	}

	if len(program.Statements) != 1 {
		t.Errorf("program has wrong number of statements. got=%d", len(program.Statements))
	}
}

func TestIdentifier(t *testing.T) {
	ident := &Identifier{Value: "foobar"}

	if ident.Value != "foobar" {
		t.Errorf("ident.Value = %q, want %q", ident.Value, "foobar")
	}
}

func TestIntegerLiteral(t *testing.T) {
	lit := &IntegerLiteral{Value: 5}

	if lit.Value != 5 {
		t.Errorf("lit.Value = %d, want %d", lit.Value, 5)
	}
}

func TestFloatLiteral(t *testing.T) {
	lit := &FloatLiteral{Value: 3.14}

	if lit.Value != 3.14 {
		t.Errorf("lit.Value = %f, want %f", lit.Value, 3.14)
	}
}

func TestStringLiteral(t *testing.T) {
	lit := &StringLiteral{Value: "hello world"}

	if lit.Value != "hello world" {
		t.Errorf("lit.Value = %q, want %q", lit.Value, "hello world")
	}
}

func TestBoolean(t *testing.T) {
	trueBool := &Boolean{Value: true}
	falseBool := &Boolean{Value: false}

	if trueBool.Value != true {
		t.Errorf("trueBool.Value = %t, want %t", trueBool.Value, true)
	}

	if falseBool.Value != false {
		t.Errorf("falseBool.Value = %t, want %t", falseBool.Value, false)
	}
}

func TestNone(t *testing.T) {
	none := &None{}
	// Just verify it can be created
	if none == nil {
		t.Error("None literal should not be nil")
	}
}

func TestFStringLiteral(t *testing.T) {
	fstr := &FStringLiteral{
		Value:       "Hello {name}",
		Expressions: []Expression{&Identifier{Value: "name"}},
		Parts:       []string{"Hello ", ""},
		FormatSpecs: []string{""},
	}

	if fstr.Value != "Hello {name}" {
		t.Errorf("fstr.Value = %q, want %q", fstr.Value, "Hello {name}")
	}

	if len(fstr.Expressions) != 1 {
		t.Errorf("fstr.Expressions length = %d, want 1", len(fstr.Expressions))
	}

	if len(fstr.Parts) != 2 {
		t.Errorf("fstr.Parts length = %d, want 2", len(fstr.Parts))
	}
}

func TestPrefixExpression(t *testing.T) {
	expr := &PrefixExpression{
		Operator: "-",
		Right:    &IntegerLiteral{Value: 5},
	}

	if expr.Operator != "-" {
		t.Errorf("expr.Operator = %q, want %q", expr.Operator, "-")
	}

	if expr.Right == nil {
		t.Error("expr.Right should not be nil")
	}
}

func TestInfixExpression(t *testing.T) {
	expr := &InfixExpression{
		Left:     &IntegerLiteral{Value: 5},
		Operator: "+",
		Right:    &IntegerLiteral{Value: 10},
	}

	if expr.Operator != "+" {
		t.Errorf("expr.Operator = %q, want %q", expr.Operator, "+")
	}

	if expr.Left == nil || expr.Right == nil {
		t.Error("expr.Left and expr.Right should not be nil")
	}
}

func TestConditionalExpression(t *testing.T) {
	expr := &ConditionalExpression{
		Condition: &Boolean{Value: true},
		TrueExpr:  &IntegerLiteral{Value: 1},
		FalseExpr: &IntegerLiteral{Value: 0},
	}

	if expr.Condition == nil || expr.TrueExpr == nil || expr.FalseExpr == nil {
		t.Error("Conditional expression parts should not be nil")
	}
}

func TestAssignStatement(t *testing.T) {
	stmt := &AssignStatement{
		Left:  &Identifier{Value: "x"},
		Value: &IntegerLiteral{Value: 5},
	}

	if stmt.Left == nil || stmt.Value == nil {
		t.Error("Assign statement parts should not be nil")
	}
}

func TestAugmentedAssignStatement(t *testing.T) {
	stmt := &AugmentedAssignStatement{
		Name:     &Identifier{Value: "x"},
		Operator: "+=",
		Value:    &IntegerLiteral{Value: 5},
	}

	if stmt.Operator != "+=" {
		t.Errorf("stmt.Operator = %q, want %q", stmt.Operator, "+=")
	}

	if stmt.Name == nil || stmt.Value == nil {
		t.Error("Augmented assign statement parts should not be nil")
	}
}

func TestMultipleAssignStatement(t *testing.T) {
	stmt := &MultipleAssignStatement{
		Names: []*Identifier{
			{Value: "x"},
			{Value: "y"},
		},
		Value: &ListLiteral{
			Elements: []Expression{
				&IntegerLiteral{Value: 1},
				&IntegerLiteral{Value: 2},
			},
		},
	}

	if len(stmt.Names) != 2 {
		t.Errorf("stmt.Names length = %d, want 2", len(stmt.Names))
	}
}

func TestExpressionStatement(t *testing.T) {
	stmt := &ExpressionStatement{
		Expression: &IntegerLiteral{Value: 42},
	}

	if stmt.Expression == nil {
		t.Error("Expression statement expression should not be nil")
	}
}

func TestBlockStatement(t *testing.T) {
	stmt := &BlockStatement{
		Statements: []Statement{
			&ExpressionStatement{Expression: &IntegerLiteral{Value: 1}},
		},
	}

	if len(stmt.Statements) != 1 {
		t.Errorf("Block statement count = %d, want 1", len(stmt.Statements))
	}
}

func TestIfStatement(t *testing.T) {
	stmt := &IfStatement{
		Condition:   &Boolean{Value: true},
		Consequence: &BlockStatement{},
		ElifClauses: []*ElifClause{
			{
				Condition:   &Boolean{Value: false},
				Consequence: &BlockStatement{},
			},
		},
		Alternative: &BlockStatement{},
	}

	if stmt.Condition == nil || stmt.Consequence == nil {
		t.Error("If statement parts should not be nil")
	}

	if len(stmt.ElifClauses) != 1 {
		t.Errorf("stmt.ElifClauses length = %d, want 1", len(stmt.ElifClauses))
	}
}

func TestWhileStatement(t *testing.T) {
	stmt := &WhileStatement{
		Condition: &Boolean{Value: true},
		Body:      &BlockStatement{},
	}

	if stmt.Condition == nil || stmt.Body == nil {
		t.Error("While statement parts should not be nil")
	}
}

func TestForStatement(t *testing.T) {
	stmt := &ForStatement{
		Variables: []Expression{&Identifier{Value: "x"}},
		Iterable:  &Identifier{Value: "items"},
		Body:      &BlockStatement{},
	}

	if len(stmt.Variables) != 1 {
		t.Errorf("stmt.Variables length = %d, want 1", len(stmt.Variables))
	}

	if stmt.Iterable == nil || stmt.Body == nil {
		t.Error("For statement parts should not be nil")
	}
}

func TestFunctionLiteral(t *testing.T) {
	fn := &FunctionLiteral{
		Parameters: []*Identifier{
			{Value: "x"},
			{Value: "y"},
		},
		DefaultValues: map[string]Expression{
			"y": &IntegerLiteral{Value: 5},
		},
		Variadic: &Identifier{Value: "args"},
		Body:     &BlockStatement{},
	}

	if len(fn.Parameters) != 2 {
		t.Errorf("fn.Parameters length = %d, want 2", len(fn.Parameters))
	}

	if fn.Variadic == nil || fn.Body == nil {
		t.Error("Function literal parts should not be nil")
	}
}

func TestFunctionStatement(t *testing.T) {
	stmt := &FunctionStatement{
		Name: &Identifier{Value: "myFunc"},
		Function: &FunctionLiteral{
			Parameters: []*Identifier{{Value: "x"}},
			Body:       &BlockStatement{},
		},
	}

	if stmt.Name == nil || stmt.Function == nil {
		t.Error("Function statement parts should not be nil")
	}
}

func TestClassStatement(t *testing.T) {
	stmt := &ClassStatement{
		Name:      &Identifier{Value: "MyClass"},
		BaseClass: &Identifier{Value: "BaseClass"},
		Body:      &BlockStatement{},
	}

	if stmt.Name == nil || stmt.Body == nil {
		t.Error("Class statement parts should not be nil")
	}
}

func TestCallExpression(t *testing.T) {
	expr := &CallExpression{
		Function: &Identifier{Value: "print"},
		Arguments: []Expression{
			&StringLiteral{Value: "hello"},
		},
		Keywords: map[string]Expression{
			"sep": &StringLiteral{Value: ", "},
		},
	}

	if len(expr.Arguments) != 1 {
		t.Errorf("expr.Arguments length = %d, want 1", len(expr.Arguments))
	}

	if len(expr.Keywords) != 1 {
		t.Errorf("expr.Keywords length = %d, want 1", len(expr.Keywords))
	}
}

func TestMethodCallExpression(t *testing.T) {
	expr := &MethodCallExpression{
		Object: &Identifier{Value: "obj"},
		Method: &Identifier{Value: "method"},
		Arguments: []Expression{
			&IntegerLiteral{Value: 1},
		},
		Keywords: map[string]Expression{},
	}

	if expr.Object == nil || expr.Method == nil {
		t.Error("Method call expression parts should not be nil")
	}
}

func TestReturnStatement(t *testing.T) {
	stmt := &ReturnStatement{
		ReturnValue: &IntegerLiteral{Value: 42},
	}

	if stmt.ReturnValue == nil {
		t.Error("Return statement value should not be nil")
	}

	// Test return without value
	stmt2 := &ReturnStatement{}
	if stmt2.ReturnValue != nil {
		t.Error("Return statement without value should have nil ReturnValue")
	}
}

func TestBreakStatement(t *testing.T) {
	stmt := &BreakStatement{}
	if stmt == nil {
		t.Error("Break statement should not be nil")
	}
}

func TestContinueStatement(t *testing.T) {
	stmt := &ContinueStatement{}
	if stmt == nil {
		t.Error("Continue statement should not be nil")
	}
}

func TestPassStatement(t *testing.T) {
	stmt := &PassStatement{}
	if stmt == nil {
		t.Error("Pass statement should not be nil")
	}
}

func TestImportStatement(t *testing.T) {
	stmt := &ImportStatement{
		Name: &Identifier{Value: "os"},
	}

	if stmt.Name == nil {
		t.Error("Import statement name should not be nil")
	}

	if stmt.FullName() != "os" {
		t.Errorf("stmt.FullName() = %q, want %q", stmt.FullName(), "os")
	}

	// Test with alias
	stmt2 := &ImportStatement{
		Name:  &Identifier{Value: "os"},
		Alias: &Identifier{Value: "operating_system"},
	}

	if stmt2.Alias == nil {
		t.Error("Import statement alias should not be nil")
	}

	// Test with multiple imports
	stmt3 := &ImportStatement{
		Name: &Identifier{Value: "os"},
		AdditionalNames: []*Identifier{
			{Value: "sys"},
			{Value: "json"},
		},
	}

	if len(stmt3.AdditionalNames) != 2 {
		t.Errorf("stmt3.AdditionalNames length = %d, want 2", len(stmt3.AdditionalNames))
	}
}

func TestFromImportStatement(t *testing.T) {
	stmt := &FromImportStatement{
		Module: &Identifier{Value: "bs4"},
		Names: []*Identifier{
			{Value: "BeautifulSoup"},
		},
		Aliases: []*Identifier{
			{Value: "BS"},
		},
	}

	if stmt.Module == nil {
		t.Error("From import statement module should not be nil")
	}

	if len(stmt.Names) != 1 {
		t.Errorf("stmt.Names length = %d, want 1", len(stmt.Names))
	}
}

func TestListLiteral(t *testing.T) {
	lst := &ListLiteral{
		Elements: []Expression{
			&IntegerLiteral{Value: 1},
			&IntegerLiteral{Value: 2},
			&IntegerLiteral{Value: 3},
		},
	}

	if len(lst.Elements) != 3 {
		t.Errorf("lst.Elements length = %d, want 3", len(lst.Elements))
	}
}

func TestDictLiteral(t *testing.T) {
	dct := &DictLiteral{
		Pairs: map[Expression]Expression{
			&StringLiteral{Value: "key"}: &IntegerLiteral{Value: 1},
			&StringLiteral{Value: "key2"}: &IntegerLiteral{Value: 2},
		},
	}

	if len(dct.Pairs) != 2 {
		t.Errorf("dct.Pairs length = %d, want 2", len(dct.Pairs))
	}
}

func TestIndexExpression(t *testing.T) {
	expr := &IndexExpression{
		Left:  &Identifier{Value: "arr"},
		Index: &IntegerLiteral{Value: 0},
	}

	if expr.Left == nil || expr.Index == nil {
		t.Error("Index expression parts should not be nil")
	}
}

func TestSliceExpression(t *testing.T) {
	expr := &SliceExpression{
		Left:  &Identifier{Value: "arr"},
		Start: &IntegerLiteral{Value: 0},
		End:   &IntegerLiteral{Value: 10},
		Step:  nil,
	}

	if expr.Left == nil || expr.Start == nil || expr.End == nil {
		t.Error("Slice expression parts should not be nil")
	}
}

func TestTryStatement(t *testing.T) {
	stmt := &TryStatement{
		Body:      &BlockStatement{},
		ExceptClauses: []*ExceptClause{
			{
				Body:      &BlockStatement{},
				ExceptVar: &Identifier{Value: "e"},
			},
		},
		Finally:   &BlockStatement{},
	}

	if stmt.Body == nil || len(stmt.ExceptClauses) == 0 || stmt.Finally == nil {
		t.Error("Try statement parts should not be nil")
	}
}

func TestRaiseStatement(t *testing.T) {
	stmt := &RaiseStatement{
		Message: &StringLiteral{Value: "error occurred"},
	}

	if stmt.Message == nil {
		t.Error("Raise statement message should not be nil")
	}
}

func TestGlobalStatement(t *testing.T) {
	stmt := &GlobalStatement{
		Names: []*Identifier{
			{Value: "x"},
			{Value: "y"},
		},
	}

	if len(stmt.Names) != 2 {
		t.Errorf("stmt.Names length = %d, want 2", len(stmt.Names))
	}
}

func TestNonlocalStatement(t *testing.T) {
	stmt := &NonlocalStatement{
		Names: []*Identifier{
			{Value: "count"},
		},
	}

	if len(stmt.Names) != 1 {
		t.Errorf("stmt.Names length = %d, want 1", len(stmt.Names))
	}
}

func TestAssertStatement(t *testing.T) {
	stmt := &AssertStatement{
		Condition: &Boolean{Value: true},
		Message:   &StringLiteral{Value: "assertion failed"},
	}

	if stmt.Condition == nil || stmt.Message == nil {
		t.Error("Assert statement parts should not be nil")
	}
}

func TestListComprehension(t *testing.T) {
	lc := &ListComprehension{
		Expression: &Identifier{Value: "x"},
		Variables:  []Expression{&Identifier{Value: "x"}},
		Iterable:   &Identifier{Value: "items"},
		Condition:  &Boolean{Value: true},
	}

	if lc.Expression == nil || lc.Iterable == nil {
		t.Error("List comprehension parts should not be nil")
	}

	if len(lc.Variables) != 1 {
		t.Errorf("lc.Variables length = %d, want 1", len(lc.Variables))
	}
}

func TestLambda(t *testing.T) {
	lambda := &Lambda{
		Parameters: []*Identifier{
			{Value: "x"},
		},
		DefaultValues: map[string]Expression{},
		Variadic:      nil,
		Body:          &InfixExpression{Operator: "+"},
	}

	if len(lambda.Parameters) != 1 {
		t.Errorf("lambda.Parameters length = %d, want 1", len(lambda.Parameters))
	}

	if lambda.Body == nil {
		t.Error("Lambda body should not be nil")
	}
}

func TestTupleLiteral(t *testing.T) {
	tuple := &TupleLiteral{
		Elements: []Expression{
			&IntegerLiteral{Value: 1},
			&StringLiteral{Value: "hello"},
		},
	}

	if len(tuple.Elements) != 2 {
		t.Errorf("tuple.Elements length = %d, want 2", len(tuple.Elements))
	}
}

func TestMatchStatement(t *testing.T) {
	stmt := &MatchStatement{
		Subject: &Identifier{Value: "value"},
		Cases: []*CaseClause{
			{
				Pattern: &IntegerLiteral{Value: 1},
				Body:    &BlockStatement{},
			},
			{
				Pattern: &IntegerLiteral{Value: 2},
				Body:    &BlockStatement{},
			},
		},
	}

	if stmt.Subject == nil {
		t.Error("Match statement subject should not be nil")
	}

	if len(stmt.Cases) != 2 {
		t.Errorf("stmt.Cases length = %d, want 2", len(stmt.Cases))
	}
}

func TestCaseClause(t *testing.T) {
	clause := &CaseClause{
		Pattern:   &IntegerLiteral{Value: 1},
		Guard:     &Boolean{Value: true},
		Body:      &BlockStatement{},
		CaptureAs: &Identifier{Value: "x"},
	}

	if clause.Pattern == nil || clause.Body == nil {
		t.Error("Case clause pattern and body should not be nil")
	}
}

func TestNodeMethods(t *testing.T) {
	// Test TokenLiteral and Line methods on various node types
	tests := []struct {
		name     string
		node     Node
		expected string
	}{
		{
			name:     "Program",
			node:     &Program{Statements: []Statement{&ExpressionStatement{}}},
			expected: "",
		},
		{
			name:     "Identifier",
			node:     &Identifier{Value: "x"},
			expected: "",
		},
		{
			name:     "IntegerLiteral",
			node:     &IntegerLiteral{Value: 42},
			expected: "",
		},
		{
			name:     "StringLiteral",
			node:     &StringLiteral{Value: "hello"},
			expected: "",
		},
		{
			name:     "Boolean",
			node:     &Boolean{Value: true},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify methods exist and return something
			literal := tt.node.TokenLiteral()
			line := tt.node.Line()
			_ = literal
			_ = line
		})
	}
}

func TestExpressionString(t *testing.T) {
	// Test String() method on expressions
	tests := []struct {
		name     string
		expr     Expression
		contains string
	}{
		{
			name:     "Identifier",
			expr:     &Identifier{Value: "myVar"},
			contains: "",
		},
		{
			name:     "IntegerLiteral",
			expr:     &IntegerLiteral{Value: 42},
			contains: "",
		},
		{
			name:     "FloatLiteral",
			expr:     &FloatLiteral{Value: 3.14},
			contains: "",
		},
		{
			name:     "StringLiteral",
			expr:     &StringLiteral{Value: "hello"},
			contains: "",
		},
		{
			name:     "Boolean",
			expr:     &Boolean{Value: true},
			contains: "",
		},
		{
			name:     "None",
			expr:     &None{},
			contains: "",
		},
		{
			name:     "PrefixExpression",
			expr:     &PrefixExpression{Operator: "-", Right: &IntegerLiteral{Value: 5}},
			contains: "",
		},
		{
			name:     "InfixExpression",
			expr:     &InfixExpression{Operator: "+", Left: &IntegerLiteral{Value: 1}, Right: &IntegerLiteral{Value: 2}},
			contains: "",
		},
		{
			name:     "ConditionalExpression",
			expr:     &ConditionalExpression{Condition: &Boolean{}, TrueExpr: &IntegerLiteral{Value: 1}, FalseExpr: &IntegerLiteral{Value: 0}},
			contains: "",
		},
		{
			name:     "ListLiteral",
			expr:     &ListLiteral{Elements: []Expression{&IntegerLiteral{Value: 1}}},
			contains: "",
		},
		{
			name:     "DictLiteral",
			expr:     &DictLiteral{Pairs: map[Expression]Expression{&StringLiteral{Value: "key"}: &IntegerLiteral{Value: 1}}},
			contains: "",
		},
		{
			name:     "IndexExpression",
			expr:     &IndexExpression{Left: &Identifier{Value: "arr"}, Index: &IntegerLiteral{Value: 0}},
			contains: "",
		},
		{
			name:     "SliceExpression",
			expr:     &SliceExpression{Left: &Identifier{Value: "arr"}, Start: &IntegerLiteral{Value: 0}, End: &IntegerLiteral{Value: 10}},
			contains: "",
		},
		{
			name:     "CallExpression",
			expr:     &CallExpression{Function: &Identifier{Value: "print"}},
			contains: "",
		},
		{
			name:     "MethodCallExpression",
			expr:     &MethodCallExpression{Object: &Identifier{Value: "obj"}, Method: &Identifier{Value: "method"}},
			contains: "",
		},
		{
			name:     "Lambda",
			expr:     &Lambda{Parameters: []*Identifier{{Value: "x"}}, Body: &Identifier{Value: "x"}},
			contains: "",
		},
		{
			name:     "ListComprehension",
			expr:     &ListComprehension{Expression: &Identifier{Value: "x"}, Variables: []Expression{&Identifier{Value: "x"}}, Iterable: &Identifier{Value: "items"}},
			contains: "",
		},
		{
			name:     "TupleLiteral",
			expr:     &TupleLiteral{Elements: []Expression{&IntegerLiteral{Value: 1}}},
			contains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify TokenLiteral and Line methods exist and return something
			literal := tt.expr.TokenLiteral()
			line := tt.expr.Line()
			_ = literal
			_ = line
		})
	}
}

func TestStatementString(t *testing.T) {
	// Test TokenLiteral method on statements
	tests := []struct {
		name string
		stmt Statement
	}{
		{name: "AssignStatement", stmt: &AssignStatement{Left: &Identifier{Value: "x"}, Value: &IntegerLiteral{Value: 1}}},
		{name: "AugmentedAssignStatement", stmt: &AugmentedAssignStatement{Name: &Identifier{Value: "x"}, Operator: "+=", Value: &IntegerLiteral{Value: 1}}},
		{name: "MultipleAssignStatement", stmt: &MultipleAssignStatement{Names: []*Identifier{{Value: "x"}}, Value: &ListLiteral{}}},
		{name: "ExpressionStatement", stmt: &ExpressionStatement{Expression: &IntegerLiteral{Value: 42}}},
		{name: "BlockStatement", stmt: &BlockStatement{Statements: []Statement{}}},
		{name: "IfStatement", stmt: &IfStatement{Condition: &Boolean{}, Consequence: &BlockStatement{}}},
		{name: "WhileStatement", stmt: &WhileStatement{Condition: &Boolean{}, Body: &BlockStatement{}}},
		{name: "ForStatement", stmt: &ForStatement{Variables: []Expression{&Identifier{Value: "x"}}, Iterable: &Identifier{Value: "items"}, Body: &BlockStatement{}}},
		{name: "FunctionStatement", stmt: &FunctionStatement{Name: &Identifier{Value: "foo"}, Function: &FunctionLiteral{}}},
		{name: "ClassStatement", stmt: &ClassStatement{Name: &Identifier{Value: "Foo"}, Body: &BlockStatement{}}},
		{name: "ReturnStatement", stmt: &ReturnStatement{ReturnValue: &IntegerLiteral{Value: 42}}},
		{name: "BreakStatement", stmt: &BreakStatement{}},
		{name: "ContinueStatement", stmt: &ContinueStatement{}},
		{name: "PassStatement", stmt: &PassStatement{}},
		{name: "ImportStatement", stmt: &ImportStatement{Name: &Identifier{Value: "os"}}},
		{name: "FromImportStatement", stmt: &FromImportStatement{Module: &Identifier{Value: "os"}, Names: []*Identifier{{Value: "path"}}}},
		{name: "TryStatement", stmt: &TryStatement{Body: &BlockStatement{}, ExceptClauses: []*ExceptClause{{Body: &BlockStatement{}}}}},
		{name: "RaiseStatement", stmt: &RaiseStatement{Message: &StringLiteral{Value: "error"}}},
		{name: "GlobalStatement", stmt: &GlobalStatement{Names: []*Identifier{{Value: "x"}}}},
		{name: "NonlocalStatement", stmt: &NonlocalStatement{Names: []*Identifier{{Value: "y"}}}},
		{name: "AssertStatement", stmt: &AssertStatement{Condition: &Boolean{}}},
		{name: "MatchStatement", stmt: &MatchStatement{Subject: &Identifier{Value: "x"}, Cases: []*CaseClause{{Pattern: &IntegerLiteral{Value: 1}, Body: &BlockStatement{}}}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify TokenLiteral and Line methods exist and return something
			literal := tt.stmt.TokenLiteral()
			line := tt.stmt.Line()
			_ = literal
			_ = line
		})
	}
}

func TestProgramTokenLiteral(t *testing.T) {
	program := &Program{
		Statements: []Statement{
			&AssignStatement{Left: &Identifier{Value: "x"}, Value: &IntegerLiteral{Value: 1}},
			&AssignStatement{Left: &Identifier{Value: "y"}, Value: &IntegerLiteral{Value: 2}},
		},
	}

	// Verify TokenLiteral and Line methods exist and return something
	literal := program.TokenLiteral()
	line := program.Line()
	_ = literal
	_ = line
}

func TestExpressionTokenLiteral(t *testing.T) {
	// Test TokenLiteral method on expressions
	tests := []struct {
		name string
		expr Expression
	}{
		{name: "Identifier", expr: &Identifier{}},
		{name: "IntegerLiteral", expr: &IntegerLiteral{}},
		{name: "FloatLiteral", expr: &FloatLiteral{}},
		{name: "StringLiteral", expr: &StringLiteral{}},
		{name: "Boolean", expr: &Boolean{}},
		{name: "None", expr: &None{}},
		{name: "PrefixExpression", expr: &PrefixExpression{}},
		{name: "InfixExpression", expr: &InfixExpression{}},
		{name: "ConditionalExpression", expr: &ConditionalExpression{}},
		{name: "ListLiteral", expr: &ListLiteral{}},
		{name: "DictLiteral", expr: &DictLiteral{}},
		{name: "IndexExpression", expr: &IndexExpression{}},
		{name: "SliceExpression", expr: &SliceExpression{}},
		{name: "CallExpression", expr: &CallExpression{}},
		{name: "MethodCallExpression", expr: &MethodCallExpression{}},
		{name: "Lambda", expr: &Lambda{}},
		{name: "ListComprehension", expr: &ListComprehension{}},
		{name: "TupleLiteral", expr: &TupleLiteral{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify TokenLiteral and Line methods exist and return something
			literal := tt.expr.TokenLiteral()
			line := tt.expr.Line()
			_ = literal
			_ = line
		})
	}
}

func TestStatementTokenLiteral(t *testing.T) {
	// Test TokenLiteral method on statements
	tests := []struct {
		name string
		stmt Statement
	}{
		{name: "AssignStatement", stmt: &AssignStatement{}},
		{name: "AugmentedAssignStatement", stmt: &AugmentedAssignStatement{}},
		{name: "MultipleAssignStatement", stmt: &MultipleAssignStatement{}},
		{name: "ExpressionStatement", stmt: &ExpressionStatement{}},
		{name: "BlockStatement", stmt: &BlockStatement{}},
		{name: "IfStatement", stmt: &IfStatement{}},
		{name: "WhileStatement", stmt: &WhileStatement{}},
		{name: "ForStatement", stmt: &ForStatement{}},
		{name: "FunctionStatement", stmt: &FunctionStatement{}},
		{name: "ClassStatement", stmt: &ClassStatement{}},
		{name: "ReturnStatement", stmt: &ReturnStatement{}},
		{name: "BreakStatement", stmt: &BreakStatement{}},
		{name: "ContinueStatement", stmt: &ContinueStatement{}},
		{name: "PassStatement", stmt: &PassStatement{}},
		{name: "ImportStatement", stmt: &ImportStatement{}},
		{name: "FromImportStatement", stmt: &FromImportStatement{}},
		{name: "TryStatement", stmt: &TryStatement{}},
		{name: "RaiseStatement", stmt: &RaiseStatement{}},
		{name: "GlobalStatement", stmt: &GlobalStatement{}},
		{name: "NonlocalStatement", stmt: &NonlocalStatement{}},
		{name: "AssertStatement", stmt: &AssertStatement{}},
		{name: "MatchStatement", stmt: &MatchStatement{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify TokenLiteral and Line methods exist and return something
			literal := tt.stmt.TokenLiteral()
			line := tt.stmt.Line()
			_ = literal
			_ = line
		})
	}
}
