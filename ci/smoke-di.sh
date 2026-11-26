#!/bin/bash
# CI Smoke Test for nOS Debian Installer
# Tests that the installer boots and reaches the language selection menu

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ISO_PATH="${1:-$REPO_ROOT/dist/iso/nOS*.iso}"

# Find the ISO file
ISO_FILE=""
for iso in $ISO_PATH; do
    if [ -f "$iso" ]; then
        ISO_FILE="$iso"
        break
    fi
done

if [ ! -f "$ISO_FILE" ]; then
    echo "ERROR: ISO file not found at $ISO_PATH"
    exit 1
fi

echo "Testing ISO: $ISO_FILE"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# Check for QEMU
if ! command -v qemu-system-x86_64 >/dev/null 2>&1; then
    log_error "qemu-system-x86_64 not found. Please install QEMU."
    exit 1
fi

# Test function
test_boot() {
    local boot_type="$1"
    local extra_args="$2"
    local test_name="$3"
    local timeout="${4:-90}"
    
    log_info "Testing $test_name boot..."
    
    # Create temp file for serial output
    local serial_output=$(mktemp)
    
    # Start QEMU in background
    (
        timeout $timeout qemu-system-x86_64 \
            -M q35 \
            -m 2048 \
            -cdrom "$ISO_FILE" \
            -serial file:"$serial_output" \
            -display none \
            -boot d \
            $extra_args \
            >/dev/null 2>&1
    ) &
    
    local qemu_pid=$!
    
    # Wait for installer to appear in serial output
    local elapsed=0
    local found=false
    
    while [ $elapsed -lt $timeout ]; do
        if grep -q -E "(Debian installer|Choose a language|Select a language|debian-installer)" "$serial_output" 2>/dev/null; then
            found=true
            break
        fi
        
        # Also check for kernel boot messages
        if grep -q -E "(Linux version|Booting kernel|initrd.*loading)" "$serial_output" 2>/dev/null; then
            log_info "  Kernel is booting..."
        fi
        
        sleep 2
        elapsed=$((elapsed + 2))
        
        # Show progress
        if [ $((elapsed % 10)) -eq 0 ]; then
            log_info "  Waiting for installer... (${elapsed}s/${timeout}s)"
        fi
    done
    
    # Kill QEMU
    kill $qemu_pid 2>/dev/null || true
    wait $qemu_pid 2>/dev/null || true
    
    # Check result
    if [ "$found" = true ]; then
        log_info "✓ $test_name boot test PASSED - Installer detected"
        
        # Show relevant lines from serial output
        log_info "  Installer output:"
        grep -E "(Debian installer|Choose a language|Select a language|debian-installer)" "$serial_output" | head -5 | while read -r line; do
            echo "    $line"
        done
        
        rm -f "$serial_output"
        return 0
    else
        log_error "✗ $test_name boot test FAILED - Installer not detected within ${timeout}s"
        
        # Show last lines of serial output for debugging
        log_error "  Last output:"
        tail -20 "$serial_output" | while read -r line; do
            echo "    $line"
        done
        
        rm -f "$serial_output"
        return 1
    fi
}

# Test BIOS boot
log_info "=== Testing BIOS Boot ==="
if test_boot "bios" "" "BIOS" 90; then
    bios_result="PASS"
else
    bios_result="FAIL"
fi

# Test UEFI boot (if OVMF firmware is available)
OVMF_CODE="/usr/share/OVMF/OVMF_CODE.fd"
OVMF_VARS="/usr/share/OVMF/OVMF_VARS.fd"

# Alternative paths for OVMF
for path in \
    "/usr/share/qemu/OVMF.fd" \
    "/usr/share/ovmf/OVMF.fd" \
    "/usr/share/edk2-ovmf/x64/OVMF_CODE.fd" \
    "/usr/share/edk2/ovmf/OVMF_CODE.fd"
do
    if [ -f "$path" ]; then
        OVMF_CODE="$path"
        break
    fi
done

if [ -f "$OVMF_CODE" ]; then
    log_info "=== Testing UEFI Boot ==="
    
    # Create temp NVRAM
    NVRAM_TEMP=$(mktemp)
    
    # Find OVMF_VARS or create empty NVRAM
    if [ -f "$OVMF_VARS" ]; then
        cp "$OVMF_VARS" "$NVRAM_TEMP"
    else
        # Create empty 256K NVRAM file
        dd if=/dev/zero of="$NVRAM_TEMP" bs=1k count=256 2>/dev/null
    fi
    
    if test_boot "uefi" "-bios $OVMF_CODE -pflash $NVRAM_TEMP" "UEFI" 120; then
        uefi_result="PASS"
    else
        uefi_result="FAIL"
    fi
    
    rm -f "$NVRAM_TEMP"
else
    log_warn "OVMF firmware not found, skipping UEFI test"
    uefi_result="SKIP"
fi

# Summary
echo ""
log_info "=== Test Summary ==="
echo "  BIOS Boot: $bios_result"
echo "  UEFI Boot: $uefi_result"

# Exit with error if any test failed
if [ "$bios_result" = "FAIL" ] || [ "$uefi_result" = "FAIL" ]; then
    log_error "Some tests failed!"
    exit 1
fi

log_info "All tests passed!"
exit 0
