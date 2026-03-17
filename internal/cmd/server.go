package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/danielehrhardt/tcp-over-dns/internal/config"
	"github.com/danielehrhardt/tcp-over-dns/internal/platform"
	"github.com/danielehrhardt/tcp-over-dns/internal/tunnel"
	"github.com/danielehrhardt/tcp-over-dns/internal/ui"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Server-side commands (setup, start, stop, status)",
	Long:  "Manage the DNS tunnel server (iodined) on your VPS.",
}

var serverSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Automated VPS setup — installs and configures everything",
	Long: `Runs the full server setup on your VPS:

  1. Installs iodine (iodined)
  2. Resolves port 53 conflicts (systemd-resolved)
  3. Enables IP forwarding
  4. Configures iptables NAT rules
  5. Creates a systemd service for persistence
  6. Generates a secure password
  7. Prints the client command to connect

Run this on your VPS with root privileges.`,
	RunE: runServerSetup,
}

var serverStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the iodined server",
	RunE:  runServerStart,
}

var serverStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the iodined server",
	RunE:  runServerStop,
}

var serverStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check server status",
	RunE:  runServerStatus,
}

func init() {
	serverCmd.AddCommand(serverSetupCmd)
	serverCmd.AddCommand(serverStartCmd)
	serverCmd.AddCommand(serverStopCmd)
	serverCmd.AddCommand(serverStatusCmd)

	serverSetupCmd.Flags().String("domain", "", "tunnel domain (e.g., t.example.com)")
	serverSetupCmd.Flags().String("password", "", "tunnel password (auto-generated if empty)")
	serverSetupCmd.Flags().String("tunnel-ip", "10.0.0.1", "tunnel network IP")
	serverSetupCmd.Flags().Int("tunnel-subnet", 27, "tunnel subnet mask")
	serverSetupCmd.Flags().Int("port", 53, "DNS listen port")
	serverSetupCmd.Flags().Int("mtu", 1130, "tunnel MTU")
}

