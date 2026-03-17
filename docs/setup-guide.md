# Setup Guide

This guide walks you through a complete tcpdns setup: from a fresh VPS to a working DNS tunnel with a SOCKS5 proxy.

**Time required:** 15-30 minutes  
**What you need:** A VPS with a public IP, a domain name, and a machine you want to tunnel from.

---

## Overview

The setup has three parts:

1. **DNS configuration** — point a subdomain at your VPS
2. **Server setup** — install and configure iodined on the VPS
3. **Client setup** — install tcpdns on your laptop and connect

---

## Part 1: Prerequisites

### VPS requirements

- Linux (Ubuntu 20.04+ recommended, Debian, Fedora, Arch all work)
- Root access
- Port 53 UDP open in your firewall/security group
- A public IP address

### Domain requirements

- A domain you control (any registrar works)
- Ability to add DNS records

### Client requirements

- macOS, Linux, or Windows
- tcpdns installed (see [README](../README.md#installation))

---

## Part 2: DNS Configuration

Before touching the server, set up your DNS records. iodine needs a subdomain delegated to your VPS so it can receive DNS queries.

Add these two records at your registrar or DNS provider:

| Type | Name | Value | TTL |
|------|------|-------|-----|
| A | `dns` | `YOUR_VPS_IP` | 300 |
| NS | `t` | `dns.yourdomain.com` | 300 |

Replace `yourdomain.com` with your actual domain and `YOUR_VPS_IP` with your VPS's public IP.

**What these do:**
- The `A` record creates a hostname (`dns.yourdomain.com`) that resolves to your VPS
- The `NS` record tells the internet that `t.yourdomain.com` is handled by your VPS

DNS propagation takes 5-30 minutes. Verify it's working before proceeding:

```bash
dig +short NS t.yourdomain.com
# Should return: dns.yourdomain.com.

dig +short A dns.yourdomain.com
# Should return: YOUR_VPS_IP
```

For registrar-specific instructions, see [docs/dns-configuration.md](dns-configuration.md).

---

## Part 3: Server Setup

SSH into your VPS and run the setup command. It handles everything automatically.

### Option A: Using tcpdns (recommended)

Install tcpdns on the VPS first:

```bash
curl -sSL https://raw.githubusercontent.com/tcpdns/tcpdns/main/scripts/install.sh | bash
```

Then run setup:

```bash
sudo tcpdns server setup
```

The setup wizard will ask for your tunnel domain, then handle:

1. Checking for port 53 conflicts (and fixing systemd-resolved if needed)
2. Installing iodine via your package manager
3. Enabling IP forwarding
4. Configuring iptables NAT rules
5. Creating a systemd service for persistence
6. Generating a secure random password
7. Starting the server

At the end, it prints the password and the client command to connect.

### Option B: VPS setup script (no tcpdns on server)

If you'd rather not install tcpdns on the server:

```bash
curl -sSL https://raw.githubusercontent.com/tcpdns/tcpdns/main/scripts/vps-setup.sh | sudo bash
```

This script does the same thing but runs standalone.

### Verify the server is running

```bash
sudo tcpdns server status
```

Or check directly:

```bash
systemctl status tcpdns-server
```

You should see iodined running and listening on port 53.

---

## Part 4: Client Setup

Back on your laptop.

### Initialize config

```bash
tcpdns config init
```

Enter the tunnel domain (`t.yourdomain.com`) and the password from the server setup output.

### Connect

```bash
tcpdns client connect
```

tcpdns will:
1. Check if iodine is installed (and offer to install it if not)
2. Load your config
3. Start iodine and establish the tunnel
4. Print the tunnel IP once connected

A successful connection looks like:

```
[+] iodine is installed
[*] Connecting to t.yourdomain.com
[+] Tunnel established
[+] Tunnel IP: 10.0.0.2
```

### Start the proxy

```bash
tcpdns proxy start
```

This opens an SSH connection through the tunnel and creates a SOCKS5 proxy at `127.0.0.1:1080`.

### Configure your browser

**Firefox:** Settings > Network Settings > Manual proxy > SOCKS Host: `127.0.0.1`, Port: `1080`, SOCKS v5

**Chrome/system-wide (macOS):** System Settings > Network > your interface > Proxies > SOCKS Proxy: `127.0.0.1:1080`

**curl:**
```bash
curl --socks5 127.0.0.1:1080 https://ifconfig.me
# Should return your VPS IP
```

---

## Part 5: Verify Everything Works

Run the built-in diagnostics:

```bash
tcpdns diagnose
```

This runs 10 checks and tells you exactly what's working and what isn't.

Manual verification:

```bash
# Check your public IP (should be your VPS IP)
curl --socks5 127.0.0.1:1080 https://ifconfig.me

# Ping the tunnel server
ping 10.0.0.1

# Check tunnel interface
ifconfig | grep dns0   # Linux
ifconfig | grep utun   # macOS
```

---

## Connecting or Disconnecting Later

```bash
# Connect everything in one command
tcpdns client connect --proxy

# Disconnect
tcpdns proxy stop
tcpdns client disconnect

# Check status
tcpdns client status
tcpdns proxy status
```

---

## Performance Tuning

Default settings work well in most situations. If you're getting very slow speeds, try these options when connecting:

```bash
# Force NULL record type (fastest, works on most networks)
tcpdns client connect --record-type NULL

# Force Base128 encoding (highest throughput)
tcpdns client connect --encoding Base128

# Disable lazy mode (helps with some restrictive relays)
tcpdns client connect --no-lazy

# Combine options
tcpdns client connect --record-type NULL --encoding Base128
```

Expected throughput:
- Best case (NULL + Base128 + lazy mode): 30-50 KB/s
- Typical: 10-20 KB/s
- Restrictive relay (CNAME only): 5-10 KB/s

---

## Keeping the Server Running

The setup command creates a systemd service that starts automatically on boot and restarts on failure.

```bash
# Check service status
systemctl status tcpdns-server

# View logs
journalctl -u tcpdns-server -f

# Restart
systemctl restart tcpdns-server
```

---

## Next Steps

- [DNS Configuration](dns-configuration.md) — detailed DNS setup for specific registrars
- [Troubleshooting](troubleshooting.md) — if something isn't working
