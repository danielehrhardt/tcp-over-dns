# DNS Configuration

iodine works by encoding data into DNS queries and responses. For this to work, your VPS needs to be the authoritative nameserver for a subdomain. This document explains how to set that up.

---

## How It Works

DNS tunneling relies on a delegation chain:

1. Someone queries `abc123.t.yourdomain.com`
2. The root DNS servers know `yourdomain.com` is handled by your registrar's nameservers
3. Your registrar's nameservers see the NS record for `t.yourdomain.com` and forward the query to `dns.yourdomain.com`
4. `dns.yourdomain.com` resolves to your VPS IP (via the A record)
5. iodined on your VPS receives the query and responds with encoded data

The key insight: every DNS query for `*.t.yourdomain.com` reaches your VPS. iodine uses this as a bidirectional channel.

---

## Required DNS Records

You need exactly two records:

### A Record

Creates a hostname for your VPS that the NS record can reference.

```
Type:  A
Name:  dns
Value: YOUR_VPS_IP
TTL:   300
```

This creates `dns.yourdomain.com` pointing to your VPS.

### NS Record

Delegates the tunnel subdomain to your VPS.

```
Type:  NS
Name:  t
Value: dns.yourdomain.com
TTL:   300
```

This tells the internet that `t.yourdomain.com` (and everything under it) is handled by `dns.yourdomain.com`, which is your VPS.

**Important:** The NS record value must be a hostname, not an IP address. That's why the A record is required first.

---

## Choosing Your Subdomain

The examples use `t` as the tunnel subdomain, giving you `t.yourdomain.com`. You can use any subdomain that isn't already in use.

Common choices: `t`, `dns`, `tunnel`, `vpn`

Whatever you pick, use it consistently in both the NS record and your tcpdns config.

---

## Registrar-Specific Instructions

### Cloudflare

1. Log in to the Cloudflare dashboard
2. Select your domain
3. Go to **DNS > Records**
4. Add the A record:
   - Type: `A`
   - Name: `dns`
   - IPv4 address: `YOUR_VPS_IP`
   - Proxy status: **DNS only** (grey cloud, not orange)
   - TTL: Auto
5. Add the NS record:
   - Type: `NS`
   - Name: `t`
   - Nameserver: `dns.yourdomain.com`
   - TTL: Auto

**Critical:** The A record must be set to "DNS only" (grey cloud). If it's proxied through Cloudflare (orange cloud), iodine won't work because Cloudflare intercepts the DNS queries.

### Namecheap

1. Log in and go to **Domain List**
2. Click **Manage** next to your domain
3. Go to **Advanced DNS**
4. Add a new record:
   - Type: `A Record`
   - Host: `dns`
   - Value: `YOUR_VPS_IP`
   - TTL: 300
5. Add another record:
   - Type: `NS Record`
   - Host: `t`
   - Value: `dns.yourdomain.com`
   - TTL: 300

### Google Domains / Squarespace DNS

1. Go to your domain's DNS settings
2. Under **Custom records**, add:
   - Type: `A`, Host name: `dns`, Data: `YOUR_VPS_IP`, TTL: 300
   - Type: `NS`, Host name: `t`, Data: `dns.yourdomain.com`, TTL: 300

### Route 53 (AWS)

1. Open the Route 53 console
2. Go to **Hosted zones** and select your domain
3. Create record:
   - Record name: `dns`
   - Record type: `A`
   - Value: `YOUR_VPS_IP`
   - TTL: 300
4. Create another record:
   - Record name: `t`
   - Record type: `NS`
   - Value: `dns.yourdomain.com.` (note the trailing dot)
   - TTL: 300

### DigitalOcean

1. Go to **Networking > Domains**
2. Select your domain
3. Add an A record: hostname `dns`, will direct to `YOUR_VPS_IP`
4. Add an NS record: hostname `t`, will direct to `dns.yourdomain.com`

---

## Verifying Your DNS Setup

Wait 5-30 minutes after adding records, then verify:

### Check the A record

```bash
dig +short A dns.yourdomain.com
# Expected: YOUR_VPS_IP
```

### Check the NS delegation

```bash
dig +short NS t.yourdomain.com
# Expected: dns.yourdomain.com.
```

### Verify delegation is working end-to-end

```bash
dig @dns.yourdomain.com t.yourdomain.com NS
# Should return an answer from your VPS
```

### Test with iodine's built-in check

If iodine is installed:

```bash
iodine -T TXT t.yourdomain.com
# Should show "Server tunnel IP" if the server is running
```

### Use tcpdns diagnostics

```bash
tcpdns diagnose
```

Check 4 (DNS Resolution) will verify both the NS record and query routing.

---

## Common DNS Problems

### NS record not propagating

DNS changes can take up to 48 hours in rare cases, though 5-30 minutes is typical. Check propagation status at [dnschecker.org](https://dnschecker.org) or [whatsmydns.net](https://whatsmydns.net).

### Wrong NS record format

The NS record value must be a fully qualified domain name (FQDN), not an IP address. Some registrars add the trailing dot automatically; others require it explicitly.

Correct: `dns.yourdomain.com` or `dns.yourdomain.com.`  
Wrong: `1.2.3.4`

### Cloudflare proxying the A record

If the A record for `dns.yourdomain.com` is proxied through Cloudflare (orange cloud icon), DNS queries for `*.t.yourdomain.com` will hit Cloudflare's servers instead of your VPS. Set it to DNS only.

### Subdomain already in use

If `t.yourdomain.com` already has other records (A, CNAME, MX), the NS delegation may not work correctly. Use a different subdomain or remove the conflicting records.

### Firewall blocking port 53

Your VPS firewall or cloud provider security group must allow inbound UDP on port 53. Check:

```bash
# On the VPS
sudo ufw status
sudo iptables -L INPUT -n | grep 53

# From your laptop (requires nmap)
nmap -sU -p 53 YOUR_VPS_IP
```

---

## Using a Separate Nameserver Subdomain

If `dns.yourdomain.com` conflicts with something else, you can use any hostname for the A record. Just make sure the NS record points to whatever hostname you chose.

For example, using `ns1` instead of `dns`:

```
A:  ns1.yourdomain.com -> YOUR_VPS_IP
NS: t.yourdomain.com   -> ns1.yourdomain.com
```

Update your tcpdns config accordingly:

```yaml
server:
  nameserver: ns1.yourdomain.com
  domain: t.yourdomain.com
```

---

## Using a Different Port

If port 53 is unavailable on your VPS, you can run iodined on a different port and specify a nameserver when connecting:

```bash
# Server (port 5353)
tcpdns server setup --port 5353

# Client
tcpdns client connect --nameserver YOUR_VPS_IP:5353
```

Note: this only works if you can reach the alternate port directly. Through a captive portal, you're usually limited to standard DNS (port 53).

---

## Multiple Tunnel Subdomains

You can run multiple iodine instances on the same VPS using different subdomains. Each needs its own NS record and its own tunnel IP range.

```
NS: t1.yourdomain.com -> dns.yourdomain.com  (tunnel IP: 10.0.0.1/27)
NS: t2.yourdomain.com -> dns.yourdomain.com  (tunnel IP: 10.0.1.1/27)
```

This is useful for running separate tunnels for different users or purposes.
