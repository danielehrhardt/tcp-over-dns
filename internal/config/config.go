package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	ConfigDir  = ".tcpdns"
	ConfigFile = "config.yml"
)

// Config holds all tcpdns configuration.
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Client   ClientConfig   `yaml:"client"`
	Proxy    ProxyConfig    `yaml:"proxy"`
	Advanced AdvancedConfig `yaml:"advanced"`
}

// ServerConfig holds server-side settings.
type ServerConfig struct {
	Domain       string `yaml:"domain"`
	Nameserver   string `yaml:"nameserver"`
	Password     string `yaml:"password"`
	TunnelIP     string `yaml:"tunnel_ip"`
	TunnelSubnet int    `yaml:"tunnel_subnet"`
	Port         int    `yaml:"port"`
	MTU          int    `yaml:"mtu"`
}

// ClientConfig holds client-side settings.
type ClientConfig struct {
	ServerDomain string `yaml:"server_domain"`
	Password     string `yaml:"password"`
	Nameserver   string `yaml:"nameserver,omitempty"`
}

// ProxyConfig holds SOCKS proxy settings.
type ProxyConfig struct {
	Type    string `yaml:"type"`
	Listen  string `yaml:"listen"`
	SSHUser string `yaml:"ssh_user"`
	SSHHost string `yaml:"ssh_host"`
	SSHPort int    `yaml:"ssh_port"`
	SSHKey  string `yaml:"ssh_key"`
}

// AdvancedConfig holds tuning parameters.
type AdvancedConfig struct {
	Encoding      string `yaml:"encoding"`
	RecordType    string `yaml:"record_type"`
	LazyMode      bool   `yaml:"lazy_mode"`
	MaxDownstream int    `yaml:"max_downstream"`
	RawMode       bool   `yaml:"raw_mode"`
}

// DefaultConfig returns a config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Domain:       "t.example.com",
			Nameserver:   "dns.example.com",
			Password:     "",
			TunnelIP:     "10.0.0.1",
			TunnelSubnet: 27,
			Port:         53,
			MTU:          1130,
		},
		Client: ClientConfig{
			ServerDomain: "t.example.com",
			Password:     "",
		},
		Proxy: ProxyConfig{
			Type:    "socks5",
			Listen:  "127.0.0.1:1080",
			SSHUser: "root",
			SSHHost: "10.0.0.1",
			SSHPort: 22,
			SSHKey:  "~/.ssh/id_rsa",
		},
		Advanced: AdvancedConfig{
			Encoding:      "auto",
			RecordType:    "auto",
			LazyMode:      true,
			MaxDownstream: 1024,
			RawMode:       true,
		},
	}
}

// ConfigPath returns the full path to the config file.
func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ConfigDir, ConfigFile), nil
}

// ConfigDirPath returns the path to the config directory.
func ConfigDirPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ConfigDir), nil
}

// Load reads the config from disk.
func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("cannot read config: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("cannot parse config: %w", err)
	}
	return cfg, nil
}

// Save writes the config to disk.
func Save(cfg *Config) error {
	dir, err := ConfigDirPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("cannot create config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("cannot serialize config: %w", err)
	}

	path := filepath.Join(dir, ConfigFile)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("cannot write config: %w", err)
	}
	return nil
}

// Exists checks if a config file exists.
func Exists() bool {
	path, err := ConfigPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// GeneratePassword creates a cryptographically secure random password.
func GeneratePassword() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("cannot generate password: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}
