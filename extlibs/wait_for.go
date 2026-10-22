package extlibs

import (
	"bytes"
	"context"
	stderrors "errors"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/paularlott/scriptling/errors"
	"github.com/paularlott/scriptling/extlibs/fssecurity"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/pool"
	"github.com/shirou/gopsutil/v3/process"
)

// RegisterWaitForLibrary registers the wait_for library with an optional
// network policy and no filesystem restrictions.
func RegisterWaitForLibrary(registrar interface{ RegisterLibrary(*object.Library) }, cfg ...*netsecurity.Config) {
	RegisterWaitForLibraryConfigured(registrar, nil, cfg...)
}

// RegisterWaitForLibraryConfigured registers the wait_for library with
// filesystem and network restrictions. allowedPaths confines file, dir and
// file_content as it does the os and pathlib libraries (nil = unrestricted,
// empty = deny all); cfg is the network policy for port and http.
func RegisterWaitForLibraryConfigured(registrar interface{ RegisterLibrary(*object.Library) }, allowedPaths []string, cfg ...*netsecurity.Config) {
	var guard *netsecurity.Guard
	if len(cfg) > 0 && cfg[0] != nil {
		g, gerr := netsecurity.NewGuard(cfg[0])
		if gerr != nil {
			g = netsecurity.FailClosed(gerr)
		}
		guard = g
	}
	if guard == nil && allowedPaths == nil {
		registrar.RegisterLibrary(WaitForLibrary)
		return
	}
	paths := &fssecurity.Config{AllowedPaths: fssecurity.NormalizeAllowedPaths(allowedPaths)}
	registrar.RegisterLibrary(newWaitForLibrary(guard, paths))
}

