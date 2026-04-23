//go:build darwin

package container

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

// appleClient drives Apple Containers via the `container` CLI.
type appleClient struct{}

func newAppleDriver() (ContainerDriver, error) {
	return &appleClient{}, nil
}

func (c *appleClient) run(ctx context.Context, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "container", args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// Login implements ContainerDriver.
func (c *appleClient) Login(ctx context.Context, server, username, password string) error {
	if server == "" {
		server = "registry-1.docker.io"
	}
	cmd := exec.CommandContext(ctx, "container", "registry", "login", "--username", username, "--password-stdin", server)
	cmd.Stdin = strings.NewReader(password)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("login: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// Pull implements ContainerDriver.
func (c *appleClient) Pull(ctx context.Context, image string) error {
	_, err := c.run(ctx, "pull", image)
	return err
}

// Exec implements ContainerDriver.
func (c *appleClient) Exec(ctx context.Context, nameOrID string, command []string, opts ExecOptions) (*ExecResult, error) {
	var stdout, stderr strings.Builder
	fn := func(stream, line string) {
		if stream == "stderr" {
			stderr.WriteString(line)
			stderr.WriteByte('\n')
		} else {
			stdout.WriteString(line)
			stdout.WriteByte('\n')
		}
	}
	res, err := c.ExecStream(ctx, nameOrID, command, opts, fn)
	if err != nil {
		return nil, err
	}
	res.Stdout = strings.TrimRight(stdout.String(), "\n")
	res.Stderr = strings.TrimRight(stderr.String(), "\n")
	return res, nil
}

// ExecStream implements ContainerDriver.
func (c *appleClient) ExecStream(ctx context.Context, nameOrID string, command []string, opts ExecOptions, fn func(stream, line string)) (*ExecResult, error) {
	args := []string{"exec"}
	for _, e := range opts.Env {
		args = append(args, "-e", e)
	}
	if opts.WorkDir != "" {
		args = append(args, "--cwd", opts.WorkDir)
	}
	if opts.User != "" {
		args = append(args, "--user", opts.User)
	}
	args = append(args, nameOrID)
	args = append(args, command...)

	cmd := exec.CommandContext(ctx, "container", args...)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("exec stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("exec stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("exec start: %w", err)
	}

	var wg sync.WaitGroup
	scanPipe := func(pipe io.Reader, streamName string) {
		defer wg.Done()
		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			fn(streamName, scanner.Text())
		}
	}

	wg.Add(2)
	go scanPipe(stdoutPipe, "stdout")
	go scanPipe(stderrPipe, "stderr")
	wg.Wait()

	exitCode := 0
	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("exec wait: %w", err)
		}
	}
	return &ExecResult{ExitCode: exitCode}, nil
}

