package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ResolveStoragePoolFunc resolves a configured storage pool name (including "auto").
var ResolveStoragePoolFunc = detectStoragePool

// ResolveCluster resolves defaults, profiles, templated nodes, and overrides for the selected topology.
func (c *Config) ResolveCluster(topologyName string) (*ResolvedCluster, error) {
	selectedTopology, err := c.selectTopology(topologyName)
	if err != nil {
		return nil, err
	}

	topo := c.Topologies[selectedTopology]
	resolvedNet := c.resolveNetwork()
	sshDef := c.resolveSSHDefaults()

	var resolvedNodes []ResolvedNode

	for _, nodeDecl := range topo.Nodes {
		count := nodeDecl.Count
		if count <= 0 {
			count = 1
		}

		for idx := 1; idx <= count; idx++ {
			node, err := c.resolveSingleNode(nodeDecl, idx, sshDef)
			if err != nil {
				return nil, fmt.Errorf("failed resolving node %s[%d]: %w", nodeDecl.Name, idx, err)
			}

			resolvedNodes = append(resolvedNodes, node)
		}
	}

	resolvedSteps, err := resolveSteps(c.Steps, topo.Steps, resolvedNodes)
	if err != nil {
		return nil, err
	}

	snapshotMode := c.Defaults.SnapshotMode
	if snapshotMode == "" {
		snapshotMode = "disk"
	}

	shutdownTimeout, err := resolveShutdownTimeout(c.Defaults.ShutdownTimeout)
	if err != nil {
		return nil, err
	}

	return &ResolvedCluster{
		Project:         c.Project,
		Topology:        selectedTopology,
		SnapshotMode:    snapshotMode,
		ShutdownTimeout: shutdownTimeout,
		Network:         resolvedNet,
		Images:          c.Images,
		Nodes:           resolvedNodes,
		GlobalProvision: c.GlobalProvision,
		Steps:           resolvedSteps,
	}, nil
}

func resolveShutdownTimeout(raw string) (time.Duration, error) {
	if raw == "" {
		return 180 * time.Second, nil
	}

	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("parsing defaults.shutdown_timeout %q: %w", raw, err)
	}

	if parsed <= 0 {
		return 0, fmt.Errorf("defaults.shutdown_timeout must be positive, got %q", raw)
	}

	return parsed, nil
}

func (c *Config) selectTopology(name string) (string, error) {
	if name != "" {
		if _, ok := c.Topologies[name]; !ok {
			return "", fmt.Errorf("topology %q not found (available: %v)", name, availableTopologies(c.Topologies))
		}

		return name, nil
	}

	if _, ok := c.Topologies["standard"]; ok {
		return "standard", nil
	}

	for k := range c.Topologies {
		return k, nil
	}

	return "", errors.New("no topologies defined in Errantfile")
}

func (c *Config) resolveNetwork() ResolvedNetwork {
	resolvedNet := ResolvedNetwork{
		Mode:    "private",
		Subnet:  "192.168.56.0/24",
		Bridge:  "virbr56",
		DHCP:    true,
		Gateway: c.Network.Gateway,
	}

	if c.Network.Mode != "" {
		resolvedNet.Mode = c.Network.Mode
	}

	if c.Network.Subnet != "" {
		resolvedNet.Subnet = c.Network.Subnet
	}

	if c.Network.Bridge != "" {
		resolvedNet.Bridge = c.Network.Bridge
	}

	if c.Network.DHCP != nil {
		resolvedNet.DHCP = *c.Network.DHCP
	}

	return resolvedNet
}

func (c *Config) resolveSSHDefaults() SSHConfig {
	sshDef := c.Defaults.SSH
	if sshDef.User == "" {
		sshDef.User = "errant"
	}

	if sshDef.Port == 0 {
		sshDef.Port = 22
	}

	if sshDef.PublicKey == "" {
		home, _ := os.UserHomeDir()
		idEd := filepath.Join(home, ".ssh", "id_ed25519.pub")
		idRSA := filepath.Join(home, ".ssh", "id_rsa.pub")

		if _, err := os.Stat(idEd); err == nil {
			sshDef.PublicKey = idEd
		} else if _, err := os.Stat(idRSA); err == nil {
			sshDef.PublicKey = idRSA
		}
	}

	if sshDef.PrivateKey == "" && sshDef.PublicKey != "" {
		sshDef.PrivateKey = strings.TrimSuffix(sshDef.PublicKey, ".pub")
	}

	return sshDef
}

