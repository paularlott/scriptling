package container

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// dockerClient talks to the Docker/Podman REST API over a Unix socket or TCP.
type dockerClient struct {
	httpClient *http.Client
	baseURL    string
	credsMu    sync.RWMutex
	creds      map[string]dockerAuthConfig // keyed by registry hostname
}

type dockerAuthConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
	ServerAddress string `json:"serveraddress"`
}

// newDockerClient creates a client from an endpoint string.
// Supported forms:
//   - /var/run/docker.sock          — Unix socket (raw path)
//   - unix:///var/run/docker.sock   — Unix socket (URI)
//   - tcp://host:2375               — plain TCP
//   - host:2375                     — plain TCP (shorthand)
//   - https://host:2376             — TLS TCP
func newDockerClient(endpoint string) *dockerClient {
	// Normalise: strip unix:// prefix → raw socket path
	if after, ok := strings.CutPrefix(endpoint, "unix://"); ok {
		endpoint = after
	} else if after, ok := strings.CutPrefix(endpoint, "unix:"); ok {
		endpoint = after
	}

	// Unix socket: starts with / or is a relative path without ://
	if strings.HasPrefix(endpoint, "/") || (!strings.Contains(endpoint, "://") && !strings.Contains(endpoint, ":")) {
		transport := &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", endpoint)
			},
		}
		return &dockerClient{
			httpClient: &http.Client{Transport: transport, Timeout: 30 * time.Second},
			baseURL:    "http://localhost",
			creds:      map[string]dockerAuthConfig{},
		}
	}

	// TCP: strip tcp:// prefix and build base URL
	if after, ok := strings.CutPrefix(endpoint, "tcp://"); ok {
		endpoint = after
	}

	baseURL := endpoint
	if !strings.HasPrefix(baseURL, "http") {
		baseURL = "http://" + baseURL
	}

	return &dockerClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    baseURL,
		creds:      map[string]dockerAuthConfig{},
	}
}

func (c *dockerClient) url(path string) string {
	return c.baseURL + "/v1.41" + path
}

func (c *dockerClient) do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.url(path), bodyReader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.httpClient.Do(req)
}

