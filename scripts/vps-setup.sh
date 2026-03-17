#!/usr/bin/env bash
#
# tcpdns VPS Setup Script
# Automated server setup for TCP-over-DNS tunneling
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/danielehrhardt/tcp-over-dns/main/scripts/vps-setup.sh | sudo bash
#
#   Or with options:
#   sudo bash vps-setup.sh --domain t.example.com --password mypassword
#
# Supports: Ubuntu 20.04+, Debian 11+, CentOS/RHEL 8+, Fedora 36+, Arch Linux
#

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
DIM='\033[2m'
NC='\033[0m'

# Defaults
TUNNEL_DOMAIN=""
TUNNEL_PASSWORD=""
TUNNEL_IP="10.0.0.1"
TUNNEL_SUBNET="27"
LISTEN_PORT="53"
MTU="1130"
SKIP_CONFIRM=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --domain)    TUNNEL_DOMAIN="$2"; shift 2 ;;
        --password)  TUNNEL_PASSWORD="$2"; shift 2 ;;
        --ip)        TUNNEL_IP="$2"; shift 2 ;;
        --subnet)    TUNNEL_SUBNET="$2"; shift 2 ;;
        --port)      LISTEN_PORT="$2"; shift 2 ;;
        --mtu)       MTU="$2"; shift 2 ;;
        -y|--yes)    SKIP_CONFIRM=true; shift ;;
        -h|--help)   usage; exit 0 ;;
        *)           echo "Unknown option: $1"; exit 1 ;;
    esac
done

banner() {
    echo -e "${CYAN}"
    echo '╔╦╗╔═╗╔═╗  ┌┬┐┌┐┌┌─┐'
    echo ' ║ ║  ╠═╝   │││││└─┐'
    echo ' ╩ ╚═╝╩    ─┴┘┘└┘└─┘'
    echo -e "${NC}"
    echo -e "${DIM}  TCP over DNS — VPS Setup${NC}"
    echo ""
}

info()    { echo -e "${BLUE}[*]${NC} $*"; }
success() { echo -e "${GREEN}[+]${NC} $*"; }
warn()    { echo -e "${YELLOW}[!]${NC} $*"; }
error()   { echo -e "${RED}[-]${NC} $*"; }
step()    { echo -e "${CYAN}[$1/$2]${NC} $3"; }

usage() {
    echo "Usage: sudo bash vps-setup.sh [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --domain DOMAIN     Tunnel domain (e.g., t.example.com) [required]"
    echo "  --password PASS     Tunnel password (auto-generated if omitted)"
    echo "  --ip IP             Tunnel IP address (default: 10.0.0.1)"
    echo "  --subnet MASK       Tunnel subnet mask (default: 27)"
    echo "  --port PORT         DNS listen port (default: 53)"
    echo "  --mtu MTU           Tunnel MTU (default: 1130)"
    echo "  -y, --yes           Skip confirmation prompts"
    echo "  -h, --help          Show this help"
}

check_root() {
    if [[ $EUID -ne 0 ]]; then
        error "This script must be run as root (use sudo)"
        exit 1
    fi
}

detect_os() {
    if [[ -f /etc/os-release ]]; then
        . /etc/os-release
        OS_ID="${ID}"
        OS_VERSION="${VERSION_ID:-}"
        OS_NAME="${PRETTY_NAME:-${ID}}"
    elif [[ -f /etc/redhat-release ]]; then
        OS_ID="rhel"
        OS_NAME="$(cat /etc/redhat-release)"
    else
        OS_ID="unknown"
        OS_NAME="Unknown"
    fi
    info "Detected OS: ${OS_NAME}"
}

detect_interface() {
    IFACE=$(ip route show default 2>/dev/null | awk '/default/ {print $5}' | head -1)
    if [[ -z "$IFACE" ]]; then
        IFACE="eth0"
        warn "Could not detect default interface, using ${IFACE}"
    else
        info "Default network interface: ${IFACE}"
    fi
}

