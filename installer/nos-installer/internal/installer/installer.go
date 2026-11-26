package installer

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
	"github.com/schollz/progressbar/v3"
)

type Installer struct {
	logFile      *os.File
	logger       *log.Logger
	targetDisk   string
	targetMount  string
	espPartition string
	rootPartition string
	isSSd        bool
	hostname     string
	timezone     string
}

func New() *Installer {
	return &Installer{
		targetMount: "/mnt",
		hostname:    "nos",
		timezone:    "UTC",
	}
}

func (i *Installer) Run() error {
	// Setup logging
	if err := i.setupLogging(); err != nil {
		return fmt.Errorf("failed to setup logging: %w", err)
	}
	defer i.logFile.Close()

	// Welcome screen
	i.showWelcome()

	// Step 1: Select disk
	if err := i.selectDisk(); err != nil {
		return fmt.Errorf("disk selection failed: %w", err)
	}

	// Step 2: Confirm destructive action
	if !i.confirmDestruction() {
		return fmt.Errorf("installation cancelled by user")
	}

	// Step 3: Partition disk
	if err := i.partitionDisk(); err != nil {
		return fmt.Errorf("disk partitioning failed: %w", err)
	}

	// Step 4: Create Btrfs filesystem with subvolumes
	if err := i.createBtrfsLayout(); err != nil {
		return fmt.Errorf("btrfs setup failed: %w", err)
	}

	// Step 5: Bootstrap system
	if err := i.bootstrapSystem(); err != nil {
		return fmt.Errorf("system bootstrap failed: %w", err)
	}

	// Step 6: Install bootloader
	if err := i.installBootloader(); err != nil {
		return fmt.Errorf("bootloader installation failed: %w", err)
	}

	// Step 7: Configure system
	if err := i.configureSystem(); err != nil {
		return fmt.Errorf("system configuration failed: %w", err)
	}

	// Step 8: Finalize
	if err := i.finalize(); err != nil {
		return fmt.Errorf("finalization failed: %w", err)
	}

	return nil
}

func (i *Installer) setupLogging() error {
	logPath := "/var/log/nos-installer.log"
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		// Try current directory as fallback
		logFile, err = os.OpenFile("nos-installer.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
	}
	
	i.logFile = logFile
	i.logger = log.New(io.MultiWriter(os.Stdout, logFile), "[INSTALLER] ", log.LstdFlags)
	i.logger.Println("Starting nOS installation")
	return nil
}

func (i *Installer) showWelcome() {
	color.Blue("\n╔═══════════════════════════════════════╗")
	color.Blue("║     nOS Guided Installer        ║")
	color.Blue("╚═══════════════════════════════════════╝\n")
	
	fmt.Println("This installer will guide you through the installation process.")
	fmt.Println("The following steps will be performed:")
	fmt.Println("  1. Select target disk")
	fmt.Println("  2. Partition disk (GPT with ESP + Btrfs)")
	fmt.Println("  3. Create Btrfs subvolumes")
	fmt.Println("  4. Bootstrap system")
	fmt.Println("  5. Install bootloader")
	fmt.Println("  6. Configure system")
	fmt.Println()
}

func (i *Installer) selectDisk() error {
	i.logger.Println("Selecting target disk")
	
	// Get available disks
	disks, err := i.getAvailableDisks()
	if err != nil {
		return err
	}
	
	if len(disks) == 0 {
		return fmt.Errorf("no suitable disks found")
	}
	
	// Create options for survey
	options := make([]string, len(disks))
	for idx, disk := range disks {
		options[idx] = fmt.Sprintf("%s - %s (%s)", disk.Path, disk.Model, disk.Size)
	}
	
	var selected string
	prompt := &survey.Select{
		Message: "Select target disk for installation:",
		Options: options,
	}
	
	if err := survey.AskOne(prompt, &selected); err != nil {
		return err
	}
	
	// Extract disk path from selection
	for idx, opt := range options {
		if opt == selected {
			i.targetDisk = disks[idx].Path
			i.isSSd = disks[idx].IsSSD
			break
		}
	}
	
	i.logger.Printf("Selected disk: %s (SSD: %v)", i.targetDisk, i.isSSd)
	return nil
}

