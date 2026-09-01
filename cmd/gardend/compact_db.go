package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/SilkageNet/mygardenworld/internal/store"
	"github.com/spf13/cobra"
)

func newCompactDBCmd() *cobra.Command {
	var (
		dataDir string
		yes     bool
	)
	cmd := &cobra.Command{
		Use:   "compact-db",
		Short: "Reclaim unused SQLite database space",
		Long:  "Reclaim unused SQLite database space after log cleanup. Stop gardend before running this offline maintenance command.",
		RunE: func(cmd *cobra.Command, args []string) error {
			absDataDir, err := resolveDataDirPath(dataDir)
			if err != nil {
				return err
			}
			dbPath := filepath.Join(absDataDir, "garden.db")
			info, err := os.Stat(dbPath)
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("SQLite database does not exist: %s", dbPath)
			}
			if err != nil {
				return fmt.Errorf("stat SQLite database: %w", err)
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("SQLite database is not a regular file: %s", dbPath)
			}
			if !yes {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Would compact SQLite database: %s\n", dbPath)
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Stop gardend, ensure enough free disk space, then re-run with --yes.")
				return nil
			}

			before := info.Size()
			db, err := store.Open(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("open db: %w", err)
			}
			if err := db.Compact(cmd.Context()); err != nil {
				_ = db.Close()
				return err
			}
			if err := db.Close(); err != nil {
				return fmt.Errorf("close compacted db: %w", err)
			}
			afterInfo, err := os.Stat(dbPath)
			if err != nil {
				return fmt.Errorf("stat compacted SQLite database: %w", err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Compacted SQLite database: %s (%d -> %d bytes)\n", dbPath, before, afterInfo.Size())
			return nil
		},
	}
	cmd.Flags().StringVar(&dataDir, "data-dir", defaultAppDir("data"), "directory containing garden.db (same default as serve)")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm offline database compaction")
	return cmd
}
