package tunnel

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/tcpdns/tcpdns/internal/config"
	"github.com/tcpdns/tcpdns/internal/ui"
)

// Status represents the tunnel connection status.
type Status int

const (
	StatusDisconnected Status = iota
	StatusConnecting
	StatusConnected
	StatusError
)

func (s Status) String() string {
	switch s {
	case StatusDisconnected:
		return "disconnected"
	case StatusConnecting:
		return "connecting"
	case StatusConnected:
		return "connected"
	case StatusError:
		return "error"
	default:
		return "unknown"
	}
}

// Tunnel manages an iodine tunnel connection.
type Tunnel struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	cancel context.CancelFunc
	status Status
	err    error
}

// NewTunnel creates a new tunnel manager.
func NewTunnel() *Tunnel {
	return &Tunnel{
		status: StatusDisconnected,
	}
}

// StartServer starts the iodined server process.
func StartServer(cfg *config.Config) error {
	if !isIodinedInstalled() {
		return fmt.Errorf("iodined is not installed — run 'tcpdns server setup' first")
	}

	args := buildServerArgs(cfg)
	ui.Info("Starting iodined server...")
	ui.Info("Command: sudo iodined %s", strings.Join(args, " "))

	cmd := exec.Command("sudo", append([]string{"iodined"}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start iodined: %w", err)
	}

	// Give it a moment to initialize
	time.Sleep(2 * time.Second)

	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		return fmt.Errorf("iodined exited immediately — check logs above for errors")
	}

	ui.Success("iodined started (PID: %d)", cmd.Process.Pid)
	return nil
}

// StopServer stops the iodined server process.
func StopServer() error {
	ui.Info("Stopping iodined...")

	cmd := exec.Command("sudo", "pkill", "-f", "iodined")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop iodined (may not be running): %w", err)
	}

	ui.Success("iodined stopped")
	return nil
}

// ServerStatus checks if iodined is running and returns info.
func ServerStatus() (bool, int, error) {
	cmd := exec.Command("pgrep", "-f", "iodined")
	output, err := cmd.Output()
	if err != nil {
		return false, 0, nil
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) > 0 && lines[0] != "" {
		var pid int
		fmt.Sscanf(lines[0], "%d", &pid)
		return true, pid, nil
	}
	return false, 0, nil
}

// Connect establishes an iodine tunnel connection.
func (t *Tunnel) Connect(cfg *config.Config) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.status == StatusConnected || t.status == StatusConnecting {
		return fmt.Errorf("tunnel is already %s", t.status)
	}

	if !isIodineInstalled() {
		return fmt.Errorf("iodine client is not installed — install it first:\n  macOS:   brew install iodine\n  Linux:   sudo apt install iodine\n  Windows: choco install iodine")
	}

	args := buildClientArgs(cfg)
	ui.Info("Connecting to DNS tunnel...")
	ui.Info("Command: sudo iodine %s", strings.Join(args, " "))

	ctx, cancel := context.WithCancel(context.Background())
	t.cancel = cancel

	cmd := exec.CommandContext(ctx, "sudo", append([]string{"iodine"}, args...)...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	t.status = StatusConnecting
	t.cmd = cmd

	if err := cmd.Start(); err != nil {
		t.status = StatusError
		t.err = err
		cancel()
		return fmt.Errorf("failed to start iodine: %w", err)
	}

	// Monitor stdout/stderr for connection status
	go t.monitorOutput(stdout, stderr)

	// Wait for connection or timeout
	connected := make(chan bool, 1)
	go func() {
		for i := 0; i < 30; i++ {
			time.Sleep(time.Second)
			if t.GetStatus() == StatusConnected {
				connected <- true
				return
			}
			if t.GetStatus() == StatusError {
				connected <- false
				return
			}
		}
		connected <- false
	}()

	select {
	case ok := <-connected:
		if ok {
			ui.Success("Tunnel connected successfully!")
			return nil
		}
		return fmt.Errorf("tunnel connection failed — check the output above")
	case <-time.After(35 * time.Second):
		return fmt.Errorf("tunnel connection timed out after 35 seconds")
	}
}

