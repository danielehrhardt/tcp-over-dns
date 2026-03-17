```
╔╦╗╔═╗╔═╗  ┌┬┐┌┐┌┌─┐
 ║ ║  ╠═╝   │││││└─┐
 ╩ ╚═╝╩    ─┴┘┘└┘└─┘
  TCP over DNS Tunnel
```

[![Go Version](https://img.shields.io/badge/go-1.21+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/danielehrhardt/tcp-over-dns)](https://github.com/danielehrhardt/tcp-over-dns/releases)
[![CI](https://img.shields.io/github/actions/workflow/status/danielehrhardt/tcp-over-dns/ci.yml?branch=main)](https://github.com/danielehrhardt/tcp-over-dns/actions)

**tcpdns** tunnels TCP traffic through DNS queries. It's a CLI wrapper around [iodine](https://code.kryo.se/iodine/) that automates the painful parts: installing dependencies, configuring firewalls, setting up systemd services, and resolving port conflicts.

Built for captive portals. Airplane WiFi, hotel networks, coffee shops — anywhere DNS works but everything else is blocked.

---

## Quick Start

```bash
# 1. Initialize your config
tcpdns config init

# 2. On your VPS — set up the server (requires root)
tcpdns server setup

# 3. On your laptop — connect
tcpdns client connect

# 4. Start the SOCKS5 proxy and route your traffic
tcpdns proxy start
```

Configure your browser or system proxy to `socks5://127.0.0.1:1080` and you're online.

---

## Features

- **Zero-friction setup** — one command installs iodine, configures the firewall, creates a systemd service, and generates a secure password
- **Auto-installs iodine** on all platforms: apt, dnf, yum, pacman, brew, choco
- **Resolves port 53 conflicts** with systemd-resolved automatically
- **SOCKS5 proxy** via SSH through the tunnel for full traffic routing
- **Comprehensive diagnostics** — 10 checks covering DNS, ports, tunnel state, and connectivity
- **Cross-platform** — macOS, Linux, Windows
- **Interactive config** with sensible defaults
- **Persistent server** via systemd with automatic restart on failure
- **Performance tuning** — configurable record types, encodings, and lazy mode

---

## Installation

### Binary (recommended)

Download the latest binary for your platform from [GitHub Releases](https://github.com/danielehrhardt/tcp-over-dns/releases).

```bash
# macOS / Linux — one-liner installer
curl -sSL https://raw.githubusercontent.com/danielehrhardt/tcp-over-dns/main/scripts/install.sh | bash
```

### Homebrew (macOS)

```bash
brew install danielehrhardt/tap/tcpdns
```

### Go

```bash
go install github.com/danielehrhardt/tcp-over-dns/cmd/tcpdns@latest
```

### Docker

```bash
docker pull ghcr.io/danielehrhardt/tcp-over-dns
docker run --rm --cap-add=NET_ADMIN ghcr.io/danielehrhardt/tcp-over-dns client connect
```

### VPS Setup Script

Run this on your server to install everything without the CLI:

```bash
curl -sSL https://raw.githubusercontent.com/danielehrhardt/tcp-over-dns/main/scripts/vps-setup.sh | sudo bash
```

---

## How It Works

```
Your laptop                    DNS Relay                    Your VPS
    |                              |                            |
    |-- DNS query (iodine) ------->|-- forwards to port 53 --->|
    |                              |                    iodined |
    |<-- DNS response (data) ------|<-- DNS response -----------|
    |                              |                            |
    | TUN interface (dns0)                          TUN (dns0) |
    |                                                           |
    |====== SSH tunnel over DNS ================================|
    |                                                           |
    |-- SOCKS5 proxy (127.0.0.1:1080) --> internet via VPS --->|
```

1. You set up two DNS records: an `A` record pointing to your VPS, and an `NS` record delegating a subdomain to it
2. The server runs `iodined` on port 53, creating a TUN interface
3. The client runs `iodine`, which encodes TCP data into DNS queries
4. Once the tunnel is up, SSH creates a SOCKS5 proxy through it
5. Your traffic routes through the VPS to the internet

Throughput is 5-50 KB/s depending on the DNS relay. Enough for browsing, SSH, and light API calls.

---

## Commands

```
tcpdns server setup          Automated VPS setup
tcpdns server start          Start iodined
tcpdns server stop           Stop iodined
tcpdns server status         Check server status

tcpdns client connect        Connect to the DNS tunnel
tcpdns client disconnect     Disconnect
tcpdns client status         Check connection status

tcpdns proxy start           Start SOCKS5 proxy via SSH
tcpdns proxy stop            Stop the proxy
tcpdns proxy status          Check proxy status

tcpdns config init           Interactive config setup
tcpdns config show           Print current config
tcpdns config path           Print config file path

tcpdns diagnose              Run 10 diagnostic checks
tcpdns version               Print version info
```

Global flags: `--config <path>`, `--verbose`

---

## DNS Setup

Before running `tcpdns server setup`, add two records to your domain:

| Type | Name | Value |
|------|------|-------|
| A | `dns` | `YOUR_VPS_IP` |
| NS | `t` | `dns.yourdomain.com` |

This delegates the `t.yourdomain.com` subdomain to your VPS. iodine uses it as a channel for DNS-encoded data.

Full details in [docs/dns-configuration.md](docs/dns-configuration.md).

---

## Documentation

- [Setup Guide](docs/setup-guide.md) — step-by-step walkthrough from zero to connected
- [DNS Configuration](docs/dns-configuration.md) — DNS records, registrar-specific instructions, verification
- [Troubleshooting](docs/troubleshooting.md) — common errors and how to fix them

---

## Configuration

Config lives at `~/.tcpdns/config.yml`. Run `tcpdns config init` to create it interactively.

```yaml
server:
  domain: t.yourdomain.com
  password: your-secure-password
  tunnel_ip: 10.0.0.1
  tunnel_subnet: 27
  port: 53
  mtu: 1130

client:
  server_domain: t.yourdomain.com
  password: your-secure-password

proxy:
  type: socks5
  listen: 127.0.0.1:1080
  ssh_user: root
  ssh_host: 10.0.0.1
  ssh_port: 22

advanced:
  encoding: auto
  record_type: auto
  lazy_mode: true
  raw_mode: true
```

---

## Contributing

Pull requests are welcome. For significant changes, open an issue first to discuss what you'd like to change.

```bash
git clone https://github.com/danielehrhardt/tcp-over-dns
cd tcpdns
go build ./...
go test ./...
```

---

## License

MIT. See [LICENSE](LICENSE).
