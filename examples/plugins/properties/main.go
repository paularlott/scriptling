package main

import (
	"fmt"

	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/plugin"
)

type counter struct {
	value int
}

func main() {
	counterClass := object.NewClassBuilder("Counter").
		Constructor(func(start int) *counter {
			return &counter{value: start}
		}).
		PropertyWithSetter("value",
			func(self *counter) int {
				return self.value
			},
			func(self *counter, value int) {
				self.value = value
			},
		).
		Property("label", func(self *counter) string {
			return fmt.Sprintf("counter:%d", self.value)
		}).
		Method("add", func(self *counter, amount int) int {
			self.value += amount
			return self.value
		})

	server := plugin.NewServer("properties", "1.0.0", "Property example plugin")
	server.RegisterClass(counterClass)

	if err := server.Run(); err != nil {
		panic(err)
	}
}
