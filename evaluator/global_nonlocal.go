package evaluator

import (
	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/object"
)

func evalGlobalStatement(gs *ast.GlobalStatement, env *object.Environment) object.Object {
	for _, name := range gs.Names {
		env.MarkGlobal(name.Value)
	}
	return NULL
}

func evalNonlocalStatement(ns *ast.NonlocalStatement, env *object.Environment) object.Object {
	for _, name := range ns.Names {
		env.MarkNonlocal(name.Value)
	}
	return NULL
}
