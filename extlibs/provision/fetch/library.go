package fetch

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/object"
)

const (
	LibraryName = "scriptling.provision.fetch"
	LibraryDesc = "Fetch provisioning utilities for downloading files and unpacking zip archives"

	defaultFileMode = 0o644
	defaultDirMode  = 0o755
	defaultTimeout  = 30

	StatusCreated   = "created"
	StatusUpdated   = "updated"
	StatusUnchanged = "unchanged"
)

var (
	library     *object.Library
	libraryOnce sync.Once
)

func Register(registrar interface{ RegisterLibrary(*object.Library) }) {
	libraryOnce.Do(func() {
		library = buildLibrary()
	})
	registrar.RegisterLibrary(library)
}

func buildLibrary() *object.Library {
	return object.NewLibrary(LibraryName, map[string]*object.Builtin{
		"file": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) != 2 {
					return &object.Error{Message: fmt.Sprintf("file expected 2 arguments, got %d", len(args))}
				}

				src, coerceErr := args[0].CoerceString()
				if coerceErr != nil {
					return &object.Error{Message: "file: url must be a string"}
				}
				dest, coerceErr := args[1].CoerceString()
				if coerceErr != nil {
					return &object.Error{Message: "file: dest must be a string"}
				}

				insecure := kwargs.MustGetBool("insecure", false)
				unpackZip := kwargs.MustGetBool("unpack_zip", false)
				timeoutSecs := int(kwargs.MustGetInt("timeout", defaultTimeout))
				mode := int(kwargs.MustGetInt("mode", defaultFileMode))
				dirMode := int(kwargs.MustGetInt("dir_mode", defaultDirMode))

				if timeoutSecs <= 0 {
					return &object.Error{Message: "file: timeout must be greater than zero"}
				}
				if mode < 0 {
					return &object.Error{Message: "file: mode must be non-negative"}
				}
				if dirMode < 0 {
					return &object.Error{Message: "file: dir_mode must be non-negative"}
				}

				data, err := fetchURL(ctx, src, insecure, time.Duration(timeoutSecs)*time.Second)
				if err != nil {
					return &object.Error{Message: "file: " + err.Error()}
				}

				dest = expandPath(dest)
				var result fetchResult
				if unpackZip {
					result, err = unpackZipBytes(data, dest, os.FileMode(mode), os.FileMode(dirMode))
				} else {
					result, err = writeFetchedFile(data, dest, os.FileMode(mode), os.FileMode(dirMode))
				}
				if err != nil {
					return &object.Error{Message: "file: " + err.Error()}
				}
				result.URL = src
				result.Path = dest
				result.Bytes = int64(len(data))
				result.Unpacked = unpackZip
				return conversion.FromGo(result.toMap())
			},
			HelpText: `file(url, dest, insecure=False, unpack_zip=False, timeout=30, mode=0o644, dir_mode=0o755) - Fetch a file over HTTP/HTTPS

Downloads url to dest. Parent directories are created automatically. When
unpack_zip is True, dest is treated as a destination directory and the fetched
body is unpacked as a zip archive instead of being written as one file.

Zip extraction is constrained to dest; entries that would escape the target
directory are rejected.

Parameters:
  url (str): http:// or https:// URL to fetch
  dest (str): Destination file path, or destination directory when unpack_zip=True
  insecure (bool): If True, skip HTTPS certificate verification (default False)
  unpack_zip (bool): If True, unpack the response body as a zip archive (default False)
  timeout (int): Request timeout in seconds (default 30)
  mode (int): File permission mode for written files (default 0o644)
  dir_mode (int): Directory permission mode for created directories (default 0o755)

Returns:
  dict: {"status": "created|updated|unchanged", "url": url, "path": dest,
         "bytes": response_size, "unpacked": bool, "files": [paths...]}

Example:
  import scriptling.provision.fetch as fetch

  result = fetch.file("https://example.com/app.conf", "~/.config/app/app.conf")
  if result["status"] != fetch.UNCHANGED:
      print("Fetched " + result["path"])

  archive = fetch.file("https://example.com/site.zip", "/srv/site", unpack_zip=True)
  print("Extracted " + str(len(archive["files"])) + " files")`,
		},
	}, map[string]object.Object{
		"CREATED":   object.NewString(StatusCreated),
		"UPDATED":   object.NewString(StatusUpdated),
		"UNCHANGED": object.NewString(StatusUnchanged),
	}, LibraryDesc)
}

type fetchResult struct {
	Status   string
	URL      string
	Path     string
	Bytes    int64
	Unpacked bool
	Files    []string
}

func (r fetchResult) toMap() map[string]interface{} {
	return map[string]interface{}{
		"status":   r.Status,
		"url":      r.URL,
		"path":     r.Path,
		"bytes":    r.Bytes,
		"unpacked": r.Unpacked,
		"files":    r.Files,
	}
}