generate_password() {
    TUNNEL_PASSWORD=$(openssl rand -hex 16 2>/dev/null || head -c 32 /dev/urandom | xxd -p | head -c 32)
    success "Generated password: ${BOLD}${TUNNEL_PASSWORD}${NC}"
}

prompt_config() {
    if [[ -z "$TUNNEL_DOMAIN" ]]; then
        echo -en "${BOLD}Tunnel domain (e.g., t.example.com): ${NC}"
        read -r TUNNEL_DOMAIN
        if [[ -z "$TUNNEL_DOMAIN" ]]; then
            error "Domain is required. Set up DNS first:"
            echo "  1. A record:  dns.yourdomain.com -> YOUR_VPS_IP"
            echo "  2. NS record: t.yourdomain.com -> dns.yourdomain.com"
            exit 1
        fi
    fi

    if [[ -z "$TUNNEL_PASSWORD" ]]; then
        generate_password
    fi

    echo ""
    info "Configuration:"
    echo -e "  ${DIM}Domain:${NC}   ${TUNNEL_DOMAIN}"
    echo -e "  ${DIM}Password:${NC} ${TUNNEL_PASSWORD}"
    echo -e "  ${DIM}Tunnel IP:${NC} ${TUNNEL_IP}/${TUNNEL_SUBNET}"
    echo -e "  ${DIM}Port:${NC}     ${LISTEN_PORT}"
    echo -e "  ${DIM}MTU:${NC}      ${MTU}"
    echo ""

    if [[ "$SKIP_CONFIRM" != true ]]; then
        echo -en "${BOLD}Proceed with setup? [Y/n]: ${NC}"
        read -r confirm
        if [[ "$confirm" =~ ^[Nn] ]]; then
            info "Aborted."
            exit 0
        fi
    fi
}

install_iodine() {
    if command -v iodined &>/dev/null; then
        success "iodine is already installed"
        return
    fi

    case "$OS_ID" in
        ubuntu|debian|linuxmint|pop)
            info "Installing iodine via apt..."
            apt-get update -qq
            apt-get install -y -qq iodine
            ;;
        fedora)
            info "Installing iodine via dnf..."
            dnf install -y -q iodine
            ;;
        centos|rhel|rocky|alma)
            info "Installing iodine via yum..."
            yum install -y -q epel-release 2>/dev/null || true
            yum install -y -q iodine
            ;;
        arch|manjaro)
            info "Installing iodine via pacman..."
            pacman -Sy --noconfirm iodine
            ;;
        *)
            error "Unsupported OS: ${OS_ID}"
            error "Please install iodine manually: https://github.com/yarrick/iodine"
            exit 1
            ;;
    esac

    if command -v iodined &>/dev/null; then
        success "iodine installed successfully"
    else
        error "iodine installation failed"
        exit 1
    fi
}

fix_port53() {
    if ss -tulpn 2>/dev/null | grep -q ':53 '; then
        warn "Port 53 is in use"

        if systemctl is-active systemd-resolved &>/dev/null; then
            info "Disabling systemd-resolved stub listener..."
            mkdir -p /etc/systemd/resolved.conf.d
            cat > /etc/systemd/resolved.conf.d/tcpdns.conf <<EOF
[Resolve]
DNSStubListener=no
EOF
            systemctl restart systemd-resolved
            ln -sf /run/systemd/resolve/resolv.conf /etc/resolv.conf 2>/dev/null || true
            sleep 1

            if ss -tulpn 2>/dev/null | grep -q ':53 '; then
                warn "Port 53 still in use after disabling systemd-resolved"
                warn "You may need to stop the offending service manually"
            else
                success "Port 53 is now free"
            fi
        else
            warn "Another service is using port 53. Consider:"
            warn "  - Stopping the service using port 53"
            warn "  - Using --port to run on a different port (requires iptables forwarding)"
        fi
    else
        success "Port 53 is available"
    fi
}

