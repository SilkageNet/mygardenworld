package updatecmd

import (
	"fmt"
	"os"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/buildinfo"
	"github.com/SilkageNet/mygardenworld/internal/updater"
	"github.com/spf13/cobra"
)

func New(binaryName string) *cobra.Command {
	var (
		repo    string
		version string
		target  string
		force   bool
		dryRun  bool
		timeout time.Duration
	)
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update this binary from GitHub Releases",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := updater.ContextWithTimeout(cmd.Context(), timeout)
			defer cancel()
			result, err := updater.Run(ctx, updater.Options{
				Repo:           repo,
				Version:        version,
				BinaryName:     binaryName,
				CurrentVersion: buildinfo.GetVersion(),
				TargetPath:     target,
				Force:          force,
				DryRun:         dryRun,
			})
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintln(os.Stdout, result.Message)
			if result.AssetName != "" {
				_, _ = fmt.Fprintf(os.Stdout, "asset: %s\n", result.AssetName)
			}
			if result.TargetPath != "" {
				_, _ = fmt.Fprintf(os.Stdout, "target: %s\n", result.TargetPath)
			}
			if result.BackupPath != "" {
				_, _ = fmt.Fprintf(os.Stdout, "backup: %s\n", result.BackupPath)
			}
			if result.ChecksumOK {
				_, _ = fmt.Fprintln(os.Stdout, "checksum: ok")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&repo, "repo", updater.DefaultRepo, "GitHub repository in owner/name form")
	cmd.Flags().StringVar(&version, "version", "latest", "release tag to install, or latest")
	cmd.Flags().StringVar(&target, "target", "", "path to replace (default: current executable)")
	cmd.Flags().BoolVar(&force, "force", false, "install even when the target version is not newer")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be installed without changing files")
	cmd.Flags().DurationVar(&timeout, "update-timeout", 2*time.Minute, "update network timeout")
	return cmd
}
