#!/bin/bash
# CI verification script for GRUB theme in ISO
set -e

ISO_PATH="${1:-nos.iso}"
TEMP_DIR=$(mktemp -d)
EXIT_CODE=0

echo "=== nOS GRUB Theme Verification ==="
echo "ISO: $ISO_PATH"
echo ""

# Check if ISO exists
if [ ! -f "$ISO_PATH" ]; then
    echo "ERROR: ISO file not found: $ISO_PATH"
    exit 1
fi

# Extract ISO contents
echo "Extracting ISO for verification..."
7z x -o"$TEMP_DIR" "$ISO_PATH" >/dev/null 2>&1 || {
    echo "ERROR: Failed to extract ISO (is 7z installed?)"
    rm -rf "$TEMP_DIR"
    exit 1
}

# Function to check file existence
check_file() {
    local file="$1"
    local description="$2"
    
    if [ -f "$TEMP_DIR/$file" ]; then
        echo "✓ $description found: $file"
        return 0
    else
        echo "✗ $description missing: $file"
        return 1
    fi
}

# Function to check content in file
check_content() {
    local file="$1"
    local pattern="$2"
    local description="$3"
    
    if [ -f "$TEMP_DIR/$file" ]; then
        if grep -q "$pattern" "$TEMP_DIR/$file" 2>/dev/null; then
            echo "✓ $description in $file"
            return 0
        else
            echo "✗ $description not found in $file"
            return 1
        fi
    else
        echo "✗ File not found for content check: $file"
        return 1
    fi
}

echo "=== Checking GRUB Theme Files ==="

# Check theme configuration
check_file "boot/grub/themes/nithron/theme.txt" "Theme configuration" || EXIT_CODE=1

# Check fonts
check_file "boot/grub/themes/nithron/DroidSans-32.pf2" "DroidSans font" || EXIT_CODE=1
check_file "boot/grub/themes/nithron/DroidSans-Bold-32.pf2" "DroidSans Bold font" || EXIT_CODE=1

# Check background
check_file "boot/grub/themes/nithron/background.png" "Background image" || EXIT_CODE=1

# Check GRUB configurations
echo ""
echo "=== Checking GRUB Configuration ==="

# Check main grub.cfg
check_file "boot/grub/grub.cfg" "Main GRUB config" || EXIT_CODE=1

# Check theme references in grub.cfg
check_content "boot/grub/grub.cfg" "set theme=" "Theme setting" || EXIT_CODE=1
check_content "boot/grub/grub.cfg" "terminal_output gfxterm" "Graphics terminal" || EXIT_CODE=1
check_content "boot/grub/grub.cfg" "nOS" "nOS branding" || EXIT_CODE=1

# Check EFI configuration if present
if [ -d "$TEMP_DIR/EFI" ]; then
    echo ""
    echo "=== Checking EFI Boot ==="
    
    # EFI files are optional - theme can be loaded from /boot partition
    check_file "EFI/boot/grub.cfg" "EFI GRUB config" || {
        echo "  (Optional - theme may be loaded from /boot partition)"
    }
    
    if [ -f "$TEMP_DIR/EFI/boot/grub.cfg" ]; then
        check_content "EFI/boot/grub.cfg" "set theme=" "EFI theme setting" || {
            echo "  (Optional - theme may be loaded from /boot partition)"
        }
    fi
    
    # Check for EFI theme files
    check_file "EFI/boot/grub/themes/nithron/theme.txt" "EFI theme configuration" || {
        echo "  (Optional - theme may be loaded from /boot partition)"
    }
fi

# Check isolinux for BIOS boot
if [ -d "$TEMP_DIR/isolinux" ]; then
    echo ""
    echo "=== Checking BIOS Boot ==="
    
    if [ -f "$TEMP_DIR/isolinux/grub.cfg" ]; then
        # BIOS GRUB config is optional - theme can be loaded from /boot partition
        check_file "isolinux/grub.cfg" "BIOS GRUB config" || {
            echo "  (Optional - theme may be loaded from /boot partition)"
        }
        check_content "isolinux/grub.cfg" "set theme=" "BIOS theme setting" || {
            echo "  (Optional - theme may be loaded from /boot partition)"
        }
    fi
fi

# Check menu entries
echo ""
echo "=== Checking Menu Entries ==="

check_content "boot/grub/grub.cfg" "menuentry.*nOS Live" "nOS Live entry" || EXIT_CODE=1
check_content "boot/grub/grub.cfg" "menuentry.*failsafe\|Safe Graphics" "Failsafe entry" || EXIT_CODE=1

# Summary
echo ""
echo "=== Verification Summary ==="

if [ $EXIT_CODE -eq 0 ]; then
    echo "✅ All GRUB theme checks passed!"
else
    echo "❌ Some GRUB theme checks failed"
    echo "   Please review the errors above"
fi

# Cleanup
rm -rf "$TEMP_DIR"

exit $EXIT_CODE
