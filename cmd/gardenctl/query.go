package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	connect "connectrpc.com/connect"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/timestamppb"
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

func newHarvestStatsCmd(opts *ctlOpts) *cobra.Command {
	var runGap time.Duration
	var limitItems int32
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "harvest-stats",
		Short: "Show harvested resources from the latest run",
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, cancel := ctxWithTimeout(opts)
			defer cancel()
			resp, err := queryClient(opts).GetHarvestStats(ctx, connect.NewRequest(&pb.GetHarvestStatsRequest{
				AccountId:     opts.AccountID,
				AccountName:   opts.Account,
				RunGapSeconds: int32(runGap.Seconds()),
				LimitItems:    limitItems,
			}))
			if err != nil {
				return err
			}
			if jsonOut {
				printJSON(resp.Msg)
				return nil
			}
			printHarvestStats(resp.Msg)
			return nil
		},
	}
	cmd.Flags().DurationVar(&runGap, "run-gap", 30*time.Minute, "gap that separates harvest runs")
	cmd.Flags().Int32Var(&limitItems, "limit-items", 0, "max item rows per account (0 = all)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print raw JSON response")
	return cmd
}

func printHarvestStats(stats *pb.GetHarvestStatsResponse) {
	if stats.GetHarvestOps() == 0 {
		fmt.Println("No successful harvest operations found.")
		return
	}
	fmt.Printf("Latest harvest run: %s - %s  (%d ops, gap <= %s)\n",
		formatProtoTime(stats.GetWindowStart()), formatProtoTime(stats.GetWindowEnd()),
		stats.GetHarvestOps(), (time.Duration(stats.GetRunGapSeconds()) * time.Second).String())
	for _, account := range stats.GetAccounts() {
		fmt.Printf("\n%s: %d ops, %s - %s\n",
			account.GetAccountName(), account.GetHarvestOps(),
			formatProtoTime(account.GetFirstHarvestAt()), formatProtoTime(account.GetLastHarvestAt()))
		fmt.Printf("  Totals: EXP %d, flowers %d, essences %d",
			account.GetExperienceTotal(), account.GetFlowerTotal(), account.GetEssenceTotal())
		if account.GetOtherTotal() > 0 {
			fmt.Printf(", other %d", account.GetOtherTotal())
		}
		fmt.Println()
		for _, group := range groupedHarvestItems(account.GetItems()) {
			fmt.Printf("  %s: %s\n", group.label, strings.Join(group.parts, ", "))
		}
	}
}

type harvestItemGroup struct {
	label string
	parts []string
}

func groupedHarvestItems(items []*pb.HarvestItemTotal) []harvestItemGroup {
	labels := map[string]string{
		"experience": "Experience",
		"flower":     "Flowers",
		"essence":    "Essences",
		"item":       "Items",
	}
	order := []string{"experience", "flower", "essence", "item"}
	parts := make(map[string][]string, len(order))
	for _, item := range items {
		category := item.GetCategory()
		if category == "" {
			category = "item"
		}
		parts[category] = append(parts[category], fmt.Sprintf("%s x%d", item.GetItemName(), item.GetCount()))
	}
	out := make([]harvestItemGroup, 0, len(order))
	for _, category := range order {
		if len(parts[category]) == 0 {
			continue
		}
		out = append(out, harvestItemGroup{label: labels[category], parts: parts[category]})
	}
	return out
}

func formatProtoTime(ts *timestamppb.Timestamp) string {
	if ts == nil {
		return "-"
	}
	return ts.AsTime().Local().Format("2006-01-02 15:04:05")
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
