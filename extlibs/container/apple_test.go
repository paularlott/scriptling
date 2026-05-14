//go:build darwin

package container

import "testing"

func TestNormalizeContainerReference(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "plain id",
			in:   "abc123\n",
			want: "abc123",
		},
		{
			name: "apple cli progress output",
			in:   "[0/6] [0s]\n[1/6] Fetching image [0s]\n[6/6] Starting container [0s]\npaul-mtest\n",
			want: "paul-mtest",
		},
		{
			name: "carriage returns and whitespace",
			in:   "\r\n  step-one  \r\n  final-name  \r\n",
			want: "final-name",
		},
		{
			name: "empty output",
			in:   " \n\t\r\n",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeContainerReference(tt.in)
			if got != tt.want {
				t.Fatalf("normalizeContainerReference(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseAppleInspectOutput(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		item, err := parseAppleInspectOutput("[]")
		if err == nil || err.Error() != "container inspect: not found" {
			t.Fatalf("expected not found error, got item=%v err=%v", item, err)
		}
	})

	t.Run("valid item", func(t *testing.T) {
		item, err := parseAppleInspectOutput(`[{"status":"running","configuration":{"id":"paul-mtest","image":{"reference":"alpine:latest"}}}]`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if item.Configuration.ID != "paul-mtest" {
			t.Fatalf("expected id paul-mtest, got %q", item.Configuration.ID)
		}
		if item.Status != "running" {
			t.Fatalf("expected status running, got %q", item.Status)
		}
	})
}
