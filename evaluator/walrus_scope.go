package evaluator

import (
	"slices"

	"github.com/paularlott/scriptling/ast"
)

// comprehensionWalrusTargets returns the names that assignment expressions
// (name := v) bind inside a comprehension's expressions, nested
// comprehensions included. Python binds them in the scope enclosing the
// comprehension, not the comprehension's own. A lambda has its own scope, so
// one inside a lambda is not counted. It reads the AST only, once, when the
// comprehension is compiled.
func comprehensionWalrusTargets(clauses []ast.ComprehensionClause, exprs ...ast.Expression) []string {
	var names []string
	for _, e := range exprs {
		collectWalrusTargets(e, &names)
	}
	walkClauses(clauses, &names)
	return names
}

func collectWalrusTargets(e ast.Expression, names *[]string) {
	walk := func(es ...ast.Expression) {
		for _, x := range es {
			collectWalrusTargets(x, names)
		}
	}
	switch n := e.(type) {
	case nil:
	case *ast.WalrusExpression:
		if n.Target != nil && !slices.Contains(*names, n.Target.Value()) {
			*names = append(*names, n.Target.Value())
		}
		walk(n.Value)
	case *ast.PrefixExpression:
		walk(n.Right)
	case *ast.InfixExpression:
		walk(n.Left, n.Right)
	case *ast.ChainedComparison:
		walk(n.First)
		for _, l := range n.Links {
			walk(l.Operand)
		}
	case *ast.ConditionalExpression:
		walk(n.TrueExpr, n.Condition, n.FalseExpr)
	case *ast.CallExpression:
		walk(n.Function)
		walk(n.Arguments...)
		for _, v := range n.GetKeywords() {
			walk(v)
		}
		walk(n.GetArgsUnpack()...)
		walk(n.GetKwargsUnpack())
	case *ast.MethodCallExpression:
		walk(n.Receiver)
		walk(n.Arguments...)
		for _, v := range n.GetKeywords() {
			walk(v)
		}
	case *ast.IndexExpression:
		walk(n.Left, n.Index)
	case *ast.SliceExpression:
		walk(n.Left, n.Start, n.End, n.GetStep())
	case *ast.StarredElement:
		walk(n.Value)
	case *ast.ListLiteral:
		walk(n.Elements...)
	case *ast.TupleLiteral:
		walk(n.Elements...)
	case *ast.SetLiteral:
		walk(n.Elements...)
	case *ast.DictLiteral:
		for _, p := range n.Pairs {
			walk(p.Key, p.Value)
		}
	case *ast.FStringLiteral:
		walk(n.Expressions...)
	case *ast.ListComprehension:
		walk(n.Expression, n.Iterable, n.Condition)
		walkClauses(n.AdditionalClauses, names)
	case *ast.SetComprehension:
		walk(n.Expression, n.Iterable, n.Condition)
		walkClauses(n.AdditionalClauses, names)
	case *ast.DictComprehension:
		walk(n.Key, n.Value, n.Iterable, n.Condition)
		walkClauses(n.AdditionalClauses, names)
	}
}

func walkClauses(clauses []ast.ComprehensionClause, names *[]string) {
	for _, cl := range clauses {
		collectWalrusTargets(cl.Iterable, names)
		collectWalrusTargets(cl.Condition, names)
	}
}