// Disconnect tears down the tunnel.
func (t *Tunnel) Disconnect() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.cancel != nil {
		t.cancel()
	}

	// Also kill any running iodine client processes
	exec.Command("sudo", "pkill", "-f", "iodine -").Run()

	t.status = StatusDisconnected
	t.cmd = nil
	t.cancel = nil

	ui.Success("Tunnel disconnected")
	return nil
}

// GetStatus returns the current tunnel status.
func (t *Tunnel) GetStatus() Status {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.status
}

// ClientStatus checks if an iodine client is running.
func ClientStatus() (bool, int, error) {
	cmd := exec.Command("pgrep", "-f", "iodine -")
	output, err := cmd.Output()
	if err != nil {
		// Also try without the dash
		cmd = exec.Command("pgrep", "iodine")
		output, err = cmd.Output()
		if err != nil {
			return false, 0, nil
		}
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) > 0 && lines[0] != "" {
		var pid int
		fmt.Sscanf(lines[0], "%d", &pid)
		return true, pid, nil
	}
	return false, 0, nil
}

func (t *Tunnel) monitorOutput(stdout, stderr io.ReadCloser) {
	scanner := bufio.NewScanner(io.MultiReader(stdout, stderr))
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Printf("  %s%s%s\n", ui.Dim, line, ui.Reset)

		lower := strings.ToLower(line)
		if strings.Contains(lower, "connection setup complete") ||
			strings.Contains(lower, "detaching from terminal") ||
			strings.Contains(lower, "tunnel up") {
			t.mu.Lock()
			t.status = StatusConnected
			t.mu.Unlock()
		}

		if strings.Contains(lower, "error") ||
			strings.Contains(lower, "failed") ||
			strings.Contains(lower, "no suitable dns query type") {
			t.mu.Lock()
			t.status = StatusError
			t.err = fmt.Errorf("%s", line)
			t.mu.Unlock()
		}
	}
}

func buildServerArgs(cfg *config.Config) []string {
	args := []string{"-f", "-c"}

	if cfg.Server.Password != "" {
		args = append(args, "-P", cfg.Server.Password)
	}

	if cfg.Server.Port != 0 && cfg.Server.Port != 53 {
		args = append(args, "-p", fmt.Sprintf("%d", cfg.Server.Port))
	}

	if cfg.Server.MTU != 0 && cfg.Server.MTU != 1130 {
		args = append(args, "-m", fmt.Sprintf("%d", cfg.Server.MTU))
	}

	tunnelAddr := cfg.Server.TunnelIP
	if cfg.Server.TunnelSubnet > 0 {
		tunnelAddr = fmt.Sprintf("%s/%d", tunnelAddr, cfg.Server.TunnelSubnet)
	}
	args = append(args, tunnelAddr)

	args = append(args, cfg.Server.Domain)

	return args
}

func buildClientArgs(cfg *config.Config) []string {
	args := []string{"-f"}

	if cfg.Client.Password != "" {
		args = append(args, "-P", cfg.Client.Password)
	}

	// Advanced options
	if cfg.Advanced.RecordType != "" && cfg.Advanced.RecordType != "auto" {
		args = append(args, "-T", cfg.Advanced.RecordType)
	}

	if cfg.Advanced.Encoding != "" && cfg.Advanced.Encoding != "auto" {
		args = append(args, "-O", cfg.Advanced.Encoding)
	}

	if !cfg.Advanced.LazyMode {
		args = append(args, "-L", "0")
	}

	if !cfg.Advanced.RawMode {
		args = append(args, "-r")
	}

	if cfg.Advanced.MaxDownstream > 0 && cfg.Advanced.MaxDownstream != 1024 {
		args = append(args, "-m", fmt.Sprintf("%d", cfg.Advanced.MaxDownstream))
	}

	// Nameserver (optional)
	if cfg.Client.Nameserver != "" {
		args = append(args, cfg.Client.Nameserver)
	}

	// Domain (required, always last)
	args = append(args, cfg.Client.ServerDomain)

	return args
}

func isIodineInstalled() bool {
	_, err := exec.LookPath("iodine")
	return err == nil
}

func isIodinedInstalled() bool {
	_, err := exec.LookPath("iodined")
	return err == nil
}
