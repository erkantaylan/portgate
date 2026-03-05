package main

import "testing"

func TestIsNewer(t *testing.T) {
	tests := []struct {
		local, remote string
		want          bool
	}{
		// Basic semver comparisons
		{"v0.4.0", "v0.4.1", true},
		{"v0.4.1", "v0.4.0", false},
		{"v0.4.1", "v0.4.1", false},
		{"v0.3.9", "v0.4.0", true},
		{"v1.0.0", "v0.9.9", false},
		{"v0.9.9", "v1.0.0", true},

		// Pre-release is older than same release version
		{"v0.4.1-dev", "v0.4.1", true},
		{"v0.4.1-rc1", "v0.4.1", true},

		// Release is NOT older than same pre-release
		{"v0.4.1", "v0.4.1-dev", false},

		// Both pre-release, same numeric version: neither is newer
		{"v0.4.1-dev", "v0.4.1-rc1", false},

		// Pre-release of lower version vs higher release
		{"v0.4.0-dev", "v0.4.1", true},

		// Pre-release of higher version vs lower release
		{"v0.4.2-dev", "v0.4.1", false},

		// Invalid versions
		{"invalid", "v0.4.1", false},
		{"v0.4.1", "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.local+"_vs_"+tt.remote, func(t *testing.T) {
			got := isNewer(tt.local, tt.remote)
			if got != tt.want {
				t.Errorf("isNewer(%q, %q) = %v, want %v", tt.local, tt.remote, got, tt.want)
			}
		})
	}
}
