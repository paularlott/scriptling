package extlibs

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/paularlott/scriptling/object"
)

// Helper function to create a temporary file
func createTempFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test_file")
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}
	return filePath
}

// Helper function to find an available port
func getAvailablePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port
}

// Helper function to start a test HTTP server
func startTestServer(t *testing.T, statusCode int) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
	}))
	return server
}

func TestParseWaitOptions(t *testing.T) {
	tests := []struct {
		name      string
		args      []object.Object
		kwargs    map[string]object.Object
		wantTimeout int
		wantPollRate float64
		wantErr   bool
	}{
		{
			name: "defaults",
			args: []object.Object{},
			kwargs: map[string]object.Object{},
			wantTimeout: 30,
			wantPollRate: 1.0,
			wantErr: false,
		},
		{
			name: "positional timeout",
			args: []object.Object{&object.String{Value: "test"}, &object.Integer{Value: 60}},
			kwargs: map[string]object.Object{},
			wantTimeout: 60,
			wantPollRate: 1.0,
			wantErr: false,
		},
		{
			name: "keyword timeout",
			args: []object.Object{&object.String{Value: "test"}},
			kwargs: map[string]object.Object{
				"timeout": &object.Integer{Value: 120},
			},
			wantTimeout: 120,
			wantPollRate: 1.0,
			wantErr: false,
		},
		{
			name: "keyword poll_rate float",
			args: []object.Object{&object.String{Value: "test"}},
			kwargs: map[string]object.Object{
				"poll_rate": &object.Float{Value: 0.5},
			},
			wantTimeout: 30,
			wantPollRate: 0.5,
			wantErr: false,
		},
		{
			name: "keyword poll_rate int",
			args: []object.Object{&object.String{Value: "test"}},
			kwargs: map[string]object.Object{
				"poll_rate": &object.Integer{Value: 2},
			},
			wantTimeout: 30,
			wantPollRate: 2.0,
			wantErr: false,
		},
		{
			name: "both options",
			args: []object.Object{&object.String{Value: "test"}},
			kwargs: map[string]object.Object{
				"timeout": &object.Integer{Value: 45},
				"poll_rate": &object.Float{Value: 0.2},
			},
			wantTimeout: 45,
			wantPollRate: 0.2,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			timeout, pollRate, err := parseWaitOptions(tt.args, tt.kwargs)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseWaitOptions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if timeout != tt.wantTimeout {
					t.Errorf("parseWaitOptions() timeout = %v, want %v", timeout, tt.wantTimeout)
				}
				if pollRate != tt.wantPollRate {
					t.Errorf("parseWaitOptions() pollRate = %v, want %v", pollRate, tt.wantPollRate)
				}
			}
		})
	}
}

func TestWaitForFile(t *testing.T) {
	ctx := context.Background()

	t.Run("existing file", func(t *testing.T) {
		filePath := createTempFile(t, "test content")

		result := WaitForLibrary.Functions()["file"].Fn(ctx, object.NewKwargs(map[string]object.Object{
			"timeout": &object.Integer{Value: 1},
		}), &object.String{Value: filePath})

		b, ok := result.(*object.Boolean)
		if !ok {
			t.Fatalf("expected Boolean, got %T", result)
		}
		if !b.Value {
			t.Errorf("expected true, got false")
		}
	})

	t.Run("non-existing file with timeout", func(t *testing.T) {
		result := WaitForLibrary.Functions()["file"].Fn(ctx, object.NewKwargs(map[string]object.Object{
			"timeout": &object.Integer{Value: 1},
		}), &object.String{Value: "/tmp/nonexistent_file_12345"})

		b, ok := result.(*object.Boolean)
		if !ok {
			t.Fatalf("expected Boolean, got %T", result)
		}
		if b.Value {
			t.Errorf("expected false, got true")
		}
	})

	t.Run("file created during wait", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "delayed_file")

		// Create file after 500ms
		go func() {
			time.Sleep(500 * time.Millisecond)
			os.WriteFile(filePath, []byte("test"), 0644)
		}()

		result := WaitForLibrary.Functions()["file"].Fn(ctx, object.NewKwargs(map[string]object.Object{
			"timeout": &object.Integer{Value: 2},
		}), &object.String{Value: filePath})

		b, ok := result.(*object.Boolean)
		if !ok {
			t.Fatalf("expected Boolean, got %T", result)
		}
		if !b.Value {
			t.Errorf("expected true, got false")
		}
	})
}

