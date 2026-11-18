package config

import (
	"testing"
)

func TestResolveBox(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "UbuntuResolute",
			input:    "ubuntu/resolute64",
			expected: "https://cloud-images.ubuntu.com/resolute/current/resolute-server-cloudimg-amd64.img",
		},
		{
			name:     "UbuntuNoble",
			input:    "ubuntu/noble64",
			expected: "https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img",
		},
		{
			name:     "DebianBookworm",
			input:    "debian/bookworm64",
			expected: "https://cloud.debian.org/images/cloud/bookworm/latest/debian-12-genericcloud-amd64.qcow2",
		},
		{
			name:     "NixOS2605",
			input:    "nixos/26.05",
			expected: "nix://channel:nixos-26.05?format=qcow",
		},
		{
			name:     "NixOS",
			input:    "nixos/24.11",
			expected: "nix://channel:nixos-24.11?format=qcow",
		},
		{
			name:     "Fedora",
			input:    "fedora/41",
			expected: fedora41URL,
		},
		{
			name:     "Fedora44",
			input:    "fedora-44",
			expected: fedora44URL,
		},
		{
			name:     "FedoraServer",
			input:    "fedora-server-44",
			expected: fedoraServer44URL,
		},
		{
			name:     "FedoraServerShorthand",
			input:    "fedora/server",
			expected: fedoraServer44URL,
		},
		{
			name:     "CustomURLDirect",
			input:    "https://example.com/custom.qcow2",
			expected: "https://example.com/custom.qcow2",
		},
		{
			name:     "LocalFilePath",
			input:    "/var/lib/libvirt/images/custom.qcow2",
			expected: "/var/lib/libvirt/images/custom.qcow2",
		},
		{
			name:     "WhitespaceTrimmed",
			input:    "  ubuntu/noble64  ",
			expected: "https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveBox(tc.input)
			if got != tc.expected {
				t.Errorf("ResolveBox(%q) = %q, expected %q", tc.input, got, tc.expected)
			}
		})
	}
}
