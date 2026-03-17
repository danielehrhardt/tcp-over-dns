# Troubleshooting

Start with diagnostics. They catch most problems automatically:

```bash
tcpdns diagnose
```

If that doesn't point you to the issue, find your error message below.

---

## Connection Errors

### "No downstream data received in 60 seconds"

iodine connected but isn't receiving data back from the server. Three common causes:

**1. DNS delegation isn't working**

The NS record isn't pointing to your VPS, so DNS queries aren't reaching iodined.

```bash
# Verify the NS record
dig +short NS t.yourdomain.com
# Should return: dns.yourdomain.com.

# Verify the A record
dig +short A dns.yourdomain.com
# Should return: YOUR_VPS_IP
```

If either of these is wrong or empty, fix your DNS records and wait for propagation. See [dns-configuration.md](dns-configuration.md).

**2. Firewall blocking port 53 UDP**

Your VPS firewall or cloud provider security group is dropping UDP traffic on port 53.

```bash
# On the VPS, check if iodined is listening
sudo ss -ulnp | grep 53

# Check firewall rules
sudo ufw status
sudo iptables -L INPUT -n | grep 53

# From your laptop (requires nmap)
nmap -sU -p 53 YOUR_VPS_IP
```

Open port 53 UDP in your firewall:

```bash
# UFW
sudo ufw allow 53/udp

# iptables
sudo iptables -A INPUT -p udp --dport 53 -j ACCEPT
```

**3. systemd-resolved is intercepting queries**

On Ubuntu/Debian servers, systemd-resolved listens on port 53 and intercepts DNS queries before iodined can see them.

```bash
# Check if systemd-resolved is on port 53
sudo ss -ulnp | grep 53
# If you see systemd-resolve, that's the problem

# Fix it (tcpdns server setup does this automatically)
sudo mkdir -p /etc/systemd/resolved.conf.d
echo -e '[Resolve]\nDNSStubListener=no' | sudo tee /etc/systemd/resolved.conf.d/tcpdns.conf
sudo systemctl restart systemd-resolved
sudo ln -sf /run/systemd/resolve/resolv.conf /etc/resolv.conf

# Restart iodined
sudo systemctl restart tcpdns-server
```

---

### "Connection timed out" or iodine exits immediately

**DNS records not set up or not propagated yet**

```bash
dig +short NS t.yourdomain.com
```

If this returns nothing, your NS record isn't in place. Add it and wait 5-30 minutes.

**Server not running**

```bash
# On the VPS
sudo tcpdns server status
systemctl status tcpdns-server
```

If it's stopped, start it:

```bash
sudo tcpdns server start
# or
sudo systemctl start tcpdns-server
```

**Wrong domain in config**

```bash
tcpdns config show
```

Make sure `client.server_domain` matches the subdomain you delegated (e.g., `t.yourdomain.com`, not `yourdomain.com`).

---

### "VNAK" error

Version mismatch between iodine client and iodined server. They must be the same version.

```bash
# Check client version
iodine -v

# Check server version (on VPS)
iodined -v
```

Update both to the same version. On Ubuntu:

```bash
sudo apt update && sudo apt install iodine
```

---

### "VFUL" error

The server has reached its maximum number of connected clients. iodine defaults to a small limit.

On the VPS, restart iodined. If you need more concurrent connections, you'll need to run multiple iodined instances on different subdomains.

---

### "BADIP" error

The server is rejecting your connection because your IP address changed mid-session. This happens when you're behind NAT and your external IP shifts.

Add the `-c` flag to disable IP checking on the server side. The `tcpdns server setup` command includes `-c` by default in the systemd service.

If you set up iodined manually, add `-c` to the command:

```bash
iodined -f -c -P yourpassword 10.0.0.1/27 t.yourdomain.com
```

---

## Speed Problems

### Very slow speeds (under 5 KB/s)

The DNS relay between you and the server is restrictive. Try different record types and encodings:

```bash
# Try each record type, from fastest to most compatible
tcpdns client connect --record-type NULL
tcpdns client connect --record-type TXT
tcpdns client connect --record-type MX
tcpdns client connect --record-type CNAME

# Try different encodings
tcpdns client connect --encoding Base128
tcpdns client connect --encoding Base64
tcpdns client connect --encoding Base32

# Disable lazy mode (some relays don't handle it well)
tcpdns client connect --no-lazy

# Disable raw UDP mode
tcpdns client connect --no-raw
```

Best combination for most networks: `--record-type NULL --encoding Base128`

For very restrictive relays (CNAME only): `--record-type CNAME --encoding Base32 --no-lazy`

### SERVFAIL errors in iodine output

These are normal. Impatient DNS relays return SERVFAIL when they don't get a response fast enough. iodine handles them automatically and retries. You'll see them in verbose output but they don't indicate a real problem unless the connection fails entirely.

---

## Platform-Specific Issues

### macOS: TUN interface not created

macOS uses `utun` interfaces instead of `dns0`. iodine should handle this automatically, but if it doesn't:

