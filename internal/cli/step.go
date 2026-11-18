package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/xenos76/errant/internal/config"
	"github.com/xenos76/errant/internal/snapshot"
)

var validCustomStepName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func newStepCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "step",
		Short: "Manage declarative lifecycle steps and VM snapshots",
	}

	cmd.AddCommand(newStepListCmd())
	cmd.AddCommand(newStepApplyCmd())
	cmd.AddCommand(newStepSaveCmd())
	cmd.AddCommand(newStepRevertCmd())
	cmd.AddCommand(newStepDeleteCmd())

	return cmd
}

func newStepListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List configured lifecycle steps and their snapshot status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cluster, _, err := loadResolvedCluster()
			if err != nil {
				return err
			}

			state, err := snapshot.LoadState(cluster.ProjectDir)
			if err != nil {
				return err
			}

			printStepList(cmd.OutOrStdout(), cluster, state)

			return nil
		},
	}
}

func newStepApplyCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "apply [<step-index-or-name>]",
		Short: "Apply pending lifecycle steps up to the target step and freeze snapshots",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			cluster, cfgPath, err := loadResolvedCluster()
			if err != nil {
				return err
			}

			if len(cluster.Steps) == 0 {
				return fmt.Errorf("no steps defined in %s", cfgPath)
			}

			sshConfigFile := filepath.Join(cluster.ProjectDir, ".errant", "ssh_config")
			if _, err := os.Stat(sshConfigFile); err != nil {
				return fmt.Errorf("ssh_config not found at %s; please run 'errant up' first", sshConfigFile)
			}

			targetIdx := len(cluster.Steps)
			if len(args) > 0 {
				idx, _, err := resolveDeclarativeStepRef(cluster, args[0])
				if err != nil {
					return err
				}

				targetIdx = idx
			}

			mgr := snapshotManagerFactory()

			return applyStepsWithOpts(ctx, cmd.OutOrStdout(), mgr, cluster, applyOptions{
				SSHConfigFile: sshConfigFile,
				TargetIdx:     targetIdx,
				Force:         force,
			})
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Allow applying declarative steps from an ad-hoc user step")

	return cmd
}

func newStepSaveCmd() *cobra.Command {
	var (
		description string
		force       bool
	)

	cmd := &cobra.Command{
		Use:   "save <name>",
		Short: "Freeze the current VM state into a named ad-hoc step snapshot",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			cluster, _, err := loadResolvedCluster()
			if err != nil {
				return err
			}

			mgr := snapshotManagerFactory()

			return saveCustomStep(ctx, cmd.OutOrStdout(), mgr, cluster, saveCustomOptions{
				Name:        args[0],
				Description: description,
				Force:       force,
			})
		},
	}

	cmd.Flags().StringVarP(&description, "description", "m", "", "Optional description for the saved step")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite an existing saved step with the same name")

	return cmd
}

func newStepRevertCmd() *cobra.Command {
	var prune bool

	cmd := &cobra.Command{
		Use:   "revert <step-index-or-name>",
		Short: "Revert VMs to a previously frozen step snapshot",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			cluster, _, err := loadResolvedCluster()
			if err != nil {
				return err
			}

			state, err := snapshot.LoadState(cluster.ProjectDir)
			if err != nil {
				return err
			}

			ref, err := resolveStepRef(cluster, state, args[0])
			if err != nil {
				return err
			}

			if ref.Ambiguous {
				note := fmt.Sprintf(
					"Note: custom step 'user:%s' also exists; use 'user:%s' to target it.",
					ref.Name, ref.Name,
				)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\n", formatStep("Errant", note))
			}

			mgr := snapshotManagerFactory()

			return revertToStep(ctx, cmd.OutOrStdout(), mgr, cluster, state, revertOptions{
				Ref:   ref,
				Prune: prune,
			})
		},
	}

	cmd.Flags().BoolVar(&prune, "prune", false, "Delete downstream declarative snapshots created after the target step")

	return cmd
}

func newStepDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <step-index-or-name>",
		Short: "Delete the snapshot associated with a lifecycle step",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			cluster, _, err := loadResolvedCluster()
			if err != nil {
				return err
			}

			state, err := snapshot.LoadState(cluster.ProjectDir)
			if err != nil {
				return err
			}

			ref, err := resolveStepRef(cluster, state, args[0])
			if err != nil {
				return err
			}

			mgr := snapshotManagerFactory()

			for _, node := range cluster.Nodes {
				domName := fmt.Sprintf("%s-%s", cluster.Project, node.Name)
				if err := mgr.Delete(ctx, domName, ref.SnapName); err != nil {
					return err
				}
			}

			deleteStepFromState(state, ref)

			if err := snapshot.SaveState(cluster.ProjectDir, state); err != nil {
				return err
			}

			_, _ = fmt.Fprintf(
				cmd.OutOrStdout(),
				"%s\n",
				formatStep("Errant", "Deleted snapshot for "+formatStepRefLabel(ref)),
			)

			return nil
		},
	}
}

type stepRef struct {
	Kind      string
	Index     int
	Name      string
	SnapName  string
	Ambiguous bool
}

type applyOptions struct {
	SSHConfigFile string
	TargetIdx     int
	Force         bool
}

type saveCustomOptions struct {
	Name        string
	Description string
	Force       bool
}

type revertOptions struct {
	Ref   stepRef
	Prune bool
}

func deleteStepFromState(state *snapshot.State, ref stepRef) {
	if ref.Kind == snapshot.StepKindUser {
		delete(state.CustomSteps, ref.Name)

		return
	}

	delete(state.Steps, ref.Index)
}

func formatStepRefLabel(ref stepRef) string {
	if ref.Kind == snapshot.StepKindUser {
		return "user:" + ref.Name
	}

	return fmt.Sprintf("step %d (%s)", ref.Index, ref.Name)
}

func printStepList(out io.Writer, cluster *config.ResolvedCluster, state *snapshot.State) {
	header := fmt.Sprintf("Cluster: %s | Topology: %s\n\n", cluster.Project, cluster.Topology)
	_, _ = fmt.Fprint(out, header)
	_, _ = fmt.Fprintf(out, "%-2s %-5s %-18s %-12s %s\n", "", "STEP", "NAME", "SNAPSHOT", "DESCRIPTION")
	_, _ = fmt.Fprintf(out, "%-2s %-5s %-18s %-12s %s\n", "", "----", "----", "--------", "-----------")

	isDeclActive := state.CurrentKind != snapshot.StepKindUser
	baseMarker := " "

	if isDeclActive && state.CurrentStep == 0 && len(state.Steps) > 0 {
		baseMarker = "*"
	}

	baseStatus := "pending"
	if _, ok := state.Steps[0]; ok {
		baseStatus = "saved"
	}

	_, _ = fmt.Fprintf(out, "%-2s %-5d %-18s %-12s %s\n", baseMarker, 0, "base", baseStatus, "Outcome of 'errant up'")

	for _, step := range cluster.Steps {
		marker := " "
		if isDeclActive && state.CurrentStep == step.Index {
			marker = "*"
		}

		status := declarativeStepStatus(state, step)
		_, _ = fmt.Fprintf(out, "%-2s %-5d %-18s %-12s %s\n", marker, step.Index, step.Name, status, step.Description)
	}

	printCustomStepList(out, state)
}

func declarativeStepStatus(state *snapshot.State, step config.ResolvedStep) string {
	rec, ok := state.Steps[step.Index]
	if !ok {
		return "pending"
	}

	if rec.Hash != "" && rec.Hash != snapshot.StepHash(step) {
		return "modified"
	}

	return "saved"
}

