# Jellyfin

Free and open-source media server for streaming movies, TV shows, music, and more.

## Features

- Stream media to any device
- No subscription or fees
- Live TV and DVR support
- Hardware acceleration support
- Mobile and TV apps available
- Metadata fetching and organization
- User management with parental controls

## Initial Setup

1. Access Jellyfin at http://localhost:8096
2. Follow the setup wizard:
   - Choose language
   - Create admin account
   - Add media libraries
   - Configure metadata providers
   - Set up remote access (optional)

## Media Organization

Recommended folder structure:
```
/media/
├── Movies/
│   ├── Movie Name (Year)/
│   │   └── Movie Name (Year).mkv
├── TV Shows/
│   ├── Show Name/
│   │   ├── Season 01/
│   │   │   ├── S01E01 - Episode Name.mkv
│   │   │   └── S01E02 - Episode Name.mkv
├── Music/
│   ├── Artist Name/
│   │   ├── Album Name/
│   │   │   ├── 01 - Track Name.mp3
```

## Configuration

- **Web Port**: HTTP port for web interface (default: 8096)
- **Discovery Port**: UDP port for client discovery
- **DLNA Port**: UDP port for DLNA devices
- **Media Path**: Location of your media files
- **User/Group ID**: Run as specific user for file permissions

## Hardware Acceleration

For transcoding performance, hardware acceleration can be enabled:

### Intel Quick Sync
Uncomment the device mapping in compose.yaml:
```yaml
devices:
  - /dev/dri:/dev/dri
```

### NVIDIA GPU
Add GPU support to the container (requires nvidia-docker2)

## Client Applications

Jellyfin has clients for:
- Web browsers
- Android/iOS
- Android TV/Apple TV
- Roku
- Kodi
- Windows/Mac/Linux desktop

## Network Access

### Local Network
Jellyfin will be accessible at:
- http://[server-ip]:8096

### Remote Access
For external access:
1. Configure port forwarding
2. Set up Dynamic DNS
3. Use reverse proxy with HTTPS
4. Or use Jellyfin's built-in HTTPS

## Plugins

Popular plugins:
- Anime metadata providers
- Subtitle downloaders
- Authentication providers
- Notification services
- Backup tools

Install via: Dashboard → Plugins → Catalog

## Performance Tips

- Use hardware acceleration when possible
- Pre-transcode content for common devices
- Optimize media formats (H.264/H.265)
- Use wired connections for 4K streaming
- Enable chapter image extraction during quiet hours

## Troubleshooting

### Playback Issues
- Check transcoding logs
- Verify codec support
- Test network bandwidth
- Check client compatibility

### Library Scanning
- Verify file permissions
- Check naming conventions
- Review metadata providers
- Check for locked database

## Security

- Use strong admin password
- Configure user permissions carefully
- Disable DLNA if not needed
- Use HTTPS for remote access
- Regular backups of config folder
