# Nextcloud

Self-hosted file sync, sharing, and collaboration platform.

## Features

- File synchronization across devices
- File sharing with internal and external users
- Collaborative document editing
- Calendar, contacts, and task management
- Video calls and chat
- Extensive app ecosystem

## Initial Setup

After installation, Nextcloud will automatically configure itself with:
- PostgreSQL database
- Redis caching
- Admin account (using provided credentials)

First login: Use the admin username and password you configured during installation.

## Services

This deployment includes:
- **Nextcloud**: Main application server
- **PostgreSQL**: Database backend
- **Redis**: Cache and session storage
- **Cron**: Background job processor

## Configuration

### Required
- **Admin Password**: Strong password for admin account
- **Database Password**: PostgreSQL database password

### Optional
- **Port**: Web interface port (default: 8081)
- **Admin Username**: Administrator username (default: admin)
- **Trusted Domains**: Domains allowed to access Nextcloud
- **Protocol**: HTTP or HTTPS (configure HTTPS via reverse proxy)

## Post-Installation

1. Log in with admin credentials
2. Configure email settings (Settings → Basic settings)
3. Set up external storage if needed
4. Install additional apps from the app store
5. Create users and groups

## Data Storage

- **Files**: `/data/nextcloud/data/`
- **Apps**: `/data/apps/`
- **Config**: `/data/config/`
- **Database**: `/data/postgres/`

## Security Notes

- Change default passwords immediately
- Enable 2FA for admin accounts
- Regular backups recommended
- Keep Nextcloud updated

## Performance Tuning

The deployment includes Redis for caching and uses PostgreSQL for better performance with large datasets.

For large deployments, consider:
- Increasing PHP memory limits
- Tuning PostgreSQL settings
- Adding more Redis memory

## External Access

To access Nextcloud from outside your network:
1. Configure port forwarding or reverse proxy
2. Set up HTTPS with valid certificates
3. Update trusted domains configuration