```bash
# Check for utun interfaces
ifconfig | grep utun

# If iodine fails to create a TUN device, try running with sudo
sudo iodine -f -P yourpassword t.yourdomain.com
```

On newer macOS versions (Ventura+), you may need to approve the network extension in System Settings > Privacy & Security.

### macOS: "Operation not permitted" when creating TUN

This is a permissions issue. iodine needs root to create TUN devices:

```bash
sudo tcpdns client connect
```

### Windows: TAP driver issues

iodine on Windows requires the TAP-Windows driver (from OpenVPN).

1. Download and install [OpenVPN](https://openvpn.net/community-downloads/) — this installs the TAP driver
2. Or install the TAP driver standalone from the OpenVPN GitHub releases
3. After installing, retry the connection

If you see "No TAP-Windows adapters found":

```
# In an elevated PowerShell
netsh interface show interface
# Look for a TAP adapter in the list
```

If no TAP adapter appears, reinstall the TAP driver.

### Windows: iodine not found

Install iodine via Chocolatey:

```powershell
choco install iodine
```

Or download the Windows binary from the [iodine releases page](https://code.kryo.se/iodine/).

---

## Server-Side Issues

### Cannot reach the internet through the tunnel

The tunnel is up (you can ping `10.0.0.1`) but you can't reach the internet through it. This means IP forwarding or NAT isn't configured correctly on the server.

**Check IP forwarding:**

```bash
# On the VPS
sysctl net.ipv4.ip_forward
# Should return: net.ipv4.ip_forward = 1

# Enable it if not set
sudo sysctl -w net.ipv4.ip_forward=1
echo 'net.ipv4.ip_forward=1' | sudo tee /etc/sysctl.d/99-tcpdns.conf
```

**Check iptables NAT rules:**

```bash
sudo iptables -t nat -L POSTROUTING -n -v
# Should show a MASQUERADE rule for your main interface
```

If the MASQUERADE rule is missing, add it (replace `eth0` with your actual interface):

```bash
# Find your main interface
ip route | grep default
# Example output: default via 1.2.3.1 dev eth0

# Add the NAT rule
sudo iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
sudo iptables -A FORWARD -i eth0 -o dns0 -m state --state RELATED,ESTABLISHED -j ACCEPT
sudo iptables -A FORWARD -i dns0 -o eth0 -j ACCEPT
```

`tcpdns server setup` configures all of this automatically. If you set up manually, these rules are easy to miss.

**Persist iptables rules across reboots:**

```bash
# Debian/Ubuntu
sudo apt install iptables-persistent
sudo netfilter-persistent save

# RHEL/Fedora
sudo service iptables save
```

### Port 53 in use after setup

If something else grabbed port 53 after setup:

```bash
# Find what's using port 53
sudo ss -ulnp | grep :53
sudo lsof -i UDP:53

# Common culprits:
# systemd-resolved — fix with the resolved.conf override (see above)
# dnsmasq — sudo systemctl stop dnsmasq && sudo systemctl disable dnsmasq
# bind9 — sudo systemctl stop bind9
```

---

## Password Issues

### Authentication fails

**Wrong password**

The password must match exactly between server and client. Check both:

```bash
# Client config
tcpdns config show | grep password

# Server config (on VPS)
cat ~/.tcpdns/config.yml | grep password
# or check the systemd service
sudo systemctl cat tcpdns-server | grep -P
```

**Password too long**

iodine has a 32-character password limit. If your password is longer, it gets silently truncated on one side but not the other, causing a mismatch.

```bash
# Check password length
echo -n "yourpassword" | wc -c
```

Keep passwords at 32 characters or fewer.

**Using IODINE_PASS environment variable**

You can set the password via environment variable instead of the `-P` flag:

```bash
export IODINE_PASS=yourpassword
iodine -f t.yourdomain.com
```

This is useful if you don't want the password visible in process listings.

---

## Diagnostic Commands

Quick reference for manual debugging:

```bash
# Check DNS delegation
dig +short NS t.yourdomain.com
dig +short A dns.yourdomain.com

# Check if iodined is running on the VPS
sudo ss -ulnp | grep 53
systemctl status tcpdns-server

# Check tunnel interface (client)
ifconfig dns0        # Linux
ifconfig utun0       # macOS

# Ping through the tunnel
ping 10.0.0.1

# Check IP forwarding (server)
sysctl net.ipv4.ip_forward

# Check NAT rules (server)
sudo iptables -t nat -L -n -v

# View iodined logs
journalctl -u tcpdns-server -f

# Run iodine in verbose mode
sudo iodine -f -v -P yourpassword t.yourdomain.com
```

---

## Still Stuck?

Run `tcpdns diagnose` and share the output when asking for help. It captures the most relevant state in one shot.

Open an issue at [github.com/tcpdns/tcpdns/issues](https://github.com/tcpdns/tcpdns/issues) with:
- Your OS and version
- Output of `tcpdns diagnose`
- The exact error message from iodine (run with `--verbose` to get more detail)
- Your DNS record setup (you can redact the actual domain)
