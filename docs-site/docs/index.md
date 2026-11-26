# Welcome to nOS Documentation

<div align="center">
  <img src="assets/logo.svg" alt="nOS Logo" width="200">
  
  **The secure, self-hosted storage and application platform**
  
  [![Version](https://img.shields.io/badge/version-1.0.0-blue)](releases/v1.0.md)
  [![License](https://img.shields.io/badge/license-Proprietary-red)](https://github.com/Nithron/nOS/blob/main/LICENSE)
  [![Build Status](https://img.shields.io/github/actions/workflow/status/Nithron/nOS/ci-complete.yml)](https://github.com/Nithron/nOS/actions)
</div>

## What is nOS?

nOS is a comprehensive storage and application platform designed for self-hosting enthusiasts and small businesses. Built on Debian Linux with enterprise-grade features, it provides:

- **🗄️ Advanced Storage Management** - Btrfs-based with snapshots, RAID, and automatic health monitoring
- **📦 Application Ecosystem** - Docker-based app deployment with one-click installs
- **🔒 Security First** - Built-in firewall, VPN, 2FA, and comprehensive audit logging
- **💾 Automated Backups** - Scheduled snapshots with flexible retention and replication
- **📊 Real-time Monitoring** - System metrics, alerts, and health dashboards
- **🌐 Remote Access** - Secure remote management via WireGuard VPN or reverse proxy
- **🚀 Easy Setup** - Guided installer and web-based configuration

## Quick Links

<div class="grid cards" markdown>

-   :material-rocket-launch: **Getting Started**
    
    ---
    
    New to nOS? Start here with installation and initial setup.
    
    [:octicons-arrow-right-24: Installation Guide](getting-started/installation/iso-install.md)

-   :material-book-open-variant: **User Guide**
    
    ---
    
    Learn how to use nOS features and manage your system.
    
    [:octicons-arrow-right-24: User Documentation](user-guide/index.md)

-   :material-code-braces: **API Reference**
    
    ---
    
    Integrate with nOS using our comprehensive REST API.
    
    [:octicons-arrow-right-24: API Documentation](api/index.md)

-   :material-console: **CLI Reference**
    
    ---
    
    Command-line tools for automation and scripting.
    
    [:octicons-arrow-right-24: nosctl Documentation](cli/index.md)

</div>

## System Requirements

### Minimum Requirements

- **CPU**: 64-bit x86 processor (2+ cores recommended)
- **RAM**: 2 GB (4 GB recommended)
- **Storage**: 20 GB for system (more for data storage)
- **Network**: Ethernet connection

### Recommended Requirements

- **CPU**: 4+ core modern processor
- **RAM**: 8 GB or more
- **Storage**: 
  - 128 GB SSD for system
  - Additional drives for data storage (RAID recommended)
- **Network**: Gigabit Ethernet

## Key Features

### Storage Management
- Btrfs filesystem with compression and deduplication
- Automatic snapshots with configurable retention
- RAID support (0, 1, 5, 6, 10)
- SMART monitoring and alerts
- Scheduled scrubs for data integrity

### Application Platform
- Docker-based containerization
- One-click app installation
- App catalog with popular services
- Custom app support via SDK
- Automatic updates with rollback

### Security
- Role-based access control (Admin, Operator, Viewer)
- Two-factor authentication (TOTP)
- API tokens with fine-grained scopes
- Comprehensive audit logging
- Automated security updates

### Backup & Recovery
- Scheduled backups with GFS retention
- Multiple destinations (local, SSH, cloud)
- Incremental replication
- Point-in-time recovery
- Application-aware backups

### Monitoring & Alerts
- Real-time system metrics
- Customizable alert rules
- Multiple notification channels (Email, Webhook, Ntfy)
- Historical data with retention
- Service health monitoring

### Networking
- Built-in firewall management
- WireGuard VPN server
- Let's Encrypt integration
- Reverse proxy with Caddy
- Remote access wizard

## Community & Support

- **GitHub**: [github.com/Nithron/nOS](https://github.com/Nithron/nOS)
- **Discord**: [discord.gg/qzB37WS5AT](https://discord.gg/qzB37WS5AT)
- **Forum**: [GitHub Discussions](https://github.com/Nithron/nOS/discussions)

## Contributing

We welcome contributions! See our [Contributing Guide](developer/contributing/guidelines.md) to get started.

## License

nOS is proprietary software. See the [LICENSE](https://github.com/Nithron/nOS/blob/main/LICENSE) file for details.

---

!!! tip "New to nOS?"
    Start with our [Quick Start Guide](getting-started/quickstart.md) to get up and running in minutes!

!!! info "Looking for the latest release?"
    Check out the [Release Notes](releases/index.md) for new features and improvements.
