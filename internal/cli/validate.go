package cli

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/xenos76/errant/internal/config"
)

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate Errantfile.yaml against the JSON schema and test resolution",
		RunE: func(_ *cobra.Command, _ []string) error {
			path, err := config.FindConfigFile(configFile)
			if err != nil {
				return err
			}

			cfg, err := config.LoadFile(path)
			if err != nil {
				return err
			}

			styledProject := lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render(cfg.Project)
			fmt.Printf("%s: %s (Project: %s, Schema Version: %s)\n",
				formatCheckmark("Schema valid"), path, styledProject, cfg.Version)
			fmt.Println(formatCheckmark("Available topologies:"))

			for name := range cfg.Topologies {
				resolved, err := cfg.ResolveCluster(name)
				if err != nil {
					return fmt.Errorf("topology %q failed resolution: %w", name, err)
				}

				styledName := lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render(name)
				fmt.Printf("  - %-12s (%d nodes)\n", styledName, len(resolved.Nodes))
			}

			return nil
		},
	}
}