func printCustomStepList(out io.Writer, state *snapshot.State) {
	if len(state.CustomSteps) == 0 {
		return
	}

	names := make([]string, 0, len(state.CustomSteps))
	for name := range state.CustomSteps {
		names = append(names, name)
	}

	slices.Sort(names)

	_, _ = fmt.Fprintln(out, "\nSaved (Ad-hoc) Steps:")
	_, _ = fmt.Fprintf(out, "%-2s %-22s %-18s %-12s %s\n", "", "NAME", "PARENT", "SNAPSHOT", "DESCRIPTION")
	_, _ = fmt.Fprintf(out, "%-2s %-22s %-18s %-12s %s\n", "", "----", "------", "--------", "-----------")

	for _, name := range names {
		rec := state.CustomSteps[name]
		marker := " "

		if state.CurrentKind == snapshot.StepKindUser && state.CurrentStepName == name {
			marker = "*"
		}

		parent := formatCustomParent(rec)
		_, _ = fmt.Fprintf(out, "%-2s %-22s %-18s %-12s %s\n", marker, "user:"+name, parent, "saved", rec.Description)
	}
}

func formatCustomParent(rec snapshot.CustomStepRecord) string {
	if rec.ParentStep >= 0 {
		return fmt.Sprintf("%d (%s)", rec.ParentStep, rec.ParentName)
	}

	if rec.ParentName != "" {
		return rec.ParentName
	}

	return "-"
}

func resolveDeclarativeStepRef(cluster *config.ResolvedCluster, ref string) (int, string, error) {
	if ref == "0" || ref == "base" {
		return 0, "base", nil
	}

	if idx, err := strconv.Atoi(ref); err == nil {
		for _, s := range cluster.Steps {
			if s.Index == idx {
				return s.Index, s.Name, nil
			}
		}

		return 0, "", fmt.Errorf("step index %d not found in topology %q", idx, cluster.Topology)
	}

	for _, s := range cluster.Steps {
		if s.Name == ref {
			return s.Index, s.Name, nil
		}
	}

	return 0, "", fmt.Errorf("step %q not found in topology %q", ref, cluster.Topology)
}

func resolveStepRef(cluster *config.ResolvedCluster, state *snapshot.State, ref string) (stepRef, error) {
	if strings.HasPrefix(ref, "user:") {
		name := strings.TrimPrefix(ref, "user:")
		if _, ok := state.CustomSteps[name]; !ok {
			return stepRef{}, fmt.Errorf("saved step %q not found", ref)
		}

		return stepRef{
			Kind:     snapshot.StepKindUser,
			Name:     name,
			SnapName: snapshot.UserSnapshotName(name),
		}, nil
	}

	if idx, name, err := resolveDeclarativeStepRef(cluster, ref); err == nil {
		_, hasCustom := state.CustomSteps[name]

		return stepRef{
			Kind:      snapshot.StepKindDeclarative,
			Index:     idx,
			Name:      name,
			SnapName:  snapshot.StepSnapshotName(idx),
			Ambiguous: hasCustom && idx > 0,
		}, nil
	}

	if _, ok := state.CustomSteps[ref]; ok {
		return stepRef{
			Kind:     snapshot.StepKindUser,
			Name:     ref,
			SnapName: snapshot.UserSnapshotName(ref),
		}, nil
	}

	return stepRef{}, fmt.Errorf("step %q not found in topology %q or saved steps", ref, cluster.Topology)
}

