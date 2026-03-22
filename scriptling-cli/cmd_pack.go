package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/paularlott/cli"
	"github.com/paularlott/scriptling/scriptling-cli/pack"
)

func packCmd() *cli.Command {
	return &cli.Command{
		Name:  "pack",
		Usage: "Pack a directory into a package, or manage packages",
		Commands: []*cli.Command{
			manifestCmd(),
			docsCmd(),
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "output",
				Usage:    "Output package path",
				Aliases:  []string{"o"},
				Required: false,
			},
			&cli.BoolFlag{
				Name:    "force",
				Usage:   "Overwrite existing package",
				Aliases: []string{"f"},
			},
			&cli.BoolFlag{
				Name:    "hash",
				Usage:   "Print the sha256 hash of an existing package file",
				Aliases: []string{"H"},
			},
		},
		Arguments: []cli.Argument{
			&cli.StringArg{
				Name:     "dir",
				Usage:    "Source directory to pack, or package file when using --hash",
				Required: true,
			},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			if cmd.GetBool("hash") {
				data, err := readFile(cmd.GetStringArg("dir"))
				if err != nil {
					return fmt.Errorf("failed to read file: %w", err)
				}
				fmt.Printf("sha256=%s\n", pack.HashBytes(data))
				return nil
			}
			output := cmd.GetString("output")
			if output == "" {
				return fmt.Errorf("--output is required when packing")
			}
			hash, err := pack.Pack(cmd.GetStringArg("dir"), output, cmd.GetBool("force"))
			if err != nil {
				return err
			}
			fmt.Printf("sha256=%s\n", hash)
			return nil
		},
	}
}

func unpackCmd() *cli.Command {
	return &cli.Command{
		Name:  "unpack",
		Usage: "Unpack a package to a directory",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:         "dir",
				Usage:        "Destination directory",
				Aliases:      []string{"d"},
				DefaultValue: ".",
			},
			&cli.BoolFlag{
				Name:    "force",
				Usage:   "Overwrite existing files",
				Aliases: []string{"f"},
			},
			&cli.BoolFlag{
				Name:    "remove",
				Usage:   "Remove previously unpacked files instead of extracting",
				Aliases: []string{"r"},
			},
			&cli.BoolFlag{
				Name:  "list",
				Usage: "List contents only, don't extract",
			},
			&cli.BoolFlag{
				Name:    "insecure",
				Usage:   "Allow self-signed/insecure HTTPS certificates",
				Aliases: []string{"k"},
			},
		},
		Arguments: []cli.Argument{
			&cli.StringArg{
				Name:     "src",
				Usage:    "Package path or URL",
				Required: true,
			},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			if cmd.GetBool("remove") {
				return pack.UnpackRemove(cmd.GetStringArg("src"), cmd.GetBool("insecure"), cmd.GetString("dir"))
			}
			return pack.Unpack(cmd.GetStringArg("src"), pack.UnpackOptions{
				DestDir:  cmd.GetString("dir"),
				Force:    cmd.GetBool("force"),
				List:     cmd.GetBool("list"),
				Insecure: cmd.GetBool("insecure"),
			})
		},
	}
}

func manifestCmd() *cli.Command {
	return &cli.Command{
		Name:  "manifest",
		Usage: "Show manifest from a package or source directory",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "json",
				Usage: "Output as JSON",
			},
			&cli.BoolFlag{
				Name:    "insecure",
				Usage:   "Allow self-signed/insecure HTTPS certificates",
				Aliases: []string{"k"},
			},
		},
		Arguments: []cli.Argument{
			&cli.StringArg{
				Name:     "src",
				Usage:    "Package path, URL, or source directory",
				Required: true,
			},
		},
		Run: func(ctx context.Context, cmd *cli.Command) error {
			src := cmd.GetStringArg("src")
			insecure := cmd.GetBool("insecure")

			var manifest pack.Manifest
			if pack.IsURL(src) || strings.HasSuffix(src, pack.Extension) {
				data, err := pack.Fetch(src, insecure)
				if err != nil {
					return err
				}
				p, err := pack.Open(bytesReaderAt(data), int64(len(data)))
				if err != nil {
					return err
				}
				manifest = p.Manifest
			} else {
				m, err := pack.ReadManifestFromDir(src)
				if err != nil {
					return err
				}
				manifest = m
			}

			if cmd.GetBool("json") {
				out, err := json.MarshalIndent(manifest, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(out))
				return nil
			}

			fmt.Printf("Name:        %s\n", manifest.Name)
			fmt.Printf("Version:     %s\n", manifest.Version)
			if manifest.Description != "" {
				fmt.Printf("Description: %s\n", manifest.Description)
			}
			if manifest.Main != "" {
				fmt.Printf("Main:        %s\n", manifest.Main)
			}
			return nil
		},
	}
}

func cacheCmd() *cli.Command {
	return &cli.Command{
		Name:  "cache",
		Usage: "Manage the package download cache",
		Commands: []*cli.Command{
			{
				Name:  "clear",
				Usage: "Remove all cached remote packages",
				Run: func(ctx context.Context, cmd *cli.Command) error {
					cacheDir := cmd.GetString("cache-dir")
					if err := pack.ClearCache(cacheDir); err != nil {
						return err
					}
					if cacheDir == "" {
						cacheDir, _ = pack.DefaultCacheDir()
					}
					fmt.Printf("Cache cleared: %s\n", cacheDir)
					return nil
				},
			},
		},
	}
}

// bytesReaderAt wraps a byte slice as an io.ReaderAt for use with pack.Open.
type bytesReaderAt []byte

func (b bytesReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off >= int64(len(b)) {
		return 0, nil
	}
	return copy(p, b[off:]), nil
}
