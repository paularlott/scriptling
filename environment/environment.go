package environment

import "github.com/paularlott/scriptling/object"

type Environment = object.Environment

func New() *Environment {
	return object.NewEnvironment()
}

func NewEnclosed(outer *Environment) *Environment {
	return object.NewEnclosedEnvironment(outer)
}
