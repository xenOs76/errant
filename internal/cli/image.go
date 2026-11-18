package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/xenos76/errant/internal/builder"
	"github.com/xenos76/errant/internal/cache"
	"github.com/xenos76/errant/internal/config"
)

func newImageCmd() *cobra.Command {
	imageCmd := &cobra.Command{
		Use:   "image",
		Short: "Manage and build cached base images",
		Long:  "Commands to list, build, and prune local cached virtual machine base images.",
	}

	imageCmd.AddCommand(newListImagesCmd())
	imageCmd.AddCommand(newBuildImageCmd())
	imageCmd.AddCommand(newPruneImagesCmd())

	return imageCmd
}

func newListImagesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List cached base images",
		RunE: func(_ *cobra.Command, _ []string) error {
			cacheDir := cache.GetCacheDir()

			entries, err := os.ReadDir(cacheDir)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Printf("No cached images found in %s\n", cacheDir)

					return nil
				}

				return fmt.Errorf("reading cache directory %s: %w", cacheDir, err)
			}

			fmt.Printf("Cache Directory: %s\n\n", lipgloss.NewStyle().Foreground(colorCyan).Bold(true).Render(cacheDir))
			fmt.Printf("%-44s %-12s %s\n", "NAME", "SIZE", "MODIFIED")
			fmt.Printf("%-44s %-12s %s\n", "----", "----", "--------")

			count, totalSize := renderImageEntries(entries)
			if count == 0 {
				fmt.Println("No image files (.qcow2, .img, .iso) found.")

				return nil
			}

			fmt.Printf("\nTotal: %d cached image(s) (%s)\n", count, cache.FormatBytes(totalSize))

			return nil
		},
	}
}

func renderImageEntries(entries []os.DirEntry) (int, int64) {
	var (
		count     int
		totalSize int64
	)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !isImageFile(name) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		count++
		totalSize += info.Size()

		sizeStr := cache.FormatBytes(info.Size())
		modStr := info.ModTime().Format("2006-01-02 15:04")
		nameStyled := padStyled(lipgloss.NewStyle().Foreground(colorCyan).Render(name), 44)

		fmt.Printf("%s %-12s %s\n", nameStyled, sizeStr, modStr)
	}

	return count, totalSize
}

func isImageFile(name string) bool {
	return strings.HasSuffix(name, ".qcow2") ||
		strings.HasSuffix(name, ".img") ||
		strings.HasSuffix(name, ".iso")
}

func newBuildImageCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "build [name]",
		Short: "Build declarative VM base image recipes",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			targetName := ""
			if len(args) > 0 {
				targetName = args[0]
			}

			return executeBuildImage(ctx, targetName)
		},
	}
}

func buildFromURIOrAlias(ctx context.Context, targetName, cacheDir string) (bool, error) {
	if targetName == "" {
		return false, nil
	}

	aliasOrURI := targetName
	if resolved := config.ResolveBox(targetName); resolved != targetName {
		aliasOrURI = resolved
	}

	if !builder.IsNixURI(aliasOrURI) {
		return false, nil
	}

	cfg, err := builder.ParseNixURI(aliasOrURI)
	if err != nil {
		return true, err
	}

	_, err = builder.BuildImage(ctx, "", cfg, cacheDir)

	return true, err
}

func buildDeclaredRecipes(ctx context.Context, cfg *config.Config, targetName, cacheDir, cfgPath string) error {
	if targetName != "" {
		recipe, ok := cfg.Images[targetName]
		if !ok {
			return fmt.Errorf("image %q not declared in %s", targetName, cfgPath)
		}

		_, err := builder.BuildImage(ctx, targetName, recipe, cacheDir)

		return err
	}

	if len(cfg.Images) == 0 {
		fmt.Printf("No images declared under 'images:' block in %s\n", cfgPath)

		return nil
	}

	for name, recipe := range cfg.Images {
		if _, err := builder.BuildImage(ctx, name, recipe, cacheDir); err != nil {
			return fmt.Errorf("building image %s: %w", name, err)
		}
	}

	return nil
}

func executeBuildImage(ctx context.Context, targetName string) error {
	cacheDir := cache.GetCacheDir()

	handled, err := buildFromURIOrAlias(ctx, targetName, cacheDir)
	if handled || err != nil {
		return err
	}

	cfgPath, err := config.FindConfigFile(configFile)
	if err != nil {
		return fmt.Errorf("finding configuration file: %w", err)
	}

	cfg, err := config.LoadFile(cfgPath)
	if err != nil {
		return fmt.Errorf("loading configuration file %s: %w", cfgPath, err)
	}

	return buildDeclaredRecipes(ctx, cfg, targetName, cacheDir, cfgPath)
}

func newPruneImagesCmd() *cobra.Command {
	var pruneAll bool

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Remove incomplete or cached base images",
		RunE: func(_ *cobra.Command, _ []string) error {
			cacheDir := cache.GetCacheDir()

			entries, err := os.ReadDir(cacheDir)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Printf("Cache directory %s does not exist.\n", cacheDir)

					return nil
				}

				return fmt.Errorf("reading cache directory %s: %w", cacheDir, err)
			}

			removedCount, freedBytes := pruneCacheEntries(cacheDir, entries, pruneAll)
			if removedCount == 0 {
				fmt.Println("No matching cache files to prune.")

				return nil
			}

			fmt.Printf("\nPruned %d file(s), freed %s\n", removedCount, cache.FormatBytes(freedBytes))

			return nil
		},
	}

	cmd.Flags().BoolVarP(&pruneAll, "all", "a", false, "Remove all cached images (not just .part files)")

	return cmd
}

func pruneCacheEntries(cacheDir string, entries []os.DirEntry, pruneAll bool) (int, int64) {
	var (
		removed int
		freed   int64
	)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		shouldRemove := strings.HasSuffix(name, ".part") || (pruneAll && isImageFile(name))
		if !shouldRemove {
			continue
		}

		fullPath := filepath.Join(cacheDir, name)

		info, err := entry.Info()
		if err == nil {
			freed += info.Size()
		}

		if err := os.Remove(fullPath); err == nil {
			removed++

			fmt.Printf("  - Removed: %s\n", name)
		}
	}

	return removed, freed
}