func runServerSetup(cmd *cobra.Command, args []string) error {
	ui.Banner(appVersion)
	ui.Header("Server Setup")

	info := platform.Detect()

	// Check platform
	if info.OS != platform.Linux {
		ui.Warn("Server setup is designed for Linux VPS. Current OS: %s", runtime.GOOS)
		if !ui.Confirm("Continue anyway?", false) {
			return nil
		}
	}

	// Check root
	if !info.IsRoot {
		return fmt.Errorf("server setup requires root privileges — run with sudo")
	}

	totalSteps := 8

	// Step 1: Gather configuration
	ui.Step(1, totalSteps, "Gathering configuration...")

	cfg := config.DefaultConfig()

	domain, _ := cmd.Flags().GetString("domain")
	if domain == "" {
		domain = ui.Prompt("Tunnel domain (e.g., t.example.com)", "")
		if domain == "" {
			return fmt.Errorf("domain is required — set up DNS first (see: tcpdns docs)")
		}
	}
	cfg.Server.Domain = domain

	password, _ := cmd.Flags().GetString("password")
	if password == "" {
		generated, err := config.GeneratePassword()
		if err != nil {
			return fmt.Errorf("failed to generate password: %w", err)
		}
		password = generated
		ui.Success("Generated secure password: %s%s%s", ui.Bold, password, ui.Reset)
	}
	cfg.Server.Password = password
	cfg.Client.Password = password
	cfg.Client.ServerDomain = domain

	tunnelIP, _ := cmd.Flags().GetString("tunnel-ip")
	cfg.Server.TunnelIP = tunnelIP
	cfg.Proxy.SSHHost = tunnelIP

	subnet, _ := cmd.Flags().GetInt("tunnel-subnet")
	cfg.Server.TunnelSubnet = subnet

	port, _ := cmd.Flags().GetInt("port")
	cfg.Server.Port = port

	mtu, _ := cmd.Flags().GetInt("mtu")
	cfg.Server.MTU = mtu

	// Step 2: Check and resolve port 53 conflicts
	ui.Step(2, totalSteps, "Checking for port 53 conflicts...")

	inUse, process := platform.CheckPort53()
	if inUse {
		ui.Warn("Port 53 is in use: %s", process)

		if strings.Contains(process, "systemd-resolve") {
			ui.Info("Disabling systemd-resolved stub listener...")
			if err := disableSystemdResolved(); err != nil {
				ui.Warn("Could not auto-fix: %v", err)
				ui.Warn("Manually stop systemd-resolved or use a different port")
			} else {
				ui.Success("systemd-resolved stub listener disabled")
			}
		} else {
			ui.Warn("Another service is using port 53. You may need to stop it or use --port to pick another port.")
		}
	} else {
		ui.Success("Port 53 is available")
	}

	// Step 3: Install iodine
	ui.Step(3, totalSteps, "Installing iodine...")

	if platform.IodinedInstalled() {
		ui.Success("iodined is already installed")
	} else {
		installCmd, err := platform.InstallCommand(info.PackageManager)
		if err != nil {
			return fmt.Errorf("cannot install iodine: %w", err)
		}

		ui.Info("Running: %s", installCmd)
		c := exec.Command("bash", "-c", installCmd)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			return fmt.Errorf("failed to install iodine: %w", err)
		}
		ui.Success("iodine installed successfully")
	}

	// Step 4: Enable IP forwarding
	ui.Step(4, totalSteps, "Enabling IP forwarding...")

	if err := enableIPForwarding(); err != nil {
		ui.Warn("Failed to enable IP forwarding: %v", err)
	} else {
		ui.Success("IP forwarding enabled")
	}

	// Step 5: Configure iptables
	ui.Step(5, totalSteps, "Configuring firewall rules...")

	iface := platform.GetDefaultInterface()
	if err := configureIptables(iface); err != nil {
		ui.Warn("Failed to configure iptables: %v", err)
	} else {
		ui.Success("Firewall rules configured (NAT via %s)", iface)
	}

	// Step 6: Create systemd service
	ui.Step(6, totalSteps, "Creating systemd service...")

	if info.HasSystemd {
		if err := createSystemdService(cfg); err != nil {
			ui.Warn("Failed to create systemd service: %v", err)
		} else {
			ui.Success("Systemd service created and enabled")
		}
	} else {
		ui.Warn("systemd not available — you'll need to start iodined manually")
	}

	// Step 7: Save configuration
	ui.Step(7, totalSteps, "Saving configuration...")

	if err := config.Save(cfg); err != nil {
		ui.Warn("Failed to save config: %v", err)
	} else {
		path, _ := config.ConfigPath()
		ui.Success("Configuration saved to %s", path)
	}

	// Step 8: Start the server
	ui.Step(8, totalSteps, "Starting iodined server...")

	if info.HasSystemd {
		c := exec.Command("systemctl", "start", "tcpdns-server")
		if err := c.Run(); err != nil {
			ui.Warn("Failed to start via systemd: %v — starting directly", err)
			if err := tunnel.StartServer(cfg); err != nil {
				return err
			}
		} else {
			ui.Success("Server started via systemd")
		}
	} else {
		if err := tunnel.StartServer(cfg); err != nil {
			return err
		}
	}

	// Print summary
	ui.Header("Setup Complete!")

	ui.Box("Client Connection Command", fmt.Sprintf(
		"tcpdns client connect\n\nOr manually:\n  sudo iodine -f -P %s %s",
		cfg.Server.Password, cfg.Server.Domain,
	))

	fmt.Println()
	ui.Info("Make sure your DNS is configured:")
	ui.Table([][]string{
		{"A Record:", fmt.Sprintf("dns.yourdomain.com -> YOUR_VPS_IP")},
		{"NS Record:", fmt.Sprintf("%s -> dns.yourdomain.com", cfg.Server.Domain)},
	})
	fmt.Println()
	ui.Info("Test DNS delegation: dig +short NS %s", cfg.Server.Domain)
	fmt.Println()

	return nil
}

func runServerStart(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Try systemd first
	c := exec.Command("systemctl", "start", "tcpdns-server")
	if err := c.Run(); err == nil {
		ui.Success("Server started via systemd")
		return nil
	}

	return tunnel.StartServer(cfg)
}

func runServerStop(cmd *cobra.Command, args []string) error {
	// Try systemd first
	c := exec.Command("systemctl", "stop", "tcpdns-server")
	if err := c.Run(); err == nil {
		ui.Success("Server stopped via systemd")
		return nil
	}

	return tunnel.StopServer()
}

