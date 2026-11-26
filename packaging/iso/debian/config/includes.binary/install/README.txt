nOS Installer
===================

This installer uses the standard Debian installer to:
1. Install a base Debian Bookworm system
2. Configure storage (Btrfs recommended)
3. Set up networking and base services
4. Create the admin user account

After installation:
- Reboot the system
- Navigate to http://your-server-ip in a web browser
- Complete the nOS web setup

The installer is fully automated using preseed configuration.
If you need to customize the installation, you can edit the
preseed file or use Expert Mode from the boot menu.
