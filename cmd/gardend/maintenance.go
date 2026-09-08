package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/store"
	"github.com/spf13/cobra"
)

func newMaintenanceCmd() *cobra.Command {
	var dataDir string
	var wait time.Duration
	var resume bool
	cmd := &cobra.Command{
		Use:       "maintenance [on|off|status]",
		Short:     "Pause game I/O without changing user policies (requires local database access)",
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs: []string{"on", "off", "status"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if wait < 0 {
				return fmt.Errorf("wait must not be negative")
			}
			if resume && args[0] != "off" {
				return fmt.Errorf("--resume-enabled requires off")
			}
			path := filepath.Join(dataDir, "garden.db")
			if _, err := os.Stat(path); err != nil {
				return fmt.Errorf("existing daemon database required: %w", err)
			}
			db, err := store.Open(cmd.Context(), path)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()
			if args[0] == "status" {
				s, err := db.Maintenance(cmd.Context())
				if err != nil {
					return err
				}
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "maintenance=%t revision=%d acknowledged=%t (durable state, not a daemon health check)\n", s.Enabled, s.Revision, s.AppliedRevision == s.Revision)
				return err
			}
			s, err := db.RequestMaintenance(cmd.Context(), args[0] == "on", resume)
			if err != nil {
				return err
			}
			if wait == 0 {
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "maintenance=%t requested (revision %d); not yet confirmed, will apply when daemon runs\n", s.Enabled, s.Revision)
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), wait)
			defer cancel()
			ticker := time.NewTicker(200 * time.Millisecond)
			defer ticker.Stop()
			for {
				current, err := db.Maintenance(ctx)
				if err != nil {
					return fmt.Errorf("request saved, confirmation failed: %w", err)
				}
				if current.Revision != s.Revision {
					return fmt.Errorf("maintenance request superseded by another operator command")
				}
				if current.AppliedRevision == s.Revision {
					_, err = fmt.Fprintf(cmd.OutOrStdout(), "maintenance=%t confirmed by daemon; user policies unchanged\n", s.Enabled)
					return err
				}
				select {
				case <-ctx.Done():
					return fmt.Errorf("request saved but daemon has not confirmed completion; check maintenance status: %w", ctx.Err())
				case <-ticker.C:
				}
			}
		},
	}
	cmd.Flags().StringVar(&dataDir, "data-dir", defaultAppDir("data"), "existing daemon data directory")
	cmd.Flags().DurationVar(&wait, "wait", 60*time.Second, "wait for daemon acknowledgement; 0 only queues the request")
	cmd.Flags().BoolVar(&resume, "resume-enabled", false, "on exit, explicitly restore active users' automation-enabled accounts")
	return cmd
}