setup_forwarding() {
    info "Enabling IP forwarding..."

    sysctl -w net.ipv4.ip_forward=1 >/dev/null

    # Make persistent
    if [[ ! -f /etc/sysctl.d/99-tcpdns.conf ]] || ! grep -q "ip_forward" /etc/sysctl.d/99-tcpdns.conf 2>/dev/null; then
        echo "net.ipv4.ip_forward=1" > /etc/sysctl.d/99-tcpdns.conf
    fi

    success "IP forwarding enabled"
}

setup_firewall() {
    info "Configuring iptables NAT rules..."

    # NAT masquerade
    iptables -t nat -C POSTROUTING -o "$IFACE" -j MASQUERADE 2>/dev/null ||
        iptables -t nat -A POSTROUTING -o "$IFACE" -j MASQUERADE

    # Forward rules
    iptables -C FORWARD -i "$IFACE" -o dns0 -m state --state RELATED,ESTABLISHED -j ACCEPT 2>/dev/null ||
        iptables -A FORWARD -i "$IFACE" -o dns0 -m state --state RELATED,ESTABLISHED -j ACCEPT

    iptables -C FORWARD -i dns0 -o "$IFACE" -j ACCEPT 2>/dev/null ||
        iptables -A FORWARD -i dns0 -o "$IFACE" -j ACCEPT

    # Allow DNS traffic
    iptables -C INPUT -p udp --dport "$LISTEN_PORT" -j ACCEPT 2>/dev/null ||
        iptables -A INPUT -p udp --dport "$LISTEN_PORT" -j ACCEPT

    # Persist rules
    if command -v netfilter-persistent &>/dev/null; then
        netfilter-persistent save 2>/dev/null || true
    elif command -v iptables-save &>/dev/null; then
        mkdir -p /etc/iptables
        iptables-save > /etc/iptables/rules.v4 2>/dev/null || true
    fi

    success "Firewall rules configured"
}

create_systemd_service() {
    if ! command -v systemctl &>/dev/null; then
        warn "systemd not found — skipping service creation"
        warn "You'll need to start iodined manually"
        return
    fi

    info "Creating systemd service..."

    cat > /etc/systemd/system/tcpdns-server.service <<EOF
[Unit]
Description=tcpdns DNS Tunnel Server (iodined)
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=$(command -v iodined) -f -c -P ${TUNNEL_PASSWORD} -p ${LISTEN_PORT} -m ${MTU} ${TUNNEL_IP}/${TUNNEL_SUBNET} ${TUNNEL_DOMAIN}
Restart=always
RestartSec=5
LimitNOFILE=65536

# Security hardening
NoNewPrivileges=yes
PrivateTmp=yes
ProtectHome=yes
ProtectSystem=strict
ReadWritePaths=/dev/net/tun

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable tcpdns-server
    success "Systemd service created and enabled"
}

start_server() {
    if command -v systemctl &>/dev/null; then
        info "Starting iodined via systemd..."
        systemctl start tcpdns-server

        sleep 2
        if systemctl is-active tcpdns-server &>/dev/null; then
            success "iodined is running"
        else
            error "Failed to start iodined. Check: journalctl -u tcpdns-server"
            exit 1
        fi
    else
        info "Starting iodined directly..."
        iodined -f -c -P "$TUNNEL_PASSWORD" -p "$LISTEN_PORT" -m "$MTU" "${TUNNEL_IP}/${TUNNEL_SUBNET}" "$TUNNEL_DOMAIN" &
        sleep 2
        if pgrep -f iodined >/dev/null; then
            success "iodined is running (PID: $(pgrep -f iodined | head -1))"
        else
            error "Failed to start iodined"
            exit 1
        fi
    fi
}

save_config() {
    CONFIG_DIR="/etc/tcpdns"
    CONFIG_FILE="${CONFIG_DIR}/server.conf"

    mkdir -p "$CONFIG_DIR"
    chmod 700 "$CONFIG_DIR"

    cat > "$CONFIG_FILE" <<EOF
# tcpdns server configuration
# Generated on $(date -Iseconds)
TUNNEL_DOMAIN=${TUNNEL_DOMAIN}
TUNNEL_PASSWORD=${TUNNEL_PASSWORD}
TUNNEL_IP=${TUNNEL_IP}
TUNNEL_SUBNET=${TUNNEL_SUBNET}
LISTEN_PORT=${LISTEN_PORT}
MTU=${MTU}
EOF

    chmod 600 "$CONFIG_FILE"
    success "Configuration saved to ${CONFIG_FILE}"
}

