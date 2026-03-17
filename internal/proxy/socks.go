package proxy

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/danielehrhardt/tcp-over-dns/internal/config"
	"github.com/danielehrhardt/tcp-over-dns/internal/ui"
)

// Proxy manages a SOCKS5 proxy connection via SSH.
type Proxy struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	cancel context.CancelFunc
	active bool
}

// NewProxy creates a new proxy manager.
func NewProxy() *Proxy {
	return &Proxy{}
}

// Start begins the SOCKS5 proxy via SSH tunnel.
func (p *Proxy) Start(cfg *config.Config) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.active {
		return fmt.Errorf("proxy is already running")
	}

	if !isSSHInstalled() {
		return fmt.Errorf("SSH client not found — please install OpenSSH")
	}

	args := buildSSHArgs(cfg)
	ui.Info("Starting SOCKS5 proxy via SSH tunnel...")
	ui.Info("Command: ssh %s", strings.Join(args, " "))

	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel

	cmd := exec.CommandContext(ctx, "ssh", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("failed to start SSH tunnel: %w", err)
	}

	p.cmd = cmd
	p.active = true

	ui.Success("SOCKS5 proxy started on %s", cfg.Proxy.Listen)
	ui.Info("Configure your applications to use SOCKS5 proxy at %s", cfg.Proxy.Listen)

	// Monitor process in background
	go func() {
		_ = cmd.Wait()
		p.mu.Lock()
		p.active = false
		p.mu.Unlock()
	}()

	return nil
}

// Stop terminates the SOCKS5 proxy.
func (p *Proxy) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.active {
		return fmt.Errorf("proxy is not running")
	}

	if p.cancel != nil {
		p.cancel()
	}

	// Also kill any SSH SOCKS proxy processes
	_ = exec.Command("pkill", "-f", "ssh -D").Run()

	p.active = false
	p.cmd = nil
	p.cancel = nil

	ui.Success("SOCKS5 proxy stopped")
	return nil
}

// IsActive returns whether the proxy is running.
func (p *Proxy) IsActive() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.active
}

// ProxyStatus checks if an SSH SOCKS proxy is currently running.
func ProxyStatus() (bool, int, error) {
	cmd := exec.Command("pgrep", "-f", "ssh -D")
	output, err := cmd.Output()
	if err != nil {
		return false, 0, nil
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) > 0 && lines[0] != "" {
		var pid int
		_, _ = fmt.Sscanf(lines[0], "%d", &pid)
		return true, pid, nil
	}
	return false, 0, nil
}

func buildSSHArgs(cfg *config.Config) []string {
	args := []string{
		"-D", cfg.Proxy.Listen,
		"-N", // No remote command
		"-C", // Enable compression
		"-q", // Quiet mode
	}

	// SSH key
	if cfg.Proxy.SSHKey != "" {
		expandedKey := expandPath(cfg.Proxy.SSHKey)
		args = append(args, "-i", expandedKey)
	}

	// Port
	if cfg.Proxy.SSHPort != 0 && cfg.Proxy.SSHPort != 22 {
		args = append(args, "-p", fmt.Sprintf("%d", cfg.Proxy.SSHPort))
	}

	// Disable host key checking for tunnel IPs (private range)
	args = append(args, "-o", "StrictHostKeyChecking=no")
	args = append(args, "-o", "UserKnownHostsFile=/dev/null")
	args = append(args, "-o", "LogLevel=ERROR")

	// Keep alive
	args = append(args, "-o", "ServerAliveInterval=30")
	args = append(args, "-o", "ServerAliveCountMax=3")

	// User@host
	target := fmt.Sprintf("%s@%s", cfg.Proxy.SSHUser, cfg.Proxy.SSHHost)
	args = append(args, target)

	return args
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return strings.Replace(path, "~", home, 1)
		}
	}
	return path
}

func isSSHInstalled() bool {
	_, err := exec.LookPath("ssh")
	return err == nil
}
