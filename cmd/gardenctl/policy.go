package main

import (
	"fmt"
	"os"

	connect "connectrpc.com/connect"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/spf13/cobra"
)

func newPolicyCmd(opts *ctlOpts) *cobra.Command {
	cmd := &cobra.Command{Use: "policy", Short: "Inspect / edit per-account policy"}

	get := &cobra.Command{
		Use:   "get",
		Short: "Print the effective policy",
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, cancel := ctxWithTimeout(opts)
			defer cancel()
			resp, err := policyClient(opts).GetPolicy(ctx, connect.NewRequest(&pb.GetPolicyRequest{
				AccountId:   opts.AccountID,
				AccountName: opts.Account,
			}))
			if err != nil {
				return err
			}
			printJSON(resp.Msg.GetPolicy())
			return nil
		},
	}

	set := &cobra.Command{
		Use:   "set <key=value> [<key=value>...]",
		Short: "Patch one or more policy keys (dot-path)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			ctx, cancel := ctxWithTimeout(opts)
			defer cancel()
			resp, err := policyClient(opts).UpdatePolicy(ctx, connect.NewRequest(&pb.UpdatePolicyRequest{
				AccountId:   opts.AccountID,
				AccountName: opts.Account,
				Entries:     args,
			}))
			if err != nil {
				return err
			}
			for _, e := range resp.Msg.GetErrors() {
				fmt.Fprintln(os.Stderr, "patch error:", e.GetEntry(), "->", e.GetMessage())
			}
			printJSON(resp.Msg.GetPolicy())
			return nil
		},
	}

	cmd.AddCommand(get, set)
	return cmd
}