// ImageList implements ContainerDriver.
func (c *appleClient) ImageList(ctx context.Context) ([]ImageInfo, error) {
	out, err := c.run(ctx, "image", "list", "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("image list: %s", out)
	}
	var raw []struct {
		Reference string `json:"reference"`
		Descriptor struct {
			Digest string `json:"digest"`
			Size   int64  `json:"size"`
		} `json:"descriptor"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("image list: failed to parse output")
	}
	result := make([]ImageInfo, len(raw))
	for i, r := range raw {
		result[i] = ImageInfo{
			ID:        r.Descriptor.Digest,
			Reference: r.Reference,
			Digest:    r.Descriptor.Digest,
			Size:      r.Descriptor.Size,
		}
	}
	return result, nil
}

// ImageRemove implements ContainerDriver.
func (c *appleClient) ImageRemove(ctx context.Context, image string) error {
	out, err := c.run(ctx, "image", "rm", image)
	if err != nil && !strings.Contains(out, "not found") {
		return fmt.Errorf("image remove: %s", out)
	}
	return nil
}

// Run implements ContainerDriver.
func (c *appleClient) Run(ctx context.Context, image string, opts RunOptions) (string, error) {
	args := []string{"run", "-d"}
	if opts.Name != "" {
		args = append(args, "--name", opts.Name)
	}
	for _, e := range opts.Env {
		args = append(args, "-e", e)
	}
	for _, p := range opts.Ports {
		args = append(args, "-p", p)
	}
	for _, v := range opts.Volumes {
		args = append(args, "-v", v)
	}
	if opts.Network != "" {
		args = append(args, "--network", opts.Network)
	}
	args = append(args, image)
	args = append(args, opts.Command...)

	out, err := c.run(ctx, args...)
	if err != nil {
		return "", fmt.Errorf("container run: %s", out)
	}
	return out, nil
}

// Stop implements ContainerDriver.
func (c *appleClient) Stop(ctx context.Context, nameOrID string) error {
	out, err := c.run(ctx, "stop", nameOrID)
	if err != nil && !strings.Contains(out, "not found") {
		return fmt.Errorf("container stop: %s", out)
	}
	return nil
}

// Remove implements ContainerDriver.
func (c *appleClient) Remove(ctx context.Context, nameOrID string) error {
	out, err := c.run(ctx, "rm", nameOrID)
	if err != nil && !strings.Contains(out, "not found") {
		return fmt.Errorf("container remove: %s", out)
	}
	return nil
}

type appleListItem struct {
	Status        string `json:"status"`
	Configuration struct {
		ID    string `json:"id"`
		Image struct {
			Reference string `json:"reference"`
		} `json:"image"`
	} `json:"configuration"`
}

type appleInspect = appleListItem

// Inspect implements ContainerDriver.
func (c *appleClient) Inspect(ctx context.Context, nameOrID string) (*ContainerInfo, error) {
	out, err := c.run(ctx, "inspect", nameOrID)
	if err != nil {
		return nil, fmt.Errorf("container inspect: %s", out)
	}
	var items []appleInspect
	if err := json.Unmarshal([]byte(out), &items); err != nil || len(items) == 0 {
		return nil, fmt.Errorf("container inspect: failed to parse output")
	}
	item := items[0]
	return &ContainerInfo{
		ID:      item.Configuration.ID,
		Name:    item.Configuration.ID,
		Status:  item.Status,
		Image:   item.Configuration.Image.Reference,
		Running: item.Status == "running",
	}, nil
}

// List implements ContainerDriver.
func (c *appleClient) List(ctx context.Context) ([]ContainerInfo, error) {
	out, err := c.run(ctx, "list", "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("container list: %s", out)
	}
	var items []appleListItem
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		return nil, fmt.Errorf("container list: failed to parse output")
	}
	result := make([]ContainerInfo, len(items))
	for i, item := range items {
		result[i] = ContainerInfo{
			ID:      item.Configuration.ID,
			Name:    item.Configuration.ID,
			Status:  item.Status,
			Image:   item.Configuration.Image.Reference,
			Running: item.Status == "running",
		}
	}
	return result, nil
}

// VolumeCreate implements ContainerDriver.
func (c *appleClient) VolumeCreate(ctx context.Context, name string) error {
	out, err := c.run(ctx, "volume", "create", name)
	if err != nil {
		return fmt.Errorf("volume create: %s", out)
	}
	return nil
}

// VolumeRemove implements ContainerDriver.
func (c *appleClient) VolumeRemove(ctx context.Context, name string) error {
	out, err := c.run(ctx, "volume", "rm", name)
	if err != nil && !strings.Contains(out, "not found") {
		return fmt.Errorf("volume remove: %s", out)
	}
	return nil
}

// VolumeList implements ContainerDriver.
func (c *appleClient) VolumeList(ctx context.Context) ([]string, error) {
	out, err := c.run(ctx, "volume", "list", "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("volume list: %s", out)
	}
	var items []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		return nil, fmt.Errorf("volume list: failed to parse output")
	}
	names := make([]string, len(items))
	for i, v := range items {
		names[i] = v.Name
	}
	return names, nil
}
