package main

import (
	"context"
	"fmt"
	"time"

	connect "connectrpc.com/connect"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/spf13/cobra"
)

func newStatusCmd(opts *ctlOpts) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show connected/actionable status per account",
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, cancel := ctxWithTimeout(opts)
			defer cancel()
			resp, err := queryClient(opts).GetStatus(ctx, connect.NewRequest(&pb.GetStatusRequest{AccountId: opts.AccountID, AccountName: opts.Account}))
			if err != nil {
				return err
			}
			printJSON(resp.Msg.GetAccounts())
			return nil
		},
	}
}

func newSnapshotCmd(opts *ctlOpts) *cobra.Command {
	return &cobra.Command{
		Use:   "snapshot",
		Short: "Dump per-land state and inventory for the account",
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, cancel := ctxWithTimeout(opts)
			defer cancel()
			resp, err := queryClient(opts).GetSnapshot(ctx, connect.NewRequest(&pb.GetSnapshotRequest{AccountId: opts.AccountID, AccountName: opts.Account}))
			if err != nil {
				return err
			}
			printJSON(resp.Msg)
			return nil
		},
	}
}

func newWatchCmd(opts *ctlOpts) *cobra.Command {
	var kinds []string
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Stream events from the daemon until Ctrl-C",
		RunE: func(_ *cobra.Command, _ []string) error {
			stream, err := queryClient(opts).StreamEvents(context.Background(), connect.NewRequest(&pb.StreamEventsRequest{
				AccountId:   opts.AccountID,
				AccountName: opts.Account,
				Kinds:       kinds,
			}))
			if err != nil {
				return err
			}
			defer stream.Close()
			for stream.Receive() {
				e := stream.Msg()
				label := e.GetLabel()
				if label == "" {
					label = e.GetKind()
				}
				category := e.GetCategory()
				if category == "" {
					category = "system"
				}
				level := e.GetLevel()
				if level == "" {
					level = "info"
				}
				fmt.Printf("[%s] [%s/%s/%s] %s %s\n",
					e.GetTs().AsTime().Format(time.RFC3339),
					category, label, level, e.GetAccountName(), e.GetMessage())
			}
			return stream.Err()
		},
	}
	cmd.Flags().StringSliceVar(&kinds, "kind", nil, "filter event kinds (repeatable)")
	return cmd
}
