package config

import (
	"strings"
)

const (
	fedora40URL = "https://download.fedoraproject.org/pub/fedora/linux/releases/40/Cloud/x86_64/images/" +
		"Fedora-Cloud-Base-Generic.x86_64-40-1.14.qcow2"
	fedora41URL = "https://download.fedoraproject.org/pub/fedora/linux/releases/41/Cloud/x86_64/images/" +
		"Fedora-Cloud-Base-Generic-41-1.4.x86_64.qcow2"
	alma9URL = "https://repo.almalinux.org/almalinux/9/cloud/x86_64/images/" +
		"AlmaLinux-9-GenericCloud-latest.x86_64.qcow2"
	rocky9URL = "https://download.rockylinux.org/pub/rocky/9/images/x86_64/" +
		"Rocky-9-GenericCloud-Base.latest.x86_64.qcow2"
	debian13URL = "https://cloud.debian.org/images/cloud/trixie/daily/latest/" +
		"debian-13-genericcloud-amd64-daily.qcow2"
	fedoraServer44URL = "https://download.fedoraproject.org/pub/fedora/linux/releases/44/Server/x86_64/images/" +
		"Fedora-Server-Guest-Generic-44-1.7.x86_64.qcow2"
	fedora44URL = "https://download.fedoraproject.org/pub/fedora/linux/releases/44/Cloud/x86_64/images/" +
		"Fedora-Cloud-Base-Generic-44-1.7.x86_64.qcow2"
)

var defaultBoxAliases = map[string]string{
	// Ubuntu
	"ubuntu/resolute64": "https://cloud-images.ubuntu.com/resolute/current/resolute-server-cloudimg-amd64.img",
	"ubuntu/resolute":   "https://cloud-images.ubuntu.com/resolute/current/resolute-server-cloudimg-amd64.img",
	"ubuntu-26.04":      "https://cloud-images.ubuntu.com/resolute/current/resolute-server-cloudimg-amd64.img",
	"ubuntu/noble64":    "https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img",
	"ubuntu/noble":      "https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img",
	"ubuntu-24.04":      "https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img",
	"ubuntu/jammy64":    "https://cloud-images.ubuntu.com/jammy/current/jammy-server-cloudimg-amd64.img",
	"ubuntu/jammy":      "https://cloud-images.ubuntu.com/jammy/current/jammy-server-cloudimg-amd64.img",
	"ubuntu-22.04":      "https://cloud-images.ubuntu.com/jammy/current/jammy-server-cloudimg-amd64.img",
	"ubuntu/focal64":    "https://cloud-images.ubuntu.com/focal/current/focal-server-cloudimg-amd64.img",
	"ubuntu/focal":      "https://cloud-images.ubuntu.com/focal/current/focal-server-cloudimg-amd64.img",
	"ubuntu-20.04":      "https://cloud-images.ubuntu.com/focal/current/focal-server-cloudimg-amd64.img",

	// Debian
	"debian/bookworm64": "https://cloud.debian.org/images/cloud/bookworm/latest/debian-12-genericcloud-amd64.qcow2",
	"debian/bookworm":   "https://cloud.debian.org/images/cloud/bookworm/latest/debian-12-genericcloud-amd64.qcow2",
	"debian-12":         "https://cloud.debian.org/images/cloud/bookworm/latest/debian-12-genericcloud-amd64.qcow2",
	"debian/trixie64":   debian13URL,
	"debian/trixie":     debian13URL,
	"debian-13":         debian13URL,
	"debian/bullseye64": "https://cloud.debian.org/images/cloud/bullseye/latest/debian-11-genericcloud-amd64.qcow2",
	"debian-11":         "https://cloud.debian.org/images/cloud/bullseye/latest/debian-11-genericcloud-amd64.qcow2",

	// NixOS
	"nixos/26.05": "nix://channel:nixos-26.05?format=qcow",
	"nixos-26.05": "nix://channel:nixos-26.05?format=qcow",
	"nixos/24.11": "nix://channel:nixos-24.11?format=qcow",
	"nixos-24.11": "nix://channel:nixos-24.11?format=qcow",
	"nixos":       "nix://channel:nixos-26.05?format=qcow",

	// Fedora & Enterprise Linux
	"fedora/server":    fedoraServer44URL,
	"fedora-server":    fedoraServer44URL,
	"fedora/server-44": fedoraServer44URL,
	"fedora-server-44": fedoraServer44URL,
	"fedora/40":        fedora40URL,
	"fedora-40":        fedora40URL,
	"fedora/41":        fedora41URL,
	"fedora-41":        fedora41URL,
	"fedora/44":        fedora44URL,
	"fedora-44":        fedora44URL,
	"almalinux/9":      alma9URL,
	"almalinux-9":      alma9URL,
	"rockylinux/9":     rocky9URL,
	"rocky-9":          rocky9URL,
}

// ResolveBox translates a Vagrant-style box shorthand into a canonical Cloud-Init image URL.
// If the input is not a recognized alias, it returns the trimmed input unchanged.
func ResolveBox(ref string) string {
	trimmed := strings.TrimSpace(ref)
	if url, exists := defaultBoxAliases[strings.ToLower(trimmed)]; exists {
		return url
	}

	return trimmed
}
