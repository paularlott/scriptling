package extlibs

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/object"
	"github.com/shirou/gopsutil/v3/process"
)

func RegisterWaitForLibrary(registrar interface{ RegisterLibrary(string, *object.Library) }) {
	registrar.RegisterLibrary(WaitForLibraryName, WaitForLibrary)
}

// parseWaitOptions parses common wait options from args and kwargs
// Returns (timeout, pollRate, errorOrNil)
func parseWaitOptions(args []object.Object, kwargs map[string]object.Object) (int, float64, object.Object) {
	timeout := 30  // default timeout in seconds
	pollRate := 1.0 // default poll rate in seconds

	// Handle positional args (timeout can be positional)
	if len(args) > 1 {
		if t, ok := args[1].AsInt(); ok {
			timeout = int(t)
		} else {
			return 0, 0, errors.NewTypeError("INT", args[1].Type().String())
		}
	}

	return parseWaitOptionsKwargsOnly(timeout, pollRate, kwargs)
}

// parseWaitOptionsKwargsOnly parses wait options only from kwargs (no positional args)
// Returns (timeout, pollRate, errorOrNil)
func parseWaitOptionsKwargsOnly(defaultTimeout int, defaultPollRate float64, kwargs map[string]object.Object) (int, float64, object.Object) {
	timeout := defaultTimeout
	pollRate := defaultPollRate

	// Handle keyword args
	for k, v := range kwargs {
		switch k {
		case "timeout":
			if t, ok := v.AsInt(); ok {
				timeout = int(t)
			} else {
				return 0, 0, errors.NewTypeError("INT", v.Type().String())
			}
		case "poll_rate":
			if f, ok := v.AsFloat(); ok {
				pollRate = f
			} else {
				return 0, 0, errors.NewTypeError("FLOAT", v.Type().String())
			}
		}
	}

	return timeout, pollRate, nil
}

