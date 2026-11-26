#!/usr/bin/env bash
set -euo pipefail

cat <<'EOF'
nOS Recovery Mode
=======================

Overview
--------
If kernel arg `nos.recovery=1` (or env NOS_RECOVERY=1) is set, nosd exposes
localhost-only recovery APIs under /api/v1/recovery/*.

Endoints (localhost only)
-------------------------
1) Reset password:
   curl -sS -X POST http://127.0.0.1:9000/api/v1/recovery/reset-password \
     -H 'Content-Type: application/json' \
     -d '{"username":"admin","password":"NewStrongPassword123!"}'

2) Disable 2FA:
   curl -sS -X POST http://127.0.0.1:9000/api/v1/recovery/disable-2fa \
     -H 'Content-Type: application/json' \
     -d '{"username":"admin"}'

3) Generate one-time setup OTP (first-boot flow):
   curl -sS -X POST http://127.0.0.1:9000/api/v1/recovery/generate-otp | jq .

Safety Notes
------------
- Recovery mode should be used only with physical console access.
- APIs are bound to localhost and require root/sudo or console access.
- Remove `nos.recovery=1` after completing recovery.
EOF