func saveCustomStep(
	ctx context.Context,
	out io.Writer,
	mgr snapshot.Manager,
	cluster *config.ResolvedCluster,
	opts saveCustomOptions,
) error {
	if !validCustomStepName.MatchString(opts.Name) {
		return fmt.Errorf("invalid step name %q: must match ^[a-zA-Z0-9_-]+$", opts.Name)
	}

	if opts.Name == "0" || opts.Name == "base" {
		return fmt.Errorf("step name %q is reserved for the initial baseline", opts.Name)
	}

	state, err := snapshot.LoadState(cluster.ProjectDir)
	if err != nil {
		return err
	}

	if _, exists := state.CustomSteps[opts.Name]; exists && !opts.Force {
		return fmt.Errorf("saved step %q already exists (use --force to overwrite)", opts.Name)
	}

	snapName := snapshot.UserSnapshotName(opts.Name)
	desc := opts.Description

	if desc == "" {
		desc = "user:" + opts.Name
	}

	_, _ = fmt.Fprintf(out, "%s\n", formatStep("Errant", fmt.Sprintf("Saving ad-hoc step user:%s...", opts.Name)))

	nodeNames, err := createClusterSnapshots(ctx, mgr, cluster, snapName, desc)
	if err != nil {
		return err
	}

	parentStep, parentName := resolveParentForCustomStep(state)

	state.Topology = cluster.Topology
	state.CurrentKind = snapshot.StepKindUser
	state.CurrentStepName = opts.Name
	state.CustomSteps[opts.Name] = snapshot.CustomStepRecord{
		Name:        opts.Name,
		Description: opts.Description,
		Snapshot:    snapName,
		ParentStep:  parentStep,
		ParentName:  parentName,
		Nodes:       nodeNames,
		CreatedAt:   time.Now().UTC(),
	}

	if err := snapshot.SaveState(cluster.ProjectDir, state); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(out, "%s\n", formatStep("Errant", fmt.Sprintf("Saved ad-hoc step user:%s.", opts.Name)))

	return nil
}

func resolveParentForCustomStep(state *snapshot.State) (parentStep int, parentName string) {
	if state.CurrentKind == snapshot.StepKindUser {
		return -1, "user:" + state.CurrentStepName
	}

	if len(state.Steps) > 0 {
		return state.CurrentStep, state.CurrentStepName
	}

	return -1, ""
}

func ensureStepZero(
	ctx context.Context,
	out io.Writer,
	mgr snapshot.Manager,
	cluster *config.ResolvedCluster,
	state *snapshot.State,
) error {
	if _, ok := state.Steps[0]; ok {
		return nil
	}

	_, _ = fmt.Fprintf(out, "%s\n", formatStep("Errant", "Freezing step 0 (base) snapshot..."))

	return captureStepSnapshot(ctx, mgr, cluster, state, 0, "base", "")
}

func applyStepsThrough(
	ctx context.Context,
	out io.Writer,
	mgr snapshot.Manager,
	cluster *config.ResolvedCluster,
	sshConfigFile string,
	targetIdx int,
) error {
	return applyStepsWithOpts(ctx, out, mgr, cluster, applyOptions{
		SSHConfigFile: sshConfigFile,
		TargetIdx:     targetIdx,
	})
}

func applyStepsWithOpts(
	ctx context.Context,
	out io.Writer,
	mgr snapshot.Manager,
	cluster *config.ResolvedCluster,
	opts applyOptions,
) error {
	state, err := snapshot.LoadState(cluster.ProjectDir)
	if err != nil {
		return err
	}

	if state.CurrentKind == snapshot.StepKindUser && !opts.Force {
		return fmt.Errorf(
			"currently on ad-hoc step 'user:%s'; revert to a declarative step first or pass --force",
			state.CurrentStepName,
		)
	}

	state.Topology = cluster.Topology

	if err := ensureStepZero(ctx, out, mgr, cluster, state); err != nil {
		return err
	}

	if opts.TargetIdx == 0 {
		return nil
	}

	for _, step := range cluster.Steps {
		if step.Index <= state.CurrentStep || step.Index > opts.TargetIdx {
			continue
		}

		stepLabel := lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render(
			fmt.Sprintf("%d (%s)", step.Index, step.Name),
		)
		_, _ = fmt.Fprintf(out, "\n%s\n", formatStep("Errant", "Applying step "+stepLabel+"..."))

		if err := provisionStepFunc(ctx, cluster, opts.SSHConfigFile, step); err != nil {
			return err
		}

		_, _ = fmt.Fprintf(out, "%s\n", formatStep("Errant", "Freezing snapshot for step "+stepLabel+"..."))

		err := captureStepSnapshot(ctx, mgr, cluster, state, step.Index, step.Name, snapshot.StepHash(step))
		if err != nil {
			return err
		}
	}

	return nil
}

