package main

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/paularlott/cli"
	"github.com/paularlott/cli/tui"
	"github.com/paularlott/scriptling/scriptling-cli/pack"
)

func docsCmd() *cli.Command {
	return &cli.Command{
		Name:  "docs",
		Usage: "Browse package documentation",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "insecure",
				Usage:   "Allow self-signed/insecure HTTPS certificates",
				Aliases: []string{"k"},
			},
			&cli.BoolFlag{
				Name:  "list",
				Usage: "List available docs without launching TUI",
			},
		},
		Arguments: []cli.Argument{
			&cli.StringArg{
				Name:     "src",
				Usage:    "Package path, URL, or unpacked directory",
				Required: true,
			},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			src := cmd.GetStringArg("src")
			insecure := cmd.GetBool("insecure")

			reader, err := openDocReader(src, insecure)
			if err != nil {
				return err
			}

			docs := reader.ListDocs()
			if len(docs) == 0 {
				return fmt.Errorf("no documentation found in %s", src)
			}

			if cmd.GetBool("list") {
				for _, d := range docs {
					fmt.Println(d)
				}
				return nil
			}

			return runDocsTUI(ctx, reader, docs)
		},
	}
}

// openDocReader returns the appropriate DocReader for src.
func openDocReader(src string, insecure bool) (pack.DocReader, error) {
	if pack.IsURL(src) {
		return pack.NewZipDocReader(src, insecure)
	}
	if strings.HasSuffix(src, pack.Extension) {
		return pack.NewZipDocReader(src, insecure)
	}
	return pack.NewDirDocReader(src), nil
}

func runDocsTUI(ctx context.Context, reader pack.DocReader, docs []string) error {
	var t *tui.TUI

	menu := &tui.Menu{
		Title: "Documentation",
	}

	// Build a two-level menu: folders → files
	menu.Items = buildDocMenuItems(reader, docs, func(name string) {
		showDoc(t, reader, name, menu)
	})

	t = tui.New(tui.Config{
		HideHeaders:  true,
		StatusLeft:   reader.Name(),
		StatusRight:  "Esc menu · Ctrl+C exit",
		InputEnabled: func() *bool { b := false; return &b }(),
		OnEscape: func() {
			t.OpenMenu(menu)
		},
	})

	t.OpenMenu(menu)

	return t.Run(ctx)
}

// buildDocMenuItems organises doc paths into a folder-grouped menu.
// Files at the root level appear directly; files in subdirs appear under a submenu.
func buildDocMenuItems(reader pack.DocReader, docs []string, onSelect func(string)) []*tui.MenuItem {
	type folder struct {
		order int
		items []*tui.MenuItem
	}
	folders := map[string]*folder{}
	var rootItems []*tui.MenuItem
	order := 0

	for _, path := range docs {
		dir := filepath.ToSlash(filepath.Dir(path))
		label := filepath.Base(path)
		p := path // capture

		item := &tui.MenuItem{
			Label: label,
			Value: p,
			OnSelect: func(mi *tui.MenuItem, _ string) {
				onSelect(mi.Value)
			},
		}

		if dir == "." {
			rootItems = append(rootItems, item)
		} else {
			if _, ok := folders[dir]; !ok {
				folders[dir] = &folder{order: order}
				order++
			}
			folders[dir].items = append(folders[dir].items, item)
		}
	}

	// Sort folder names and append as submenus after root items
	dirNames := make([]string, 0, len(folders))
	for d := range folders {
		dirNames = append(dirNames, d)
	}
	sort.Strings(dirNames)

	for _, dir := range dirNames {
		f := folders[dir]
		rootItems = append(rootItems, &tui.MenuItem{
			Label:    dir + "/",
			Children: f.items,
		})
	}

	return rootItems
}

// showDoc renders a doc file into the TUI output area.
func showDoc(t *tui.TUI, reader pack.DocReader, name string, menu *tui.Menu) {
	data, err := reader.ReadDoc(name)
	if err != nil {
		t.AddMessage(tui.RoleSystem, "Error reading "+name+": "+err.Error())
		return
	}
	t.ClearOutput()
	t.AddMessage(tui.RoleAssistant, string(data))
	t.CloseMenu()
}
