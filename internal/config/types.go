package config

import "time"

// Config represents the raw root Errantfile.yaml structure.
type Config struct {
	Version         string                      `yaml:"version" json:"version"`
	Project         string                      `yaml:"project" json:"project"`
	Defaults        DefaultsConfig              `yaml:"defaults,omitempty" json:"defaults,omitempty"`
	Network         NetworkConfig               `yaml:"network,omitempty" json:"network,omitempty"`
	Images          map[string]ImageBuildConfig `yaml:"images,omitempty" json:"images,omitempty"`
	Profiles        map[string]ProfileConfig    `yaml:"profiles,omitempty" json:"profiles,omitempty"`
	Topologies      map[string]TopologyConfig   `yaml:"topologies" json:"topologies"`
	GlobalProvision []ProvisionConfig           `yaml:"global_provision,omitempty" json:"global_provision,omitempty"`
	Steps           []StepConfig                `yaml:"steps,omitempty" json:"steps,omitempty"`
}

// ImageBuildConfig defines a declarative image build recipe (e.g. NixOS or script).
type ImageBuildConfig struct {
	Builder  string            `yaml:"builder,omitempty" json:"builder,omitempty"` // "nixos-generators", "nix"
	Channel  string            `yaml:"channel,omitempty" json:"channel,omitempty"`
	Flake    string            `yaml:"flake,omitempty" json:"flake,omitempty"`
	Format   string            `yaml:"format,omitempty" json:"format,omitempty"` // "qcow", "openstack", "raw-efi"
	Packages []string          `yaml:"packages,omitempty" json:"packages,omitempty"`
	Modules  []string          `yaml:"modules,omitempty" json:"modules,omitempty"`
	Inline   string            `yaml:"inline,omitempty" json:"inline,omitempty"`
	Source   string            `yaml:"source,omitempty" json:"source,omitempty"`
	Env      map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
}

// DefaultsConfig contains global fallback settings.
type DefaultsConfig struct {
	Image           string               `yaml:"image,omitempty" json:"image,omitempty"`
	ImageAlias      string               `yaml:"image_alias,omitempty" json:"image_alias,omitempty"`
	Box             string               `yaml:"box,omitempty" json:"box,omitempty"`
	Driver          string               `yaml:"driver,omitempty" json:"driver,omitempty"`
	MemoryMB        int                  `yaml:"memory_mb,omitempty" json:"memory_mb,omitempty"`
	CPUs            int                  `yaml:"cpus,omitempty" json:"cpus,omitempty"`
	RootDiskSize    string               `yaml:"root_disk_size,omitempty" json:"root_disk_size,omitempty"`
	SSH             SSHConfig            `yaml:"ssh,omitempty" json:"ssh,omitempty"`
	SyncedFolders   []SyncedFolderConfig `yaml:"synced_folders,omitempty" json:"synced_folders,omitempty"`
	SnapshotMode    string               `yaml:"snapshot_mode,omitempty" json:"snapshot_mode,omitempty"`
	ShutdownTimeout string               `yaml:"shutdown_timeout,omitempty" json:"shutdown_timeout,omitempty"`
	StoragePool     string               `yaml:"storage_pool,omitempty" json:"storage_pool,omitempty"`
}

// SSHConfig holds SSH connection parameters.
type SSHConfig struct {
	User       string `yaml:"user,omitempty" json:"user,omitempty"`
	PublicKey  string `yaml:"public_key,omitempty" json:"public_key,omitempty"`
	PrivateKey string `yaml:"private_key,omitempty" json:"private_key,omitempty"`
	Port       int    `yaml:"port,omitempty" json:"port,omitempty"`
}

// NetworkConfig specifies the private network bridge and DHCP setup.
type NetworkConfig struct {
	Mode    string `yaml:"mode,omitempty" json:"mode,omitempty"`
	Subnet  string `yaml:"subnet,omitempty" json:"subnet,omitempty"`
	Bridge  string `yaml:"bridge,omitempty" json:"bridge,omitempty"`
	DHCP    *bool  `yaml:"dhcp,omitempty" json:"dhcp,omitempty"`
	Gateway string `yaml:"gateway,omitempty" json:"gateway,omitempty"`
}

// ProfileConfig defines a reusable node class (control-plane, worker, etc.).
type ProfileConfig struct {
	Role          string               `yaml:"role,omitempty" json:"role,omitempty"`
	Image         string               `yaml:"image,omitempty" json:"image,omitempty"`
	Box           string               `yaml:"box,omitempty" json:"box,omitempty"`
	MemoryMB      int                  `yaml:"memory_mb,omitempty" json:"memory_mb,omitempty"`
	CPUs          int                  `yaml:"cpus,omitempty" json:"cpus,omitempty"`
	RootDiskSize  string               `yaml:"root_disk_size,omitempty" json:"root_disk_size,omitempty"`
	StoragePool   string               `yaml:"storage_pool,omitempty" json:"storage_pool,omitempty"`
	Disks         []DiskConfig         `yaml:"disks,omitempty" json:"disks,omitempty"`
	SyncedFolders []SyncedFolderConfig `yaml:"synced_folders,omitempty" json:"synced_folders,omitempty"`
	Provision     []ProvisionConfig    `yaml:"provision,omitempty" json:"provision,omitempty"`
}

