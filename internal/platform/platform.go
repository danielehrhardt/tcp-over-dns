package platform

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// OS represents the detected operating system.
type OS string

const (
	Linux   OS = "linux"
	MacOS   OS = "darwin"
	Windows OS = "windows"
	Unknown OS = "unknown"
)

// PackageManager represents the system package manager.
type PackageManager string

const (
	APT      PackageManager = "apt"
	YUM      PackageManager = "yum"
	DNF      PackageManager = "dnf"
	Pacman   PackageManager = "pacman"
	Brew     PackageManager = "brew"
	Choco    PackageManager = "choco"
	Scoop    PackageManager = "scoop"
	NoPkgMgr PackageManager = "none"
)

// Info holds platform information.
type Info struct {
	OS             OS
	Arch           string
	PackageManager PackageManager
	IsRoot         bool
	HasSystemd     bool
	InitSystem     string
}

// Detect gathers information about the current platform.
func Detect() *Info {
	info := &Info{
		OS:   OS(runtime.GOOS),
		Arch: runtime.GOARCH,
	}

	info.PackageManager = detectPackageManager(info.OS)
	info.IsRoot = checkRoot()
	info.HasSystemd = checkSystemd()

	if info.HasSystemd {
		info.InitSystem = "systemd"
	} else {
		info.InitSystem = "other"
	}

	return info
}

func detectPackageManager(os OS) PackageManager {
	switch os {
	case Linux:
		managers := []struct {
			cmd string
			pm  PackageManager
		}{
			{"apt-get", APT},
			{"dnf", DNF},
			{"yum", YUM},
			{"pacman", Pacman},
		}
		for _, m := range managers {
			if _, err := exec.LookPath(m.cmd); err == nil {
				return m.pm
			}
		}
	case MacOS:
		if _, err := exec.LookPath("brew"); err == nil {
			return Brew
		}
	case Windows:
		if _, err := exec.LookPath("choco"); err == nil {
			return Choco
		}
		if _, err := exec.LookPath("scoop"); err == nil {
			return Scoop
		}
	}
	return NoPkgMgr
}

func checkRoot() bool {
	if runtime.GOOS == "windows" {
		// On Windows, check if running as Administrator
		cmd := exec.Command("net", "session")
		if err := cmd.Run(); err != nil {
			return false
		}
		return true
	}
	// Unix: check effective UID
	cmd := exec.Command("id", "-u")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == "0"
}

func checkSystemd() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	_, err := exec.LookPath("systemctl")
	return err == nil
}

// IodineInstalled checks if iodine is available on the system.
func IodineInstalled() bool {
	_, err := exec.LookPath("iodine")
	return err == nil
}

// IodinedInstalled checks if iodined (server) is available.
func IodinedInstalled() bool {
	_, err := exec.LookPath("iodined")
	return err == nil
}

// InstallCommand returns the command to install iodine for the current platform.
func InstallCommand(pm PackageManager) (string, error) {
	switch pm {
	case APT:
		return "apt-get update && apt-get install -y iodine", nil
	case DNF:
		return "dnf install -y iodine", nil
	case YUM:
		return "yum install -y iodine", nil
	case Pacman:
		return "pacman -Sy --noconfirm iodine", nil
	case Brew:
		return "brew install iodine", nil
	case Choco:
		return "choco install iodine -y", nil
	case Scoop:
		return "scoop install iodine", nil
	default:
		return "", fmt.Errorf("no known package manager found — please install iodine manually: https://github.com/yarrick/iodine")
	}
}

// SSHInstalled checks if ssh client is available.
func SSHInstalled() bool {
	_, err := exec.LookPath("ssh")
	return err == nil
}

// GetDefaultInterface returns the default network interface name.
func GetDefaultInterface() string {
	switch runtime.GOOS {
	case "linux":
		cmd := exec.Command("ip", "route", "show", "default")
		output, err := cmd.Output()
		if err != nil {
			return "eth0"
		}
		fields := strings.Fields(string(output))
		for i, field := range fields {
			if field == "dev" && i+1 < len(fields) {
				return fields[i+1]
			}
		}
		return "eth0"
	case "darwin":
		cmd := exec.Command("route", "-n", "get", "default")
		output, err := cmd.Output()
		if err != nil {
			return "en0"
		}
		for _, line := range strings.Split(string(output), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "interface:") {
				return strings.TrimSpace(strings.TrimPrefix(line, "interface:"))
			}
		}
		return "en0"
	default:
		return "eth0"
	}
}

// CheckPort53 checks if port 53 is in use and returns the process using it.
func CheckPort53() (bool, string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("ss", "-tulpn")
	case "darwin":
		cmd = exec.Command("lsof", "-i", ":53", "-P", "-n")
	default:
		return false, ""
	}

	output, err := cmd.Output()
	if err != nil {
		return false, ""
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, ":53 ") || strings.Contains(line, ":53\t") {
			return true, strings.TrimSpace(line)
		}
	}
	return false, ""
}
