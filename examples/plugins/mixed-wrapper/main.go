package main

import "github.com/paularlott/scriptling/plugin"

type Config struct {
	values map[string]any
}

func main() {
	server := plugin.NewServer("mixed", "1.0.0", "Mixed generated and supplied wrappers")

	server.Function("generated", func(name string) string {
		return "generated:" + name
	})

	server.HiddenFunction("_decorate", func(name string) string {
		return "wrapped:" + name
	})

	server.Class("Config").
		Constructor(func(values map[string]any) *Config {
			return &Config{values: values}
		}).
		Method("get", func(c *Config, key string) any {
			return c.values[key]
		})

	server.Wrapper("decorate", `
import scriptling.plugin

def decorate(name):
    return scriptling.plugin.call_function("plugin.mixed", "_decorate", name) + "!"
`)

	if err := server.Run(); err != nil {
		panic(err)
	}
}