func (i *Installer) confirmDestruction() bool {
	color.Red("\n⚠️  WARNING: This will DESTROY ALL DATA on %s", i.targetDisk)
	
	confirm := false
	prompt := &survey.Confirm{
		Message: "Do you want to continue?",
		Default: false,
	}
	
	if err := survey.AskOne(prompt, &confirm); err != nil {
		return false
	}
	
	if confirm {
		// Double confirmation
		confirmMsg := ""
		prompt := &survey.Input{
			Message: "Type 'DESTROY' to confirm:",
		}
		
		if err := survey.AskOne(prompt, &confirmMsg); err != nil {
			return false
		}
		
		return confirmMsg == "DESTROY"
	}
	
	return false
}

func (i *Installer) partitionDisk() error {
	i.logger.Printf("Partitioning disk %s", i.targetDisk)
	
	bar := progressbar.Default(4, "Partitioning disk")
	
	// Wipe existing partition table
	bar.Describe("Wiping partition table")
	if err := i.runCmd("wipefs", "-af", i.targetDisk); err != nil {
		return fmt.Errorf("failed to wipe disk: %w", err)
	}
	bar.Add(1)
	
	// Create GPT partition table
	bar.Describe("Creating GPT partition table")
	if err := i.runCmd("parted", "-s", i.targetDisk, "mklabel", "gpt"); err != nil {
		return fmt.Errorf("failed to create GPT table: %w", err)
	}
	bar.Add(1)
	
	// Create ESP partition (512 MiB)
	bar.Describe("Creating ESP partition")
	if err := i.runCmd("parted", "-s", i.targetDisk, "mkpart", "ESP", "fat32", "1MiB", "513MiB"); err != nil {
		return fmt.Errorf("failed to create ESP partition: %w", err)
	}
	if err := i.runCmd("parted", "-s", i.targetDisk, "set", "1", "esp", "on"); err != nil {
		return fmt.Errorf("failed to set ESP flag: %w", err)
	}
	bar.Add(1)
	
	// Create root partition (rest of disk)
	bar.Describe("Creating root partition")
	if err := i.runCmd("parted", "-s", i.targetDisk, "mkpart", "root", "btrfs", "513MiB", "100%"); err != nil {
		return fmt.Errorf("failed to create root partition: %w", err)
	}
	bar.Add(1)
	
	// Update partition paths
	if strings.HasPrefix(i.targetDisk, "/dev/nvme") || strings.HasPrefix(i.targetDisk, "/dev/mmcblk") {
		i.espPartition = i.targetDisk + "p1"
		i.rootPartition = i.targetDisk + "p2"
	} else {
		i.espPartition = i.targetDisk + "1"
		i.rootPartition = i.targetDisk + "2"
	}
	
	// Wait for partitions to appear
	time.Sleep(2 * time.Second)
	
	i.logger.Printf("Created partitions: ESP=%s, root=%s", i.espPartition, i.rootPartition)
	return nil
}

func (i *Installer) createBtrfsLayout() error {
	i.logger.Println("Creating Btrfs filesystem and subvolumes")
	
	bar := progressbar.Default(10, "Setting up Btrfs")
	
	// Format ESP
	bar.Describe("Formatting ESP partition")
	if err := i.runCmd("mkfs.vfat", "-F32", "-n", "ESP", i.espPartition); err != nil {
		return fmt.Errorf("failed to format ESP: %w", err)
	}
	bar.Add(1)
	
	// Format root as Btrfs
	bar.Describe("Creating Btrfs filesystem")
	if err := i.runCmd("mkfs.btrfs", "-f", "-L", "nOS", i.rootPartition); err != nil {
		return fmt.Errorf("failed to create Btrfs filesystem: %w", err)
	}
	bar.Add(1)
	
	// Mount root temporarily
	bar.Describe("Mounting filesystem")
	if err := i.runCmd("mount", i.rootPartition, i.targetMount); err != nil {
		return fmt.Errorf("failed to mount root: %w", err)
	}
	bar.Add(1)
	
	// Create subvolumes
	subvols := []string{"@", "@home", "@var", "@log", "@snapshots"}
	for _, subvol := range subvols {
		bar.Describe(fmt.Sprintf("Creating subvolume %s", subvol))
		subvolPath := filepath.Join(i.targetMount, subvol)
		if err := i.runCmd("btrfs", "subvolume", "create", subvolPath); err != nil {
			return fmt.Errorf("failed to create subvolume %s: %w", subvol, err)
		}
		bar.Add(1)
	}
	
	// Unmount to remount with subvolumes
	bar.Describe("Remounting with subvolumes")
	if err := i.runCmd("umount", i.targetMount); err != nil {
		return fmt.Errorf("failed to unmount: %w", err)
	}
	
	// Mount options
	mountOpts := "defaults,noatime,compress=zstd:3"
	if i.isSSd {
		mountOpts += ",ssd,discard=async"
	}
	
	// Mount @ as root
	if err := i.runCmd("mount", "-o", mountOpts+",subvol=@", i.rootPartition, i.targetMount); err != nil {
		return fmt.Errorf("failed to mount @ subvolume: %w", err)
	}
	
	// Create mount points
	for _, dir := range []string{"home", "var", "var/log", "snapshots", "boot/efi"} {
		if err := os.MkdirAll(filepath.Join(i.targetMount, dir), 0755); err != nil {
			return fmt.Errorf("failed to create mount point %s: %w", dir, err)
		}
	}
	
	// Mount other subvolumes
	subvolMounts := map[string]string{
		"@home":      "home",
		"@var":       "var", 
		"@log":       "var/log",
		"@snapshots": "snapshots",
	}
	
	for subvol, mountPoint := range subvolMounts {
		mountPath := filepath.Join(i.targetMount, mountPoint)
		if err := i.runCmd("mount", "-o", mountOpts+",subvol="+subvol, i.rootPartition, mountPath); err != nil {
			return fmt.Errorf("failed to mount %s: %w", subvol, err)
		}
	}
	
	// Mount ESP
	bar.Describe("Mounting ESP")
	if err := i.runCmd("mount", i.espPartition, filepath.Join(i.targetMount, "boot/efi")); err != nil {
		return fmt.Errorf("failed to mount ESP: %w", err)
	}
	bar.Add(1)
	
	i.logger.Println("Btrfs layout created successfully")
	return nil
}

