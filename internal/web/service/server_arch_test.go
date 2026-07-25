package service

import "testing"

func TestXrayReleaseArch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		goarch string
		goarm  string
		want   string
	}{
		{name: "amd64", goarch: "amd64", want: "64"},
		{name: "386", goarch: "386", want: "32"},
		{name: "arm64", goarch: "arm64", want: "arm64-v8a"},
		{name: "arm v5 numeric", goarch: "arm", goarm: "5", want: "arm32-v5"},
		{name: "arm v6 prefixed", goarch: "arm", goarm: "v6", want: "arm32-v6"},
		{name: "arm v7", goarch: "arm", goarm: "7", want: "arm32-v7a"},
		{name: "arm default", goarch: "arm", want: "arm32-v7a"},
		{name: "s390x", goarch: "s390x", want: "s390x"},
		{name: "unknown passthrough", goarch: "riscv64", want: "riscv64"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := xrayReleaseArch(tt.goarch, tt.goarm); got != tt.want {
				t.Fatalf("xrayReleaseArch(%q, %q) = %q, want %q", tt.goarch, tt.goarm, got, tt.want)
			}
		})
	}
}