func fetchURL(ctx context.Context, rawURL string, insecure bool, timeout time.Duration) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported URL scheme %q (use http or https)", u.Scheme)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("URL host is required")
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	client := &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, rawURL)
	}
	return io.ReadAll(resp.Body)
}

func writeFetchedFile(data []byte, dest string, mode, dirMode os.FileMode) (fetchResult, error) {
	if dest == "" {
		return fetchResult{}, fmt.Errorf("dest must not be empty")
	}
	dir := filepath.Dir(dest)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, dirMode); err != nil {
			return fetchResult{}, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	existing, err := os.ReadFile(dest)
	if err == nil && bytes.Equal(existing, data) {
		if err := os.Chmod(dest, mode); err != nil {
			return fetchResult{}, fmt.Errorf("failed to set mode on %s: %w", dest, err)
		}
		return fetchResult{Status: StatusUnchanged, Files: []string{dest}}, nil
	}

	status := StatusCreated
	if err == nil {
		status = StatusUpdated
	} else if !os.IsNotExist(err) {
		return fetchResult{}, fmt.Errorf("failed to read existing %s: %w", dest, err)
	}

	if err := os.WriteFile(dest, data, mode); err != nil {
		return fetchResult{}, fmt.Errorf("failed to write %s: %w", dest, err)
	}
	if err := os.Chmod(dest, mode); err != nil {
		return fetchResult{}, fmt.Errorf("failed to set mode on %s: %w", dest, err)
	}
	return fetchResult{Status: status, Files: []string{dest}}, nil
}

func unpackZipBytes(data []byte, dest string, mode, dirMode os.FileMode) (fetchResult, error) {
	if dest == "" {
		return fetchResult{}, fmt.Errorf("dest must not be empty")
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fetchResult{}, fmt.Errorf("failed to read zip archive: %w", err)
	}
	if err := os.MkdirAll(dest, dirMode); err != nil {
		return fetchResult{}, fmt.Errorf("failed to create directory %s: %w", dest, err)
	}

	cleanDest, err := filepath.Abs(dest)
	if err != nil {
		return fetchResult{}, err
	}
	status := StatusUnchanged
	files := make([]string, 0, len(reader.File))

	for _, f := range reader.File {
		if f.Name == "" {
			continue
		}
		target, err := safeZipTarget(cleanDest, f.Name)
		if err != nil {
			return fetchResult{}, err
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, dirMode); err != nil {
				return fetchResult{}, fmt.Errorf("failed to create directory %s: %w", target, err)
			}
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return fetchResult{}, fmt.Errorf("failed to open zip entry %s: %w", f.Name, err)
		}
		content, readErr := io.ReadAll(rc)
		closeErr := rc.Close()
		if readErr != nil {
			return fetchResult{}, fmt.Errorf("failed to read zip entry %s: %w", f.Name, readErr)
		}
		if closeErr != nil {
			return fetchResult{}, fmt.Errorf("failed to close zip entry %s: %w", f.Name, closeErr)
		}

		if err := os.MkdirAll(filepath.Dir(target), dirMode); err != nil {
			return fetchResult{}, fmt.Errorf("failed to create directory %s: %w", filepath.Dir(target), err)
		}

		existing, err := os.ReadFile(target)
		if err == nil && bytes.Equal(existing, content) {
			if err := os.Chmod(target, mode); err != nil {
				return fetchResult{}, fmt.Errorf("failed to set mode on %s: %w", target, err)
			}
			files = append(files, target)
			continue
		}
		if err == nil {
			status = StatusUpdated
		} else if os.IsNotExist(err) {
			if status == StatusUnchanged {
				status = StatusCreated
			}
		} else {
			return fetchResult{}, fmt.Errorf("failed to read existing %s: %w", target, err)
		}

		if err := os.WriteFile(target, content, mode); err != nil {
			return fetchResult{}, fmt.Errorf("failed to write %s: %w", target, err)
		}
		if err := os.Chmod(target, mode); err != nil {
			return fetchResult{}, fmt.Errorf("failed to set mode on %s: %w", target, err)
		}
		files = append(files, target)
	}

	return fetchResult{Status: status, Files: files}, nil
}

func safeZipTarget(dest, name string) (string, error) {
	cleanName := filepath.Clean(filepath.FromSlash(name))
	if cleanName == "." || filepath.IsAbs(cleanName) || strings.HasPrefix(cleanName, ".."+string(filepath.Separator)) || cleanName == ".." {
		return "", fmt.Errorf("zip entry %q escapes destination", name)
	}
	target := filepath.Join(dest, cleanName)
	rel, err := filepath.Rel(dest, target)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("zip entry %q escapes destination", name)
	}
	return target, nil
}

func expandPath(path string) string {
	if path == "" {
		return path
	}
	if path[0] == '~' {
		home, err := os.UserHomeDir()
		if err == nil {
			if len(path) == 1 || path[1] == '/' {
				return filepath.Join(home, path[1:])
			}
		}
	}
	return path
}