func (c *Config) resolveSingleNode(nodeDecl NodeConfig, index int, defaultSSH SSHConfig) (ResolvedNode, error) {
	var profile ProfileConfig

	if nodeDecl.Profile != "" {
		p, ok := c.Profiles[nodeDecl.Profile]
		if !ok {
			return ResolvedNode{}, fmt.Errorf("referenced profile %q not found", nodeDecl.Profile)
		}

		profile = p
	}

	name := strings.ReplaceAll(nodeDecl.Name, "${index}", strconv.Itoa(index))
	role := c.resolveRole(profile, nodeDecl)
	image := c.resolveImage(profile, nodeDecl)
	memoryMB, cpus, rootDisk := c.resolveSizing(profile, nodeDecl)
	storagePool := c.resolveStoragePool(profile, nodeDecl)

	ip, err := c.resolveIP(nodeDecl, index)
	if err != nil {
		return ResolvedNode{}, err
	}

	disks := c.resolveDisks(profile, nodeDecl, index, storagePool)

	var syncedFolders []SyncedFolderConfig

	syncedFolders = append(syncedFolders, c.Defaults.SyncedFolders...)
	syncedFolders = append(syncedFolders, profile.SyncedFolders...)
	syncedFolders = append(syncedFolders, nodeDecl.SyncedFolders...)

	var provisioners []ProvisionConfig

	provisioners = append(provisioners, profile.Provision...)
	provisioners = append(provisioners, nodeDecl.Provision...)

	return ResolvedNode{
		Name:          name,
		Role:          role,
		IP:            ip,
		Image:         image,
		MemoryMB:      memoryMB,
		CPUs:          cpus,
		RootDiskSize:  rootDisk,
		StoragePool:   storagePool,
		SSH:           defaultSSH,
		Disks:         disks,
		SyncedFolders: syncedFolders,
		Provision:     provisioners,
	}, nil
}

func (*Config) resolveRole(profile ProfileConfig, nodeDecl NodeConfig) string {
	role := "worker"
	if profile.Role != "" {
		role = profile.Role
	}

	if nodeDecl.Role != "" {
		role = nodeDecl.Role
	}

	return role
}

func (c *Config) resolveImage(profile ProfileConfig, nodeDecl NodeConfig) string {
	image := c.Defaults.Image
	if c.Defaults.Box != "" {
		image = c.Defaults.Box
	}

	if profile.Image != "" {
		image = profile.Image
	}

	if profile.Box != "" {
		image = profile.Box
	}

	if nodeDecl.Image != "" {
		image = nodeDecl.Image
	}

	if nodeDecl.Box != "" {
		image = nodeDecl.Box
	}

	return ResolveBox(image)
}

func (c *Config) resolveSizing(profile ProfileConfig, nodeDecl NodeConfig) (memoryMB int, cpus int, rootDisk string) {
	memoryMB = 2048
	if c.Defaults.MemoryMB > 0 {
		memoryMB = c.Defaults.MemoryMB
	}

	if profile.MemoryMB > 0 {
		memoryMB = profile.MemoryMB
	}

	if nodeDecl.MemoryMB > 0 {
		memoryMB = nodeDecl.MemoryMB
	}

	cpus = 2
	if c.Defaults.CPUs > 0 {
		cpus = c.Defaults.CPUs
	}

	if profile.CPUs > 0 {
		cpus = profile.CPUs
	}

	if nodeDecl.CPUs > 0 {
		cpus = nodeDecl.CPUs
	}

	rootDisk = "20G"
	if c.Defaults.RootDiskSize != "" {
		rootDisk = c.Defaults.RootDiskSize
	}

	if profile.RootDiskSize != "" {
		rootDisk = profile.RootDiskSize
	}

	if nodeDecl.RootDiskSize != "" {
		rootDisk = nodeDecl.RootDiskSize
	}

	return memoryMB, cpus, rootDisk
}

func (*Config) resolveIP(nodeDecl NodeConfig, index int) (string, error) {
	if nodeDecl.IP != "" {
		return nodeDecl.IP, nil
	}

	if nodeDecl.IPStart != "" {
		return calculateOffsetIP(nodeDecl.IPStart, index-1)
	}

	return "", nil
}

func (c *Config) resolveStoragePool(profile ProfileConfig, nodeDecl NodeConfig) string {
	pool := "default"
	if c.Defaults.StoragePool != "" {
		pool = c.Defaults.StoragePool
	}

	if profile.StoragePool != "" {
		pool = profile.StoragePool
	}

	if nodeDecl.StoragePool != "" {
		pool = nodeDecl.StoragePool
	}

	return ResolveStoragePoolFunc(pool)
}

