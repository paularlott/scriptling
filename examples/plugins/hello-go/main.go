package main

import "github.com/paularlott/scriptling/plugin"

type Config struct {
	values map[string]any
}

func main() {
	server := plugin.NewServer("hello", "1.0.0", "Hello plugin example")

	server.Function("greet", func(name string) string {
		return "Hello, " + name
	})

	server.Class("Config").
		Constructor(func(values map[string]any) *Config {
			return &Config{values: values}
		}).
		Method("get", func(c *Config, key string) any {
			return c.values[key]
		}).
		Destructor(func(c *Config) {
			c.values = nil
		})

	if err := server.Run(); err != nil {
		panic(err)
	}
}

