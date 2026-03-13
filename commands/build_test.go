package commands

import (
	"strings"
	"testing"
)

func TestDetectRegistry(t *testing.T) {
	tests := []struct {
		image string
		want  string
	}{
		{image: "myapp:1.2.3", want: "Docker Hub"},
		{image: "localhost:5001/demo:test", want: "Local registry"},
		{image: "ghcr.io/user/app:latest", want: "GitHub Container Registry"},
	}

	for _, tt := range tests {
		got := stripANSI(detectRegistry(tt.image))
		if !strings.Contains(got, tt.want) {
			t.Fatalf("detectRegistry(%q) = %q, want substring %q", tt.image, got, tt.want)
		}
	}
}
