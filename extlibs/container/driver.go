package container

import (
	"context"
	"time"
)

const (
	DriverDocker = "docker"
	DriverPodman = "podman"
	DriverApple  = "apple"
)

// RunOptions holds parameters for starting a container.
type RunOptions struct {
	Name       string
	Ports      []string // "hostPort:containerPort"
	Env        []string // "KEY=value"
	Volumes    []string // "name:/path" or "/host:/container"
	Command    []string
	Privileged bool
	Network    string
}

// ContainerInfo is the common container representation returned by inspect/list.
type ContainerInfo struct {
	ID      string
	Name    string
	Status  string
	Image   string
	Running bool
}

// ExecOptions holds optional parameters for Exec / ExecStream.
type ExecOptions struct {
	Env     []string
	WorkDir string
	User    string
}

// ExecResult holds the output of a completed exec.
type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// ImageInfo is the common image representation returned by image_list.
type ImageInfo struct {
	ID        string
	Reference string
	Digest    string
	Size      int64
}

// ContainerDriver is the interface every runtime driver must implement.
type ContainerDriver interface {
	// Login authenticates with a container registry.
	Login(ctx context.Context, server, username, password string) error

	// Pull fetches an image from a registry.
	Pull(ctx context.Context, image string) error

	// Exec runs a command inside a running container and captures output.
	Exec(ctx context.Context, nameOrID string, command []string, opts ExecOptions) (*ExecResult, error)

	// ExecStream runs a command inside a running container, calling fn for each output line.
	ExecStream(ctx context.Context, nameOrID string, command []string, opts ExecOptions, fn func(stream, line string)) (*ExecResult, error)

	// ImageList returns locally available images.
	ImageList(ctx context.Context) ([]ImageInfo, error)

	// ImageRemove removes a local image.
	ImageRemove(ctx context.Context, image string) error

	// Run creates and starts a container, returning its ID.
	Run(ctx context.Context, image string, opts RunOptions) (string, error)

	// Stop stops a running container (graceful then force).
	Stop(ctx context.Context, nameOrID string) error

	// WaitStopped polls until the container is no longer running, or the
	// timeout elapses. Returns true if the container reached a stopped
	// state, false if the timeout was hit first. An error is returned if
	// the container cannot be found/inspected for reasons other than "not
	// running" (e.g. it was fully removed while waiting is treated as stopped).
	WaitStopped(ctx context.Context, nameOrID string, timeout time.Duration) (bool, error)

	// Remove removes a stopped container.
	Remove(ctx context.Context, nameOrID string) error

	// Inspect returns details about a container.
	Inspect(ctx context.Context, nameOrID string) (*ContainerInfo, error)

	// List returns all containers (running and stopped).
	List(ctx context.Context) ([]ContainerInfo, error)

	// VolumeCreate creates a named volume. size is optional (e.g. "20G") and
	// only honoured by Apple Containers; other drivers silently ignore it.
	VolumeCreate(ctx context.Context, name, size string) error

	// VolumeRemove removes a named volume.
	VolumeRemove(ctx context.Context, name string) error

	// VolumeList returns all named volumes.
	VolumeList(ctx context.Context) ([]string, error)
}
