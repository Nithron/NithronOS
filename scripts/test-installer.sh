#!/bin/bash
# nOS Installer Test Script
# Tests the installer in a QEMU VM

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
VM_NAME="nos-installer-test"
VM_DISK="/tmp/${VM_NAME}.qcow2"
VM_SIZE="20G"
VM_RAM="2048"
VM_CPUS="2"
ISO_PATH="${1:-nos-live.iso}"
LOG_FILE="/tmp/${VM_NAME}.log"

# Functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

cleanup() {
    log_info "Cleaning up..."
    
    # Stop VM if running
    if pgrep -f "qemu.*${VM_NAME}" > /dev/null; then
        log_info "Stopping VM..."
        pkill -f "qemu.*${VM_NAME}" || true
        sleep 2
    fi
    
    # Remove disk image
    if [ -f "$VM_DISK" ]; then
        rm -f "$VM_DISK"
    fi
}

check_requirements() {
    log_info "Checking requirements..."
    
    # Check for QEMU
    if ! command -v qemu-system-x86_64 &> /dev/null; then
        log_error "qemu-system-x86_64 not found. Please install QEMU."
        exit 1
    fi
    
    # Check for ISO
    if [ ! -f "$ISO_PATH" ]; then
        log_error "ISO not found: $ISO_PATH"
        exit 1
    fi
    
    # Check for OVMF (UEFI firmware)
    OVMF_PATH="/usr/share/OVMF/OVMF_CODE.fd"
    if [ ! -f "$OVMF_PATH" ]; then
        # Try alternative paths
        for path in /usr/share/qemu/OVMF.fd /usr/share/edk2-ovmf/x64/OVMF_CODE.fd; do
            if [ -f "$path" ]; then
                OVMF_PATH="$path"
                break
            fi
        done
        
        if [ ! -f "$OVMF_PATH" ]; then
            log_warn "UEFI firmware not found. Testing with BIOS mode."
            UEFI_ARGS=""
        else
            UEFI_ARGS="-bios $OVMF_PATH"
        fi
    else
        UEFI_ARGS="-bios $OVMF_PATH"
    fi
}

create_disk() {
    log_info "Creating virtual disk..."
    qemu-img create -f qcow2 "$VM_DISK" "$VM_SIZE"
}

run_installer_test() {
    log_info "Starting installer test VM..."
    
    # Build QEMU command
    QEMU_CMD="qemu-system-x86_64 \
        -name $VM_NAME \
        -m $VM_RAM \
        -smp $VM_CPUS \
        -enable-kvm \
        -cpu host \
        $UEFI_ARGS \
        -drive file=$VM_DISK,format=qcow2,if=virtio \
        -cdrom $ISO_PATH \
        -boot d \
        -netdev user,id=net0,hostfwd=tcp::10443-:443,hostfwd=tcp::10080-:80,hostfwd=tcp::10022-:22 \
        -device virtio-net-pci,netdev=net0 \
        -vnc :1 \
        -monitor stdio"
    
    log_info "VM will be accessible via:"
    log_info "  VNC: localhost:5901"
    log_info "  SSH: localhost:10022"
    log_info "  HTTPS: https://localhost:10443"
    log_info ""
    log_info "Starting VM (press Ctrl+C to stop)..."
    
    # Run VM
    $QEMU_CMD 2>&1 | tee "$LOG_FILE"
}

automated_install_test() {
    log_info "Running automated installation test..."
    
    # Create expect script for automation
    cat > /tmp/installer-test.exp << 'EOF'
#!/usr/bin/expect -f
set timeout 300

# Connect to VM console
spawn ssh -p 10022 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null root@localhost

# Wait for prompt
expect "# "

# Run installer
send "nos-installer\r"

# Wait for disk selection
expect "Select target disk"
send "\r"  # Select first disk

# Confirm destruction
expect "Do you want to continue?"
send "y\r"
expect "Type 'DESTROY'"
send "DESTROY\r"

# Wait for completion
expect "Installation completed successfully"

# Reboot
send "reboot\r"
expect eof
EOF
    
    chmod +x /tmp/installer-test.exp
    
    # Start VM in background
    $QEMU_CMD &> "$LOG_FILE" &
    QEMU_PID=$!
    
    # Wait for VM to boot
    log_info "Waiting for VM to boot..."
    sleep 30
    
    # Run automated test
    if /tmp/installer-test.exp; then
        log_info "Automated installation completed successfully"
    else
        log_error "Automated installation failed"
        cat "$LOG_FILE"
        kill $QEMU_PID
        exit 1
    fi
    
    # Stop VM
    kill $QEMU_PID
    wait $QEMU_PID 2>/dev/null || true
}

verify_installation() {
    log_info "Verifying installation..."
    
    # Boot from installed disk
    QEMU_CMD="qemu-system-x86_64 \
        -name ${VM_NAME}-verify \
        -m $VM_RAM \
        -smp $VM_CPUS \
        -enable-kvm \
        -cpu host \
        $UEFI_ARGS \
        -drive file=$VM_DISK,format=qcow2,if=virtio \
        -netdev user,id=net0,hostfwd=tcp::10443-:443,hostfwd=tcp::10080-:80,hostfwd=tcp::10022-:22 \
        -device virtio-net-pci,netdev=net0 \
        -display none \
        -serial stdio"
    
    # Start VM
    timeout 120 $QEMU_CMD &> "$LOG_FILE" &
    QEMU_PID=$!
    
    # Wait for boot
    sleep 60
    
    # Check if services are running
    log_info "Checking services..."
    
    # Try to access web interface
    if curl -k -f https://localhost:10443 &> /dev/null; then
        log_info "✓ Web interface is accessible"
    else
        log_error "✗ Web interface is not accessible"
        FAILED=1
    fi
    
    # Check for OTP in logs
    if grep -q "First-boot OTP:" "$LOG_FILE"; then
        log_info "✓ OTP was generated"
    else
        log_warn "⚠ OTP not found in logs (may be normal if not first boot)"
    fi
    
    # Stop VM
    kill $QEMU_PID 2>/dev/null || true
    wait $QEMU_PID 2>/dev/null || true
    
    if [ "${FAILED:-0}" -eq 1 ]; then
        log_error "Installation verification failed"
        exit 1
    else
        log_info "Installation verification passed"
    fi
}

# Main execution
main() {
    log_info "nOS Installer Test"
    log_info "========================"
    
    # Set up cleanup on exit
    trap cleanup EXIT
    
    # Check requirements
    check_requirements
    
    # Clean any existing resources
    cleanup
    
    # Create disk
    create_disk
    
    # Run test based on mode
    case "${2:-manual}" in
        auto)
            automated_install_test
            verify_installation
            ;;
        verify)
            verify_installation
            ;;
        *)
            run_installer_test
            ;;
    esac
    
    log_info "Test completed"
}

# Run main
main "$@"
