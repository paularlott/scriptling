package container

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/paularlott/scriptling/evaliface"
	"github.com/paularlott/scriptling/object"
)

const (
	LibraryName = "scriptling.container"
	LibraryDesc = "Container management library supporting Docker, Podman, and Apple Containers"

	DefaultDockerSocket = "/var/run/docker.sock"
	DefaultPodmanSocket = "/var/run/podman.sock"
)

var (
	library              *object.Library
	libraryOnce          sync.Once
	overrideDockerSocket string
	overrridePodmanSocket string
)

// Register registers the scriptling.container library with the given registrar.
// dockerSock and podmanSock override the default socket paths (empty = use default).
func Register(registrar interface{ RegisterLibrary(*object.Library) }, dockerSock, podmanSock string) {
	if dockerSock != "" {
		overrideDockerSocket = dockerSock
	}
	if podmanSock != "" {
		overrridePodmanSocket = podmanSock
	}
	libraryOnce.Do(func() {
		library = buildLibrary()
	})
	registrar.RegisterLibrary(library)
}

func buildLibrary() *object.Library {
	b := object.NewLibraryBuilder(LibraryName, LibraryDesc)

	b.FunctionWithHelp("runtimes", func() []any {
		available := []any{}
		for _, driver := range []string{DriverDocker, DriverPodman, DriverApple} {
			if isAvailable(driver) {
				available = append(available, driver)
			}
		}
		return available
	}, `runtimes() - List available container runtimes

Returns a list of container runtime names that are currently available and running
on this system. Possible values: "docker", "podman", "apple".

Returns:
  list: Available runtime names e.g. ["docker", "podman"]

Example:
  available = container.runtimes()
  print("Available runtimes:", available)`)

	b.FunctionWithHelp("Client", func(ctx context.Context, kwargs object.Kwargs, driver string) (object.Object, error) {
		socket := kwargs.MustGetString("socket", "")
		d, err := newDriverWithSocket(driver, socket)
		if err != nil {
			return nil, err
		}
		return newClientInstance(driver, d), nil
	}, `Client(driver, **kwargs) - Create a container client

Creates a new container client for the specified runtime driver.

Parameters:
  driver (str): Runtime to use — "docker", "podman", or "apple"
  socket (str, optional): Override the Unix socket path (docker/podman only).
                          Defaults to DOCKER_SOCK / PODMAN_SOCK env vars,
                          then /var/run/docker.sock or /run/user/1000/podman/podman.sock

Returns:
  ContainerClient: A client instance

Example:
  c = container.Client("docker")
  c = container.Client("podman")
  c = container.Client("podman", socket="/run/user/500/podman/podman.sock")
  c = container.Client("apple")`)

	return b.Build()
}

// isAvailable checks whether a runtime daemon/CLI is reachable.
func isAvailable(driver string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var cmd *exec.Cmd
	switch driver {
	case DriverDocker:
		cmd = exec.CommandContext(ctx, "docker", "info")
	case DriverPodman:
		cmd = exec.CommandContext(ctx, "podman", "info")
	case DriverApple:
		cmd = exec.CommandContext(ctx, "container", "system", "status")
	default:
		return false
	}
	return cmd.Run() == nil
}

// dockerSocket returns the Docker endpoint: flag override > default.
func dockerSocket() string {
	if overrideDockerSocket != "" {
		return overrideDockerSocket
	}
	return DefaultDockerSocket
}

// podmanSocket returns the Podman endpoint: flag override > default.
func podmanSocket() string {
	if overrridePodmanSocket != "" {
		return overrridePodmanSocket
	}
	return DefaultPodmanSocket
}

// newDriverWithSocket constructs the appropriate ContainerDriver, with an
// optional endpoint override (empty string = use env var / default).
// Accepted endpoint forms for docker/podman:
//   - /var/run/docker.sock          — Unix socket path
//   - unix:///var/run/docker.sock   — Unix socket URI
//   - tcp://host:2375               — plain TCP
//   - host:2375                     — plain TCP shorthand
//   - https://host:2376             — TLS TCP
func newDriverWithSocket(driver, endpoint string) (ContainerDriver, error) {
	switch driver {
	case DriverDocker:
		if endpoint == "" {
			endpoint = dockerSocket()
		}
		return newDockerClient(endpoint), nil
	case DriverPodman:
		if endpoint == "" {
			endpoint = podmanSocket()
		}
		return newDockerClient(endpoint), nil
	case DriverApple:
		return newAppleDriver()
	}
	return nil, fmt.Errorf("unknown container driver %q: must be \"docker\", \"podman\", or \"apple\"", driver)
}