func createClusterSnapshots(
	ctx context.Context,
	mgr snapshot.Manager,
	cluster *config.ResolvedCluster,
	snapName string,
	desc string,
) ([]string, error) {
	nodeNames := make([]string, 0, len(cluster.Nodes))

	for _, node := range cluster.Nodes {
		domName := fmt.Sprintf("%s-%s", cluster.Project, node.Name)
		if err := mgr.Create(ctx, domName, snapName, desc, cluster.SnapshotMode); err != nil {
			return nil, err
		}

		nodeNames = append(nodeNames, node.Name)
	}

	return nodeNames, nil
}

func captureStepSnapshot(
	ctx context.Context,
	mgr snapshot.Manager,
	cluster *config.ResolvedCluster,
	state *snapshot.State,
	index int,
	name string,
	hash string,
) error {
	snapName := snapshot.StepSnapshotName(index)

	nodeNames, err := createClusterSnapshots(ctx, mgr, cluster, snapName, name)
	if err != nil {
		return err
	}

	state.Topology = cluster.Topology
	state.CurrentKind = snapshot.StepKindDeclarative
	state.CurrentStep = index
	state.CurrentStepName = name
	state.Steps[index] = snapshot.StepRecord{
		Index:     index,
		Name:      name,
		Snapshot:  snapName,
		Hash:      hash,
		Nodes:     nodeNames,
		CreatedAt: time.Now().UTC(),
	}

	return snapshot.SaveState(cluster.ProjectDir, state)
}

func revertToStep(
	ctx context.Context,
	out io.Writer,
	mgr snapshot.Manager,
	cluster *config.ResolvedCluster,
	state *snapshot.State,
	opts revertOptions,
) error {
	label := formatStepRefLabel(opts.Ref)
	_, _ = fmt.Fprintf(
		out,
		"%s\n",
		formatStep("Errant", fmt.Sprintf("Reverting topology %q to %s...", cluster.Topology, label)),
	)

	if err := revertClusterNodes(ctx, mgr, cluster, opts); err != nil {
		return err
	}

	state.Topology = cluster.Topology
	state.CurrentKind = opts.Ref.Kind
	state.CurrentStepName = opts.Ref.Name

	if opts.Ref.Kind == snapshot.StepKindDeclarative {
		state.CurrentStep = opts.Ref.Index

		if opts.Prune {
			pruneStateSteps(state, opts.Ref.Index)
		}
	}

	if err := snapshot.SaveState(cluster.ProjectDir, state); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(out, "%s\n", formatStep("Errant", fmt.Sprintf("Reverted to %s.", label)))

	return nil
}

func revertClusterNodes(
	ctx context.Context,
	mgr snapshot.Manager,
	cluster *config.ResolvedCluster,
	opts revertOptions,
) error {
	for _, node := range cluster.Nodes {
		domName := fmt.Sprintf("%s-%s", cluster.Project, node.Name)
		if err := mgr.Revert(ctx, domName, opts.Ref.SnapName); err != nil {
			return err
		}

		if opts.Prune && opts.Ref.Kind == snapshot.StepKindDeclarative {
			if err := pruneDownstreamSnapshots(ctx, mgr, domName, opts.Ref.Index); err != nil {
				return err
			}
		}
	}

	return nil
}

func pruneStateSteps(state *snapshot.State, targetIdx int) {
	for k := range state.Steps {
		if k > targetIdx {
			delete(state.Steps, k)
		}
	}
}

func pruneDownstreamSnapshots(
	ctx context.Context,
	mgr snapshot.Manager,
	domName string,
	targetIdx int,
) error {
	snaps, err := mgr.List(ctx, domName)
	if err != nil {
		return err
	}

	for _, s := range snaps {
		var sIdx int
		if _, err := fmt.Sscanf(s, snapshot.SnapshotPrefix+"%d", &sIdx); err == nil && sIdx > targetIdx {
			if err := mgr.Delete(ctx, domName, s); err != nil {
				return err
			}
		}
	}

	return nil
}
