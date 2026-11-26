# nOS — Pre-Alpha Recovery Checklist

> For **testers** during pre-alpha. These steps assume local/physical access. Use carefully — some actions are destructive.

## Before you start
- **Console access:** Attach monitor + keyboard (or serial).
- **Power:** Avoid outages during recovery.
- **Version:** Note the ISO/tag (e.g., `v0.1.2-pre-alpha`) when reporting.
- **Capture logs later:**
    journalctl -u nosd -b --no-pager > /tmp/nosd.log

---

## Common recovery scenarios

### 1) Lost first-boot OTP / re-run setup
**Goal:** Trigger setup wizard again.
    
    sudo mkdir -p /etc/nos
    sudo mv -f /etc/nos/users.json /etc/nos/users.json.bak 2>/dev/null || true
    sudo systemctl restart nosd

Reload UI — setup/OTP flow should appear.

### 2) Forgot admin password or lost 2FA
**A) Recovery mode (preferred)**

1. At GRUB, highlight nOS, press **e**, append to the linux line:
       
       nos.recovery=1

   Boot with **F10** / **Ctrl+X**.

2. Use helper:
       
       sudo bash scripts/recovery-help.sh
       # Choose: reset admin / disable 2FA / generate one-time OTP

3. Reboot (remove the kernel arg next boot).

**B) Force re-setup (destructive to accounts)**  
Do scenario **1** (removes `users.json`).

### 3) Locked out by rate limits (many failures)

    sudo rm -f /var/lib/nos/ratelimit.json
    sudo systemctl restart nosd

### 4) Remote access / firewall blocks UI
Reset to LAN-only and allow local web:

    sudo nft flush ruleset
    sudo bash deploy/firewall/apply.sh --mode lan-only || true
    sudo systemctl restart nftables nosd

### 5) Update failed — rollback snapshot
From UI: **Settings → Updates → Rollback**  
Via API:

    curl -sS -X POST http://127.0.0.1:18080/api/v1/update/rollback

Reboot if prompted.

### 6) Agent can’t register (daemon ↔ agent trust)
Regenerate/inspect token:

    sudo ls -l /etc/nos/agent-token || true
    sudo truncate -s 0 /etc/nos/agent-token
    head -c 32 /dev/urandom | base64 | sudo tee /etc/nos/agent-token >/dev/null
    sudo chmod 600 /etc/nos/agent-token
    sudo systemctl restart nosd nos-agent

---

## After recovery
- Remove `nos.recovery=1` on next boot.
- Change passwords, re-enable 2FA, regenerate recovery codes.
- Take a snapshot (good known state).

## Useful commands

    # Service status & logs
    sudo systemctl status nosd --no-pager
    journalctl -u nosd -b --no-pager | tail -200

    # Local health probe
    curl -sS http://127.0.0.1:18080/health || true

## Reporting an issue
Include tag/version (e.g., `v0.1.2-pre-alpha`), hardware summary, exact steps taken, and attach:
- `/tmp/nosd.log`
- Output of: `journalctl -b --no-pager | tail -500`


