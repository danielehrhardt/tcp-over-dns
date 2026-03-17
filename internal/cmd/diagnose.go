package cmd

import (
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/danielehrhardt/tcp-over-dns/internal/config"
	"github.com/danielehrhardt/tcp-over-dns/internal/platform"
	"github.com/danielehrhardt/tcp-over-dns/internal/proxy"
	"github.com/danielehrhardt/tcp-over-dns/internal/tunnel"
	"github.com/danielehrhardt/tcp-over-dns/internal/ui"
	"github.com/spf13/cobra"
)

var diagnoseCmd = &cobra.Command{
	Use:   "diagnose",
	Short: "Run diagnostics to identify issues",
	Long: `Runs a comprehensive set of diagnostic checks:

  - Platform and privileges
  - iodine installation
  - DNS record verification
  - Port 53 availability
  - Tunnel interface status
  - Proxy connectivity
  - Internet connectivity through tunnel

Use this command when something isn't working to quickly identify the issue.`,
	RunE: runDiagnose,
}

func runDiagnose(cmd *cobra.Command, args []string) error {
	ui.Banner(appVersion)
	ui.Header("Diagnostics")

	cfg, _ := config.Load()
	passed := 0
	failed := 0
	warnings := 0

	// Check 1: Platform
	fmt.Printf("\n%s1. Platform%s\n", ui.Bold, ui.Reset)
	info := platform.Detect()
	ui.StatusLine("OS:", string(info.OS))
	ui.StatusLine("Arch:", info.Arch)
	ui.StatusLine("Root/Admin:", boolToYesNo(info.IsRoot))
	ui.StatusLine("Package Manager:", string(info.PackageManager))
	if runtime.GOOS == "linux" {
		ui.StatusLine("Systemd:", boolToYesNo(info.HasSystemd))
	}
	passed++

	// Check 2: iodine installation
	fmt.Printf("\n%s2. iodine Installation%s\n", ui.Bold, ui.Reset)
	if platform.IodineInstalled() {
		ui.StatusLine("iodine client:", "ok")
		// Get version
		verOut, err := exec.Command("iodine", "-v").CombinedOutput()
		if err == nil {
			ui.StatusLine("Version:", strings.TrimSpace(string(verOut)))
		}
		passed++
	} else {
		ui.StatusLine("iodine client:", "fail")
		ui.Info("  Install: brew install iodine (macOS) | apt install iodine (Ubuntu)")
		failed++
	}

	if platform.IodinedInstalled() {
		ui.StatusLine("iodined server:", "ok")
		passed++
	} else {
		ui.StatusLine("iodined server:", "fail")
		if runtime.GOOS == "linux" {
			ui.Info("  Install: apt install iodine (includes server)")
		} else {
			ui.Info("  Server should be installed on your VPS, not locally")
		}
		// Not a failure for client-only machines
		if runtime.GOOS == "linux" {
			failed++
		} else {
			warnings++
		}
	}

	// Check 3: Configuration
	fmt.Printf("\n%s3. Configuration%s\n", ui.Bold, ui.Reset)
	if config.Exists() {
		ui.StatusLine("Config file:", "ok")
		if cfg != nil {
			ui.StatusLine("Domain:", cfg.Server.Domain)
			ui.StatusLine("Tunnel IP:", cfg.Server.TunnelIP)
			ui.StatusLine("Proxy:", cfg.Proxy.Listen)
		}
		passed++
	} else {
		ui.StatusLine("Config file:", "fail")
		ui.Info("  Run: tcpdns config init")
		failed++
	}

	// Check 4: DNS Resolution
	fmt.Printf("\n%s4. DNS Resolution%s\n", ui.Bold, ui.Reset)
	if cfg != nil && cfg.Server.Domain != "" && cfg.Server.Domain != "t.example.com" {
		domain := cfg.Server.Domain

		// Check NS record
		nsRecords, err := net.LookupNS(domain)
		if err == nil && len(nsRecords) > 0 {
			ui.StatusLine("NS record:", "ok")
			for _, ns := range nsRecords {
				ui.StatusLine("  ->", ns.Host)
			}
			passed++
		} else {
			ui.StatusLine("NS record:", "fail")
			ui.Info("  No NS record found for %s", domain)
			ui.Info("  Add NS record: %s -> dns.yourdomain.com", domain)
			failed++
		}

		// Try to resolve something under the domain
		_, err = net.LookupHost("test." + domain)
		if err != nil {
			ui.StatusLine("DNS query test:", "pending")
			ui.Info("  Could not resolve test.%s (server may not be running)", domain)
			warnings++
		} else {
			ui.StatusLine("DNS query test:", "ok")
			passed++
		}
	} else {
		ui.StatusLine("DNS resolution:", "pending")
		ui.Info("  Configure domain first: tcpdns config init")
		warnings++
	}

	// Check 5: Port 53
	fmt.Printf("\n%s5. Port 53%s\n", ui.Bold, ui.Reset)
	inUse, process := platform.CheckPort53()
	if inUse {
		ui.StatusLine("Port 53:", "in use")
		ui.Info("  Process: %s", truncate(process, 60))
		if strings.Contains(process, "systemd-resolve") {
			ui.Info("  Fix: tcpdns server setup handles this automatically")
		}
		if runtime.GOOS == "linux" {
			warnings++
		}
	} else {
		ui.StatusLine("Port 53:", "ok")
		passed++
	}

	// Check 6: Tunnel status
	fmt.Printf("\n%s6. Tunnel Status%s\n", ui.Bold, ui.Reset)
	clientRunning, clientPid, _ := tunnel.ClientStatus()
	serverRunning, serverPid, _ := tunnel.ServerStatus()

	if clientRunning {
		ui.StatusLine("Client:", fmt.Sprintf("running (PID: %d)", clientPid))
		passed++
	} else {
		ui.StatusLine("Client:", "stopped")
	}

	if serverRunning {
		ui.StatusLine("Server:", fmt.Sprintf("running (PID: %d)", serverPid))
		passed++
	} else {
		ui.StatusLine("Server:", "stopped")
	}

	// Check TUN interface
	if checkTunInterface() {
		ui.StatusLine("TUN interface:", "ok")
		passed++
	} else {
		ui.StatusLine("TUN interface:", "down")
	}

	// Check 7: Tunnel connectivity
	fmt.Printf("\n%s7. Tunnel Connectivity%s\n", ui.Bold, ui.Reset)
	if cfg != nil && cfg.Server.TunnelIP != "" && cfg.Server.TunnelIP != "10.0.0.1" {
		if pingHost(cfg.Server.TunnelIP) {
			ui.StatusLine("Tunnel ping:", "ok")
			passed++
		} else {
			ui.StatusLine("Tunnel ping:", "fail")
			ui.Info("  Cannot reach %s through tunnel", cfg.Server.TunnelIP)
			if !clientRunning {
				ui.Info("  Tunnel client is not running: tcpdns client connect")
			}
		}
	} else {
		ui.StatusLine("Tunnel ping:", "pending")
		ui.Info("  Tunnel not configured or using default IP")
	}

	// Check 8: Proxy status
	fmt.Printf("\n%s8. Proxy Status%s\n", ui.Bold, ui.Reset)
	proxyRunning, proxyPid, _ := proxy.ProxyStatus()
	if proxyRunning {
		ui.StatusLine("SOCKS5 proxy:", fmt.Sprintf("running (PID: %d)", proxyPid))
		passed++
	} else {
		ui.StatusLine("SOCKS5 proxy:", "stopped")
	}

	// Check 9: SSH
	fmt.Printf("\n%s9. SSH%s\n", ui.Bold, ui.Reset)
	if platform.SSHInstalled() {
		ui.StatusLine("SSH client:", "ok")
		passed++
	} else {
		ui.StatusLine("SSH client:", "fail")
		ui.Info("  Install OpenSSH client")
		failed++
	}

	// Check 10: Internet connectivity
	fmt.Printf("\n%s10. Internet Connectivity%s\n", ui.Bold, ui.Reset)
	if checkInternet() {
		ui.StatusLine("Direct internet:", "ok")
		passed++
	} else {
		ui.StatusLine("Direct internet:", "fail")
		ui.Info("  No direct internet — you may be behind a captive portal")
		ui.Info("  This is the situation DNS tunneling is designed for!")
		warnings++
	}

	// Summary
	ui.Header("Summary")
	fmt.Printf("  %s%d passed%s", ui.Green, passed, ui.Reset)
	if warnings > 0 {
		fmt.Printf("  %s%d warnings%s", ui.Yellow, warnings, ui.Reset)
	}
	if failed > 0 {
		fmt.Printf("  %s%d failed%s", ui.Red, failed, ui.Reset)
	}
	fmt.Println()

	if failed > 0 {
		fmt.Println()
		ui.Info("Fix the failed checks above, then run 'tcpdns diagnose' again.")
		ui.Info("For detailed help: https://github.com/danielehrhardt/tcp-over-dns/docs/troubleshooting.md")
	} else if warnings > 0 {
		fmt.Println()
		ui.Info("Some checks have warnings — review them above.")
	} else {
		fmt.Println()
		ui.Success("All checks passed! Your setup looks good.")
	}
	fmt.Println()

	return nil
}

func pingHost(host string) bool {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("ping", "-n", "1", "-w", "3000", host)
	default:
		cmd = exec.Command("ping", "-c", "1", "-W", "3", host)
	}
	return cmd.Run() == nil
}

func checkInternet() bool {
	conn, err := net.DialTimeout("tcp", "1.1.1.1:443", 5*time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func boolToYesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
