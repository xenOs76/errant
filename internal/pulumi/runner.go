package pulumi

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/events"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optdestroy"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/xenos76/errant/internal/config"
)

const (
	// LibvirtPluginVersion specifies the bridged terraform-provider-libvirt version.
	LibvirtPluginVersion = "v0.5.4"
)

// DomainStatus encapsulates node runtime state.
type DomainStatus struct {
	Name   string
	State  string
	IP     string
	Role   string
	Active bool
}

// Outputs captures exported variables from a Pulumi deployment.
type Outputs struct {
	NodeIPs map[string]string
	NodeIDs map[string]string
}

// Runner drives the in-process Pulumi Automation API against Libvirt.
type Runner struct {
	ProjectName string
	WorkDir     string
	BackendURL  string
	Stdout      io.Writer
	Stderr      io.Writer
}

// NewRunner initializes a Pulumi Automation Runner with a local state backend.
func NewRunner(workDir string, projectName string) *Runner {
	if abs, err := filepath.Abs(workDir); err == nil {
		workDir = abs
	}

	errantDir := filepath.Join(workDir, ".errant")
	stateDir := filepath.Join(errantDir, "state")
	_ = os.MkdirAll(stateDir, 0o755)

	if os.Getenv("PULUMI_CONFIG_PASSPHRASE") == "" && os.Getenv("PULUMI_CONFIG_PASSPHRASE_FILE") == "" {
		_ = os.Setenv("PULUMI_CONFIG_PASSPHRASE", "")
	}

	return &Runner{
		ProjectName: projectName,
		WorkDir:     errantDir,
		BackendURL:  "file://" + filepath.ToSlash(stateDir),
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
	}
}

// Apply applies the cluster topology using the Pulumi Automation API.
func (r *Runner) Apply(ctx context.Context, cluster *config.ResolvedCluster) error {
	eventCh := make(chan events.EngineEvent)
	go ProcessEngineEvents(eventCh, r.Stdout)

	_, err := r.Up(ctx, cluster, eventCh)
	if err != nil {
		return err
	}

	return nil
}

// Destroy destroys the cluster topology using the Pulumi Automation API.
func (r *Runner) Destroy(ctx context.Context, cluster *config.ResolvedCluster) error {
	eventCh := make(chan events.EngineEvent)
	go ProcessEngineEvents(eventCh, r.Stdout)

	err := r.destroyInternal(ctx, cluster, eventCh)
	r.cleanupOrphanedNetwork(ctx, cluster)

	return err
}

func (*Runner) cleanupOrphanedNetwork(ctx context.Context, cluster *config.ResolvedCluster) {
	netName := cluster.Project + "-net"
	checkCmd := exec.CommandContext(ctx, "virsh", "-c", "qemu:///system", "net-info", netName)

	if err := checkCmd.Run(); err == nil {
		_ = exec.CommandContext(ctx, "virsh", "-c", "qemu:///system", "net-destroy", netName).Run()
		_ = exec.CommandContext(ctx, "virsh", "-c", "qemu:///system", "net-undefine", netName).Run()
	}
}

func (r *Runner) destroyInternal(
	ctx context.Context,
	cluster *config.ResolvedCluster,
	eventCh chan<- events.EngineEvent,
) error {
	stack, err := r.prepareStack(ctx, cluster)
	if err != nil {
		return err
	}

	var destroyOpts []optdestroy.Option
	if eventCh != nil {
		destroyOpts = append(destroyOpts, optdestroy.EventStreams(eventCh))
	}

	if _, err := stack.Destroy(ctx, destroyOpts...); err != nil {
		return fmt.Errorf("destroying cluster infrastructure via pulumi: %w", err)
	}

	return nil
}

// Halt gracefully stops running VM domains without destroying disks.
func (r *Runner) Halt(ctx context.Context, cluster *config.ResolvedCluster) error {
	for _, node := range cluster.Nodes {
		domName := fmt.Sprintf("%s-%s", cluster.Project, node.Name)
		_, _ = fmt.Fprintf(r.Stdout, "==> Gracefully shutting down domain %s...\n", domName)

		cmd := exec.CommandContext(ctx, "virsh", "-c", "qemu:///system", "shutdown", domName)
		cmd.Stdout = r.Stdout
		cmd.Stderr = r.Stderr
		_ = cmd.Run() // best effort
	}

	return nil
}