func (i *Installer) bootstrapSystem() error {
	i.logger.Println("Bootstrapping system")
	
	// Check if we should copy from live system or use debootstrap
	if _, err := os.Stat("/usr/share/nos/live-base.tar.gz"); err == nil {
		return i.bootstrapFromLive()
	}
	
	return i.bootstrapDebootstrap()
}

func (i *Installer) bootstrapFromLive() error {
	bar := progressbar.Default(3, "Copying system from live image")
	
	bar.Describe("Extracting base system")
	cmd := exec.Command("tar", "-xzf", "/usr/share/nos/live-base.tar.gz", "-C", i.targetMount)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to extract base system: %w", err)
	}
	bar.Add(1)
	
	bar.Describe("Copying kernel and initramfs")
	// Copy kernel and initramfs from live system
	for _, file := range []string{"vmlinuz", "initrd.img"} {
		src := filepath.Join("/boot", file)
		dst := filepath.Join(i.targetMount, "boot", file)
		if err := i.copyFile(src, dst); err != nil {
			i.logger.Printf("Warning: failed to copy %s: %v", file, err)
		}
	}
	bar.Add(1)
	
	bar.Describe("Installing packages")
	// Install required packages in chroot
	packages := []string{
		"linux-image-amd64",
		"grub-efi-amd64", 
		"nosd",
		"nos-agent",
		"nos-web",
		"caddy",
		"wireguard",
		"nftables",
		"btrfs-progs",
		"systemd",
		"systemd-resolved",
		"openssh-server",
	}
	
	if err := i.chrootRun("apt-get", "update"); err != nil {
		return fmt.Errorf("failed to update package list: %w", err)
	}
	
	args := append([]string{"install", "-y"}, packages...)
	if err := i.chrootRun("apt-get", args...); err != nil {
		return fmt.Errorf("failed to install packages: %w", err)
	}
	bar.Add(1)
	
	return nil
}

