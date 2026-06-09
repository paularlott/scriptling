package main

import (
	"context"

	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/plugin"
)

func main() {
	work := object.NewFunctionBuilder()
	work.Function(func(ctx context.Context, name string, tags []any) string {
		plugin.Logger(ctx).With("plugin", "logger").Info("plugin work started",
			"name", name,
			"tags", tags,
		)
		return "logged:" + name
	})

	server := plugin.NewServer("logger", "1.0.0", "Host logger example plugin")
	server.RegisterFunc("work", work)

	if err := server.Run(); err != nil {
		panic(err)
	}
}
