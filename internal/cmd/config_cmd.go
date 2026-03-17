package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/danielehrhardt/tcp-over-dns/internal/config"
	"github.com/danielehrhardt/tcp-over-dns/internal/ui"
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration commands (init, show)",
	Long:  "Manage tcpdns configuration.",
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create or update configuration interactively",
	Long: `Walks you through setting up your tcpdns configuration.

The configuration is stored at ~/.tcpdns/config.yml and includes
server settings, client settings, proxy settings, and advanced options.`,
	RunE: runConfigInit,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display current configuration",
	RunE:  runConfigShow,
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the config file path",
	RunE:  runConfigPath,
}

func init() {
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configPathCmd)
}

func runConfigInit(cmd *cobra.Command, args []string) error {
	ui.Banner(appVersion)
	ui.Header("Configuration Setup")

	var cfg *config.Config
	if config.Exists() {
		ui.Info("Existing configuration found. Loading as defaults...")
		var err error
		cfg, err = config.Load()
		if err != nil {
			ui.Warn("Could not load existing config, using defaults: %v", err)
			cfg = config.DefaultConfig()
		}
	} else {
		cfg = config.DefaultConfig()
	}

	// Server configuration
	ui.Header("Server Configuration")
	ui.Info("These settings are for your VPS / tunnel server.")
	fmt.Println()

	cfg.Server.Domain = ui.Prompt("Tunnel domain (e.g., t.yourdomain.com)", cfg.Server.Domain)
	cfg.Server.Nameserver = ui.Prompt("Nameserver hostname (e.g., dns.yourdomain.com)", cfg.Server.Nameserver)

	if cfg.Server.Password == "" {
		if ui.Confirm("Generate a secure random password?", true) {
			pass, err := config.GeneratePassword()
			if err != nil {
				return fmt.Errorf("failed to generate password: %w", err)
			}
			cfg.Server.Password = pass
			ui.Success("Generated password: %s%s%s", ui.Bold, pass, ui.Reset)
		} else {
			cfg.Server.Password = ui.PromptSecret("Tunnel password")
		}
	} else {
		if ui.Confirm("Keep existing password?", true) {
			// keep it
		} else {
			cfg.Server.Password = ui.PromptSecret("New tunnel password")
		}
	}

	cfg.Server.TunnelIP = ui.Prompt("Tunnel IP address", cfg.Server.TunnelIP)

	// Client configuration (sync from server)
	cfg.Client.ServerDomain = cfg.Server.Domain
	cfg.Client.Password = cfg.Server.Password

	// Proxy configuration
	ui.Header("Proxy Configuration")
	ui.Info("These settings control the SOCKS5 proxy (runs after connecting).")
	fmt.Println()

	cfg.Proxy.Listen = ui.Prompt("SOCKS5 listen address", cfg.Proxy.Listen)
	cfg.Proxy.SSHUser = ui.Prompt("SSH username for tunnel server", cfg.Proxy.SSHUser)
	cfg.Proxy.SSHHost = ui.Prompt("SSH host (tunnel server IP)", cfg.Proxy.SSHHost)
	cfg.Proxy.SSHKey = ui.Prompt("SSH key path", cfg.Proxy.SSHKey)

	// Advanced options
	if ui.Confirm("Configure advanced options?", false) {
		ui.Header("Advanced Options")

		cfg.Advanced.Encoding = ui.Prompt("Downstream encoding (auto, Base32, Base64, Base128, Raw)", cfg.Advanced.Encoding)
		cfg.Advanced.RecordType = ui.Prompt("DNS record type (auto, NULL, TXT, CNAME, MX, SRV)", cfg.Advanced.RecordType)

		if !ui.Confirm("Enable lazy mode? (better performance, may not work with all DNS relays)", cfg.Advanced.LazyMode) {
			cfg.Advanced.LazyMode = false
		}
	}

	// Save
	fmt.Println()
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	path, _ := config.ConfigPath()
	ui.Success("Configuration saved to %s", path)

	// Show summary
	fmt.Println()
	ui.Header("Quick Reference")
	ui.Table([][]string{
		{"Tunnel domain:", cfg.Server.Domain},
		{"Tunnel IP:", cfg.Server.TunnelIP},
		{"Proxy:", cfg.Proxy.Listen},
		{"SSH:", fmt.Sprintf("%s@%s", cfg.Proxy.SSHUser, cfg.Proxy.SSHHost)},
	})

	fmt.Println()
	ui.Info("Next steps:")
	fmt.Println("  1. Set up DNS records (see: tcpdns docs)")
	fmt.Println("  2. Run server setup:  tcpdns server setup")
	fmt.Println("  3. Connect:           tcpdns client connect")
	fmt.Println()

	return nil
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	if !config.Exists() {
		ui.Warn("No configuration found. Run 'tcpdns config init' to create one.")
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Mask password for display
	displayCfg := *cfg
	if displayCfg.Server.Password != "" {
		displayCfg.Server.Password = displayCfg.Server.Password[:4] + "****"
	}
	if displayCfg.Client.Password != "" {
		displayCfg.Client.Password = displayCfg.Client.Password[:4] + "****"
	}

	data, err := yaml.Marshal(&displayCfg)
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	path, _ := config.ConfigPath()
	ui.Header("Configuration")
	ui.Info("File: %s", path)
	fmt.Println()
	fmt.Println(string(data))

	return nil
}

func runConfigPath(cmd *cobra.Command, args []string) error {
	path, err := config.ConfigPath()
	if err != nil {
		return err
	}
	fmt.Println(path)
	return nil
}
