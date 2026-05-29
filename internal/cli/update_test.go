package cli

import "testing"

func TestCompareReleaseVersions(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want int
	}{
		{name: "equal", a: "1.1.7", b: "1.1.7", want: 0},
		{name: "newer patch", a: "1.1.7", b: "1.1.6", want: 1},
		{name: "older patch", a: "1.1.6", b: "1.1.7", want: -1},
		{name: "missing patch treated as zero", a: "1.2", b: "1.2.0", want: 0},
		{name: "leading v", a: "v1.3.0", b: "1.2.9", want: 1},
		{name: "ignore prerelease suffix", a: "1.1.7-rc1", b: "1.1.7", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compareReleaseVersions(tt.a, tt.b); got != tt.want {
				t.Fatalf("compareReleaseVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