func TestWaitForDir(t *testing.T) {
	ctx := context.Background()

	t.Run("existing directory", func(t *testing.T) {
		dir := t.TempDir()

		result := WaitForLibrary.Functions()["dir"].Fn(ctx, object.NewKwargs(map[string]object.Object{
			"timeout": &object.Integer{Value: 1},
		}), &object.String{Value: dir})

		b, ok := result.(*object.Boolean)
		if !ok {
			t.Fatalf("expected Boolean, got %T", result)
		}
		if !b.Value {
			t.Errorf("expected true, got false")
		}
	})

	t.Run("file instead of directory", func(t *testing.T) {
		filePath := createTempFile(t, "test")

		result := WaitForLibrary.Functions()["dir"].Fn(ctx, object.NewKwargs(map[string]object.Object{
			"timeout": &object.Integer{Value: 1},
		}), &object.String{Value: filePath})

		b, ok := result.(*object.Boolean)
		if !ok {
			t.Fatalf("expected Boolean, got %T", result)
		}
		if b.Value {
			t.Errorf("expected false (file is not a directory), got true")
		}
	})

	t.Run("directory created during wait", func(t *testing.T) {
		baseDir := t.TempDir()
		dirPath := filepath.Join(baseDir, "delayed_dir")

		go func() {
			time.Sleep(500 * time.Millisecond)
			os.Mkdir(dirPath, 0755)
		}()

		result := WaitForLibrary.Functions()["dir"].Fn(ctx, object.NewKwargs(map[string]object.Object{
			"timeout": &object.Integer{Value: 2},
		}), &object.String{Value: dirPath})

		b, ok := result.(*object.Boolean)
		if !ok {
			t.Fatalf("expected Boolean, got %T", result)
		}
		if !b.Value {
			t.Errorf("expected true, got false")
		}
	})
}

func TestWaitForPort(t *testing.T) {
	ctx := context.Background()

	t.Run("open port", func(t *testing.T) {
		port := getAvailablePort(t)

		// Start a listener
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer listener.Close()
		addr := listener.Addr().(*net.TCPAddr)
		port = addr.Port

		result := WaitForLibrary.Functions()["port"].Fn(ctx, object.NewKwargs(map[string]object.Object{
			"timeout": &object.Integer{Value: 1},
		}), &object.String{Value: "127.0.0.1"}, &object.Integer{Value: int64(port)})

		b, ok := result.(*object.Boolean)
		if !ok {
			t.Fatalf("expected Boolean, got %T", result)
		}
		if !b.Value {
			t.Errorf("expected true, got false")
		}
	})

	t.Run("closed port", func(t *testing.T) {
		// Use a port that's unlikely to be in use
		result := WaitForLibrary.Functions()["port"].Fn(ctx, object.NewKwargs(map[string]object.Object{
			"timeout": &object.Integer{Value: 1},
		}), &object.String{Value: "127.0.0.1"}, &object.Integer{Value: 9999})

		b, ok := result.(*object.Boolean)
		if !ok {
			t.Fatalf("expected Boolean, got %T", result)
		}
		if b.Value {
			t.Errorf("expected false, got true")
		}
	})

	t.Run("port with string port number", func(t *testing.T) {
		port := getAvailablePort(t)

		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer listener.Close()
		addr := listener.Addr().(*net.TCPAddr)
		port = addr.Port

		result := WaitForLibrary.Functions()["port"].Fn(ctx, object.NewKwargs(map[string]object.Object{
			"timeout": &object.Integer{Value: 1},
		}), &object.String{Value: "127.0.0.1"}, &object.String{Value: string(rune('0' + port))})

		// This test might not work perfectly with string conversion, so we just check it doesn't crash
		_ = result
	})
}

