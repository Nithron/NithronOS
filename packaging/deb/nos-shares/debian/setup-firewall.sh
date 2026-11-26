#!/bin/bash
# Setup firewall rules for nOS shares
# This script is idempotent - safe to run multiple times

set -e

# Check if nftables is available
if ! command -v nft >/dev/null 2>&1; then
    echo "Warning: nftables not found, skipping firewall setup"
    exit 0
fi

# Function to check if a rule exists
rule_exists() {
    local chain="$1"
    local rule="$2"
    nft list chain inet filter "$chain" 2>/dev/null | grep -q "$rule"
}

# Function to add rule if it doesn't exist
add_rule_if_missing() {
    local chain="$1"
    local rule="$2"
    local comment="$3"
    
    if ! rule_exists "$chain" "$rule"; then
        echo "Adding firewall rule: $comment"
        nft add rule inet filter "$chain" $rule comment \"$comment\"
    fi
}

# Ensure filter table and input chain exist
nft list table inet filter >/dev/null 2>&1 || nft add table inet filter
nft list chain inet filter input >/dev/null 2>&1 || nft add chain inet filter input { type filter hook input priority 0 \; }

# SMB/CIFS ports (445, 139) - LAN only
add_rule_if_missing "input" \
    "ip saddr { 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16 } tcp dport 445 accept" \
    "SMB/CIFS"

add_rule_if_missing "input" \
    "ip saddr { 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16 } tcp dport 139 accept" \
    "NetBIOS"

# NFS ports (111, 2049) - LAN only
add_rule_if_missing "input" \
    "ip saddr { 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16 } tcp dport { 111, 2049 } accept" \
    "NFS TCP"

add_rule_if_missing "input" \
    "ip saddr { 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16 } udp dport { 111, 2049 } accept" \
    "NFS UDP"

# mDNS/Avahi (5353) - All sources for discovery
add_rule_if_missing "input" \
    "udp dport 5353 accept" \
    "mDNS/Avahi"

echo "Firewall rules for shares configured successfully"
exit 0
