package cli

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/xenos76/errant/internal/wizard"
)

var (
	initNonInteractive bool
	initArchetype      string
	initOutput         string
)

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "init",
		Aliases: []string{"wizard"},
		Short:   "Initialize a new Errantfile.yaml via an interactive Bubble Tea wizard",
		Long: `Launch an interactive Bubble Tea terminal wizard to scaffold a new
Errantfile.yaml with custom VM sizing, network subnets, base OS cloud images,
and multi-node topologies.`,
		RunE: runInit,
	}

	cmd.Flags().BoolVarP(
		&initNonInteractive, "non-interactive", "y", false,
		"Generate Errantfile.yaml using defaults without interactive prompts",
	)
	cmd.Flags().StringVarP(
		&initArchetype, "archetype", "a", "k8s-cluster",
		"Topology archetype (k8s-cluster, k8s-ha, single-vm, custom)",
	)
	cmd.Flags().StringVarP(
		&initOutput, "output", "o", "Errantfile.yaml",
		"Target output file path",
	)

	return cmd
}

func runInit(_ *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	projectName := "errant-cluster"

	if err == nil {
		projectName = filepath.Base(cwd)
	}

	if initNonInteractive {
		ans := wizard.NewDefaultAnswers(projectName)
		ans.ArchetypeID = initArchetype
		ans.OutputPath = initOutput

		written, writeErr := wizard.WriteErrantfile(ans)
		if writeErr != nil {
			return writeErr
		}

		fmt.Printf("✔ Successfully created %s (archetype: %s)\n", written, initArchetype)

		return nil
	}

	model := wizard.NewModel(projectName)
	p := tea.NewProgram(model)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("running interactive wizard: %w", err)
	}

	return nil
}
