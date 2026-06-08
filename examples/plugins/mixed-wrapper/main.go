package main

import (
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/plugin"
)

func main() {
	greetBuilder := object.NewFunctionBuilder()
	greetBuilder.Function(func(name string) string {
		return "Hello, " + name
	})

	server := plugin.NewServer("wrap", "1.0.0", "Wrapper example")
	server.RegisterFunc("greet", greetBuilder)
	server.Wrapper("greet", `
import scriptling.plugin

def greet(name):
    return scriptling.plugin.call_function("plugin.wrap", "greet", name) + "!"
`)

	if err := server.Run(); err != nil {
		panic(err)
	}
}
