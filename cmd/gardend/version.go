package main

import (
	"fmt"

	"github.com/SilkageNet/mygardenworld/internal/buildinfo"
	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print build info",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("gardend %s (commit=%s date=%s)\n", buildinfo.GetVersion(), buildinfo.Commit, buildinfo.Date)
		},
	}
}