func detectStoragePool(pool string) string {
	if pool == "" {
		return "default"
	}

	if pool != "auto" {
		return pool
	}

	if err := exec.Command("virsh", "-c", "qemu:///system", "pool-info", "vmstore").Run(); err == nil {
		return "vmstore"
	}

	return "default"
}

func (*Config) resolveDisks(profile ProfileConfig, nodeDecl NodeConfig, index int, defaultPool string) []DiskConfig {
	var disks []DiskConfig

	if defaultPool == "" {
		defaultPool = "default"
	}

	candidateDisks := append([]DiskConfig{}, profile.Disks...)
	if len(nodeDecl.Disks) > 0 {
		candidateDisks = nodeDecl.Disks
	}

	for _, d := range candidateDisks {
		if d.Condition != "" && !evaluateIndexCondition(d.Condition, index) {
			continue
		}

		if d.Format == "" {
			d.Format = "qcow2"
		}

		if d.Pool == "" {
			d.Pool = defaultPool
		}

		disks = append(disks, d)
	}

	return disks
}

func calculateOffsetIP(startIP string, offset int) (string, error) {
	ip := net.ParseIP(startIP)
	if ip == nil {
		return "", fmt.Errorf("unable to parse IPv4 address %q", startIP)
	}

	ipv4 := ip.To4()
	if ipv4 == nil {
		return "", fmt.Errorf("not an IPv4 address %q", startIP)
	}

	val := uint32(ipv4[0])<<24 | uint32(ipv4[1])<<16 | uint32(ipv4[2])<<8 | uint32(ipv4[3])
	val += uint32(offset)

	return net.IPv4(byte(val>>24), byte(val>>16), byte(val>>8), byte(val)).String(), nil
}

func evaluateIndexCondition(cond string, index int) bool {
	cond = strings.TrimSpace(cond)
	expanded := strings.ReplaceAll(cond, "${index}", strconv.Itoa(index))
	parts := strings.Split(expanded, "==")

	if len(parts) == 2 {
		left := strings.TrimSpace(parts[0])
		right := strings.TrimSpace(parts[1])

		return left == right
	}

	return true
}

func availableTopologies(topos map[string]TopologyConfig) []string {
	var list []string

	for k := range topos {
		list = append(list, k)
	}

	return list
}

func resolveSteps(rootSteps, topoSteps []StepConfig, nodes []ResolvedNode) ([]ResolvedStep, error) {
	rawSteps := rootSteps
	if len(topoSteps) > 0 {
		rawSteps = topoSteps
	}

	if len(rawSteps) == 0 {
		return nil, nil
	}

	nodeSet, roleSet := buildNodeAndRoleSets(nodes)
	seen := make(map[string]struct{}, len(rawSteps))
	resolved := make([]ResolvedStep, 0, len(rawSteps))

	for i, s := range rawSteps {
		if err := validateStepItem(s, seen, nodeSet, roleSet); err != nil {
			return nil, fmt.Errorf("step %d (%s): %w", i+1, s.Name, err)
		}

		seen[s.Name] = struct{}{}

		resolved = append(resolved, ResolvedStep{
			Index:       i + 1,
			Name:        s.Name,
			Description: s.Description,
			Nodes:       append([]string{}, s.Nodes...),
			Roles:       append([]string{}, s.Roles...),
			Provision:   append([]ProvisionConfig{}, s.Provision...),
		})
	}

	return resolved, nil
}

func buildNodeAndRoleSets(nodes []ResolvedNode) (nodeSet, roleSet map[string]struct{}) {
	nodeSet = make(map[string]struct{}, len(nodes))
	roleSet = make(map[string]struct{})

	for _, n := range nodes {
		nodeSet[n.Name] = struct{}{}

		if n.Role != "" {
			roleSet[n.Role] = struct{}{}
		}
	}

	return nodeSet, roleSet
}

func validateStepItem(
	s StepConfig,
	seen map[string]struct{},
	nodeSet map[string]struct{},
	roleSet map[string]struct{},
) error {
	if s.Name == "" {
		return errors.New("step name cannot be empty")
	}

	if s.Name == "0" || s.Name == "base" {
		return fmt.Errorf("step name %q is reserved for the initial baseline", s.Name)
	}

	if _, dup := seen[s.Name]; dup {
		return fmt.Errorf("duplicate step name %q", s.Name)
	}

	for _, targetNode := range s.Nodes {
		if _, ok := nodeSet[targetNode]; !ok {
			return fmt.Errorf("unknown target node %q", targetNode)
		}
	}

	for _, targetRole := range s.Roles {
		if _, ok := roleSet[targetRole]; !ok {
			return fmt.Errorf("unknown target role %q", targetRole)
		}
	}

	return nil
}