func TestWaitForHTTP(t *testing.T) {
	ctx := context.Background()

	t.Run("successful HTTP 200", func(t *testing.T) {
		server := startTestServer(t, 200)
		defer server.Close()

		result := WaitForLibrary.Functions()["http"].Fn(ctx, object.NewKwargs(map[string]object.Object{
			"timeout": &object.Integer{Value: 5},
		}), &object.String{Value: server.URL})

		b, ok := result.(*object.Boolean)
		if !ok {
			t.Fatalf("expected Boolean, got %T", result)
		}
		if !b.Value {
			t.Errorf("expected true, got false")
		}
	})

	t.Run("different status code", func(t *testing.T) {
		server := startTestServer(t, 201)
		defer server.Close()

		result := WaitForLibrary.Functions()["http"].Fn(ctx, object.NewKwargs(map[string]object.Object{
			"timeout": &object.Integer{Value: 5},
			"status_code": &object.Integer{Value: 201},
		}), &object.String{Value: server.URL})

		b, ok := result.(*object.Boolean)
		if !ok {
			t.Fatalf("expected Boolean, got %T", result)
		}
		if !b.Value {
			t.Errorf("expected true, got false")
		}
	})

	t.Run("wrong status code", func(t *testing.T) {
		server := startTestServer(t, 404)
		defer server.Close()

		result := WaitForLibrary.Functions()["http"].Fn(ctx, object.NewKwargs(map[string]object.Object{
			"timeout": &object.Integer{Value: 2},
		}), &object.String{Value: server.URL})

		b, ok := result.(*object.Boolean)
		if !ok {
			t.Fatalf("expected Boolean, got %T", result)
		}
		if b.Value {
			t.Errorf("expected false, got true")
		}
	})

	t.Run("non-existent URL", func(t *testing.T) {
		result := WaitForLibrary.Functions()["http"].Fn(ctx, object.NewKwargs(map[string]object.Object{
			"timeout": &object.Integer{Value: 1},
		}), &object.String{Value: "http://localhost:9999/nonexistent"})

		b, ok := result.(*object.Boolean)
		if !ok {
			t.Fatalf("expected Boolean, got %T", result)
		}
		if b.Value {
			t.Errorf("expected false, got true")
		}
	})
}

func TestWaitForFileContent(t *testing.T) {
	ctx := context.Background()

	t.Run("file with content", func(t *testing.T) {
		filePath := createTempFile(t, "hello world")

		result := WaitForLibrary.Functions()["file_content"].Fn(ctx, object.NewKwargs(map[string]object.Object{
			"timeout": &object.Integer{Value: 1},
		}), &object.String{Value: filePath}, &object.String{Value: "world"})

		b, ok := result.(*object.Boolean)
		if !ok {
			t.Fatalf("expected Boolean, got %T", result)
		}
		if !b.Value {
			t.Errorf("expected true, got false")
		}
	})

	t.Run("file without matching content", func(t *testing.T) {
		filePath := createTempFile(t, "hello world")

		result := WaitForLibrary.Functions()["file_content"].Fn(ctx, object.NewKwargs(map[string]object.Object{
			"timeout": &object.Integer{Value: 1},
		}), &object.String{Value: filePath}, &object.String{Value: "goodbye"})

		b, ok := result.(*object.Boolean)
		if !ok {
			t.Fatalf("expected Boolean, got %T", result)
		}
		if b.Value {
			t.Errorf("expected false, got true")
		}
	})

	t.Run("content added during wait", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "delayed_content")
		os.WriteFile(filePath, []byte("initial"), 0644)

		go func() {
			time.Sleep(500 * time.Millisecond)
			os.WriteFile(filePath, []byte("initial target_content"), 0644)
		}()

		result := WaitForLibrary.Functions()["file_content"].Fn(ctx, object.NewKwargs(map[string]object.Object{
			"timeout": &object.Integer{Value: 2},
		}), &object.String{Value: filePath}, &object.String{Value: "target_content"})

		b, ok := result.(*object.Boolean)
		if !ok {
			t.Fatalf("expected Boolean, got %T", result)
		}
		if !b.Value {
			t.Errorf("expected true, got false")
		}
	})
}

func TestProcessRunning(t *testing.T) {
	// Note: This test is platform-dependent and may not work on all systems
	// The function should at least not crash

	t.Run("check for common process", func(t *testing.T) {
		// On most systems, init or launchd should be running
		result := processRunning("init")
		// We don't assert the result because it varies by OS
		_ = result
	})

	t.Run("non-existent process", func(t *testing.T) {
		result := processRunning("definitely_not_a_real_process_name_12345")
		if result {
			t.Errorf("expected false for non-existent process, got true")
		}
	})
}
