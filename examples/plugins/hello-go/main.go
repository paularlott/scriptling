package main

import (
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/plugin"
)

func main() {
	server := plugin.NewServer("hello", "1.0.0", "Hello plugin example")

	greetBuilder := object.NewFunctionBuilder()
	greetBuilder.Function(func(name string) string {
		return "Hello, " + name
	})
	server.RegisterFunc("greet", greetBuilder)

	labelBuilder := object.NewFunctionBuilder()
	labelBuilder.Function(func(name string) string {
		return "built:" + name
	})
	server.RegisterFunc("label", labelBuilder)

	configBuilder := object.NewClassBuilder("Config").
		Method("__init__", func(self *object.Instance, name string) {
			self.SetField("name", object.NewString(name))
		}).
		Method("get", func(self *object.Instance, key string) string {
			return self.Field("name").(*object.String).StringValue()
		})
	server.RegisterClass(configBuilder)

	counter := object.NewClassBuilder("Counter").
		Method("__init__", func(self *object.Instance, start int) {
			self.SetField("value", object.NewInteger(int64(start)))
		}).
		Method("inc", func(self *object.Instance, amount int) int {
			current := self.Field("value").(*object.Integer).IntValue()
			next := current + int64(amount)
			self.SetField("value", object.NewInteger(next))
			return int(next)
		}).
		Method("get", func(self *object.Instance) int {
			return int(self.Field("value").(*object.Integer).IntValue())
		})
	server.RegisterClass(counter)

	server.Constant("default_name", "World")

	if err := server.Run(); err != nil {
		panic(err)
	}
}