// ── ContainerClient class ────────────────────────────────────────────────────

type clientInstance struct {
	driver     ContainerDriver
	driverName string
}

var (
	containerClientClass     *object.Class
	containerClientClassOnce sync.Once
)

func getContainerClientClass() *object.Class {
	containerClientClassOnce.Do(func() {
		containerClientClass = buildClientClass()
	})
	return containerClientClass
}

func newClientInstance(driverName string, d ContainerDriver) *object.Instance {
	return &object.Instance{
		Class: getContainerClientClass(),
		Fields: map[string]object.Object{
			"_client": &object.ClientWrapper{
				TypeName: "ContainerClient",
				Client:   &clientInstance{driver: d, driverName: driverName},
			},
		},
	}
}

func getClientInstance(self *object.Instance) (*clientInstance, *object.Error) {
	wrapper, ok := object.GetClientField(self, "_client")
	if !ok || wrapper.Client == nil {
		return nil, &object.Error{Message: "ContainerClient: missing internal client"}
	}
	ci, ok := wrapper.Client.(*clientInstance)
	if !ok {
		return nil, &object.Error{Message: "ContainerClient: invalid internal client"}
	}
	return ci, nil
}

func buildClientClass() *object.Class {
	cb := object.NewClassBuilder("ContainerClient")

	cb.MethodWithHelp("login", func(self *object.Instance, ctx context.Context, server, username, password string) object.Object {
		ci, err := getClientInstance(self)
		if err != nil {
			return err
		}
		if goErr := ci.driver.Login(ctx, server, username, password); goErr != nil {
			return &object.Error{Message: goErr.Error()}
		}
		return &object.Null{}
	}, `login(server, username, password) - Authenticate with a container registry

For Docker/Podman the credentials are stored on the client and injected
automatically into subsequent image_pull calls for the same registry.
For Apple Containers the host CLI credential store is updated.

Parameters:
  server (str): Registry server e.g. "ghcr.io" or "registry.example.com".
               Pass "" to target Docker Hub.
  username (str): Registry username
  password (str): Registry password or access token

Example:
  c.login("", "myuser", "mytoken")          # Docker Hub
  c.login("ghcr.io", "myuser", "ghp_token")  # GitHub Container Registry
  c.image_pull("ghcr.io/myorg/myimage:latest")`)

	cb.MethodWithHelp("driver", func(self *object.Instance) string {
		ci, err := getClientInstance(self)
		if err != nil {
			return ""
		}
		return ci.driverName
	}, `driver() - Return the name of the active runtime driver

Returns:
  str: "docker", "podman", or "apple"`)

	cb.MethodWithHelp("exec", func(self *object.Instance, ctx context.Context, kwargs object.Kwargs, nameOrID string, command []string) object.Object {
		ci, err := getClientInstance(self)
		if err != nil {
			return err
		}
		opts := execOptsFromKwargs(kwargs)
		res, goErr := ci.driver.Exec(ctx, nameOrID, command, opts)
		if goErr != nil {
			return &object.Error{Message: goErr.Error()}
		}
		return execResultToDict(res)
	}, `exec(name_or_id, command, **kwargs) - Run a command in a container and capture output

Parameters:
  name_or_id (str): Container name or ID
  command (list): Command and arguments e.g. ["/bin/sh", "-c", "echo hi"]
  env (list, optional): Environment variables e.g. ["KEY=value"]
  workdir (str, optional): Working directory inside the container
  user (str, optional): User to run as e.g. "root" or "1000:1000"

Returns:
  dict: {stdout, stderr, exit_code}

Example:
  result = c.exec("app", ["/bin/sh", "-c", "cat /etc/os-release"])
  print(result["stdout"])
  print("exit:", result["exit_code"])`)

	cb.MethodWithHelp("exec_stream", func(self *object.Instance, ctx context.Context, kwargs object.Kwargs, nameOrID string, command []string, callback object.Object) object.Object {
		ci, err := getClientInstance(self)
		if err != nil {
			return err
		}
		eval := evaliface.FromContext(ctx)
		if eval == nil {
			return &object.Error{Message: "exec_stream: evaluator not available"}
		}
		opts := execOptsFromKwargs(kwargs)
		fn := func(stream, line string) {
			eval.CallObjectFunction(ctx, callback, []object.Object{
				&object.String{Value: stream},
				&object.String{Value: line},
			}, nil, nil)
		}
		res, goErr := ci.driver.ExecStream(ctx, nameOrID, command, opts, fn)
		if goErr != nil {
			return &object.Error{Message: goErr.Error()}
		}
		return execResultToDict(res)
	}, `exec_stream(name_or_id, command, callback, **kwargs) - Run a command and stream output line by line

Calls callback(stream, line) for each line of output as it arrives.
stream is "stdout" or "stderr".

Parameters:
  name_or_id (str): Container name or ID
  command (list): Command and arguments e.g. ["/bin/sh", "-c", "echo hi"]
  callback (callable): Function called with (stream, line) for each output line
  env (list, optional): Environment variables e.g. ["KEY=value"]
  workdir (str, optional): Working directory inside the container
  user (str, optional): User to run as e.g. "root" or "1000:1000"

Returns:
  dict: {stdout, stderr, exit_code} — stdout and stderr are empty strings (output was streamed)

Example:
  def on_line(stream, line):
    print(f"[{stream}] {line}")

  result = c.exec_stream("app", ["/bin/sh", "-c", "for i in 1 2 3; do echo $i; done"], on_line)
  print("exit:", result["exit_code"])`)

	cb.MethodWithHelp("exec", func(self *object.Instance, ctx context.Context, kwargs object.Kwargs, nameOrID string, command []string) object.Object {
		ci, err := getClientInstance(self)
		if err != nil {
			return err
		}
		opts := execOptsFromKwargs(kwargs)
		res, goErr := ci.driver.Exec(ctx, nameOrID, command, opts)
		if goErr != nil {
			return &object.Error{Message: goErr.Error()}
		}
		return execResultToDict(res)
	}, `exec(name_or_id, command, **kwargs) - Run a command in a running container and capture output

Parameters:
  name_or_id (str): Container name or ID
  command (list): Command and arguments e.g. ["/bin/sh", "-c", "echo hi"]
  env (list, optional): Environment variables e.g. ["KEY=value"]
  workdir (str, optional): Working directory inside the container
  user (str, optional): User to run as e.g. "root" or "1000:1000"

Returns:
  dict: {stdout, stderr, exit_code}

Example:
  result = c.exec("app", ["/bin/sh", "-c", "cat /etc/os-release"])
  print(result["stdout"])
  print("exit:", result["exit_code"])`)

	cb.MethodWithHelp("exec_stream", func(self *object.Instance, ctx context.Context, kwargs object.Kwargs, nameOrID string, command []string, callback object.Object) object.Object {
		ci, err := getClientInstance(self)
		if err != nil {
			return err
		}
		eval := evaliface.FromContext(ctx)
		if eval == nil {
			return &object.Error{Message: "exec_stream: evaluator not available"}
		}
		opts := execOptsFromKwargs(kwargs)
		fn := func(stream, line string) {
			eval.CallObjectFunction(ctx, callback, []object.Object{
				&object.String{Value: stream},
				&object.String{Value: line},
			}, nil, nil)
		}
		res, goErr := ci.driver.ExecStream(ctx, nameOrID, command, opts, fn)
		if goErr != nil {
			return &object.Error{Message: goErr.Error()}
		}
		return execResultToDict(res)
	}, `exec_stream(name_or_id, command, callback, **kwargs) - Run a command in a running container and stream output line by line

Calls callback(stream, line) for each line of output as it arrives.
stream is "stdout" or "stderr".

Parameters:
  name_or_id (str): Container name or ID
  command (list): Command and arguments e.g. ["/bin/sh", "-c", "echo hi"]
  callback (callable): Function called with (stream, line) for each output line
  env (list, optional): Environment variables e.g. ["KEY=value"]
  workdir (str, optional): Working directory inside the container
  user (str, optional): User to run as e.g. "root" or "1000:1000"

Returns:
  dict: {exit_code} — stdout and stderr are empty (output was streamed to callback)

Example:
  def on_line(stream, line):
    print(f"[{stream}] {line}")

  result = c.exec_stream("app", ["/bin/sh", "-c", "for i in 1 2 3; do echo $i; done"], on_line)
  print("exit:", result["exit_code"])`)

	cb.MethodWithHelp("image_list", func(self *object.Instance, ctx context.Context) object.Object {
		ci, err := getClientInstance(self)
		if err != nil {
			return err
		}
		images, goErr := ci.driver.ImageList(ctx)
		if goErr != nil {
			return &object.Error{Message: goErr.Error()}
		}
		elements := make([]object.Object, len(images))
		for i, img := range images {
			elements[i] = object.NewStringDict(map[string]object.Object{
				"id":        &object.String{Value: img.ID},
				"reference": &object.String{Value: img.Reference},
				"digest":    &object.String{Value: img.Digest},
				"size":      object.NewInteger(img.Size),
			})
		}
		return &object.List{Elements: elements}
	}, `image_list() - List locally available images

Returns:
  list: List of dicts with {id, reference, digest, size}
    - id (str): Image ID (digest for Apple, full ID for Docker/Podman)
    - reference (str): Image reference e.g. "ubuntu:24.04"
    - digest (str): Content digest e.g. "sha256:abc123..."
    - size (int): Manifest size in bytes

Example:
  for img in c.image_list():
    print(img["reference"], img["digest"])`)

	cb.MethodWithHelp("image_pull", func(self *object.Instance, ctx context.Context, image string) object.Object {
		ci, err := getClientInstance(self)
		if err != nil {
			return err
		}
		if goErr := ci.driver.Pull(ctx, image); goErr != nil {
			return &object.Error{Message: goErr.Error()}
		}
		return &object.Null{}
	}, `image_pull(image) - Pull an image from a registry

Parameters:
  image (str): Image reference e.g. "ubuntu:24.04"

Example:
  c.image_pull("ubuntu:24.04")`)

	cb.MethodWithHelp("image_remove", func(self *object.Instance, ctx context.Context, image string) object.Object {
		ci, err := getClientInstance(self)
		if err != nil {
			return err
		}
		if goErr := ci.driver.ImageRemove(ctx, image); goErr != nil {
			return &object.Error{Message: goErr.Error()}
		}
		return &object.Null{}
	}, `image_remove(image) - Remove a local image

Parameters:
  image (str): Image reference e.g. "ubuntu:24.04"

Example:
  c.image_remove("ubuntu:24.04")`)

	cb.MethodWithHelp("run", func(self *object.Instance, ctx context.Context, kwargs object.Kwargs, image string) object.Object {
		ci, err := getClientInstance(self)
		if err != nil {
			return err
		}
		opts := RunOptions{
			Name:       kwargs.MustGetString("name", ""),
			Network:    kwargs.MustGetString("network", ""),
			Privileged: kwargs.MustGetBool("privileged", false),
		}
		for _, p := range kwargs.MustGetList("ports", nil) {
			if s, e := p.AsString(); e == nil {
				opts.Ports = append(opts.Ports, s)
			}
		}
		for _, e := range kwargs.MustGetList("env", nil) {
			if s, goErr := e.AsString(); goErr == nil {
				opts.Env = append(opts.Env, s)
			}
		}
		for _, v := range kwargs.MustGetList("volumes", nil) {
			if s, e := v.AsString(); e == nil {
				opts.Volumes = append(opts.Volumes, s)
			}
		}
		for _, cmd := range kwargs.MustGetList("command", nil) {
			if s, e := cmd.AsString(); e == nil {
				opts.Command = append(opts.Command, s)
			}
		}
		id, goErr := ci.driver.Run(ctx, image, opts)
		if goErr != nil {
			return &object.Error{Message: goErr.Error()}
		}
		return &object.String{Value: id}
	}, `run(image, **kwargs) - Create and start a container

Parameters:
  image (str): Image reference e.g. "nginx:latest"
  name (str, optional): Container name
  ports (list, optional): Port mappings e.g. ["8080:80"]
  env (list, optional): Environment variables e.g. ["KEY=value"]
  volumes (list, optional): Volume mounts e.g. ["mydata:/data"]
  command (list, optional): Override command e.g. ["/bin/sh", "-c", "echo hi"]
  network (str, optional): Network name
  privileged (bool, optional): Run privileged (default False)

Returns:
  str: Container ID

Example:
  id = c.run("nginx:latest", name="web", ports=["8080:80"])`)

	cb.MethodWithHelp("stop", func(self *object.Instance, ctx context.Context, nameOrID string) object.Object {
		ci, err := getClientInstance(self)
		if err != nil {
			return err
		}
		if goErr := ci.driver.Stop(ctx, nameOrID); goErr != nil {
			return &object.Error{Message: goErr.Error()}
		}
		return &object.Null{}
	}, `stop(name_or_id) - Stop a running container

Parameters:
  name_or_id (str): Container name or ID

Example:
  c.stop("web")`)

	cb.MethodWithHelp("remove", func(self *object.Instance, ctx context.Context, nameOrID string) object.Object {
		ci, err := getClientInstance(self)
		if err != nil {
			return err
		}
		if goErr := ci.driver.Remove(ctx, nameOrID); goErr != nil {
			return &object.Error{Message: goErr.Error()}
		}
		return &object.Null{}
	}, `remove(name_or_id) - Remove a stopped container

Parameters:
  name_or_id (str): Container name or ID

Example:
  c.remove("web")`)

	cb.MethodWithHelp("inspect", func(self *object.Instance, ctx context.Context, nameOrID string) object.Object {
		ci, err := getClientInstance(self)
		if err != nil {
			return err
		}
		info, goErr := ci.driver.Inspect(ctx, nameOrID)
		if goErr != nil {
			return &object.Error{Message: goErr.Error()}
		}
		return containerInfoToDict(info)
	}, `inspect(name_or_id) - Get container details

Parameters:
  name_or_id (str): Container name or ID

Returns:
  dict: {id, name, status, image, running}

Example:
  info = c.inspect("web")
  print(info["status"])`)

	cb.MethodWithHelp("list", func(self *object.Instance, ctx context.Context) object.Object {
		ci, err := getClientInstance(self)
		if err != nil {
			return err
		}
		items, goErr := ci.driver.List(ctx)
		if goErr != nil {
			return &object.Error{Message: goErr.Error()}
		}
		elements := make([]object.Object, len(items))
		for i, item := range items {
			elements[i] = containerInfoToDict(&item)
		}
		return &object.List{Elements: elements}
	}, `list() - List all containers

Returns:
  list: List of dicts with {id, name, status, image, running}

Example:
  for item in c.list():
    print(item["name"], item["status"])`)

	cb.MethodWithHelp("volume_create", func(self *object.Instance, ctx context.Context, kwargs object.Kwargs, name string) object.Object {
		ci, err := getClientInstance(self)
		if err != nil {
			return err
		}
		size := kwargs.MustGetString("size", "")
		if goErr := ci.driver.VolumeCreate(ctx, name, size); goErr != nil {
			return &object.Error{Message: goErr.Error()}
		}
		return &object.Null{}
	}, `volume_create(name, **kwargs) - Create a named volume

Parameters:
  name (str): Volume name
  size (str, optional): Volume size e.g. "20G" or "512M" (Apple Containers only;
                        silently ignored for Docker and Podman)

Example:
  c.volume_create("mydata")
  c.volume_create("mydata", size="20G")`)

	cb.MethodWithHelp("volume_remove", func(self *object.Instance, ctx context.Context, name string) object.Object {
		ci, err := getClientInstance(self)
		if err != nil {
			return err
		}
		if goErr := ci.driver.VolumeRemove(ctx, name); goErr != nil {
			return &object.Error{Message: goErr.Error()}
		}
		return &object.Null{}
	}, `volume_remove(name) - Remove a named volume

Parameters:
  name (str): Volume name

Example:
  c.volume_remove("mydata")`)

	cb.MethodWithHelp("volume_list", func(self *object.Instance, ctx context.Context) object.Object {
		ci, err := getClientInstance(self)
		if err != nil {
			return err
		}
		names, goErr := ci.driver.VolumeList(ctx)
		if goErr != nil {
			return &object.Error{Message: goErr.Error()}
		}
		elements := make([]object.Object, len(names))
		for i, n := range names {
			elements[i] = &object.String{Value: n}
		}
		return &object.List{Elements: elements}
	}, `volume_list() - List named volumes

Returns:
  list: Volume names

Example:
  for v in c.volume_list():
    print(v)`)

	return cb.Build()
}

func execOptsFromKwargs(kwargs object.Kwargs) ExecOptions {
	opts := ExecOptions{
		WorkDir: kwargs.MustGetString("workdir", ""),
		User:    kwargs.MustGetString("user", ""),
	}
	for _, e := range kwargs.MustGetList("env", nil) {
		if s, err := e.AsString(); err == nil {
			opts.Env = append(opts.Env, s)
		}
	}
	return opts
}

func execResultToDict(res *ExecResult) *object.Dict {
	return object.NewStringDict(map[string]object.Object{
		"stdout":    &object.String{Value: res.Stdout},
		"stderr":    &object.String{Value: res.Stderr},
		"exit_code": object.NewInteger(int64(res.ExitCode)),
	})
}

func containerInfoToDict(info *ContainerInfo) *object.Dict {
	return object.NewStringDict(map[string]object.Object{
		"id":      &object.String{Value: info.ID},
		"name":    &object.String{Value: info.Name},
		"status":  &object.String{Value: info.Status},
		"image":   &object.String{Value: info.Image},
		"running": object.NewBoolean(info.Running),
	})
}
