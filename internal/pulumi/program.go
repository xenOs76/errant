package pulumi

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pulumi/pulumi-libvirt/sdk/go/libvirt"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/xenos76/errant/internal/cache"
	"github.com/xenos76/errant/internal/config"
)

// ResolveImageFunc delegates to cache.ResolveClusterImage, exposed for mocking in tests.
var ResolveImageFunc = cache.ResolveClusterImage

// NewClusterProgram creates an inline Pulumi program from a resolved cluster topology.
func NewClusterProgram(cluster *config.ResolvedCluster) pulumi.RunFunc {
	return func(ctx *pulumi.Context) error {
		network, err := createNetwork(ctx, cluster)
		if err != nil {
			return fmt.Errorf("creating network: %w", err)
		}

		baseImageVolumes, err := createBaseImageVolumes(ctx, cluster)
		if err != nil {
			return fmt.Errorf("creating base image volumes: %w", err)
		}

		for idx, node := range cluster.Nodes {
			if err := createNodeResources(ctx, cluster, node, idx, network, baseImageVolumes); err != nil {
				return fmt.Errorf("creating node %s resources: %w", node.Name, err)
			}
		}

		return nil
	}
}

func createNetwork(ctx *pulumi.Context, cluster *config.ResolvedCluster) (*libvirt.Network, error) {
	netName := cluster.Project + "-net"
	netArgs := &libvirt.NetworkArgs{
		Name:      pulumi.String(netName),
		Mode:      pulumi.String("nat"),
		Addresses: pulumi.StringArray{pulumi.String(cluster.Network.Subnet)},
		Autostart: pulumi.Bool(true),
	}

	if cluster.Network.Bridge != "" {
		netArgs.Bridge = pulumi.String(cluster.Network.Bridge)
	}

	if cluster.Network.DHCP {
		netArgs.Dhcp = &libvirt.NetworkDhcpArgs{
			Enabled: pulumi.Bool(true),
		}
	}

	network, err := libvirt.NewNetwork(ctx, "cluster_net", netArgs)
	if err != nil {
		return nil, fmt.Errorf("declaring libvirt network %s: %w", netName, err)
	}

	return network, nil
}

func nodePool(node config.ResolvedNode) string {
	if node.StoragePool != "" {
		return node.StoragePool
	}

	return "default"
}

func createBaseImageVolumes(
	ctx *pulumi.Context,
	cluster *config.ResolvedCluster,
) (map[string]*libvirt.Volume, error) {
	imageVolumes := make(map[string]*libvirt.Volume)
	imgIdx := 1

	for _, node := range cluster.Nodes {
		if node.Image == "" {
			continue
		}

		pool := nodePool(node)
		mapKey := node.Image + "@" + pool

		if _, exists := imageVolumes[mapKey]; exists {
			continue
		}

		key := fmt.Sprintf("base_image_%d", imgIdx)
		imgIdx++

		localSource, err := ResolveImageFunc(ctx.Context(), node.Image, cluster.Images)
		if err != nil {
			return nil, fmt.Errorf("caching base image %s: %w", node.Image, err)
		}

		volArgs := &libvirt.VolumeArgs{
			Name:   pulumi.String(fmt.Sprintf("%s-%s.qcow2", cluster.Project, key)),
			Source: pulumi.String(localSource),
			Format: pulumi.String("qcow2"),
			Pool:   pulumi.String(pool),
		}

		vol, err := libvirt.NewVolume(ctx, key, volArgs)
		if err != nil {
			return nil, fmt.Errorf("declaring base image volume %s: %w", key, err)
		}

		imageVolumes[mapKey] = vol
	}

	return imageVolumes, nil
}

func createNodeResources(
	ctx *pulumi.Context,
	cluster *config.ResolvedCluster,
	node config.ResolvedNode,
	nodeIdx int,
	network *libvirt.Network,
	baseImageVolumes map[string]*libvirt.Volume,
) error {
	safeName := sanitizeResourceName(node.Name)

	rootVol, err := createRootVolume(ctx, cluster, node, safeName, baseImageVolumes)
	if err != nil {
		return err
	}

	secDisks, err := createSecondaryVolumes(ctx, cluster, node, safeName)
	if err != nil {
		return err
	}

	ciDisk, err := createCloudInitDisk(ctx, cluster, node, safeName)
	if err != nil {
		return err
	}

	mac := generateMAC(nodeIdx + 1)
	res := domainResourceSet{
		safeName: safeName,
		mac:      mac,
		network:  network,
		rootVol:  rootVol,
		ciDisk:   ciDisk,
		secDisks: secDisks,
	}

	if err := createDomain(ctx, cluster, node, res); err != nil {
		return err
	}

	if node.IP != "" {
		ctx.Export("ip_"+safeName, pulumi.String(node.IP))
	}

	return nil
}

