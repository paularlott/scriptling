package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/plugin"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8081", "HTTP listen address")
	path := flag.String("path", "/json-rpc", "JSON-RPC endpoint path")
	flag.Parse()

	server := plugin.NewServer("hello_http", "1.0.0", "HTTP Scriptling plugin example")

	greet := object.NewFunctionBuilder()
	greet.Function(func(name string) string {
		return "Hello, " + name
	})
	server.RegisterFunc("greet", greet)

	counter := object.NewClassBuilder("Counter").
		Method("__init__", func(self *object.Instance, start int) {
			self.SetField("value", object.NewInteger(int64(start)))
		}).
		Method("inc", func(self *object.Instance, amount int) int {
			current := self.Field("value").(*object.Integer).IntValue()
			next := current + int64(amount)
			self.SetField("value", object.NewInteger(next))
			return int(next)
		})
	server.RegisterClass(counter)

	server.Constant("default_name", "World")

	mux := http.NewServeMux()
	mux.Handle(*path, server)
	log.Printf("HTTP Scriptling plugin listening at http://%s%s", *addr, *path)
	log.Fatal(http.ListenAndServe(*addr, mux))
}