// waitForStat stats path once paths allows it. A path outside the allowed
// paths is a PermissionError, checked on every poll so a path that appears
// later (say as a symlink out) is caught too.
func waitForStat(paths *fssecurity.Config, path string) (os.FileInfo, object.Object) {
	if !paths.IsPathAllowed(path) {
		return nil, errors.NewPermissionError("access denied: path '%s' is outside allowed directories", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, nil
	}
	return info, nil
}

// waitForContains reports whether the file at path contains needle, once
// paths allows it.
func waitForContains(paths *fssecurity.Config, path string, needle []byte) (bool, object.Object) {
	if !paths.IsPathAllowed(path) {
		return false, errors.NewPermissionError("access denied: path '%s' is outside allowed directories", path)
	}
	data, tooLarge, readErr := object.ReadFileLimited(path)
	if tooLarge != nil {
		return false, tooLarge
	}
	return readErr == nil && bytes.Contains(data, needle), nil
}

// parseWaitOptions parses common wait options from args and kwargs
// Returns (timeout, pollRate, errorOrNil)
func parseWaitOptions(args []object.Object, kwargs map[string]object.Object) (int, float64, object.Object) {
	timeout := 30   // default timeout in seconds
	pollRate := 1.0 // default poll rate in seconds

	// Handle positional args (timeout can be positional)
	if len(args) > 1 {
		if t, err := args[1].AsInt(); err == nil {
			timeout = int(t)
		} else {
			return 0, 0, err
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
			if t, err := v.AsInt(); err == nil {
				timeout = int(t)
			} else {
				return 0, 0, err
			}
		case "poll_rate":
			if f, err := v.AsFloat(); err == nil {
				pollRate = f
			} else {
				return 0, 0, err
			}
		}
	}

	return timeout, pollRate, nil
}

func stopTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

func waitForNextPoll(ctx context.Context, timer *time.Timer, pollInterval time.Duration) bool {
	stopTimer(timer)
	timer.Reset(pollInterval)

	var ok bool
	object.RunBlocking(ctx, func() {
		select {
		case <-ctx.Done():
			ok = false
		case <-timer.C:
			ok = true
		}
	})
	return ok
}

// probePort reports whether a TCP connection to address succeeds within a
// second. With a guard the dial goes through the network policy, and a policy
// rejection is returned as an error rather than reported as a closed port.
func probePort(ctx context.Context, guard *netsecurity.Guard, address string) (bool, error) {
	if guard == nil {
		conn, err := net.DialTimeout("tcp", address, time.Second)
		if err != nil {
			return false, nil
		}
		conn.Close()
		return true, nil
	}
	dialCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	conn, err := guard.Dial(dialCtx, "tcp", address)
	if err != nil {
		if stderrors.Is(err, netsecurity.ErrPolicy) {
			return false, err
		}
		return false, nil
	}
	conn.Close()
	return true, nil
}

// WaitForLibrary is the wait_for library (no network or filesystem policy)
var WaitForLibrary = newWaitForLibrary(nil, &fssecurity.Config{})

// newWaitForLibrary builds the wait_for library. A non-nil guard routes its
// HTTP and port checks through the network policy; paths confines its file,
// dir and file_content checks.
func newWaitForLibrary(guard *netsecurity.Guard, paths *fssecurity.Config) *object.Library {
	return object.NewLibrary(WaitForLibraryName,
		map[string]*object.Builtin{
			"file": {
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.MinArgs(args, 1); err != nil {
						return err
					}

					path, err := args[0].AsString()
					if err != nil {
						return err
					}

					timeout, pollRate, err := parseWaitOptions(args, kwargs.Kwargs)
					if err != nil {
						return err
					}

					deadline := time.Now().Add(time.Duration(timeout) * time.Second)
					pollInterval := time.Duration(pollRate * float64(time.Second))
					timer := time.NewTimer(pollInterval)
					defer stopTimer(timer)

					for time.Now().Before(deadline) {
						info, denied := waitForStat(paths, path)
						if denied != nil {
							return denied
						}
						if info != nil {
							return object.NewBoolean(true)
						}

						if !waitForNextPoll(ctx, timer, pollInterval) {
							return object.NewBoolean(false)
						}
					}

					// Final check
					info, denied := waitForStat(paths, path)
					if denied != nil {
						return denied
					}
					return object.NewBoolean(info != nil)
				},
				HelpText: `file(path, timeout=30, poll_rate=1) - Wait for a file to exist

Waits for the specified file to become available.

Parameters:
  path (string): Path to the file to wait for
  timeout (int): Maximum time to wait in seconds (default: 30)
  poll_rate (float): Time between checks in seconds (default: 1)

Returns:
  bool: True if file exists, False if timeout exceeded

Raises PermissionError if the path is outside the allowed paths.`,
			},
			"dir": {
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.MinArgs(args, 1); err != nil {
						return err
					}

					path, err := args[0].AsString()
					if err != nil {
						return err
					}

					timeout, pollRate, err := parseWaitOptions(args, kwargs.Kwargs)
					if err != nil {
						return err
					}

					deadline := time.Now().Add(time.Duration(timeout) * time.Second)
					pollInterval := time.Duration(pollRate * float64(time.Second))
					timer := time.NewTimer(pollInterval)
					defer stopTimer(timer)

					for time.Now().Before(deadline) {
						info, denied := waitForStat(paths, path)
						if denied != nil {
							return denied
						}
						if info != nil && info.IsDir() {
							return object.NewBoolean(true)
						}

						if !waitForNextPoll(ctx, timer, pollInterval) {
							return object.NewBoolean(false)
						}
					}

					// Final check
					info, denied := waitForStat(paths, path)
					if denied != nil {
						return denied
					}
					return object.NewBoolean(info != nil && info.IsDir())
				},
				HelpText: `dir(path, timeout=30, poll_rate=1) - Wait for a directory to exist

Waits for the specified directory to become available.

Parameters:
  path (string): Path to the directory to wait for
  timeout (int): Maximum time to wait in seconds (default: 30)
  poll_rate (float): Time between checks in seconds (default: 1)

Returns:
  bool: True if directory exists, False if timeout exceeded

Raises PermissionError if the path is outside the allowed paths.`,
			},
			"port": {
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.MinArgs(args, 2); err != nil {
						return err
					}

					host, err := args[0].AsString()
					if err != nil {
						return err
					}

					var port int
					switch v := args[1].(type) {
					case *object.Integer:
						port = int(v.IntValue())
					case *object.String:
						p, err := strconv.Atoi(v.StringValue())
						if err != nil {
							return errors.NewError("invalid port number: %s", v.StringValue())
						}
						port = p
					default:
						return errors.NewTypeError("INT|STRING", args[1].Type().String())
					}

					timeout, pollRate, err := parseWaitOptionsKwargsOnly(30, 1.0, kwargs.Kwargs)
					if err != nil {
						return err
					}

					deadline := time.Now().Add(time.Duration(timeout) * time.Second)
					pollInterval := time.Duration(pollRate * float64(time.Second))
					address := net.JoinHostPort(host, strconv.Itoa(port))
					timer := time.NewTimer(pollInterval)
					defer stopTimer(timer)

					for time.Now().Before(deadline) {
						open, policyErr := probePort(ctx, guard, address)
						if policyErr != nil {
							return errors.NewError("%s", policyErr.Error())
						}
						if open {
							return object.NewBoolean(true)
						}

						if !waitForNextPoll(ctx, timer, pollInterval) {
							return object.NewBoolean(false)
						}
					}

					// Final check
					open, policyErr := probePort(ctx, guard, address)
					if policyErr != nil {
						return errors.NewError("%s", policyErr.Error())
					}
					return object.NewBoolean(open)
				},
				HelpText: `port(host, port, timeout=30, poll_rate=1) - Wait for a TCP port to be open

Waits for the specified TCP port to accept connections.

Parameters:
  host (string): Hostname or IP address
  port (int|string): Port number
  timeout (int): Maximum time to wait in seconds (default: 30)
  poll_rate (float): Time between checks in seconds (default: 1)

Returns:
  bool: True if port is open, False if timeout exceeded

Raises an error if a configured network policy denies the address.`,
			},
			"http": {
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.MinArgs(args, 1); err != nil {
						return err
					}

					url, err := args[0].AsString()
					if err != nil {
						return err
					}

					timeout := 30
					pollRate := 1.0
					expectedStatus := int64(200)

					// Handle positional timeout
					if len(args) > 1 {
						if t, err := args[1].AsInt(); err == nil {
							timeout = int(t)
						} else {
							return err
						}
					}

					// Handle kwargs
					for k, v := range kwargs.Kwargs {
						switch k {
						case "timeout":
							if t, err := v.AsInt(); err == nil {
								timeout = int(t)
							} else {
								return err
							}
						case "poll_rate":
							if f, err := v.AsFloat(); err == nil {
								pollRate = f
							} else if i, err := v.AsInt(); err == nil {
								pollRate = float64(i)
							} else {
								return errors.NewTypeError("FLOAT", v.Type().String())
							}
						case "status_code":
							if s, err := v.AsInt(); err == nil {
								expectedStatus = s
							} else {
								return err
							}
						}
					}

					deadline := time.Now().Add(time.Duration(timeout) * time.Second)
					pollInterval := time.Duration(pollRate * float64(time.Second))
					timer := time.NewTimer(pollInterval)
					defer stopTimer(timer)

					// Use shared pool for HTTP client, or the policy-guarded client
					client := pool.GetHTTPClient()
					if guard != nil {
						client = guard.HTTPClient()
					}

					for time.Now().Before(deadline) {
						req, httpErr := http.NewRequestWithContext(ctx, "GET", url, nil)
						if httpErr != nil {
							return errors.NewError("http request error: %s", httpErr.Error())
						}

						var resp *http.Response
						object.RunBlocking(ctx, func() { resp, httpErr = client.Do(req) })
						if httpErr == nil {
							statusMatch := int64(resp.StatusCode) == expectedStatus
							resp.Body.Close()
							if statusMatch {
								return object.NewBoolean(true)
							}
						} else if stderrors.Is(httpErr, netsecurity.ErrPolicy) {
							return errors.NewError("http error: %s", httpErr.Error())
						}

						if !waitForNextPoll(ctx, timer, pollInterval) {
							return object.NewBoolean(false)
						}
					}

					// Final check
					req, httpErr := http.NewRequestWithContext(ctx, "GET", url, nil)
					if httpErr != nil {
						return object.NewBoolean(false)
					}
					var resp *http.Response
					object.RunBlocking(ctx, func() { resp, httpErr = client.Do(req) })
					if httpErr == nil {
						statusMatch := int64(resp.StatusCode) == expectedStatus
						resp.Body.Close()
						if statusMatch {
							return object.NewBoolean(true)
						}
					} else if stderrors.Is(httpErr, netsecurity.ErrPolicy) {
						return errors.NewError("http error: %s", httpErr.Error())
					}
					return object.NewBoolean(false)
				},
				HelpText: `http(url, timeout=30, poll_rate=1, status_code=200) - Wait for HTTP endpoint

Waits for the specified HTTP endpoint to respond with the expected status code.

Parameters:
  url (string): URL to check
  timeout (int): Maximum time to wait in seconds (default: 30)
  poll_rate (float): Time between checks in seconds (default: 1)
  status_code (int): Expected HTTP status code (default: 200)

Returns:
  bool: True if endpoint responds with expected status, False if timeout exceeded

Raises an error if a configured network policy denies the URL.`,
			},
			"file_content": {
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.MinArgs(args, 2); err != nil {
						return err
					}

					path, err := args[0].AsString()
					if err != nil {
						return err
					}

					content, err := args[1].AsString()
					if err != nil {
						return err
					}

					timeout, pollRate, err := parseWaitOptionsKwargsOnly(30, 1.0, kwargs.Kwargs)
					if err != nil {
						return err
					}

					deadline := time.Now().Add(time.Duration(timeout) * time.Second)
					pollInterval := time.Duration(pollRate * float64(time.Second))
					needle := []byte(content)
					timer := time.NewTimer(pollInterval)
					defer stopTimer(timer)

					for time.Now().Before(deadline) {
						found, errObj := waitForContains(paths, path, needle)
						if errObj != nil {
							return errObj
						}
						if found {
							return object.NewBoolean(true)
						}

						if !waitForNextPoll(ctx, timer, pollInterval) {
							return object.NewBoolean(false)
						}
					}

					// Final check
					found, errObj := waitForContains(paths, path, needle)
					if errObj != nil {
						return errObj
					}
					return object.NewBoolean(found)
				},
				HelpText: `file_content(path, content, timeout=30, poll_rate=1) - Wait for file to contain content

Waits for the specified file to exist and contain the given content.

Parameters:
  path (string): Path to the file to check
  content (string): Content to search for in the file
  timeout (int): Maximum time to wait in seconds (default: 30)
  poll_rate (float): Time between checks in seconds (default: 1)

Returns:
  bool: True if file contains the content, False if timeout exceeded

Raises PermissionError if the path is outside the allowed paths.`,
			},
			"process_name": {
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if err := errors.MinArgs(args, 1); err != nil {
						return err
					}

					processName, err := args[0].AsString()
					if err != nil {
						return err
					}

					timeout, pollRate, err := parseWaitOptions(args, kwargs.Kwargs)
					if err != nil {
						return err
					}

					deadline := time.Now().Add(time.Duration(timeout) * time.Second)
					pollInterval := time.Duration(pollRate * float64(time.Second))
					timer := time.NewTimer(pollInterval)
					defer stopTimer(timer)

					for time.Now().Before(deadline) {
						if processRunning(processName) {
							return object.NewBoolean(true)
						}

						if !waitForNextPoll(ctx, timer, pollInterval) {
							return object.NewBoolean(false)
						}
					}

					// Final check
					if processRunning(processName) {
						return object.NewBoolean(true)
					}
					return object.NewBoolean(false)
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
}

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