var WaitForLibrary = object.NewLibrary(
	map[string]*object.Builtin{
		"file": {
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) < 1 {
					return errors.NewArgumentError(len(args), 1)
				}

				path, ok := args[0].AsString()
				if !ok {
					return errors.NewTypeError("STRING", args[0].Type().String())
				}

				timeout, pollRate, err := parseWaitOptions(args, kwargs)
				if err != nil {
					return err
				}

				deadline := time.Now().Add(time.Duration(timeout) * time.Second)
				pollInterval := time.Duration(pollRate * float64(time.Second))

				for time.Now().Before(deadline) {
					if _, err := os.Stat(path); err == nil {
						return &object.Boolean{Value: true}
					}

					select {
					case <-ctx.Done():
						return &object.Boolean{Value: false}
					case <-time.After(pollInterval):
						// Continue polling
					}
				}

				// Final check
				if _, err := os.Stat(path); err == nil {
					return &object.Boolean{Value: true}
				}
				return &object.Boolean{Value: false}
			},
			HelpText: `file(path, timeout=30, poll_rate=1) - Wait for a file to exist

Waits for the specified file to become available.

Parameters:
  path (string): Path to the file to wait for
  timeout (int): Maximum time to wait in seconds (default: 30)
  poll_rate (float): Time between checks in seconds (default: 1)

Returns:
  bool: True if file exists, False if timeout exceeded`,
		},
		"dir": {
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) < 1 {
					return errors.NewArgumentError(len(args), 1)
				}

				path, ok := args[0].AsString()
				if !ok {
					return errors.NewTypeError("STRING", args[0].Type().String())
				}

				timeout, pollRate, err := parseWaitOptions(args, kwargs)
				if err != nil {
					return err
				}

				deadline := time.Now().Add(time.Duration(timeout) * time.Second)
				pollInterval := time.Duration(pollRate * float64(time.Second))

				for time.Now().Before(deadline) {
					if info, err := os.Stat(path); err == nil {
						if info.IsDir() {
							return &object.Boolean{Value: true}
						}
					}

					select {
					case <-ctx.Done():
						return &object.Boolean{Value: false}
					case <-time.After(pollInterval):
						// Continue polling
					}
				}

				// Final check
				if info, err := os.Stat(path); err == nil {
					if info.IsDir() {
						return &object.Boolean{Value: true}
					}
				}
				return &object.Boolean{Value: false}
			},
			HelpText: `dir(path, timeout=30, poll_rate=1) - Wait for a directory to exist

Waits for the specified directory to become available.

Parameters:
  path (string): Path to the directory to wait for
  timeout (int): Maximum time to wait in seconds (default: 30)
  poll_rate (float): Time between checks in seconds (default: 1)

Returns:
  bool: True if directory exists, False if timeout exceeded`,
		},
		"port": {
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) < 2 {
					return errors.NewArgumentError(len(args), 2)
				}

				host, ok := args[0].AsString()
				if !ok {
					return errors.NewTypeError("STRING", args[0].Type().String())
				}

				var port int
				switch v := args[1].(type) {
				case *object.Integer:
					port = int(v.Value)
				case *object.String:
					p, err := strconv.Atoi(v.Value)
					if err != nil {
						return errors.NewError("invalid port number: %s", v.Value)
					}
					port = p
				default:
					return errors.NewTypeError("INT|STRING", args[1].Type().String())
				}

				timeout, pollRate, err := parseWaitOptionsKwargsOnly(30, 1.0, kwargs)
				if err != nil {
					return err
				}

				deadline := time.Now().Add(time.Duration(timeout) * time.Second)
				pollInterval := time.Duration(pollRate * float64(time.Second))
				address := fmt.Sprintf("%s:%d", host, port)

				for time.Now().Before(deadline) {
					conn, err := net.DialTimeout("tcp", address, time.Second)
					if err == nil {
						conn.Close()
						return &object.Boolean{Value: true}
					}

					select {
					case <-ctx.Done():
						return &object.Boolean{Value: false}
					case <-time.After(pollInterval):
						// Continue polling
					}
				}

				// Final check
				if conn, err := net.DialTimeout("tcp", address, time.Second); err == nil {
					conn.Close()
					return &object.Boolean{Value: true}
				}
				return &object.Boolean{Value: false}
			},
			HelpText: `port(host, port, timeout=30, poll_rate=1) - Wait for a TCP port to be open

Waits for the specified TCP port to accept connections.

Parameters:
  host (string): Hostname or IP address
  port (int|string): Port number
  timeout (int): Maximum time to wait in seconds (default: 30)
  poll_rate (float): Time between checks in seconds (default: 1)

Returns:
  bool: True if port is open, False if timeout exceeded`,
		},
		"http": {
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) < 1 {
					return errors.NewArgumentError(len(args), 1)
				}

				url, ok := args[0].AsString()
				if !ok {
					return errors.NewTypeError("STRING", args[0].Type().String())
				}

				timeout := 30
				pollRate := 1.0
				expectedStatus := int64(200)

				// Handle positional timeout
				if len(args) > 1 {
					if t, ok := args[1].AsInt(); ok {
						timeout = int(t)
					} else {
						return errors.NewTypeError("INT", args[1].Type().String())
					}
				}

				// Handle kwargs
				for k, v := range kwargs {
					switch k {
					case "timeout":
						if t, ok := v.AsInt(); ok {
							timeout = int(t)
						} else {
							return errors.NewTypeError("INT", v.Type().String())
						}
					case "poll_rate":
						if f, ok := v.AsFloat(); ok {
							pollRate = f
						} else if i, ok := v.AsInt(); ok {
							pollRate = float64(i)
						} else {
							return errors.NewTypeError("FLOAT", v.Type().String())
						}
					case "status_code":
						if s, ok := v.AsInt(); ok {
							expectedStatus = s
						} else {
							return errors.NewTypeError("INT", v.Type().String())
						}
					}
				}

				deadline := time.Now().Add(time.Duration(timeout) * time.Second)
				pollInterval := time.Duration(pollRate * float64(time.Second))

				client := &http.Client{
					Timeout: 5 * time.Second,
				}

				for time.Now().Before(deadline) {
					req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
					if err != nil {
						return errors.NewError("http request error: %s", err.Error())
					}

					resp, err := client.Do(req)
					if err == nil {
						statusMatch := int64(resp.StatusCode) == expectedStatus
						resp.Body.Close()
						if statusMatch {
							return &object.Boolean{Value: true}
						}
					}

					select {
					case <-ctx.Done():
						return &object.Boolean{Value: false}
					case <-time.After(pollInterval):
						// Continue polling
					}
				}

				// Final check
				req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
				if err != nil {
					return &object.Boolean{Value: false}
				}
				if resp, err := client.Do(req); err == nil {
					statusMatch := int64(resp.StatusCode) == expectedStatus
					resp.Body.Close()
					if statusMatch {
						return &object.Boolean{Value: true}
					}
				}
				return &object.Boolean{Value: false}
			},
			HelpText: `http(url, timeout=30, poll_rate=1, status_code=200) - Wait for HTTP endpoint

Waits for the specified HTTP endpoint to respond with the expected status code.

Parameters:
  url (string): URL to check
  timeout (int): Maximum time to wait in seconds (default: 30)
  poll_rate (float): Time between checks in seconds (default: 1)
  status_code (int): Expected HTTP status code (default: 200)

Returns:
  bool: True if endpoint responds with expected status, False if timeout exceeded`,
		},
		"file_content": {
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) < 2 {
					return errors.NewArgumentError(len(args), 2)
				}

				path, ok := args[0].AsString()
				if !ok {
					return errors.NewTypeError("STRING", args[0].Type().String())
				}

				content, ok := args[1].AsString()
				if !ok {
					return errors.NewTypeError("STRING", args[1].Type().String())
				}

				timeout, pollRate, err := parseWaitOptionsKwargsOnly(30, 1.0, kwargs)
				if err != nil {
					return err
				}

				deadline := time.Now().Add(time.Duration(timeout) * time.Second)
				pollInterval := time.Duration(pollRate * float64(time.Second))

				for time.Now().Before(deadline) {
					if data, err := os.ReadFile(path); err == nil {
						if strings.Contains(string(data), content) {
							return &object.Boolean{Value: true}
						}
					}

					select {
					case <-ctx.Done():
						return &object.Boolean{Value: false}
					case <-time.After(pollInterval):
						// Continue polling
					}
				}

				// Final check
				if data, err := os.ReadFile(path); err == nil {
					if strings.Contains(string(data), content) {
						return &object.Boolean{Value: true}
					}
				}
				return &object.Boolean{Value: false}
			},
			HelpText: `file_content(path, content, timeout=30, poll_rate=1) - Wait for file to contain content

Waits for the specified file to exist and contain the given content.

Parameters:
  path (string): Path to the file to check
  content (string): Content to search for in the file
  timeout (int): Maximum time to wait in seconds (default: 30)
  poll_rate (float): Time between checks in seconds (default: 1)

Returns:
  bool: True if file contains the content, False if timeout exceeded`,
		},
		"process_name": {
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) < 1 {
					return errors.NewArgumentError(len(args), 1)
				}

				processName, ok := args[0].AsString()
				if !ok {
					return errors.NewTypeError("STRING", args[0].Type().String())
				}

				timeout, pollRate, err := parseWaitOptions(args, kwargs)
				if err != nil {
					return err
				}

				deadline := time.Now().Add(time.Duration(timeout) * time.Second)
				pollInterval := time.Duration(pollRate * float64(time.Second))

				for time.Now().Before(deadline) {
					if processRunning(processName) {
						return &object.Boolean{Value: true}
					}

					select {
					case <-ctx.Done():
						return &object.Boolean{Value: false}
					case <-time.After(pollInterval):
						// Continue polling
					}
				}

				// Final check
				if processRunning(processName) {
					return &object.Boolean{Value: true}
				}
				return &object.Boolean{Value: false}
			},
			HelpText: `process_name(name, timeout=30, poll_rate=1) - Wait for a process to be running

Waits for a process with the specified name to be running.

Parameters:
  name (string): Process name to search for
  timeout (int): Maximum time to wait in seconds (default: 30)
  poll_rate (float): Time between checks in seconds (default: 1)

Returns:
  bool: True if process is running, False if timeout exceeded`,
		},
	},
	nil,
	"Wait for resources to become available",
)

// processRunning checks if a process with the given name is running (cross-platform)
func processRunning(name string) bool {
	processes, err := process.Processes()
	if err != nil {
		return false
	}

	for _, p := range processes {
		// Get process name
		processName, err := p.Name()
		if err != nil {
			// Try executable as fallback
			if exe, err := p.Exe(); err == nil {
				processName = exe
				// Extract just the basename
				if idx := strings.LastIndex(processName, string(os.PathSeparator)); idx >= 0 {
					processName = processName[idx+1:]
				}
			} else {
				continue
			}
		}

		// Check if the process name contains the search term
		if strings.Contains(strings.ToLower(processName), strings.ToLower(name)) {
			return true
		}
	}

	return false
}
