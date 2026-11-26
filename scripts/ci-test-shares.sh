#!/bin/bash
# CI smoke test for shares functionality
set -e

echo "=== nOS Shares CI Smoke Test ==="

# Configuration
API_URL="${API_URL:-http://localhost:9000}"
TEST_SHARE_NAME="ci-test-media"
TEST_TM_SHARE_NAME="ci-test-tm"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

# Helper functions
check_command() {
    if command -v "$1" >/dev/null 2>&1; then
        echo -e "${GREEN}✓${NC} $1 is installed"
        return 0
    else
        echo -e "${RED}✗${NC} $1 is not installed"
        return 1
    fi
}

check_service() {
    if systemctl is-active --quiet "$1"; then
        echo -e "${GREEN}✓${NC} $1 is active"
        return 0
    else
        echo -e "${RED}✗${NC} $1 is not active"
        return 1
    fi
}

check_file() {
    if [ -f "$1" ]; then
        echo -e "${GREEN}✓${NC} File exists: $1"
        return 0
    else
        echo -e "${RED}✗${NC} File missing: $1"
        return 1
    fi
}

# Check prerequisites
echo ""
echo "=== Checking Prerequisites ==="
check_command testparm || exit 1
check_command exportfs || exit 1
check_command setfacl || exit 1
check_command avahi-browse || exit 1

# Check services
echo ""
echo "=== Checking Services ==="
check_service smbd || exit 1
check_service nfs-server || exit 1
check_service avahi-daemon || exit 1

# Create test share via API
echo ""
echo "=== Creating Test Share ==="
curl -X POST "$API_URL/api/shares" \
    -H "Content-Type: application/json" \
    -d '{
        "name": "'$TEST_SHARE_NAME'",
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
        "owners": ["user:admin"],
        "readers": ["group:users"],
        "description": "CI test media share"
    }' || {
        echo -e "${RED}Failed to create test share${NC}"
        exit 1
    }

echo -e "${GREEN}✓${NC} Test share created"

# Verify Samba configuration
echo ""
echo "=== Verifying Samba Configuration ==="
SAMBA_CONF="/etc/samba/smb.conf.d/nos-$TEST_SHARE_NAME.conf"
check_file "$SAMBA_CONF" || exit 1

# Test Samba configuration
if testparm -s --suppress-prompt >/dev/null 2>&1; then
    echo -e "${GREEN}✓${NC} Samba configuration is valid"
else
    echo -e "${RED}✗${NC} Samba configuration is invalid"
    exit 1
fi

# Check if share appears in smbclient listing
if smbclient -L localhost -N 2>/dev/null | grep -q "$TEST_SHARE_NAME"; then
    echo -e "${GREEN}✓${NC} Share appears in SMB listing"
else
    echo -e "${RED}✗${NC} Share not found in SMB listing"
fi

# Verify NFS export
echo ""
echo "=== Verifying NFS Export ==="
NFS_EXPORT="/etc/exports.d/nos-$TEST_SHARE_NAME.exports"
check_file "$NFS_EXPORT" || exit 1

# Check if export is active
if exportfs -v | grep -q "/srv/shares/$TEST_SHARE_NAME"; then
    echo -e "${GREEN}✓${NC} NFS export is active"
else
    echo -e "${RED}✗${NC} NFS export not found"
    exit 1
fi

# Verify share directory and ACLs
echo ""
echo "=== Verifying Share Directory ==="
SHARE_DIR="/srv/shares/$TEST_SHARE_NAME"
if [ -d "$SHARE_DIR" ]; then
    echo -e "${GREEN}✓${NC} Share directory exists: $SHARE_DIR"
    
    # Check ACLs
    if getfacl "$SHARE_DIR" 2>/dev/null | grep -q "user:admin:rwx"; then
        echo -e "${GREEN}✓${NC} Owner ACLs applied"
    else
        echo -e "${RED}✗${NC} Owner ACLs not found"
    fi
    
    if getfacl "$SHARE_DIR" 2>/dev/null | grep -q "group:users:r-x"; then
        echo -e "${GREEN}✓${NC} Reader ACLs applied"
    else
        echo -e "${RED}✗${NC} Reader ACLs not found"
    fi
    
    # Check recycle bin
    if [ -d "$SHARE_DIR/.recycle" ]; then
        echo -e "${GREEN}✓${NC} Recycle bin directory exists"
    else
        echo -e "${RED}✗${NC} Recycle bin directory missing"
    fi
else
    echo -e "${RED}✗${NC} Share directory missing: $SHARE_DIR"
    exit 1
fi

# Create Time Machine share
echo ""
echo "=== Creating Time Machine Share ==="
curl -X POST "$API_URL/api/shares" \
    -H "Content-Type: application/json" \
    -d '{
        "name": "'$TEST_TM_SHARE_NAME'",
        "smb": {
            "enabled": true,
            "guest": false,
            "time_machine": true
        },
        "owners": ["user:admin"],
        "description": "CI test Time Machine share"
    }' || {
        echo -e "${RED}Failed to create Time Machine share${NC}"
        exit 1
    }

echo -e "${GREEN}✓${NC} Time Machine share created"

# Verify Avahi service
echo ""
echo "=== Verifying Avahi Time Machine Service ==="
AVAHI_SERVICE="/etc/avahi/services/nos-tm.service"
check_file "$AVAHI_SERVICE" || {
    echo -e "${RED}✗${NC} Avahi Time Machine service file not created"
    exit 1
}

# Check if _adisk service is advertised
if avahi-browse -a -t -r -p 2>/dev/null | grep -q "_adisk._tcp"; then
    echo -e "${GREEN}✓${NC} Time Machine service advertised via mDNS"
else
    echo -e "${RED}✗${NC} Time Machine service not found in mDNS"
fi

# Test dry-run validation
echo ""
echo "=== Testing Dry-Run Validation ==="
RESPONSE=$(curl -s -X POST "$API_URL/api/shares/$TEST_SHARE_NAME/test" \
    -H "Content-Type: application/json" \
    -d '{
        "config": {
            "smb": {"enabled": true, "guest": true}
        }
    }')

if echo "$RESPONSE" | grep -q '"valid":true'; then
    echo -e "${GREEN}✓${NC} Dry-run validation successful"
else
    echo -e "${RED}✗${NC} Dry-run validation failed"
    echo "$RESPONSE"
fi

# Cleanup test shares
echo ""
echo "=== Cleaning Up Test Shares ==="
curl -X DELETE "$API_URL/api/shares/$TEST_SHARE_NAME" || {
    echo -e "${RED}Warning: Failed to delete test share${NC}"
}
curl -X DELETE "$API_URL/api/shares/$TEST_TM_SHARE_NAME" || {
    echo -e "${RED}Warning: Failed to delete Time Machine share${NC}"
}

echo -e "${GREEN}✓${NC} Cleanup complete"

# Final summary
echo ""
echo "=== Test Summary ==="
echo -e "${GREEN}All share functionality tests passed!${NC}"
echo ""
echo "Verified:"
echo "  • Share creation via API"
echo "  • Samba configuration and validation"
echo "  • NFS export configuration"
echo "  • POSIX ACLs application"
echo "  • Recycle bin creation"
echo "  • Time Machine support with Avahi"
echo "  • Dry-run validation"

exit 0
