// gardenctl is the operator-facing CLI. It connects to a running gardend
// over Connect (which the daemon serves alongside gRPC + gRPC-Web on the
// same HTTP/2 port). Today we use the Connect protocol for everything;
// the same generated client supports gRPC by passing
// `connect.WithGRPC()` to the constructor if needed.
//
// The address is plain http://host:port by default for a local daemon; use an
// https:// --addr when talking to a remote deployment.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/buildinfo"
	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type ctlOpts struct {
	Addr      string
	AccountID string
	Account   string
	Timeout   time.Duration
	AuthToken string
}

// baseURL converts the user-facing --addr (host:port form) into the absolute
// http:// URL Connect clients want. We keep --addr as host:port to match the
// gRPC-style flag users are familiar with.
func (o *ctlOpts) baseURL() string {
	addr := o.Addr
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		addr = "http://" + addr
	}
	return strings.TrimRight(addr, "/")
}

func newRootCmd() *cobra.Command {
	opts := &ctlOpts{}
	root := &cobra.Command{
		Use:           "gardenctl",
		Short:         "Control plane CLI for gardend (Connect / gRPC / gRPC-Web client)",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&opts.Addr, "addr", "127.0.0.1:50051", "gardend address (host:port or URL)")
	root.PersistentFlags().DurationVar(&opts.Timeout, "timeout", 30*time.Second, "RPC timeout (does not apply to streams)")
	root.PersistentFlags().StringVar(&opts.AccountID, "account-id", "", "account id (for commands that target one account)")
	root.PersistentFlags().StringVar(&opts.Account, "account", "", "account name (for commands that target one account)")
	root.PersistentFlags().StringVar(&opts.AuthToken, "auth-token", "", "JWT access token (or GARDENCTL_TOKEN env / saved auth config)")

	root.AddCommand(
		newAuthCmd(opts),
		newAccountCmd(opts),
		newPolicyCmd(opts),
		newAutomationCmd(opts),
		newStatusCmd(opts),
		newSnapshotCmd(opts),
		newWatchCmd(opts),
		newVersionCmd(),
	)
	return root
}

func ctxWithTimeout(opts *ctlOpts) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), opts.Timeout)
}

func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print build info",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("gardenctl %s (commit=%s date=%s)\n", buildinfo.GetVersion(), buildinfo.Commit, buildinfo.Date)
		},
	}
}