func createRootVolume(
	ctx *pulumi.Context,
	cluster *config.ResolvedCluster,
	node config.ResolvedNode,
	safeName string,
	baseImageVolumes map[string]*libvirt.Volume,
) (*libvirt.Volume, error) {
	pool := nodePool(node)

	baseVol, ok := baseImageVolumes[node.Image+"@"+pool]
	if !ok {
		return nil, fmt.Errorf("missing base image volume for node %s (image %s, pool %s)", node.Name, node.Image, pool)
	}

	rootArgs := &libvirt.VolumeArgs{
		Name:         pulumi.String(fmt.Sprintf("%s-%s-root.qcow2", cluster.Project, node.Name)),
		BaseVolumeId: baseVol.ID(),
		Format:       pulumi.String("qcow2"),
		Pool:         pulumi.String(pool),
	}

	if node.RootDiskSize != "" {
		if sizeBytes, err := parseSizeToBytes(node.RootDiskSize); err == nil && sizeBytes > 0 {
			rootArgs.Size = pulumi.Int(int(sizeBytes))
		}
	}

	vol, err := libvirt.NewVolume(ctx, "root_"+safeName, rootArgs)
	if err != nil {
		return nil, fmt.Errorf("declaring root volume for node %s: %w", node.Name, err)
	}

	return vol, nil
}

func createSecondaryVolumes(
	ctx *pulumi.Context,
	cluster *config.ResolvedCluster,
	node config.ResolvedNode,
	safeName string,
) ([]*libvirt.Volume, error) {
	var disks []*libvirt.Volume

	for dIdx, d := range node.Disks {
		dKey := fmt.Sprintf("disk_%s_%s_%d", safeName, d.Device, dIdx)
		dArgs := &libvirt.VolumeArgs{
			Name:   pulumi.String(fmt.Sprintf("%s-%s-%s.qcow2", cluster.Project, node.Name, d.Device)),
			Format: pulumi.String(d.Format),
			Pool:   pulumi.String(d.Pool),
		}

		if sizeBytes, err := parseSizeToBytes(d.Size); err == nil && sizeBytes > 0 {
			dArgs.Size = pulumi.Int(int(sizeBytes))
		}

		vol, err := libvirt.NewVolume(ctx, dKey, dArgs)
		if err != nil {
			return nil, fmt.Errorf("declaring secondary disk %s: %w", dKey, err)
		}

		disks = append(disks, vol)
	}

	return disks, nil
}

func createCloudInitDisk(
	ctx *pulumi.Context,
	cluster *config.ResolvedCluster,
	node config.ResolvedNode,
	safeName string,
) (*libvirt.CloudInitDisk, error) {
	userData := renderCloudInitUserData(node)
	metaData := fmt.Sprintf("instance-id: %s\nlocal-hostname: %s\n", node.Name, node.Name)

	ciArgs := &libvirt.CloudInitDiskArgs{
		Name:     pulumi.String(fmt.Sprintf("%s-%s-ci.iso", cluster.Project, node.Name)),
		UserData: pulumi.String(userData),
		MetaData: pulumi.String(metaData),
		Pool:     pulumi.String(nodePool(node)),
	}

	ci, err := libvirt.NewCloudInitDisk(ctx, "ci_"+safeName, ciArgs)
	if err != nil {
		return nil, fmt.Errorf("declaring cloudinit disk for node %s: %w", node.Name, err)
	}

	return ci, nil
}

type domainResourceSet struct {
	safeName string
	mac      string
	network  *libvirt.Network
	rootVol  *libvirt.Volume
	ciDisk   *libvirt.CloudInitDisk
	secDisks []*libvirt.Volume
}

func createDomain(
	ctx *pulumi.Context,
	cluster *config.ResolvedCluster,
	node config.ResolvedNode,
	res domainResourceSet,
) error {
	diskList := libvirt.DomainDiskArray{
		&libvirt.DomainDiskArgs{VolumeId: res.rootVol.ID()},
	}

	for _, sec := range res.secDisks {
		diskList = append(diskList, &libvirt.DomainDiskArgs{VolumeId: sec.ID()})
	}

	netIf := &libvirt.DomainNetworkInterfaceArgs{
		NetworkId:    res.network.ID(),
		Mac:          pulumi.String(res.mac),
		WaitForLease: pulumi.Bool(true),
	}

	if node.IP != "" {
		netIf.Addresses = pulumi.StringArray{pulumi.String(node.IP)}
	}

	domArgs := &libvirt.DomainArgs{
		Name:              pulumi.String(fmt.Sprintf("%s-%s", cluster.Project, node.Name)),
		Memory:            pulumi.Int(node.MemoryMB),
		Vcpu:              pulumi.Int(node.CPUs),
		Running:           pulumi.Bool(true),
		Cloudinit:         res.ciDisk.ID(),
		Disks:             diskList,
		NetworkInterfaces: libvirt.DomainNetworkInterfaceArray{netIf},
		Consoles: libvirt.DomainConsoleArray{
			&libvirt.DomainConsoleArgs{
				Type:       pulumi.String("pty"),
				TargetPort: pulumi.String("0"),
				TargetType: pulumi.String("serial"),
			},
		},
		Graphics: &libvirt.DomainGraphicsArgs{
			Type:     pulumi.String("spice"),
			Autoport: pulumi.Bool(true),
		},
	}

	filesystems := buildFilesystems(cluster, node)
	if len(filesystems) > 0 {
		domArgs.Filesystems = filesystems
	}

	domain, err := libvirt.NewDomain(ctx, "vm_"+res.safeName, domArgs)
	if err != nil {
		return fmt.Errorf("declaring libvirt domain for %s: %w", node.Name, err)
	}

	ctx.Export("id_"+res.safeName, domain.ID())

	return nil
}

