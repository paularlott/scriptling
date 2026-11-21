package evaluator

import (
	"context"
	"strings"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/object"
)

// Introspection attributes of functions, bound methods and classes:
// __name__, __qualname__, __doc__ and the like.

// docstringOf returns the docstring of a body (its first statement when that
// is a string literal), cleaned as Python 3.13+ does, or nil.
func docstringOf(body *ast.BlockStatement) object.Object {
	if body == nil || len(body.Statements) == 0 {
		return nil
	}
	es, ok := body.Statements[0].(*ast.ExpressionStatement)
	if !ok {
		return nil
	}
	lit, ok := es.Expression.(*ast.StringLiteral)
	if !ok {
		return nil
	}
	return object.NewString(cleanDocstring(lit.Value))
}

// cleanDocstring strips the first line's leading whitespace and the common
// indentation of the lines after it, as the Python 3.13+ compiler does.
func cleanDocstring(s string) string {
	if !strings.Contains(s, "\n") {
		return strings.TrimLeft(s, " \t")
	}
	lines := strings.Split(s, "\n")
	lines[0] = strings.TrimLeft(lines[0], " \t")
	indent := -1
	for _, l := range lines[1:] {
		t := strings.TrimLeft(l, " \t")
		if t == "" {
			continue
		}
		if n := len(l) - len(t); indent < 0 || n < indent {
			indent = n
		}
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimLeft(lines[i], " \t") == "" {
			lines[i] = ""
		} else if indent > 0 {
			lines[i] = lines[i][indent:]
		}
	}
	return strings.Join(lines, "\n")
}

// functionAttribute is attr of a def function, a lambda or a bound method.
func functionAttribute(ctx context.Context, fnObj object.Object, attr string) object.Object {
	switch fn := fnObj.(type) {
	case *object.Function:
		switch attr {
		case "__name__", "name", "__qualname__":
			return object.NewString(fn.Name)
		case "__doc__":
			if w := fn.Wrapped(); w != nil {
				return wrappedDoc(w)
			}
			if doc := docstringOf(fn.Body); doc != nil {
				return doc
			}
			return NULL
		case "__wrapped__":
			if w := fn.Wrapped(); w != nil {
				return w
			}
		case "__module__":
			return object.NewString("__main__")
		}
	case *object.LambdaFunction:
		switch attr {
		case "__name__", "name", "__qualname__":
			return object.NewString("<lambda>")
		case "__doc__":
			return NULL
		case "__module__":
			return object.NewString("__main__")
		}
	case *object.BoundMethod:
		switch attr {
		case "__self__":
			return fn.Instance
		case "__func__":
			return fn.Method
		}
		return evalIndexExpression(ctx, fn.Method, object.NewString(attr), true)
	}
	return attributeError(fnObj, attr)
}

// wrappedDoc is the __doc__ that functools.wraps copies from wrapped.
func wrappedDoc(wrapped object.Object) object.Object {
	switch w := wrapped.(type) {
	case *object.Function, *object.LambdaFunction, *object.BoundMethod:
		return functionAttribute(context.Background(), w, "__doc__")
	case *object.Builtin:
		if w.Attributes != nil {
			if d, ok := w.Attributes["__doc__"]; ok {
				return d
			}
		}
		if w.HelpText != "" {
			return object.NewString(w.HelpText)
		}
	case *object.Class:
		if d, ok := w.Methods["__doc__"]; ok {
			return d
		}
	}
	return NULL
}
