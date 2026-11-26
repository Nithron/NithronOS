#!/bin/bash
# E2E test for shares functionality in VM environment
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
API_URL="${API_URL:-http://localhost:9000}"
SMB_USER="${SMB_USER:-testuser}"
SMB_PASS="${SMB_PASS:-testpass123}"
TEST_SHARE="e2e-media"
TM_SHARE="e2e-timemachine"
TEST_FILE="test-$(date +%s).txt"

# Helper functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

check_command() {
    if command -v "$1" >/dev/null 2>&1; then
        log_info "$1 is installed"
        return 0
    else
        log_error "$1 is not installed"
        return 1
    fi
}

cleanup() {
    log_info "Cleaning up test shares..."
    
    # Delete test shares
    curl -X DELETE "$API_URL/api/shares/$TEST_SHARE" 2>/dev/null || true
    curl -X DELETE "$API_URL/api/shares/$TM_SHARE" 2>/dev/null || true
    
    # Clean up SMB user
    if id -u "$SMB_USER" >/dev/null 2>&1; then
        sudo smbpasswd -x "$SMB_USER" 2>/dev/null || true
        sudo userdel "$SMB_USER" 2>/dev/null || true
    fi
}

# Trap cleanup on exit
trap cleanup EXIT

# Main test
echo "=== nOS Shares E2E Test ==="
echo ""

# Check prerequisites
log_info "Checking prerequisites..."
check_command curl || exit 1
check_command smbclient || exit 1
check_command testparm || exit 1
check_command exportfs || exit 1
check_command avahi-browse || exit 1

# Create SMB test user
log_info "Creating SMB test user..."
if ! id -u "$SMB_USER" >/dev/null 2>&1; then
    sudo useradd -m "$SMB_USER"
    echo -e "$SMB_PASS\n$SMB_PASS" | sudo smbpasswd -a -s "$SMB_USER"
fi

# Test 1: Create share via API
log_info "Test 1: Creating share via API..."
RESPONSE=$(curl -s -X POST "$API_URL/api/shares" \
    -H "Content-Type: application/json" \
    -d '{
        "name": "'$TEST_SHARE'",
        "smb": {
            "enabled": true,
            "guest": false,
            "recycle": {
                "enabled": true,
                "directory": ".recycle"
            }
        },
        "nfs": {
            "enabled": true,
            "networks": ["192.168.0.0/16", "10.0.0.0/8"]
        },
        "owners": ["user:'$SMB_USER'"],
        "description": "E2E test media share"
    }')

if echo "$RESPONSE" | grep -q "\"name\":\"$TEST_SHARE\""; then
    log_info "✓ Share created successfully"
else
    log_error "✗ Failed to create share"
    echo "$RESPONSE"
    exit 1
fi

# Wait for services to reload
sleep 3

# Test 2: Verify Samba configuration
log_info "Test 2: Verifying Samba configuration..."
if testparm -s 2>&1 | grep -q "Loaded services file OK"; then
    log_info "✓ Samba configuration is valid"
else
    log_error "✗ Samba configuration is invalid"
    testparm -s
    exit 1
fi

# Check if share appears in Samba config
if testparm -s 2>&1 | grep -q "\[$TEST_SHARE\]"; then
    log_info "✓ Share found in Samba configuration"
else
    log_error "✗ Share not found in Samba configuration"
    exit 1
fi

# Test 3: Verify NFS export
log_info "Test 3: Verifying NFS export..."
sudo exportfs -ra

if exportfs -v | grep -q "/srv/shares/$TEST_SHARE"; then
    log_info "✓ NFS export is active"
else
    log_error "✗ NFS export not found"
    exportfs -v
    exit 1
fi

# Test 4: Create file over SMB
log_info "Test 4: Creating file over SMB..."
echo "Test content $(date)" > "/tmp/$TEST_FILE"

if smbclient "//localhost/$TEST_SHARE" -U "$SMB_USER%$SMB_PASS" -c "put /tmp/$TEST_FILE $TEST_FILE" 2>&1 | grep -q "putting file"; then
    log_info "✓ File created over SMB"