func buildFilesystems(cluster *config.ResolvedCluster, node config.ResolvedNode) libvirt.DomainFilesystemArray {
	var fsList libvirt.DomainFilesystemArray

	for _, sf := range node.SyncedFolders {
		if sf.Type != "" && sf.Type != "virtiofs" && sf.Type != "9p" {
			continue
		}

		src := sf.Source
		if !filepath.IsAbs(src) && cluster.ProjectDir != "" {
			src = filepath.Join(cluster.ProjectDir, src)
		}

		if abs, err := filepath.Abs(src); err == nil {
			src = abs
		}

		fsList = append(fsList, &libvirt.DomainFilesystemArgs{
			Accessmode: pulumi.String("passthrough"),
			Source:     pulumi.String(src),
			Target:     pulumi.String(sf.Target),
			Readonly:   pulumi.Bool(sf.ReadOnly),
		})
	}

	return fsList
}

func sanitizeResourceName(name string) string {
	r := strings.ReplaceAll(name, "-", "_")

	return strings.ReplaceAll(r, ".", "_")
}

func generateMAC(id int) string {
	return fmt.Sprintf("52:54:00:56:%02x:%02x", (id>>8)&0xff, id&0xff)
}

func parseSizeToBytes(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToUpper(s))
	if s == "" {
		return 0, nil
	}

	multiplier := int64(1)
	unit := s[len(s)-1]
	valStr := s

	if unit >= 'A' && unit <= 'Z' {
		valStr = s[:len(s)-1]

		switch unit {
		case 'K':
			multiplier = 1024
		case 'M':
			multiplier = 1024 * 1024
		case 'G':
			multiplier = 1024 * 1024 * 1024
		case 'T':
			multiplier = 1024 * 1024 * 1024 * 1024
		default:
			multiplier = 1
		}
	}

	val, err := strconv.ParseInt(valStr, 10, 64)
	if err != nil {
		return 0, err
	}

	return val * multiplier, nil
}

func renderCloudInitUserData(node config.ResolvedNode) string {
	var pubKeys []string

	if node.SSH.PublicKey != "" {
		if content, err := os.ReadFile(node.SSH.PublicKey); err == nil {
			pubKeys = append(pubKeys, strings.TrimSpace(string(content)))
		} else {
			pubKeys = append(pubKeys, node.SSH.PublicKey)
		}
	}

	user := node.SSH.User
	if user == "" {
		user = "errant"
	}

	var sb strings.Builder

	_, _ = sb.WriteString("#cloud-config\n")
	fmt.Fprintf(&sb, "hostname: %s\n", node.Name)
	fmt.Fprintf(&sb, "fqdn: %s.local\n", node.Name)
	_, _ = sb.WriteString("manage_etc_hosts: true\n\n")

	if len(pubKeys) > 0 {
		_, _ = sb.WriteString("ssh_authorized_keys:\n")

		for _, k := range pubKeys {
			fmt.Fprintf(&sb, "  - %s\n", k)
		}

		_, _ = sb.WriteString("\n")
	}

	_, _ = sb.WriteString("users:\n")
	fmt.Fprintf(&sb, "  - name: %s\n", user)
	_, _ = sb.WriteString("    sudo: ALL=(ALL) NOPASSWD:ALL\n")
	_, _ = sb.WriteString("    shell: /bin/bash\n")

	if len(pubKeys) > 0 {
		_, _ = sb.WriteString("    ssh_authorized_keys:\n")

		for _, k := range pubKeys {
			fmt.Fprintf(&sb, "      - %s\n", k)
		}
	}

	if len(node.SyncedFolders) > 0 {
		_, _ = sb.WriteString("\nmounts:\n")

		for _, sf := range node.SyncedFolders {
			fmt.Fprintf(
				&sb,
				"  - [ %q, %q, \"9p\", \"trans=virtio,version=9p2000.L,_netdev,nofail\", \"0\", \"0\" ]\n",
				sf.Target, sf.Target,
			)
		}
	}

	_, _ = sb.WriteString("\nruncmd:\n")
	_, _ = sb.WriteString("  - swapoff -a\n")
	fmt.Fprintf(&sb, "  - chsh -s /bin/bash %s || usermod -s /bin/bash %s\n", user, user)

	for _, sf := range node.SyncedFolders {
		fmt.Fprintf(&sb, "  - mkdir -p %s\n", sf.Target)
		fmt.Fprintf(&sb, "  - mount -t 9p -o trans=virtio,version=9p2000.L %s %s || true\n", sf.Target, sf.Target)
	}

	return sb.String()
}
