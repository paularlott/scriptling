package main

import (
	"context"

	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/plugin"
)

type tokenEvent struct {
	Token string `json:"token"`
	Index int    `json:"index"`
}

func main() {
	streamBuilder := object.NewFunctionBuilder()
	streamBuilder.Function(func(ctx context.Context, onEvent plugin.Callback) (string, error) {
		tokens := []string{"Hello", ", ", "Ada"}
		for i, token := range tokens {
			if _, err := onEvent.Call(ctx, tokenEvent{Token: token, Index: i}); err != nil {
				return "", err
			}
		}
		if _, err := onEvent.Call(ctx, []any{"done", len(tokens)}); err != nil {
			return "", err
		}
		return "Hello, Ada", nil
	})

	server := plugin.NewServer("callback", "1.0.0", "Callback streaming example")
	server.RegisterFunc("stream", streamBuilder)

	if err := server.Run(); err != nil {
		panic(err)
	}
}