// DiskConfig specifies an attached storage disk.
type DiskConfig struct {
	Device    string `yaml:"device" json:"device"`
	Size      string `yaml:"size" json:"size"`
	Format    string `yaml:"format,omitempty" json:"format,omitempty"`
	Pool      string `yaml:"pool,omitempty" json:"pool,omitempty"`
	Condition string `yaml:"condition,omitempty" json:"condition,omitempty"`
}

// SyncedFolderConfig specifies shared directory mounts.
type SyncedFolderConfig struct {
	Source   string `yaml:"source" json:"source"`
	Target   string `yaml:"target" json:"target"`
	Type     string `yaml:"type,omitempty" json:"type,omitempty"`
	ReadOnly bool   `yaml:"readonly,omitempty" json:"readonly,omitempty"`
}

// ProvisionConfig specifies shell, file, or Ansible provisioning steps.
type ProvisionConfig struct {
	Type      string            `yaml:"type" json:"type"`
	Inline    string            `yaml:"inline,omitempty" json:"inline,omitempty"`
	Script    string            `yaml:"script,omitempty" json:"script,omitempty"`
	Dir       string            `yaml:"dir,omitempty" json:"dir,omitempty"`
	Source    string            `yaml:"source,omitempty" json:"source,omitempty"`
	Target    string            `yaml:"target,omitempty" json:"target,omitempty"`
	Mode      string            `yaml:"mode,omitempty" json:"mode,omitempty"`
	Env       map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
	Playbook  string            `yaml:"playbook,omitempty" json:"playbook,omitempty"`
	Inventory string            `yaml:"inventory,omitempty" json:"inventory,omitempty"`
	ExtraVars map[string]string `yaml:"extra_vars,omitempty" json:"extra_vars,omitempty"`
	When      string            `yaml:"when,omitempty" json:"when,omitempty"`
	Scope     string            `yaml:"scope,omitempty" json:"scope,omitempty"`
}

// NodeConfig represents a node declaration inside a topology.
type NodeConfig struct {
	Name          string               `yaml:"name" json:"name"`
	Profile       string               `yaml:"profile,omitempty" json:"profile,omitempty"`
	Role          string               `yaml:"role,omitempty" json:"role,omitempty"`
	Count         int                  `yaml:"count,omitempty" json:"count,omitempty"`
	IP            string               `yaml:"ip,omitempty" json:"ip,omitempty"`
	IPStart       string               `yaml:"ip_start,omitempty" json:"ip_start,omitempty"`
	Image         string               `yaml:"image,omitempty" json:"image,omitempty"`
	Box           string               `yaml:"box,omitempty" json:"box,omitempty"`
	MemoryMB      int                  `yaml:"memory_mb,omitempty" json:"memory_mb,omitempty"`
	CPUs          int                  `yaml:"cpus,omitempty" json:"cpus,omitempty"`
	RootDiskSize  string               `yaml:"root_disk_size,omitempty" json:"root_disk_size,omitempty"`
	StoragePool   string               `yaml:"storage_pool,omitempty" json:"storage_pool,omitempty"`
	Disks         []DiskConfig         `yaml:"disks,omitempty" json:"disks,omitempty"`
	SyncedFolders []SyncedFolderConfig `yaml:"synced_folders,omitempty" json:"synced_folders,omitempty"`
	Provision     []ProvisionConfig    `yaml:"provision,omitempty" json:"provision,omitempty"`
}

// TopologyConfig is a named cluster topology (e.g., standard, ha).
type TopologyConfig struct {
	Description string       `yaml:"description,omitempty" json:"description,omitempty"`
	Nodes       []NodeConfig `yaml:"nodes" json:"nodes"`
	Steps       []StepConfig `yaml:"steps,omitempty" json:"steps,omitempty"`
}

// StepConfig defines a declarative lifecycle checkpoint and its provisioning actions.
type StepConfig struct {
	Name        string            `yaml:"name" json:"name"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	Nodes       []string          `yaml:"nodes,omitempty" json:"nodes,omitempty"`
	Roles       []string          `yaml:"roles,omitempty" json:"roles,omitempty"`
	Provision   []ProvisionConfig `yaml:"provision" json:"provision"`
}

// ResolvedStep is a validated lifecycle step with its 1-based index.
type ResolvedStep struct {
	Index       int
	Name        string
	Description string
	Nodes       []string
	Roles       []string
	Provision   []ProvisionConfig
}

// ResolvedCluster represents the final, fully-expanded topology ready for execution and artifact generation.
type ResolvedCluster struct {
	Project         string
	ProjectDir      string
	Topology        string
	SnapshotMode    string
	ShutdownTimeout time.Duration
	Network         ResolvedNetwork
	Images          map[string]ImageBuildConfig
	Nodes           []ResolvedNode
	GlobalProvision []ProvisionConfig
	Steps           []ResolvedStep
}

// ResolvedNetwork is the normalized network configuration.
type ResolvedNetwork struct {
	Mode    string
	Subnet  string
	Bridge  string
	DHCP    bool
	Gateway string
}

// ResolvedNode is an individual machine with all defaults, profiles, and overrides merged.
type ResolvedNode struct {
	Name          string
	Role          string
	IP            string
	Image         string
	MemoryMB      int
	CPUs          int
	RootDiskSize  string
	StoragePool   string
	SSH           SSHConfig
	Disks         []DiskConfig
	SyncedFolders []SyncedFolderConfig
	Provision     []ProvisionConfig
}
