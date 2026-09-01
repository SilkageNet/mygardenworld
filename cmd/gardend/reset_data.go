package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newResetDataCmd() *cobra.Command {
	var (
		dataDir string
		yes     bool
	)
	cmd := &cobra.Command{
		Use:   "reset-data",
		Short: "Delete local SQLite and state data",
		RunE: func(cmd *cobra.Command, args []string) error {
			absDataDir, err := cleanDataDirPath(dataDir)
			if err != nil {
				return err
			}
			if !yes {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Would delete local data directory: %s\n", absDataDir)
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Re-run with --yes to confirm.")
				return nil
			}
			removed, err := removeDataDir(absDataDir)
			if err != nil {
				return err
			}
			if removed {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Deleted local data directory: %s\n", absDataDir)
			} else {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Local data directory did not exist: %s\n", absDataDir)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dataDir, "data-dir", defaultAppDir("data"), "directory to delete (same default as serve)")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion without prompting")
	return cmd
}

func cleanDataDirPath(dataDir string) (string, error) {
	absDataDir, err := resolveDataDirPath(dataDir)
	if err != nil {
		return "", err
	}
	volumeRoot := filepath.VolumeName(absDataDir) + string(os.PathSeparator)
	if samePath(absDataDir, volumeRoot) {
		return "", fmt.Errorf("refusing to reset filesystem root: %s", absDataDir)
	}
	if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" && samePath(absDataDir, homeDir) {
		return "", fmt.Errorf("refusing to reset home directory: %s", absDataDir)
	}
	if cwd, err := os.Getwd(); err == nil && samePath(absDataDir, cwd) {
		return "", fmt.Errorf("refusing to reset current working directory: %s", absDataDir)
	}
	return absDataDir, nil
}

func resolveDataDirPath(dataDir string) (string, error) {
	if strings.TrimSpace(dataDir) == "" {
		return "", errors.New("--data-dir cannot be empty")
	}
	absDataDir, err := filepath.Abs(dataDir)
	if err != nil {
		return "", fmt.Errorf("resolve data-dir: %w", err)
	}
	absDataDir = filepath.Clean(absDataDir)
	return absDataDir, nil
}

func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	absA, err := filepath.Abs(a)
	if err != nil {
		absA = a
	}
	absB, err := filepath.Abs(b)
	if err != nil {
		absB = b
	}
	return strings.EqualFold(filepath.Clean(absA), filepath.Clean(absB))
}

// defaultAppDir returns the platform-appropriate default directory for app data.
// Windows: %LOCALAPPDATA%\mygardenworld\<sub>
// macOS:   ~/Library/Application Support/mygardenworld/<sub>
// Linux:   ~/.config/mygardenworld/<sub>
func defaultAppDir(sub string) string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(".", sub)
	}
	return filepath.Join(dir, "mygardenworld", sub)
}

func removeDataDir(absDataDir string) (bool, error) {
	info, err := os.Stat(absDataDir)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("stat data-dir: %w", err)
	}
	if !info.IsDir() {
		return false, fmt.Errorf("data-dir is not a directory: %s", absDataDir)
	}
	if err := os.RemoveAll(absDataDir); err != nil {
		return false, fmt.Errorf("delete data-dir: %w", err)
	}
	return true, nil
}
