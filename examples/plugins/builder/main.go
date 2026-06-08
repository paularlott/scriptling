package main

import (
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/plugin"
)

func main() {
	server := plugin.NewServer("builderdemo", "1.0.0", "Builder API plugin example")

	fn := object.NewFunctionBuilder().
		Function(func(name string) string {
			return "built:" + name
		})
	server.RegisterFunc("label", fn.Build())

	counter := object.NewClassBuilder("Counter").
		Method("__init__", func(self *object.Instance, start int) {
			self.Fields["value"] = object.NewInteger(int64(start))
		}).
		Method("inc", func(self *object.Instance, amount int) int {
			current := self.Fields["value"].(*object.Integer).IntValue()
			next := current + int64(amount)
			self.Fields["value"] = object.NewInteger(next)
			return int(next)
		}).
		Method("get", func(self *object.Instance) int {
			return int(self.Fields["value"].(*object.Integer).IntValue())
		}).
		Build()
	server.RegisterClass(counter)

	if err := server.Run(); err != nil {
		panic(err)
	}
}