func runServerStatus(cmd *cobra.Command, args []string) error {
	ui.Header("Server Status")

	running, pid, _ := tunnel.ServerStatus()

	ui.StatusLine("iodined:", boolToStatus(running))
	if running {
		ui.StatusLine("PID:", fmt.Sprintf("%d", pid))
	}

	// Check systemd
	c := exec.Command("systemctl", "is-active", "tcpdns-server")
	output, err := c.Output()
	if err == nil {
		ui.StatusLine("Systemd service:", strings.TrimSpace(string(output)))
	}

	// Check port 53
	inUse, process := platform.CheckPort53()
	if inUse {
		ui.StatusLine("Port 53:", fmt.Sprintf("in use (%s)", truncate(process, 40)))
	} else {
		ui.StatusLine("Port 53:", "available")
	}

	// Check IP forwarding
	if checkIPForwardingEnabled() {
		ui.StatusLine("IP forwarding:", "enabled")
	} else {
		ui.StatusLine("IP forwarding:", "disabled")
	}

	fmt.Println()
	return nil
}

func disableSystemdResolved() error {
	// Create resolved.conf override to disable stub listener
	content := `[Resolve]
DNSStubListener=no
`
	dir := "/etc/systemd/resolved.conf.d"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	if err := os.WriteFile(dir+"/tcpdns.conf", []byte(content), 0644); err != nil {
		return err
	}

	// Restart systemd-resolved
	exec.Command("systemctl", "restart", "systemd-resolved").Run()

	// Also update /etc/resolv.conf to point to real DNS
	exec.Command("bash", "-c", `ln -sf /run/systemd/resolve/resolv.conf /etc/resolv.conf`).Run()

	return nil
}

func enableIPForwarding() error {
	// Enable now
	cmd := exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1")
	if err := cmd.Run(); err != nil {
		return err
	}

	// Make persistent
	content := "net.ipv4.ip_forward=1\n"
	return os.WriteFile("/etc/sysctl.d/99-tcpdns.conf", []byte(content), 0644)
}

func checkIPForwardingEnabled() bool {
	cmd := exec.Command("sysctl", "-n", "net.ipv4.ip_forward")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == "1"
}

func configureIptables(iface string) error {
	rules := [][]string{
		{"iptables", "-t", "nat", "-A", "POSTROUTING", "-o", iface, "-j", "MASQUERADE"},
		{"iptables", "-A", "FORWARD", "-i", iface, "-o", "dns0", "-m", "state", "--state", "RELATED,ESTABLISHED", "-j", "ACCEPT"},
		{"iptables", "-A", "FORWARD", "-i", "dns0", "-o", iface, "-j", "ACCEPT"},
	}

	for _, rule := range rules {
		cmd := exec.Command(rule[0], rule[1:]...)
		if err := cmd.Run(); err != nil {
			ui.Warn("iptables rule failed: %s — %v", strings.Join(rule, " "), err)
		}
	}

	// Try to persist rules
	exec.Command("bash", "-c", "iptables-save > /etc/iptables/rules.v4 2>/dev/null || iptables-save > /etc/iptables.rules 2>/dev/null").Run()

	return nil
}

func createSystemdService(cfg *config.Config) error {
	tunnelAddr := fmt.Sprintf("%s/%d", cfg.Server.TunnelIP, cfg.Server.TunnelSubnet)

	service := fmt.Sprintf(`[Unit]
Description=tcpdns DNS Tunnel Server (iodined)
After=network.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/sbin/iodined -f -c -P %s -p %d -m %d %s %s
Restart=always
RestartSec=5
LimitNOFILE=65536

# Security hardening
NoNewPrivileges=yes
PrivateTmp=yes
ProtectHome=yes
ProtectSystem=strict
ReadWritePaths=/dev/net/tun

[Install]
WantedBy=multi-user.target
`, cfg.Server.Password, cfg.Server.Port, cfg.Server.MTU, tunnelAddr, cfg.Server.Domain)

	if err := os.WriteFile("/etc/systemd/system/tcpdns-server.service", []byte(service), 0644); err != nil {
		return err
	}

	exec.Command("systemctl", "daemon-reload").Run()
	exec.Command("systemctl", "enable", "tcpdns-server").Run()

	return nil
}

func boolToStatus(b bool) string {
	if b {
		return "running"
	}
	return "stopped"
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
