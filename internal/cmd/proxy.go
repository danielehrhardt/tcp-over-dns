package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/danielehrhardt/tcp-over-dns/internal/config"
	"github.com/danielehrhardt/tcp-over-dns/internal/proxy"
	"github.com/danielehrhardt/tcp-over-dns/internal/ui"
)

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "SOCKS5 proxy commands (start, stop, status)",
	Long:  "Manage the SOCKS5 proxy that routes traffic through the DNS tunnel.",
}

var proxyStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start SOCKS5 proxy via SSH tunnel",
	Long: `Starts a SOCKS5 proxy using SSH dynamic port forwarding through the DNS tunnel.

After starting, configure your applications to use:
  Proxy type: SOCKS5
  Host:       127.0.0.1
  Port:       1080

Or use the proxy in your browser via a proxy extension (e.g., FoxyProxy, SwitchyOmega).`,
	RunE: runProxyStart,
}

var proxyStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the SOCKS5 proxy",
	RunE:  runProxyStop,
}

var proxyStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check proxy status",
	RunE:  runProxyStatus,
}

func init() {
	proxyCmd.AddCommand(proxyStartCmd)
	proxyCmd.AddCommand(proxyStopCmd)
	proxyCmd.AddCommand(proxyStatusCmd)

	proxyStartCmd.Flags().String("listen", "", "proxy listen address (default: 127.0.0.1:1080)")
	proxyStartCmd.Flags().String("ssh-user", "", "SSH username")
	proxyStartCmd.Flags().String("ssh-host", "", "SSH host (tunnel server IP)")
	proxyStartCmd.Flags().Int("ssh-port", 0, "SSH port")
	proxyStartCmd.Flags().String("ssh-key", "", "path to SSH private key")
}

func runProxyStart(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Apply flag overrides
	if listen, _ := cmd.Flags().GetString("listen"); listen != "" {
		cfg.Proxy.Listen = listen
	}
	if user, _ := cmd.Flags().GetString("ssh-user"); user != "" {
		cfg.Proxy.SSHUser = user
	}
	if host, _ := cmd.Flags().GetString("ssh-host"); host != "" {
		cfg.Proxy.SSHHost = host
	}
	if port, _ := cmd.Flags().GetInt("ssh-port"); port != 0 {
		cfg.Proxy.SSHPort = port
	}
	if key, _ := cmd.Flags().GetString("ssh-key"); key != "" {
		cfg.Proxy.SSHKey = key
	}

	return runProxyStartWithConfig(cfg)
}

func runProxyStartWithConfig(cfg *config.Config) error {
	ui.Header("Starting SOCKS5 Proxy")

	p := proxy.NewProxy()
	if err := p.Start(cfg); err != nil {
		return err
	}

	fmt.Println()
	ui.Box("Proxy Configuration", fmt.Sprintf(
		"Type:    SOCKS5\nAddress: %s\n\nBrowser: Configure proxy to %s\nmacOS:   System Preferences > Network > Proxies > SOCKS\ncurl:    curl --socks5 %s https://example.com",
		cfg.Proxy.Listen, cfg.Proxy.Listen, cfg.Proxy.Listen,
	))
	fmt.Println()

	return nil
}

func runProxyStop(cmd *cobra.Command, args []string) error {
	p := proxy.NewProxy()
	return p.Stop()
}

func runProxyStatus(cmd *cobra.Command, args []string) error {
	ui.Header("Proxy Status")

	running, pid, _ := proxy.ProxyStatus()
	ui.StatusLine("SOCKS5 proxy:", boolToStatus(running))
	if running {
		ui.StatusLine("PID:", fmt.Sprintf("%d", pid))
	}

	cfg, _ := config.Load()
	if cfg != nil {
		ui.StatusLine("Listen address:", cfg.Proxy.Listen)
	}

	fmt.Println()
	return nil
}
