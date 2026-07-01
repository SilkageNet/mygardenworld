// gardencap runs a local, Go-native MITM capture proxy for authorized protocol
// analysis sessions.
package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/buildinfo"
	"github.com/SilkageNet/mygardenworld/internal/captureanalysis"
	"github.com/SilkageNet/mygardenworld/internal/captureproxy"
	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "gardencap",
		Short:         "Run a local capture proxy for mygardenworld protocol analysis",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newServeCmd(), newAnalyzeCmd(), newVersionCmd())
	return root
}

func newServeCmd() *cobra.Command {
	var (
		listen          string
		outDir          string
		sessionName     string
		hosts           string
		maxBodyBytes    int64
		maxWSFrameBytes int64
		verbose         bool
	)
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the capture proxy",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := captureproxy.Options{
				Listen:          listen,
				OutDir:          outDir,
				SessionName:     sessionName,
				HostPatterns:    splitCSV(hosts),
				MaxBodyBytes:    maxBodyBytes,
				MaxWSFrameBytes: maxWSFrameBytes,
				Verbose:         verbose,
			}.Normalize()
			proxy, err := captureproxy.New(opts)
			if err != nil {
				return err
			}
			defer proxy.Close()

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			fmt.Fprintf(cmd.OutOrStdout(), "capture session: %s\n", proxy.SessionDir())
			fmt.Fprintf(cmd.OutOrStdout(), "CA certificate: %s\n", proxy.CACertPath())
			fmt.Fprintf(cmd.OutOrStdout(), "proxy listen: http://%s\n", opts.Listen)
			printDeviceAccessHints(cmd, opts.Listen)
			fmt.Fprintln(cmd.OutOrStdout(), "Open the certificate page on the test device, install/trust the CA, then keep this proxy configured.")
			fmt.Fprintln(cmd.OutOrStdout(), "Press Ctrl+C to stop.")
			started := time.Now()
			err = proxy.Run(ctx)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "capture stopped after %s\n", time.Since(started).Round(time.Second))
			return nil
		},
	}
	cmd.Flags().StringVar(&listen, "listen", captureproxy.DefaultListen, "proxy listen address")
	cmd.Flags().StringVar(&outDir, "out-dir", captureproxy.DefaultOutDir, "capture output directory")
	cmd.Flags().StringVar(&sessionName, "session", "", "session name suffix")
	cmd.Flags().StringVar(&hosts, "hosts", captureproxy.DefaultHostPatterns, "comma-separated host filters; supports exact, *.domain, .domain, *")
	cmd.Flags().Int64Var(&maxBodyBytes, "max-body-bytes", captureproxy.DefaultMaxBodyBytes, "max HTTP body bytes stored per record")
	cmd.Flags().Int64Var(&maxWSFrameBytes, "max-ws-frame-bytes", captureproxy.DefaultMaxWSFrameBytes, "max WebSocket payload bytes stored per frame")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "enable proxy debug logging")
	return cmd
}

func newAnalyzeCmd() *cobra.Command {
	var (
		channel string
		rewrite bool
	)
	cmd := &cobra.Command{
		Use:   "analyze <session-dir>",
		Short: "Build or refresh decoded capture analysis",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ch, err := babigame.ParseChannel(channel)
			if err != nil {
				return err
			}
			report, err := captureanalysis.AnalyzeSession(args[0], captureanalysis.Options{
				Channel: ch,
				Rewrite: rewrite,
			})
			if err != nil {
				return err
			}
			captureanalysis.PrintReport(report)
			return nil
		},
	}
	cmd.Flags().StringVar(&channel, "channel", string(babigame.ChannelIOS), "protocol channel used to decode captured frames")
	cmd.Flags().BoolVar(&rewrite, "rewrite", false, "rebuild rpc.jsonl from websocket.jsonl even when it already exists")
	return cmd
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print build info",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("gardencap %s (commit=%s date=%s)\n", buildinfo.GetVersion(), buildinfo.Commit, buildinfo.Date)
		},
	}
}

func splitCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func printDeviceAccessHints(cmd *cobra.Command, listen string) {
	endpoints, phoneReachable := deviceProxyEndpoints(listen)
	if len(endpoints) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "phone proxy candidates: none detected")
		fmt.Fprintln(cmd.OutOrStdout(), "certificate page: "+displayHTTPURL(listen))
		return
	}
	if phoneReachable {
		fmt.Fprintln(cmd.OutOrStdout(), "phone proxy candidates:")
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "phone proxy candidates: loopback only; use --listen 0.0.0.0:<port> for a separate phone")
	}
	for _, endpoint := range endpoints {
		fmt.Fprintf(cmd.OutOrStdout(), "  proxy: %s\n", endpoint)
		fmt.Fprintf(cmd.OutOrStdout(), "  certificate page: http://%s/\n", endpoint)
	}
}

func deviceProxyEndpoints(listen string) ([]string, bool) {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return []string{strings.TrimPrefix(strings.TrimPrefix(listen, "http://"), "https://")}, false
	}
	host = strings.Trim(host, "[]")
	if host == "" || host == "0.0.0.0" || host == "::" {
		ips := localPrivateIPv4s()
		out := make([]string, 0, len(ips))
		for _, ip := range ips {
			out = append(out, net.JoinHostPort(ip, port))
		}
		return out, len(out) > 0
	}
	ip := net.ParseIP(host)
	if host == "localhost" || (ip != nil && ip.IsLoopback()) {
		return []string{net.JoinHostPort("127.0.0.1", port)}, false
	}
	return []string{net.JoinHostPort(host, port)}, true
}

func localPrivateIPv4s() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var out []string
	seen := make(map[string]struct{})
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			ip = ip.To4()
			if ip == nil || !ip.IsPrivate() {
				continue
			}
			s := ip.String()
			if _, ok := seen[s]; ok {
				continue
			}
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}

func displayHTTPURL(listen string) string {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return "http://" + strings.TrimRight(listen, "/") + "/"
	}
	host = strings.Trim(host, "[]")
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port) + "/"
}
