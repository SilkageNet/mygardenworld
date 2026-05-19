package main

import (
	"errors"
	"fmt"
	"os"

	connect "connectrpc.com/connect"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/spf13/cobra"
)

func newAccountCmd(opts *ctlOpts) *cobra.Command {
	cmd := &cobra.Command{Use: "account", Short: "Manage accounts"}
	var (
		username string
		password string
		channel  string
		login    bool
	)
	add := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a new game account",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			ctx, cancel := ctxWithTimeout(opts)
			defer cancel()
			ch, err := parseChannelString(channel)
			if err != nil {
				return err
			}
			resp, err := accountClient(opts).CreateAccount(ctx, connect.NewRequest(&pb.CreateAccountRequest{
				Name:     args[0],
				Username: username,
				Password: password,
				Channel:  ch,
				LoginNow: login,
			}))
			if err != nil {
				return err
			}
			printJSON(resp.Msg.GetAccount())
			if resp.Msg.GetLoginError() != "" {
				fmt.Fprintln(os.Stderr, "login_error:", resp.Msg.GetLoginError())
			}
			return nil
		},
	}
	add.Flags().StringVar(&username, "username", "", "babigame username (required)")
	add.Flags().StringVar(&password, "password", "", "babigame password (required)")
	add.Flags().StringVar(&channel, "channel", "ios", "channel: ios (only supported value today)")
	add.Flags().BoolVar(&login, "login", false, "log in immediately after adding")

	list := &cobra.Command{
		Use:   "list",
		Short: "List accounts",
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, cancel := ctxWithTimeout(opts)
			defer cancel()
			resp, err := accountClient(opts).ListAccounts(ctx, connect.NewRequest(&pb.ListAccountsRequest{}))
			if err != nil {
				return err
			}
			printJSON(resp.Msg.GetAccounts())
			return nil
		},
	}

	rm := &cobra.Command{
		Use:   "rm",
		Short: "Delete an account",
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, cancel := ctxWithTimeout(opts)
			defer cancel()
			_, err := accountClient(opts).DeleteAccount(ctx, connect.NewRequest(&pb.DeleteAccountRequest{Id: opts.AccountID, Name: opts.Account}))
			return err
		},
	}

	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Force a fresh login (rebuilds the WS)",
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, cancel := ctxWithTimeout(opts)
			defer cancel()
			resp, err := accountClient(opts).LoginAccount(ctx, connect.NewRequest(&pb.LoginAccountRequest{Id: opts.AccountID, Name: opts.Account}))
			if err != nil {
				return err
			}
			printJSON(resp.Msg.GetAccount())
			return nil
		},
	}

	cmd.AddCommand(add, list, rm, loginCmd)
	return cmd
}

func parseChannelString(s string) (pb.Channel, error) {
	switch s {
	case "ios", "CHANNEL_IOS":
		return pb.Channel_CHANNEL_IOS, nil
	case "":
		return pb.Channel_CHANNEL_UNSPECIFIED, errors.New("--channel required")
	default:
		return pb.Channel_CHANNEL_UNSPECIFIED, fmt.Errorf("unknown channel %q (try: ios)", s)
	}
}
