package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
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
			&cli.BoolFlag{
				Name:    "list",
				Usage:   "List the contents of an existing package file: manifest, convention directories with file counts, and the sha256",
				Aliases: []string{"l"},
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
			if cmd.GetBool("list") {
				return listPackage(cmd.GetStringArg("dir"))
			}
			output := cmd.GetString("output")
			if output == "" {
				return fmt.Errorf("--output is required when packing")
			}
			hash, warnings, err := pack.Pack(cmd.GetStringArg("dir"), output, cmd.GetBool("force"))
			if err != nil {
				return err
			}
			for _, w := range warnings {
				fmt.Fprintf(os.Stderr, "warning: %s\n", w)
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

// listPackage prints a package's manifest, its convention directories with
// per-directory file counts, and the sha256: a pre-deploy sanity check for
// what actually shipped in the artifact.
func listPackage(path string) error {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("failed to open package: %w", err)
	}
	defer zr.Close()

	byDir := map[string][]string{}
	var manifestName, manifestVersion, manifestServe string
	var total int
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		parts := strings.SplitN(f.Name, "/", 2)
		dir := "(root)"
		if len(parts) == 2 {
			dir = parts[0]
		}
		byDir[dir] = append(byDir[dir], f.Name)
		total++
		if f.Name == "manifest.toml" {
			if data, err := readZipFile(f); err == nil {
				for _, line := range strings.Split(string(data), "\n") {
					line = strings.TrimSpace(line)
					for _, key := range []string{"name", "version", "serve"} {
						prefix := key + " = "
						if strings.HasPrefix(line, prefix) {
							v := strings.Trim(strings.TrimPrefix(line, prefix), "\"[]")
							switch key {
							case "name":
								manifestName = v
							case "version":
								manifestVersion = v
							case "serve":
								manifestServe = v
							}
						}
					}
				}
			}
		}
	}

	if manifestName != "" {
		fmt.Printf("package: %s %s\n", manifestName, manifestVersion)
	}
	if manifestServe != "" {
		fmt.Printf("serves: %s\n", manifestServe)
	}
	dirs := make([]string, 0, len(byDir))
	for dir := range byDir {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	for _, dir := range dirs {
		fmt.Printf("  %-12s %d file(s)\n", dir+"/", len(byDir[dir]))
	}
	fmt.Printf("  %-12s %d\n", "total", total)

	if data, err := os.ReadFile(path); err == nil {
		fmt.Printf("sha256=%s\n", pack.HashBytes(data))
	}
	return nil
}

func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}
