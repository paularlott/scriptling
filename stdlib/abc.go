package stdlib

import (
	"context"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
)

// The abc module: ABC and abstractmethod. A class deriving from ABC cannot be
// instantiated while any method marked @abstractmethod, its own or inherited,
// is not overridden; the evaluator computes the list when the class statement
// runs (object.Class.AbstractMethods), so ordinary classes pay nothing.
// Classes without ABC as an ancestor ignore @abstractmethod, as in Python.

// ABCClass is abc.ABC, the base class that enables abstract-method checks.
var ABCClass = &object.Class{
	Name:    "ABC",
	Methods: map[string]object.Object{},
	IsABC:   true,
}

// markAbstract sets the abstract flag on the function inside fn (a def, or
// one wrapped by property, staticmethod or classmethod).
func markAbstract(fn object.Object) bool {
	switch f := fn.(type) {
	case *object.Function:
		f.Abstract = true
		return true
	case *object.Property:
		return markAbstract(f.Getter)
	case *object.StaticMethod:
		return markAbstract(f.Fn)
	case *object.ClassMethod:
		return markAbstract(f.Fn)
	}
	return false
}

var ABCLibrary = object.NewLibrary(ABCLibraryName, map[string]*object.Builtin{
	"abstractmethod": {
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if err := errors.ExactArgs(args, 1); err != nil {
				return err
			}
			if !markAbstract(args[0]) {
				return errors.NewTypeErrorTagged("abstractmethod() requires a function defined with def")
			}
			return args[0]
		},
		HelpText: `abstractmethod(function) - Decorator marking a method abstract

A class deriving from ABC cannot be instantiated until every abstract method
is overridden:

  from abc import ABC, abstractmethod

  class Shape(ABC):
      @abstractmethod
      def area(self): ...

  Shape()          # TypeError: Can't instantiate abstract class Shape ...

Stack it under @property, @staticmethod or @classmethod.`,
	},
}, map[string]object.Object{
	"ABC": ABCClass,
}, "Abstract base classes: ABC and abstractmethod")