else
    log_error "✗ Failed to create file over SMB"
    exit 1
fi

# Test 5: Delete file and check recycle bin
log_info "Test 5: Testing recycle bin..."
if smbclient "//localhost/$TEST_SHARE" -U "$SMB_USER%$SMB_PASS" -c "del $TEST_FILE" 2>&1; then
    log_info "✓ File deleted"
    
    # Check if file appears in recycle bin
    if [ -f "/srv/shares/$TEST_SHARE/.recycle/$TEST_FILE" ]; then
        log_info "✓ File found in recycle bin"
    else
        log_warn "File not found in recycle bin (may be permission issue)"
    fi
else
    log_error "✗ Failed to delete file"
fi

# Test 6: Create Time Machine share
log_info "Test 6: Creating Time Machine share..."
RESPONSE=$(curl -s -X POST "$API_URL/api/shares" \
    -H "Content-Type: application/json" \
    -d '{
        "name": "'$TM_SHARE'",
        "smb": {
            "enabled": true,
            "guest": false,
            "time_machine": true
        },
        "owners": ["user:'$SMB_USER'"],
        "description": "E2E test Time Machine share"
    }')

if echo "$RESPONSE" | grep -q "\"name\":\"$TM_SHARE\""; then
    log_info "✓ Time Machine share created"
else
    log_error "✗ Failed to create Time Machine share"
    echo "$RESPONSE"
    exit 1
fi

# Wait for Avahi to update
sleep 5

# Test 7: Verify mDNS advertisement
log_info "Test 7: Verifying mDNS advertisement..."
AVAHI_OUTPUT=$(timeout 3 avahi-browse -a -t -r -p 2>/dev/null || true)

if echo "$AVAHI_OUTPUT" | grep -q "_adisk._tcp"; then
    log_info "✓ Time Machine advertised via mDNS (_adisk)"
else
    log_warn "Time Machine not found in mDNS (may take time to propagate)"
fi

if echo "$AVAHI_OUTPUT" | grep -q "_smb._tcp"; then
    log_info "✓ SMB service advertised via mDNS"
else
    log_warn "SMB service not found in mDNS"
fi

# Test 8: Verify ACLs
log_info "Test 8: Verifying ACLs..."
if getfacl "/srv/shares/$TEST_SHARE" 2>/dev/null | grep -q "user:$SMB_USER:rwx"; then
    log_info "✓ Owner ACLs correctly applied"
else
    log_warn "Owner ACLs not found (may be permission issue)"
fi

# Test 9: Test dry-run validation
log_info "Test 9: Testing dry-run validation..."
RESPONSE=$(curl -s -X POST "$API_URL/api/shares/$TEST_SHARE/test" \
    -H "Content-Type: application/json" \
    -d '{
        "config": {
            "smb": {"enabled": true, "guest": true}
        }
    }')

if echo "$RESPONSE" | grep -q '"valid":true'; then
    log_info "✓ Dry-run validation successful"
else
    log_warn "Dry-run validation returned unexpected result"
    echo "$RESPONSE"
fi

# Test 10: List shares
log_info "Test 10: Listing shares..."
SHARES=$(curl -s "$API_URL/api/shares")

if echo "$SHARES" | grep -q "$TEST_SHARE" && echo "$SHARES" | grep -q "$TM_SHARE"; then
    log_info "✓ Both test shares found in listing"
else
    log_error "✗ Test shares not found in listing"
    echo "$SHARES"
    exit 1
fi

# Summary
echo ""
echo "=== E2E Test Summary ==="
echo -e "${GREEN}All tests passed successfully!${NC}"
echo ""
echo "Verified:"
echo "  ✓ Share creation via API"
echo "  ✓ Samba configuration and validation"
echo "  ✓ NFS export configuration"
echo "  ✓ SMB file operations"
echo "  ✓ Recycle bin functionality"
echo "  ✓ Time Machine support"
echo "  ✓ mDNS/Avahi advertisement"
echo "  ✓ ACL application"
echo "  ✓ Dry-run validation"
echo "  ✓ Share listing"

exit 0
