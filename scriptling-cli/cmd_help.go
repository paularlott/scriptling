package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/paularlott/cli"
	"github.com/paularlott/cli/tui"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/secretprovider"
	"github.com/paularlott/scriptling/scriptling-cli/bootstrap"
	"github.com/paularlott/scriptling/scriptling-cli/setup"
)

func helpCmd() *cli.Command {
	return &cli.Command{
		Name:  "help",
		Usage: "Show help for a module or function",
		Arguments: []cli.Argument{
			&cli.StringArg{
				Name:     "topic",
				Usage:    "Module or module.function to show help for (e.g. json or json.loads)",
				Required: false,
			},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			allowedPaths := bootstrap.ParseAllowedPaths(cmd.GetString("allowed-paths"))
			disabledLibs := cmd.GetStringSlice("disable-lib")
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to determine current working directory: %w", err)
			}
			libDirs := bootstrap.BuildLibDirs(cwd, cmd.GetStringSlice("libpath"))
			secretRegistry := secretprovider.NewRegistry()

			p := scriptling.New()
			setup.Scriptling(p, libDirs, false, allowedPaths, disabledLibs, secretRegistry, globalLogger)

			packages := cmd.GetStringSlice("package")
			if len(packages) > 0 {
				l, err := bootstrap.NewPackLoader(packages, cmd.GetBool("insecure"), cmd.GetString("cache-dir"))
				if err != nil {
					return err
				}
				bootstrap.ApplyPackLoader(p, l)
			}

			topic := cmd.GetStringArg("topic")
			if topic != "" {
				return lookupHelp(p, topic)
			}
			return runHelpTUI(ctx, p)
		},
	}
}

// lookupHelp imports the relevant module and prints help for topic.
func lookupHelp(p *scriptling.Scriptling, topic string) error {
	for t := topic; t != ""; {
		if err := p.Import(t); err == nil {
			break
		}
		if i := strings.LastIndex(t, "."); i >= 0 {
			t = t[:i]
		} else {
			break
		}
	}
	_, err := p.Eval(fmt.Sprintf("help(%q)", topic))
	return err
}

func runHelpTUI(ctx context.Context, p *scriptling.Scriptling) error {
	var t *tui.TUI

	t = tui.New(tui.Config{
		HideHeaders: true,
		StatusLeft:  "help",
		StatusRight: "Ctrl+C exit",
		OnSubmit: func(topic string) {
			topic = strings.TrimSpace(topic)
			if topic == "" {
				return
			}
			t.AddMessage(tui.RoleUser, topic)
			var buf strings.Builder
			p.SetOutputWriter(&helpWriter{b: &buf})
			_ = lookupHelp(p, topic)
			p.SetOutputWriter(nil)
			output := strings.TrimSpace(buf.String())
			if output == "" {
				output = "No help found for " + topic
			}
			t.AddMessage(tui.RoleAssistant, output)
		},
	})

	t.AddMessage(tui.RoleSystem, "Enter a module or function name to look up help, e.g. json or json.loads")
	return t.Run(ctx)
}

type helpWriter struct{ b *strings.Builder }

func (w *helpWriter) Write(p []byte) (int, error) {
	w.b.Write(p)
	return len(p), nil
}
