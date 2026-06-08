package main

import (
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/plugin"
)

type configData struct {
	name   string
	values map[string]string
}

func main() {
	generatedBuilder := object.NewFunctionBuilder()
	generatedBuilder.Function(func(name string) string {
		return "Generated, " + name
	})

	greetBuilder := object.NewFunctionBuilder()
	greetBuilder.Function(func(name string) string {
		return "Hello, " + name
	})

	server := plugin.NewServer("wrap", "1.0.0", "Wrapper example")
	server.RegisterFunc("generated", generatedBuilder)
	server.RegisterFunc("greet", greetBuilder)
	server.Wrapper("greet", `
import scriptling.plugin

def greet(name):
    return scriptling.plugin.call_function("plugin.wrap", "greet", name) + "!"
`)

	settingsBuilder := object.NewClassBuilder("Settings").
		Constructor(func(name string) *configData {
			return &configData{name: name, values: map[string]string{}}
		}).
		Method("name", func(self *configData) string {
			return self.name
		}).
		Method("set", func(self *configData, key string, value string) {
			self.values[key] = value
		}).
		Method("get", func(self *configData, key string) string {
			return self.values[key]
		})
	server.RegisterClass(settingsBuilder)

	configBuilder := object.NewClassBuilder("Config").
		Constructor(func(name string) *configData {
			return &configData{name: name, values: map[string]string{}}
		}).
		Method("set", func(self *configData, key string, value string) {
			self.values[key] = value
		}).
		Method("get", func(self *configData, key string) string {
			return self.values[key]
		})
	server.RegisterClass(configBuilder)
	server.Wrapper("Config", `
import scriptling.plugin

class Config:
    def __init__(self, name):
        self._plugin_remote = scriptling.plugin._new_object("plugin.wrap", "Config", name)

    def set(self, key, value):
        scriptling.plugin.call_method(self._plugin_remote, "set", key, value)

    def get(self, key, default=""):
        value = scriptling.plugin.call_method(self._plugin_remote, "get", key)
        if value == "":
            return default
        return value

    def __del__(self):
        scriptling.plugin.release(self._plugin_remote)
`)

	if err := server.Run(); err != nil {
		panic(err)
	}
}
