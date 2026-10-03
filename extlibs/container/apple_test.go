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

func TestParseAppleListsBothFormats(t *testing.T) {
	// container CLI 1.1 nests names under "configuration".
	images, err := parseAppleImageList(`[{"configuration":{"descriptor":{"digest":"sha256:aa","size":10},"name":"docker.io/paularlott/knot-ubuntu:26.04"},"id":"aa"}]`)
	if err != nil || len(images) != 1 || images[0].Reference != "docker.io/paularlott/knot-ubuntu:26.04" || images[0].Digest != "sha256:aa" || images[0].ID != "aa" {
		t.Fatalf("1.1 image list: %+v, %v", images, err)
	}
	// Earlier CLIs use top-level fields.
	images, err = parseAppleImageList(`[{"reference":"ubuntu:24.04","descriptor":{"digest":"sha256:bb","size":20}}]`)
	if err != nil || len(images) != 1 || images[0].Reference != "ubuntu:24.04" || images[0].ID != "sha256:bb" || images[0].Size != 20 {
		t.Fatalf("legacy image list: %+v, %v", images, err)
	}

	vols, err := parseAppleVolumeList(`[{"configuration":{"name":"data","driver":"local"},"id":"data"}]`)
	if err != nil || len(vols) != 1 || vols[0] != "data" {
		t.Fatalf("1.1 volume list: %v, %v", vols, err)
	}
	vols, err = parseAppleVolumeList(`[{"name":"old"}]`)
	if err != nil || len(vols) != 1 || vols[0] != "old" {
		t.Fatalf("legacy volume list: %v, %v", vols, err)
	}
}

func TestParseAppleInspectStatusFormats(t *testing.T) {
	// container CLI 1.1: status is an object.
	item, err := parseAppleInspectOutput(`[{"configuration":{"id":"web","image":{"reference":"docker.io/x:1"}},"id":"web","status":{"state":"running","startedDate":"2026-10-03T08:33:13Z"}}]`)
	if err != nil || string(item.Status) != "running" || item.Configuration.ID != "web" || item.Configuration.Image.Reference != "docker.io/x:1" {
		t.Fatalf("1.1 inspect: %+v, %v", item, err)
	}
	// Earlier CLIs: status is a string.
	item, err = parseAppleInspectOutput(`[{"configuration":{"id":"db","image":{"reference":"x:2"}},"status":"stopped"}]`)
	if err != nil || string(item.Status) != "stopped" {
		t.Fatalf("legacy inspect: %+v, %v", item, err)
	}
}
