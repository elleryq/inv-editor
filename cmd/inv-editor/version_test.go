package main

import "testing"

func TestResolveVersion(t *testing.T) {
	tests := []struct {
		name          string
		ldflags       string
		moduleVersion string
		want          string
	}{
		{"ldflags tag", "v1.4.0", "", "1.4.0"},
		{"ldflags without v", "1.4.0", "", "1.4.0"},
		{"ldflags wins over module", "v1.4.0", "v1.3.0", "1.4.0"},
		{"ldflags prerelease", "v1.4.0-rc1", "", "1.4.0-rc1"},
		{"module version", "", "v1.4.0", "1.4.0"},
		{"module pseudo-version", "", "v1.4.1-0.20261124120000-abcdef123456+dirty", "1.4.1-0.20261124120000-abcdef123456+dirty"},
		{"devel module", "", "(devel)", "dev"},
		{"nothing", "", "", "dev"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveVersion(tt.ldflags, tt.moduleVersion); got != tt.want {
				t.Errorf("resolveVersion(%q, %q) = %q, want %q", tt.ldflags, tt.moduleVersion, got, tt.want)
			}
		})
	}
}
