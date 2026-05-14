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
