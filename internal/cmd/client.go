package cmd

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/danielehrhardt/tcp-over-dns/internal/config"
	"github.com/danielehrhardt/tcp-over-dns/internal/platform"
	"github.com/danielehrhardt/tcp-over-dns/internal/tunnel"
	"github.com/danielehrhardt/tcp-over-dns/internal/ui"
)

var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "Client-side commands (connect, disconnect, status)",
	Long:  "Manage the DNS tunnel client connection.",
}

var clientConnectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect to the DNS tunnel",
	Long: `Establishes a DNS tunnel connection to your server.

Before connecting, make sure:
  1. Your server is running (tcpdns server start)
  2. DNS records are properly configured
  3. iodine is installed on this machine

The connection requires root/admin privileges for TUN device creation.`,
	RunE: runClientConnect,
}

var clientDisconnectCmd = &cobra.Command{
	Use:   "disconnect",
	Short: "Disconnect from the DNS tunnel",
	RunE:  runClientDisconnect,
}

var clientStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check client connection status",
	RunE:  runClientStatus,
}

func init() {
	clientCmd.AddCommand(clientConnectCmd)
	clientCmd.AddCommand(clientDisconnectCmd)
	clientCmd.AddCommand(clientStatusCmd)

	clientConnectCmd.Flags().String("domain", "", "server tunnel domain")
	clientConnectCmd.Flags().String("password", "", "tunnel password")
	clientConnectCmd.Flags().String("nameserver", "", "DNS server to use (optional)")
	clientConnectCmd.Flags().String("record-type", "auto", "DNS record type (auto, NULL, TXT, CNAME, MX, SRV)")
	clientConnectCmd.Flags().String("encoding", "auto", "downstream encoding (auto, Base32, Base64, Base128, Raw)")
	clientConnectCmd.Flags().Bool("no-lazy", false, "disable lazy mode")
	clientConnectCmd.Flags().Bool("no-raw", false, "disable raw UDP mode")
	clientConnectCmd.Flags().Bool("proxy", false, "also start SOCKS5 proxy after connecting")
}

func runClientConnect(cmd *cobra.Command, args []string) error {
	ui.Banner(appVersion)
	ui.Header("Connecting to DNS Tunnel")

	// Check iodine installation
	if !platform.IodineInstalled() {
		info := platform.Detect()
		installCmd, err := platform.InstallCommand(info.PackageManager)
		if err != nil {
			return fmt.Errorf("iodine is not installed and no package manager found.\n\nInstall manually:\n  macOS:   brew install iodine\n  Ubuntu:  sudo apt install iodine\n  Fedora:  sudo dnf install iodine\n  Arch:    sudo pacman -S iodine\n  Windows: choco install iodine")
		}

		ui.Warn("iodine is not installed")
		if ui.Confirm(fmt.Sprintf("Install iodine via %s?", info.PackageManager), true) {
			ui.Info("Running: %s", installCmd)
			if err := runShellCommand(installCmd); err != nil {
				return fmt.Errorf("failed to install iodine: %w", err)
			}
			ui.Success("iodine installed successfully")
		} else {
			return fmt.Errorf("iodine is required — install it first")
		}
	} else {
		ui.Success("iodine is installed")
	}

	// Check root
	info := platform.Detect()
	if !info.IsRoot {
		ui.Warn("DNS tunnel requires root/admin privileges for TUN device creation")
		ui.Info("The connection command will use sudo automatically")
	}

	// Load config and apply flag overrides
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if d, _ := cmd.Flags().GetString("domain"); d != "" {
		cfg.Client.ServerDomain = d
	}
	if p, _ := cmd.Flags().GetString("password"); p != "" {
		cfg.Client.Password = p
	}
	if ns, _ := cmd.Flags().GetString("nameserver"); ns != "" {
		cfg.Client.Nameserver = ns
	}
	if rt, _ := cmd.Flags().GetString("record-type"); rt != "auto" {
		cfg.Advanced.RecordType = rt
	}
	if enc, _ := cmd.Flags().GetString("encoding"); enc != "auto" {
		cfg.Advanced.Encoding = enc
	}
	if noLazy, _ := cmd.Flags().GetBool("no-lazy"); noLazy {
		cfg.Advanced.LazyMode = false
	}
	if noRaw, _ := cmd.Flags().GetBool("no-raw"); noRaw {
		cfg.Advanced.RawMode = false
	}

	// Validate config
	if cfg.Client.ServerDomain == "" || cfg.Client.ServerDomain == "t.example.com" {
		cfg.Client.ServerDomain = ui.Prompt("Server tunnel domain", "")
		if cfg.Client.ServerDomain == "" {
			return fmt.Errorf("server domain is required")
		}
	}
	if cfg.Client.Password == "" {
		cfg.Client.Password = ui.PromptSecret("Tunnel password")
		if cfg.Client.Password == "" {
			return fmt.Errorf("password is required")
		}
	}

	// Show connection info
	ui.Info("Connecting to %s%s%s", ui.Bold, cfg.Client.ServerDomain, ui.Reset)
	fmt.Println()

	// Connect
	t := tunnel.NewTunnel()
	if err := t.Connect(cfg); err != nil {
		ui.Error("Connection failed: %v", err)
		fmt.Println()
		ui.Info("Troubleshooting tips:")
		fmt.Println("  1. Verify DNS delegation: dig +short NS " + cfg.Client.ServerDomain)
		fmt.Println("  2. Check server is running: tcpdns server status")
		fmt.Println("  3. Try different record type: --record-type TXT")
		fmt.Println("  4. Try disabling lazy mode: --no-lazy")
		fmt.Println("  5. Run diagnostics: tcpdns diagnose")
		fmt.Println()
		return err
	}

	// Optionally start proxy
	startProxy, _ := cmd.Flags().GetBool("proxy")
	if startProxy {
		fmt.Println()
		return runProxyStartWithConfig(cfg)
	}

	fmt.Println()
	ui.Info("Tunnel is up! To route traffic through it:")
	ui.Info("  tcpdns proxy start    # Start SOCKS5 proxy")
	ui.Info("  ssh -D 1080 %s@%s    # Or manually via SSH", cfg.Proxy.SSHUser, cfg.Proxy.SSHHost)
	fmt.Println()

	return nil
}

func runClientDisconnect(cmd *cobra.Command, args []string) error {
	t := tunnel.NewTunnel()
	return t.Disconnect()
}

func runClientStatus(cmd *cobra.Command, args []string) error {
	ui.Header("Client Status")

	running, pid, _ := tunnel.ClientStatus()
	ui.StatusLine("iodine client:", boolToStatus(running))
	if running {
		ui.StatusLine("PID:", fmt.Sprintf("%d", pid))
	}

	// Check TUN interface
	if checkTunInterface() {
		ui.StatusLine("TUN interface:", "up")
	} else {
		ui.StatusLine("TUN interface:", "down")
	}

	fmt.Println()
	return nil
}

func checkTunInterface() bool {
	// Check for dns0 or utun interfaces
	out, err := runShellCommandOutput("ifconfig 2>/dev/null | grep -E 'dns0|utun' | head -1")
	if err != nil {
		return false
	}
	return len(out) > 0
}

func runShellCommand(command string) error {
	cmd := newShellCommand(command)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

func runShellCommandOutput(command string) (string, error) {
	cmd := newShellCommand(command)
	output, err := cmd.Output()
	return string(output), err
}

func newShellCommand(command string) *exec.Cmd {
	return exec.Command("bash", "-c", command)
}