// Start boots any domains in the cluster that are currently shut off.
func (r *Runner) Start(ctx context.Context, cluster *config.ResolvedCluster) error {
	for _, node := range cluster.Nodes {
		domName := fmt.Sprintf("%s-%s", cluster.Project, node.Name)
		state := r.GetDomainStatus(ctx, domName)

		if state == "shut off" {
			_, _ = fmt.Fprintf(r.Stdout, "==> Booting domain %s...\n", domName)

			cmd := exec.CommandContext(ctx, "virsh", "-c", "qemu:///system", "start", domName)
			cmd.Stdout = r.Stdout
			cmd.Stderr = r.Stderr

			if err := cmd.Run(); err != nil {
				return fmt.Errorf("starting domain %s: %w", domName, err)
			}
		}
	}

	return nil
}

// GetDomainStatus checks domain status using virsh.
func (*Runner) GetDomainStatus(ctx context.Context, domainName string) string {
	cmd := exec.CommandContext(ctx, "virsh", "-c", "qemu:///system", "domstate", domainName)

	out, err := cmd.Output()
	if err != nil {
		return "not created"
	}

	return strings.TrimSpace(string(out))
}

// Up initializes the stack, installs plugins, and applies the cluster topology.
func (r *Runner) Up(
	ctx context.Context,
	cluster *config.ResolvedCluster,
	eventCh chan<- events.EngineEvent,
) (*Outputs, error) {
	stack, err := r.prepareStack(ctx, cluster)
	if err != nil {
		return nil, err
	}

	var upOpts []optup.Option
	if eventCh != nil {
		upOpts = append(upOpts, optup.EventStreams(eventCh))
	}

	res, err := stack.Up(ctx, upOpts...)
	if err != nil {
		return nil, fmt.Errorf("applying cluster topology via pulumi: %w", err)
	}

	return extractOutputs(res.Outputs), nil
}

func (r *Runner) prepareStack(
	ctx context.Context,
	cluster *config.ResolvedCluster,
) (auto.Stack, error) {
	pulumiCmd, err := auto.InstallPulumiCommand(ctx, &auto.PulumiCommandOptions{})
	if err != nil {
		return auto.Stack{}, fmt.Errorf("locating/installing pulumi engine: %w", err)
	}

	program := NewClusterProgram(cluster)

	stackName := cluster.Topology
	if stackName == "" {
		stackName = "standard"
	}

	proj := workspace.Project{
		Name:    tokens.PackageName(r.ProjectName),
		Runtime: workspace.NewProjectRuntimeInfo("go", nil),
		Backend: &workspace.ProjectBackend{URL: r.BackendURL},
	}

	envVars := map[string]string{
		"PULUMI_CONFIG_PASSPHRASE": os.Getenv("PULUMI_CONFIG_PASSPHRASE"),
	}

	workspaceOpts := []auto.LocalWorkspaceOption{
		auto.Pulumi(pulumiCmd),
		auto.WorkDir(r.WorkDir),
		auto.Project(proj),
		auto.SecretsProvider("passphrase"),
		auto.EnvVars(envVars),
	}

	stack, err := auto.UpsertStackInlineSource(ctx, stackName, r.ProjectName, program, workspaceOpts...)
	if err != nil {
		return auto.Stack{}, fmt.Errorf("upserting pulumi stack %q: %w", stackName, err)
	}

	if err := stack.Workspace().InstallPlugin(ctx, "libvirt", LibvirtPluginVersion); err != nil {
		return auto.Stack{}, fmt.Errorf("installing libvirt plugin (%s): %w", LibvirtPluginVersion, err)
	}

	return stack, nil
}

func extractOutputs(outputs auto.OutputMap) *Outputs {
	res := &Outputs{
		NodeIPs: make(map[string]string),
		NodeIDs: make(map[string]string),
	}

	for k, v := range outputs {
		strVal, ok := v.Value.(string)
		if !ok {
			continue
		}

		switch {
		case strings.HasPrefix(k, "ip_"):
			nodeName := strings.TrimPrefix(k, "ip_")
			res.NodeIPs[nodeName] = strVal
		case strings.HasPrefix(k, "id_"):
			nodeName := strings.TrimPrefix(k, "id_")
			res.NodeIDs[nodeName] = strVal
		default:
		}
	}

	return res
}