print_summary() {
    VPS_IP=$(curl -4 -s --max-time 5 ifconfig.me 2>/dev/null || curl -4 -s --max-time 5 icanhazip.com 2>/dev/null || echo "YOUR_VPS_IP")

    echo ""
    echo -e "${GREEN}${BOLD}╔═══════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}${BOLD}║              Setup Complete!                      ║${NC}"
    echo -e "${GREEN}${BOLD}╚═══════════════════════════════════════════════════╝${NC}"
    echo ""
    echo -e "${BOLD}Server Info:${NC}"
    echo -e "  ${DIM}VPS IP:${NC}      ${VPS_IP}"
    echo -e "  ${DIM}Domain:${NC}      ${TUNNEL_DOMAIN}"
    echo -e "  ${DIM}Password:${NC}    ${TUNNEL_PASSWORD}"
    echo -e "  ${DIM}Tunnel Net:${NC}  ${TUNNEL_IP}/${TUNNEL_SUBNET}"
    echo -e "  ${DIM}Service:${NC}     tcpdns-server (systemd)"
    echo ""
    echo -e "${BOLD}DNS Records (set these at your registrar):${NC}"
    echo -e "  ${CYAN}A Record:${NC}   dns.yourdomain.com  ->  ${VPS_IP}"
    echo -e "  ${CYAN}NS Record:${NC}  ${TUNNEL_DOMAIN}  ->  dns.yourdomain.com"
    echo ""
    echo -e "${BOLD}Client Commands:${NC}"
    echo ""
    echo -e "  ${DIM}# Using tcpdns CLI:${NC}"
    echo -e "  ${GREEN}tcpdns client connect --domain ${TUNNEL_DOMAIN} --password ${TUNNEL_PASSWORD}${NC}"
    echo ""
    echo -e "  ${DIM}# Using iodine directly:${NC}"
    echo -e "  ${GREEN}sudo iodine -f -P ${TUNNEL_PASSWORD} ${TUNNEL_DOMAIN}${NC}"
    echo ""
    echo -e "  ${DIM}# Then start SOCKS proxy:${NC}"
    echo -e "  ${GREEN}ssh -D 1080 -N root@${TUNNEL_IP}${NC}"
    echo ""
    echo -e "${BOLD}Useful Commands:${NC}"
    echo -e "  ${DIM}Check status:${NC}    systemctl status tcpdns-server"
    echo -e "  ${DIM}View logs:${NC}       journalctl -u tcpdns-server -f"
    echo -e "  ${DIM}Restart:${NC}         systemctl restart tcpdns-server"
    echo -e "  ${DIM}Stop:${NC}            systemctl stop tcpdns-server"
    echo ""
    echo -e "${YELLOW}${BOLD}Important:${NC} Make sure to set up the DNS records above before connecting!"
    echo -e "${DIM}Test DNS: dig +short NS ${TUNNEL_DOMAIN}${NC}"
    echo ""
}

# ─────────────────────────────────────────────────────────────────────────
# Main
# ─────────────────────────────────────────────────────────────────────────

main() {
    banner
    check_root
    detect_os
    detect_interface

    TOTAL=8
    echo ""

    step 1 $TOTAL "Gathering configuration..."
    prompt_config

    step 2 $TOTAL "Checking port 53..."
    fix_port53

    step 3 $TOTAL "Installing iodine..."
    install_iodine

    step 4 $TOTAL "Enabling IP forwarding..."
    setup_forwarding

    step 5 $TOTAL "Configuring firewall..."
    setup_firewall

    step 6 $TOTAL "Creating systemd service..."
    create_systemd_service

    step 7 $TOTAL "Saving configuration..."
    save_config

    step 8 $TOTAL "Starting server..."
    start_server

    print_summary
}

main "$@"