func (i *Installer) bootstrapDebootstrap() error {
	bar := progressbar.Default(4, "Bootstrapping with debootstrap")
	
	bar.Describe("Running debootstrap")
	if err := i.runCmd("debootstrap", "--arch=amd64", "--include=systemd,systemd-sysv", "bookworm", i.targetMount, "http://deb.debian.org/debian"); err != nil {
		return fmt.Errorf("debootstrap failed: %w", err)
	}
	bar.Add(1)
	
	// Configure APT sources
	bar.Describe("Configuring APT")
	sourcesContent := `deb http://deb.debian.org/debian bookworm main contrib non-free non-free-firmware
deb http://deb.debian.org/debian bookworm-updates main contrib non-free non-free-firmware
deb http://security.debian.org/debian-security bookworm-security main contrib non-free non-free-firmware
`
	sourcesPath := filepath.Join(i.targetMount, "etc/apt/sources.list")
	if err := os.WriteFile(sourcesPath, []byte(sourcesContent), 0644); err != nil {
		return fmt.Errorf("failed to write sources.list: %w", err)
	}
	bar.Add(1)
	
	// Update and install packages
	bar.Describe("Installing packages")
	if err := i.chrootRun("apt-get", "update"); err != nil {
		return fmt.Errorf("failed to update package list: %w", err)
	}
	
	packages := []string{
		"linux-image-amd64",
		"grub-efi-amd64",
		"btrfs-progs",
		"openssh-server",
		"wireguard",
		"nftables",
		"curl",
		"wget",
		"sudo",
		"locales",
		"console-setup",
		"keyboard-configuration",
	}
	
	args := append([]string{"install", "-y"}, packages...)
	if err := i.chrootRun("apt-get", args...); err != nil {
		return fmt.Errorf("failed to install packages: %w", err)
	}
	bar.Add(1)
	
	// Install nOS packages if available
	bar.Describe("Installing nOS packages")
	nosPackages := []string{"nosd", "nos-agent", "nos-web", "caddy"}
	for _, pkg := range nosPackages {
		debPath := fmt.Sprintf("/tmp/%s.deb", pkg)
		if _, err := os.Stat(debPath); err == nil {
			targetPath := filepath.Join(i.targetMount, "tmp", fmt.Sprintf("%s.deb", pkg))
			if err := i.copyFile(debPath, targetPath); err == nil {
				i.chrootRun("dpkg", "-i", filepath.Join("/tmp", fmt.Sprintf("%s.deb", pkg)))
			}
		}
	}
	bar.Add(1)
	
	return nil
}

func (i *Installer) installBootloader() error {
	i.logger.Println("Installing bootloader")
	
	bar := progressbar.Default(5, "Installing GRUB")
	
	// Bind mount necessary filesystems
	bar.Describe("Preparing chroot environment")
	for _, mount := range [][]string{
		{"proc", "proc", "/proc"},
		{"sysfs", "sys", "/sys"},
		{"devtmpfs", "dev", "/dev"},
		{"devpts", "dev/pts", "/dev/pts"},
	} {
		target := filepath.Join(i.targetMount, mount[2])
		if err := i.runCmd("mount", "-t", mount[0], mount[1], target); err != nil {
			i.logger.Printf("Warning: failed to mount %s: %v", mount[2], err)
		}
	}
	bar.Add(1)
	
	// Install GRUB
	bar.Describe("Installing GRUB to ESP")
	if err := i.chrootRun("grub-install", "--target=x86_64-efi", "--efi-directory=/boot/efi", "--bootloader-id=nOS", "--recheck"); err != nil {
		return fmt.Errorf("failed to install GRUB: %w", err)
	}
	bar.Add(1)
	
	// Configure GRUB
	bar.Describe("Configuring GRUB")
	grubDefault := `GRUB_DEFAULT=0
GRUB_TIMEOUT=5
GRUB_DISTRIBUTOR="nOS"
GRUB_CMDLINE_LINUX_DEFAULT="quiet splash"
GRUB_CMDLINE_LINUX="rootflags=subvol=@"
`
	grubPath := filepath.Join(i.targetMount, "etc/default/grub")
	if err := os.WriteFile(grubPath, []byte(grubDefault), 0644); err != nil {
		return fmt.Errorf("failed to write GRUB config: %w", err)
	}
	bar.Add(1)
	
	// Copy branding if available
	bar.Describe("Adding branding")
	brandingSource := "/usr/share/nos/grub-theme"
	brandingTarget := filepath.Join(i.targetMount, "boot/grub/themes/nos")
	if _, err := os.Stat(brandingSource); err == nil {
		os.MkdirAll(brandingTarget, 0755)
		i.runCmd("cp", "-r", brandingSource+"/*", brandingTarget)
		
		// Add theme to config
		grubDefault += `GRUB_THEME="/boot/grub/themes/nos/theme.txt"
`
		os.WriteFile(grubPath, []byte(grubDefault), 0644)
	}
	bar.Add(1)
	
	// Generate GRUB configuration
	bar.Describe("Generating GRUB configuration")
	if err := i.chrootRun("update-grub"); err != nil {
		return fmt.Errorf("failed to update GRUB: %w", err)
	}
	bar.Add(1)
	
	return nil
}

