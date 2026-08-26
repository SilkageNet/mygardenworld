package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/store"
	"github.com/spf13/cobra"
)

func newMigrateDataCmd() *cobra.Command {
	var (
		dataDir    string
		backupPath string
		yes        bool
	)
	cmd := &cobra.Command{
		Use:   "migrate-data-v1",
		Short: "One-time migration from the final unversioned database to schema v1",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			absDataDir, err := cleanDataDirPath(dataDir)
			if err != nil {
				return err
			}
			dbPath := filepath.Join(absDataDir, "garden.db")
			inspection, err := store.InspectLegacyV0(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			if strings.TrimSpace(backupPath) == "" {
				backupPath = filepath.Join(absDataDir, "garden.db.pre-v1-"+time.Now().UTC().Format("20060102T150405Z")+".bak")
			}
			if !yes {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Legacy database ready: %s (accounts=%d sessions=%d)\n", dbPath, inspection.Accounts, inspection.Sessions)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Would back up to: %s and %s.key\n", backupPath, backupPath)
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Stop gardend, then re-run with --yes to migrate.")
				return nil
			}

			result, err := store.MigrateLegacyV0ToV1(cmd.Context(), dbPath, backupPath, upgradeLegacySessionPayload)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Migrated schema v1: accounts=%d sessions_migrated=%d sessions_dropped=%d\n",
				result.Accounts, result.SessionsMigrated, result.SessionsDropped)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Backup database: %s\nBackup key: %s\n", result.BackupPath, result.BackupKeyPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&dataDir, "data-dir", defaultAppDir("data"), "directory containing the legacy garden.db")
	cmd.Flags().StringVar(&backupPath, "backup", "", "backup database path (default: timestamped file beside garden.db)")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm backup and migration after gardend has been stopped")
	return cmd
}

// upgradeLegacySessionPayload is intentionally isolated in the one-time CLI,
// not the normal session loader. v0 and v1 share a payload shape; v1 adds an
// explicit format version and strict validation before it is encrypted.
func upgradeLegacySessionPayload(data []byte) ([]byte, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, nil
	}
	object["version"] = json.RawMessage("1")
	candidate, err := json.Marshal(object)
	if err != nil {
		return nil, err
	}
	session, err := babigame.UnmarshalSessionJSON(candidate, babigame.Config{})
	if err != nil {
		return nil, nil
	}
	return babigame.MarshalSessionJSON(session)
}