func (c *dockerClient) doJSON(ctx context.Context, method, path string, body, out any, wantStatus int) error {
	resp, err := c.do(ctx, method, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	io.Copy(io.Discard, resp.Body)
	return nil
}

// Login implements ContainerDriver.
func (c *dockerClient) Login(ctx context.Context, server, username, password string) error {
	if server == "" {
		server = "https://index.docker.io/v1/"
	}
	body := dockerAuthConfig{Username: username, Password: password, ServerAddress: server}
	if err := c.doJSON(ctx, http.MethodPost, "/auth", body, nil, http.StatusOK); err != nil {
		return fmt.Errorf("login: %w", err)
	}
	host := registryHost(server)
	c.credsMu.Lock()
	c.creds[host] = body
	c.credsMu.Unlock()
	return nil
}

// Pull implements ContainerDriver.
func (c *dockerClient) Pull(ctx context.Context, image string) error {
	name, tag := splitImageTag(image)
	path := fmt.Sprintf("/images/create?fromImage=%s&tag=%s", name, tag)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url(path), nil)
	if err != nil {
		return err
	}
	if authHeader := c.registryAuthHeader(name); authHeader != "" {
		req.Header.Set("X-Registry-Auth", authHeader)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("pull failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	// Scan streaming response for error objects.
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		var obj struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(scanner.Bytes(), &obj) == nil && obj.Error != "" {
			return fmt.Errorf("pull error: %s", obj.Error)
		}
	}
	return scanner.Err()
}

// Exec implements ContainerDriver.
func (c *dockerClient) Exec(ctx context.Context, nameOrID string, command []string, opts ExecOptions) (*ExecResult, error) {
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
func (c *dockerClient) ExecStream(ctx context.Context, nameOrID string, command []string, opts ExecOptions, fn func(stream, line string)) (*ExecResult, error) {
	// Step 1: create exec instance
	type execCreateReq struct {
		AttachStdout bool     `json:"AttachStdout"`
		AttachStderr bool     `json:"AttachStderr"`
		Cmd          []string `json:"Cmd"`
		Env          []string `json:"Env,omitempty"`
		WorkingDir   string   `json:"WorkingDir,omitempty"`
		User         string   `json:"User,omitempty"`
	}
	type execCreateResp struct {
		ID string `json:"Id"`
	}
	var createResp execCreateResp
	if err := c.doJSON(ctx, http.MethodPost, "/containers/"+nameOrID+"/exec",
		execCreateReq{
			AttachStdout: true,
			AttachStderr: true,
			Cmd:          command,
			Env:          opts.Env,
			WorkingDir:   opts.WorkDir,
			User:         opts.User,
		}, &createResp, http.StatusCreated); err != nil {
		return nil, fmt.Errorf("exec create: %w", err)
	}

	// Step 2: start exec with attached streams
	resp, err := c.do(ctx, http.MethodPost, "/exec/"+createResp.ID+"/start",
		map[string]bool{"Detach": false, "Tty": false})
	if err != nil {
		return nil, fmt.Errorf("exec start: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("exec start failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	// Docker multiplexes stdout/stderr: 8-byte header per frame.
	// Header: [stream_type(1), 0, 0, 0, size(4 big-endian)]
	// stream_type: 1=stdout, 2=stderr
	header := make([]byte, 8)
	for {
		if _, err := io.ReadFull(resp.Body, header); err != nil {
			break
		}
		size := int(header[4])<<24 | int(header[5])<<16 | int(header[6])<<8 | int(header[7])
		if size == 0 {
			continue
		}
		payload := make([]byte, size)
		if _, err := io.ReadFull(resp.Body, payload); err != nil {
			break
		}
		streamName := "stdout"
		if header[0] == 2 {
			streamName = "stderr"
		}
		for _, line := range strings.Split(strings.TrimRight(string(payload), "\n"), "\n") {
			fn(streamName, line)
		}
	}

	// Step 3: inspect to get exit code
	type execInspectResp struct {
		ExitCode int  `json:"ExitCode"`
		Running  bool `json:"Running"`
	}
	var inspectResp execInspectResp
	if err := c.doJSON(ctx, http.MethodGet, "/exec/"+createResp.ID+"/json", nil, &inspectResp, http.StatusOK); err != nil {
		return &ExecResult{ExitCode: -1}, nil
	}
	return &ExecResult{ExitCode: inspectResp.ExitCode}, nil
}

// ImageList implements ContainerDriver.
func (c *dockerClient) ImageList(ctx context.Context) ([]ImageInfo, error) {
	var raw []struct {
		ID       string   `json:"Id"`
		RepoTags []string `json:"RepoTags"`
		Size     int64    `json:"Size"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/images/json", nil, &raw, http.StatusOK); err != nil {
		return nil, err
	}
	result := make([]ImageInfo, len(raw))
	for i, r := range raw {
		ref := ""
		if len(r.RepoTags) > 0 {
			ref = r.RepoTags[0]
		}
		result[i] = ImageInfo{
			ID:        r.ID,
			Reference: ref,
			Size:      r.Size,
		}
	}
	return result, nil
}

// ImageRemove implements ContainerDriver.
func (c *dockerClient) ImageRemove(ctx context.Context, image string) error {
	resp, err := c.do(ctx, http.MethodDelete, "/images/"+image+"?force=false", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("image remove failed (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	io.Copy(io.Discard, resp.Body)
	return nil
}

type dockerCreateRequest struct {
	Image        string              `json:"Image"`
	Hostname     string              `json:"Hostname,omitempty"`
	Env          []string            `json:"Env,omitempty"`
	Cmd          []string            `json:"Cmd,omitempty"`
	ExposedPorts map[string]struct{} `json:"ExposedPorts,omitempty"`
	HostConfig   dockerHostConfig    `json:"HostConfig"`
}

type dockerHostConfig struct {
	Binds        []string                 `json:"Binds,omitempty"`
	PortBindings map[string][]portBinding `json:"PortBindings,omitempty"`
	Privileged   bool                     `json:"Privileged,omitempty"`
	NetworkMode  string                   `json:"NetworkMode,omitempty"`
	RestartPolicy struct {
		Name string `json:"Name"`
	} `json:"RestartPolicy"`
}

type portBinding struct {
	HostIP   string `json:"HostIp"`
	HostPort string `json:"HostPort"`
}

// Run implements ContainerDriver.
func (c *dockerClient) Run(ctx context.Context, image string, opts RunOptions) (string, error) {
	exposedPorts := map[string]struct{}{}
	portBindings := map[string][]portBinding{}
	for _, p := range opts.Ports {
		parts := strings.SplitN(p, ":", 2)
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid port mapping %q, expected hostPort:containerPort", p)
		}
		key := portKey(parts[1])
		exposedPorts[key] = struct{}{}
		portBindings[key] = []portBinding{{HostIP: "0.0.0.0", HostPort: parts[0]}}
	}

	req := dockerCreateRequest{
		Image:        image,
		Env:          opts.Env,
		Cmd:          opts.Command,
		ExposedPorts: exposedPorts,
		HostConfig: dockerHostConfig{
			Binds:        opts.Volumes,
			PortBindings: portBindings,
			Privileged:   opts.Privileged,
			NetworkMode:  opts.Network,
		},
	}
	req.HostConfig.RestartPolicy.Name = "unless-stopped"

	name := opts.Name
	queryName := ""
	if name != "" {
		queryName = "?name=" + name
	}

	var createResp struct {
		ID string `json:"Id"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/containers/create"+queryName, req, &createResp, http.StatusCreated); err != nil {
		return "", fmt.Errorf("container create: %w", err)
	}

	if err := c.doJSON(ctx, http.MethodPost, "/containers/"+createResp.ID+"/start", nil, nil, http.StatusNoContent); err != nil {
		// Best-effort cleanup
		c.doJSON(ctx, http.MethodDelete, "/containers/"+createResp.ID, nil, nil, http.StatusNoContent)
		return "", fmt.Errorf("container start: %w", err)
	}

	return createResp.ID, nil
}

// Stop implements ContainerDriver.
func (c *dockerClient) Stop(ctx context.Context, nameOrID string) error {
	// Use a no-timeout client so the stop grace period isn't cut short.
	stopClient := &http.Client{Transport: c.httpClient.Transport}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url("/containers/"+nameOrID+"/stop"), nil)
	if err != nil {
		return err
	}
	resp, err := stopClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	switch resp.StatusCode {
	case http.StatusNoContent, http.StatusNotModified, http.StatusNotFound:
		return nil
	}
	return fmt.Errorf("container stop failed (HTTP %d)", resp.StatusCode)
}

// Remove implements ContainerDriver.
func (c *dockerClient) Remove(ctx context.Context, nameOrID string) error {
	resp, err := c.do(ctx, http.MethodDelete, "/containers/"+nameOrID, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	return fmt.Errorf("container remove failed (HTTP %d)", resp.StatusCode)
}

// Inspect implements ContainerDriver.
func (c *dockerClient) Inspect(ctx context.Context, nameOrID string) (*ContainerInfo, error) {
	var raw struct {
		ID   string `json:"Id"`
		Name string `json:"Name"`
		Config struct {
			Image string `json:"Image"`
		} `json:"Config"`
		State struct {
			Status  string `json:"Status"`
			Running bool   `json:"Running"`
		} `json:"State"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/containers/"+nameOrID+"/json", nil, &raw, http.StatusOK); err != nil {
		return nil, err
	}
	return &ContainerInfo{
		ID:      raw.ID,
		Name:    strings.TrimPrefix(raw.Name, "/"),
		Status:  raw.State.Status,
		Image:   raw.Config.Image,
		Running: raw.State.Running,
	}, nil
}

// List implements ContainerDriver.
func (c *dockerClient) List(ctx context.Context) ([]ContainerInfo, error) {
	var raw []struct {
		ID    string   `json:"Id"`
		Names []string `json:"Names"`
		Image string   `json:"Image"`
		State string   `json:"State"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/containers/json?all=true", nil, &raw, http.StatusOK); err != nil {
		return nil, err
	}
	result := make([]ContainerInfo, len(raw))
	for i, r := range raw {
		name := ""
		if len(r.Names) > 0 {
			name = strings.TrimPrefix(r.Names[0], "/")
		}
		result[i] = ContainerInfo{
			ID:      r.ID,
			Name:    name,
			Status:  r.State,
			Image:   r.Image,
			Running: r.State == "running",
		}
	}
	return result, nil
}

// VolumeCreate implements ContainerDriver.
func (c *dockerClient) VolumeCreate(ctx context.Context, name, _ string) error {
	return c.doJSON(ctx, http.MethodPost, "/volumes/create", map[string]string{"Name": name}, nil, http.StatusCreated)
}

// VolumeRemove implements ContainerDriver.
func (c *dockerClient) VolumeRemove(ctx context.Context, name string) error {
	resp, err := c.do(ctx, http.MethodDelete, "/volumes/"+name+"?force=true", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	return fmt.Errorf("volume remove failed (HTTP %d)", resp.StatusCode)
}

// VolumeList implements ContainerDriver.
func (c *dockerClient) VolumeList(ctx context.Context) ([]string, error) {
	var raw struct {
		Volumes []struct {
			Name string `json:"Name"`
		} `json:"Volumes"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/volumes", nil, &raw, http.StatusOK); err != nil {
		return nil, err
	}
	names := make([]string, len(raw.Volumes))
	for i, v := range raw.Volumes {
		names[i] = v.Name
	}
	return names, nil
}

// registryHost extracts the hostname from a registry server string.
func registryHost(server string) string {
	server = strings.TrimPrefix(server, "https://")
	server = strings.TrimPrefix(server, "http://")
	if idx := strings.Index(server, "/"); idx != -1 {
		server = server[:idx]
	}
	return server
}

// registryAuthHeader returns a base64-encoded X-Registry-Auth value for the
// registry that hosts the given image name, or empty string if no creds stored.
func (c *dockerClient) registryAuthHeader(image string) string {
	// Determine registry host: if image contains a '.' or ':' before the first
	// '/', treat the first segment as the registry; otherwise it's Docker Hub.
	host := "index.docker.io"
	if idx := strings.Index(image, "/"); idx != -1 {
		prefix := image[:idx]
		if strings.ContainsAny(prefix, ".:" ) {
			host = prefix
		}
	}
	c.credsMu.RLock()
	auth, ok := c.creds[host]
	c.credsMu.RUnlock()
	if !ok {
		return ""
	}
	b, _ := json.Marshal(auth)
	return base64.URLEncoding.EncodeToString(b)
}

// portKey ensures a port string has a /tcp suffix.
func portKey(port string) string {
	if !strings.Contains(port, "/") {
		return port + "/tcp"
	}
	return port
}

// splitImageTag splits "image:tag" into ("image", "tag"), defaulting tag to "latest".
func splitImageTag(image string) (string, string) {
	if idx := strings.LastIndex(image, ":"); idx != -1 && !strings.Contains(image[idx:], "/") {
		return image[:idx], image[idx+1:]
	}
	return image, "latest"
}