func (i *Installer) configureSystem() error {
	i.logger.Println("Configuring system")
	
	bar := progressbar.Default(8, "System configuration")
	
	// Generate fstab
	bar.Describe("Generating fstab")
	if err := i.generateFstab(); err != nil {
		return fmt.Errorf("failed to generate fstab: %w", err)
	}
	bar.Add(1)
	
	// Set hostname
	bar.Describe("Setting hostname")
	hostnamePath := filepath.Join(i.targetMount, "etc/hostname")
	if err := os.WriteFile(hostnamePath, []byte(i.hostname+"\n"), 0644); err != nil {
		return fmt.Errorf("failed to set hostname: %w", err)
	}
	
	hostsContent := fmt.Sprintf(`127.0.0.1	localhost
127.0.1.1	%s
::1		localhost ip6-localhost ip6-loopback
ff02::1		ip6-allnodes
ff02::2		ip6-allrouters
`, i.hostname)
	hostsPath := filepath.Join(i.targetMount, "etc/hosts")
	if err := os.WriteFile(hostsPath, []byte(hostsContent), 0644); err != nil {
		return fmt.Errorf("failed to write hosts file: %w", err)
	}
	bar.Add(1)
	
	// Set timezone
	bar.Describe("Setting timezone")
	if err := i.chrootRun("ln", "-sf", fmt.Sprintf("/usr/share/zoneinfo/%s", i.timezone), "/etc/localtime"); err != nil {
		i.logger.Printf("Warning: failed to set timezone: %v", err)
	}
	bar.Add(1)
	
	// Configure locales
	bar.Describe("Configuring locales")
	localePath := filepath.Join(i.targetMount, "etc/locale.gen")
	localeContent, _ := os.ReadFile(localePath)
	localeContent = []byte(strings.ReplaceAll(string(localeContent), "# en_US.UTF-8", "en_US.UTF-8"))
	os.WriteFile(localePath, localeContent, 0644)
	i.chrootRun("locale-gen")
	bar.Add(1)
	
	// Create service users
	bar.Describe("Creating service users")
	i.chrootRun("groupadd", "-r", "nosd")
	i.chrootRun("useradd", "-r", "-g", "nosd", "-s", "/bin/false", "-d", "/var/lib/nosd", "nosd")
	bar.Add(1)
	
	// Enable services
	bar.Describe("Enabling services")
	services := []string{"nosd", "nos-agent", "caddy", "ssh", "systemd-networkd", "systemd-resolved"}
	for _, service := range services {
		i.chrootRun("systemctl", "enable", service)
	}
	bar.Add(1)
	
	// Configure Caddy
	bar.Describe("Configuring Caddy")
	if err := i.configureCaddy(); err != nil {
		i.logger.Printf("Warning: failed to configure Caddy: %v", err)
	}
	bar.Add(1)
	
	// Write os-release
	bar.Describe("Writing os-release")
	osRelease := `NAME="nOS"
VERSION="1.0"
ID=nos
ID_LIKE=debian
PRETTY_NAME="nOS 1.0"
VERSION_ID="1.0"
HOME_URL="https://github.com/Nithron/nOS"
SUPPORT_URL="https://github.com/Nithron/nOS"
BUG_REPORT_URL="https://github.com/Nithron/nOS/issues"
`
	osReleasePath := filepath.Join(i.targetMount, "etc/os-release")
	if err := os.WriteFile(osReleasePath, []byte(osRelease), 0644); err != nil {
		return fmt.Errorf("failed to write os-release: %w", err)
	}
	bar.Add(1)
	
	return nil
}

func (i *Installer) generateFstab() error {
	mountOpts := "defaults,noatime,compress=zstd:3"
	if i.isSSd {
		mountOpts += ",ssd,discard=async"
	}
	
	espUUID, _ := i.getUUID(i.espPartition)
	rootUUID, _ := i.getUUID(i.rootPartition)
	
	fstabContent := fmt.Sprintf(`# /etc/fstab: static file system information.
# <file system> <mount point> <type> <options> <dump> <pass>

# ESP
UUID=%s /boot/efi vfat defaults 0 2

# Btrfs subvolumes
UUID=%s / btrfs %s,subvol=@ 0 1
UUID=%s /home btrfs %s,subvol=@home 0 2
UUID=%s /var btrfs %s,subvol=@var 0 2
UUID=%s /var/log btrfs %s,subvol=@log 0 2
UUID=%s /snapshots btrfs %s,subvol=@snapshots 0 2
`, espUUID, rootUUID, mountOpts, rootUUID, mountOpts, rootUUID, mountOpts, rootUUID, mountOpts, rootUUID, mountOpts)
	
	fstabPath := filepath.Join(i.targetMount, "etc/fstab")
	return os.WriteFile(fstabPath, []byte(fstabContent), 0644)
}

