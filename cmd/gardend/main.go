// gardend is the long-running daemon that owns per-account WebSocket sessions
// and exposes a Connect-based control plane with JWT authentication.
//
// Subcommands:
//
//	gardend serve   --data-dir <dir> --listen <addr> --jwt-secret <secret>
//	gardend reset-data --data-dir <dir> --yes
//	gardend compact-db --data-dir <dir> --yes
//	gardend version
package main

import (
	"fmt"
	"os"

	"github.com/SilkageNet/mygardenworld/internal/updatecmd"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:           "gardend",
		Short:         "小云朵 local automation daemon",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	rootCmd.AddCommand(newServeCmd(), newResetDataCmd(), newCompactDBCmd(), newVersionCmd(), updatecmd.New("gardend"))
	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
