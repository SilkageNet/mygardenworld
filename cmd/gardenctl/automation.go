package main

import (
	connect "connectrpc.com/connect"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/spf13/cobra"
)

func newAutomationCmd(opts *ctlOpts) *cobra.Command {
	cmd := &cobra.Command{Use: "auto", Short: "Start / stop / reload the automation runner"}
	run := func(action string) func(*cobra.Command, []string) error {
		return func(_ *cobra.Command, _ []string) error {
			ctx, cancel := ctxWithTimeout(opts)
			defer cancel()
			c := automationClient(opts)
			var err error
			switch action {
			case "start":
				_, err = c.Start(ctx, connect.NewRequest(&pb.StartRequest{AccountId: opts.AccountID, AccountName: opts.Account}))
			case "stop":
				_, err = c.Stop(ctx, connect.NewRequest(&pb.StopRequest{AccountId: opts.AccountID, AccountName: opts.Account}))
			case "reload":
				_, err = c.Reload(ctx, connect.NewRequest(&pb.ReloadRequest{AccountId: opts.AccountID, AccountName: opts.Account}))
			}
			return err
		}
	}
	cmd.AddCommand(
		&cobra.Command{Use: "start", Short: "Enable automation for the account", RunE: run("start")},
		&cobra.Command{Use: "stop", Short: "Disable automation (WS stays connected)", RunE: run("stop")},
		&cobra.Command{Use: "reload", Short: "Reconnect the WS for the account", RunE: run("reload")},
	)
	return cmd
}
