package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	connect "connectrpc.com/connect"
	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/spf13/cobra"
)

type authConfig struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func authConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "mygardenworld", "gardenctl-auth.json"), nil
}

func loadAuthConfig() (authConfig, error) {
	path, err := authConfigPath()
	if err != nil {
		return authConfig{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return authConfig{}, err
	}
	var cfg authConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return authConfig{}, err
	}
	return cfg, nil
}

func saveAuthConfig(cfg authConfig) error {
	path, err := authConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func clearAuthConfig() error {
	path, err := authConfigPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func newAuthCmd(opts *ctlOpts) *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: "Authenticate gardenctl"}
	var username, password string

	login := &cobra.Command{
		Use:   "login",
		Short: "Log in and save a local access token",
		RunE: func(_ *cobra.Command, _ []string) error {
			if username == "" || password == "" {
				return errors.New("--username and --password required")
			}
			ctx, cancel := ctxWithTimeout(opts)
			defer cancel()
			resp, err := authClient(opts).Login(ctx, connect.NewRequest(&pb.LoginRequest{Username: username, Password: password}))
			if err != nil {
				return err
			}
			if err := saveAuthConfig(authConfig{AccessToken: resp.Msg.GetAccessToken(), RefreshToken: resp.Msg.GetRefreshToken()}); err != nil {
				return err
			}
			printJSON(resp.Msg.GetUser())
			return nil
		},
	}
	login.Flags().StringVar(&username, "username", "", "platform username or email")
	login.Flags().StringVar(&password, "password", "", "platform password")

	refresh := &cobra.Command{
		Use:   "refresh",
		Short: "Refresh and save the local access token",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := loadAuthConfig()
			if err != nil || cfg.RefreshToken == "" {
				return errors.New("no saved refresh token; run gardenctl auth login")
			}
			ctx, cancel := ctxWithTimeout(opts)
			defer cancel()
			resp, err := authClient(opts).Refresh(ctx, connect.NewRequest(&pb.RefreshRequest{RefreshToken: cfg.RefreshToken}))
			if err != nil {
				return err
			}
			if err := saveAuthConfig(authConfig{AccessToken: resp.Msg.GetAccessToken(), RefreshToken: resp.Msg.GetRefreshToken()}); err != nil {
				return err
			}
			printJSON(resp.Msg.GetUser())
			return nil
		},
	}

	logout := &cobra.Command{
		Use:   "logout",
		Short: "Revoke the saved refresh token and clear local auth",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, _ := loadAuthConfig()
			if cfg.RefreshToken != "" {
				ctx, cancel := ctxWithTimeout(opts)
				defer cancel()
				_, _ = authClient(opts).Logout(ctx, connect.NewRequest(&pb.LogoutRequest{RefreshToken: cfg.RefreshToken}))
			}
			return clearAuthConfig()
		},
	}

	me := &cobra.Command{
		Use:   "me",
		Short: "Show the current authenticated user",
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, cancel := ctxWithTimeout(opts)
			defer cancel()
			resp, err := authClient(opts).GetMe(ctx, connect.NewRequest(&pb.GetMeRequest{}))
			if err != nil {
				return err
			}
			printJSON(resp.Msg.GetUser())
			return nil
		},
	}

	cmd.AddCommand(login, refresh, logout, me)
	return cmd
}
