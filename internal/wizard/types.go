package wizard

// Archetype represents a pre-configured architecture template.
type Archetype struct {
	ID          string
	Title       string
	Description string
}

// DefaultArchetypes returns the list of available starting templates.
func DefaultArchetypes() []Archetype {
	return []Archetype{
		{
			ID:          "k8s-cluster",
			Title:       "Kubernetes Cluster",
			Description: "1 Control Plane (4GB RAM) + 2 Workers (2GB RAM)",
		},
		{
			ID:          "k8s-ha",
			Title:       "HA Kubernetes Cluster",
			Description: "3 Control Planes (3GB RAM) + 2 Workers (2GB RAM)",
		},
		{
			ID:          "single-vm",
			Title:       "Single Development VM",
			Description: "1 Standalone development & sandbox VM",
		},
		{
			ID:          "custom",
			Title:       "Custom Multi-Node",
			Description: "Configure custom control plane and worker node counts",
		},
	}
}

// OSImage represents a tested cloud-init base image.
type OSImage struct {
	Name        string
	Alias       string
	URL         string
	DefaultUser string
}

// DefaultOSImages returns the list of curated vendor cloud images.
func DefaultOSImages() []OSImage {
	return []OSImage{
		{
			Name:        "Ubuntu 26.04 (Resolute)",
			Alias:       "ubuntu-26.04",
			URL:         "https://cloud-images.ubuntu.com/resolute/current/resolute-server-cloudimg-amd64.img",
			DefaultUser: "errant",
		},
		{
			Name:        "Ubuntu 24.04 LTS (Noble Numbat)",
			Alias:       "ubuntu-24.04",
			URL:         "https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img",
			DefaultUser: "errant",
		},
		{
			Name:        "Debian 12 (Bookworm)",
			Alias:       "debian-12",
			URL:         "https://cloud.debian.org/images/cloud/bookworm/latest/debian-12-genericcloud-amd64.qcow2",
			DefaultUser: "errant",
		},
		{
			Name:        "NixOS Generic Cloud (x86_64)",
			Alias:       "nixos",
			URL:         "https://channels.nixos.org/nixos-26.05/latest-nixos-genericcloud-x86_64-linux.qcow2",
			DefaultUser: "errant",
		},
		{
			Name:  "Alpine Linux 3.20 (Minimal Cloud)",
			Alias: "alpine-3.20",
			URL: "https://dl-cdn.alpinelinux.org/alpine/v3.20/releases/cloud/" +
				"genericcloud_alpine-3.20.0-x86_64-bios-tiny-r0.qcow2",
			DefaultUser: "alpine",
		},
	}
}

// Answers holds all wizard answers collected from user interaction.
type Answers struct {
	ArchetypeID  string
	ProjectName  string
	ImageIndex   int
	CustomImage  string
	DefaultUser  string
	MemoryMB     int
	CPUs         int
	RootDiskSize string
	Subnet       string
	Bridge       string
	SyncFolder   bool
	ControlNodes int
	WorkerNodes  int
	OutputPath   string
}

// NewDefaultAnswers creates default answers suitable for immediate fallback or initial form values.
func NewDefaultAnswers(projectName string) Answers {
	if projectName == "" {
		projectName = "errant-cluster"
	}

	return Answers{
		ArchetypeID:  "k8s-cluster",
		ProjectName:  projectName,
		ImageIndex:   0,
		CustomImage:  "",
		DefaultUser:  "errant",
		MemoryMB:     2048,
		CPUs:         2,
		RootDiskSize: "25G",
		Subnet:       "192.168.56.0/24",
		Bridge:       "virbr56",
		SyncFolder:   true,
		ControlNodes: 1,
		WorkerNodes:  2,
		OutputPath:   "Errantfile.yaml",
	}
}