func (i *Installer) configureCaddy() error {
	caddyfile := `{
	admin off
	auto_https off
}

:80 {
	redir https://{host}{uri} permanent
}

:443 {
	tls internal
	
	handle /api/* {
		reverse_proxy 127.0.0.1:9000
	}
	
	handle {
		root * /usr/share/nos/web
		file_server
		try_files {path} /index.html
	}
	
	log {
		output file /var/log/caddy/access.log
		format json
	}
}
`
	caddyPath := filepath.Join(i.targetMount, "etc/caddy/Caddyfile")
	os.MkdirAll(filepath.Dir(caddyPath), 0755)
	return os.WriteFile(caddyPath, []byte(caddyfile), 0644)
}

func (i *Installer) finalize() error {
	i.logger.Println("Finalizing installation")
	
	bar := progressbar.Default(3, "Finalizing")
	
	// Update initramfs
	bar.Describe("Updating initramfs")
	if err := i.chrootRun("update-initramfs", "-u", "-k", "all"); err != nil {
		i.logger.Printf("Warning: failed to update initramfs: %v", err)
	}
	bar.Add(1)
	
	// Copy install log to target
	bar.Describe("Copying installation log")
	logDst := filepath.Join(i.targetMount, "var/log/nos-installer.log")
	os.MkdirAll(filepath.Dir(logDst), 0755)
	if i.logFile != nil {
		i.logFile.Sync()
		srcPath := i.logFile.Name()
		i.copyFile(srcPath, logDst)
	}
	bar.Add(1)
	
	// Unmount everything
	bar.Describe("Unmounting filesystems")
	// Unmount in reverse order
	mounts := []string{
		filepath.Join(i.targetMount, "dev/pts"),
		filepath.Join(i.targetMount, "dev"),
		filepath.Join(i.targetMount, "sys"),
		filepath.Join(i.targetMount, "proc"),
		filepath.Join(i.targetMount, "boot/efi"),
		filepath.Join(i.targetMount, "snapshots"),
		filepath.Join(i.targetMount, "var/log"),
		filepath.Join(i.targetMount, "var"),
		filepath.Join(i.targetMount, "home"),
		i.targetMount,
	}
	
	for _, mount := range mounts {
		i.runCmd("umount", "-l", mount)
	}
	bar.Add(1)
	
	i.logger.Println("Installation completed successfully")
	return nil
}

// Helper functions

func (i *Installer) runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		i.logger.Printf("Command failed: %s %v\nOutput: %s", name, args, string(output))
		return err
	}
	return nil
}

func (i *Installer) chrootRun(name string, args ...string) error {
	chrootArgs := append([]string{i.targetMount, name}, args...)
	return i.runCmd("chroot", chrootArgs...)
}

func (i *Installer) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()
	
	os.MkdirAll(filepath.Dir(dst), 0755)
	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()
	
	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}
	
	info, _ := sourceFile.Stat()
	return os.Chmod(dst, info.Mode())
}

func (i *Installer) getUUID(device string) (string, error) {
	cmd := exec.Command("blkid", "-s", "UUID", "-o", "value", device)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

type DiskInfo struct {
	Path  string
	Model string
	Size  string
	IsSSD bool
}

func (i *Installer) getAvailableDisks() ([]DiskInfo, error) {
	var disks []DiskInfo
	
	// Use lsblk to get disk information
	cmd := exec.Command("lsblk", "-ndo", "NAME,MODEL,SIZE,ROTA,TYPE")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 5 {
			continue
		}
		
		// Only consider whole disks
		if fields[4] != "disk" {
			continue
		}
		
		name := fields[0]
		model := fields[1]
		if model == "" {
			model = "Unknown"
		}
		size := fields[2]
		isSSD := fields[3] == "0" // ROTA=0 means SSD
		
		// Skip loop devices and ram disks
		if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") {
			continue
		}
		
		disks = append(disks, DiskInfo{
			Path:  "/dev/" + name,
			Model: model,
			Size:  size,
			IsSSD: isSSD,
		})
	}
	
	return disks, nil
}
