#!/bin/sh
# Runs migrate-from-nithronos.sh against a fake root laid out like a system
# installed as NithronOS, twice, and checks the result.
# The checks are passed as strings and evaluated later, hence single quotes.
# shellcheck disable=SC2016,SC2034
set -eu

here="$(cd "$(dirname "$0")" && pwd)"
R="$(mktemp -d)"
trap 'rm -rf "$R"' EXIT
fail=0
ok() { echo "ok   $*"; }
bad() { echo "FAIL $*"; fail=1; }
check() { if eval "$2"; then ok "$1"; else bad "$1"; fi; }

mk() { mkdir -p "$(dirname "$R$1")"; printf '%s\n' "$2" >"$R$1"; }

# Old layout.
mk /etc/nithronos/tls/cert.pem CERT
mk /etc/nithronos/tls/key.pem KEY
mk /etc/nithronos/firewall.nft 'table inet filter {}'
mk /etc/nithronos/update/channel beta
mk /etc/nos/secret.key SECRET
mk /var/lib/nithronos/deps-reported ''
mk /etc/nftables.d/nithronos.nft 'table inet nos {}'
mk /etc/caddy/Caddyfile ':443 {
  tls /etc/nithronos/tls/cert.pem /etc/nithronos/tls/key.pem
  root * /usr/share/nithronos/web
}'
mk /etc/systemd/system/nithronos-ensure-services.service '[Service]
ExecStart=/usr/local/bin/nithronos-ensure-services.sh'
mk /usr/local/bin/nithronos-ensure-services.sh '#!/bin/sh
systemctl start nithronos-certgen.service'
mk /etc/systemd/system/nithronos-certgen.service '[Service]
ExecStart=/usr/lib/nithronos/gen-local-cert.sh'
mk /lib/systemd/system/nos-certgen.service '[Service]
ExecStart=/usr/lib/nos/gen-local-cert.sh'
mk /lib/systemd/system/nos-deps-report.service '[Service]'
mk /etc/systemd/system/nos-tls-gen.service '[Service]
ExecStart=/usr/lib/nithronos/nos-tls-gen.sh'
mk /usr/lib/nithronos/nos-tls-gen.sh 'TLS_DIR=/etc/nithronos/tls'
mkdir -p "$R/etc/systemd/system/multi-user.target.wants"
ln -s /etc/systemd/system/nithronos-ensure-services.service "$R/etc/systemd/system/multi-user.target.wants/nithronos-ensure-services.service"
ln -s /etc/systemd/system/nithronos-certgen.service "$R/etc/systemd/system/multi-user.target.wants/nithronos-certgen.service"
ln -s /lib/systemd/system/nithronos-deps-report.service "$R/etc/systemd/system/multi-user.target.wants/nithronos-deps-report.service"
ln -s /lib/systemd/system/nithronos-gone.service "$R/etc/systemd/system/multi-user.target.wants/nithronos-gone.service"
mk /etc/os-release 'PRETTY_NAME="NithronOS 0.9.5-pre-alpha"
NAME="NithronOS"
ID=nithronos
ID_LIKE=debian
HOME_URL="https://github.com/Nithron/NithronOS"'
mk /etc/issue '  NithronOS 0.9.5  -  \n  (\l)'
mk /etc/apt/sources.list.d/nithronos.list 'deb [signed-by=/usr/share/keyrings/nithronos-archive-keyring.gpg] https://example.invalid stable main'
mk /usr/share/keyrings/nithronos-archive-keyring.gpg KEYRING
mkdir -p "$R/etc/issue.d"
ln -s /run/nos/firstboot.issue "$R/etc/issue.d/50-nithronos-setup.issue"

NOS_ROOT="$R" sh "$here/migrate-from-nithronos.sh" >/dev/null
NOS_ROOT="$R" sh "$here/migrate-from-nithronos.sh" >"$R.second" 2>&1 || bad "second run failed"

w="$R/etc/systemd/system/multi-user.target.wants"
check "tls moved to /etc/ssl/nos" '[ "$(cat "$R/etc/ssl/nos/cert.pem")" = CERT ] && [ "$(cat "$R/etc/ssl/nos/key.pem")" = KEY ]'
check "config merged into /etc/nos" '[ -f "$R/etc/nos/firewall.nft" ] && [ "$(cat "$R/etc/nos/update/channel")" = beta ] && [ "$(cat "$R/etc/nos/secret.key")" = SECRET ]'
check "no tls under /etc/nos" '[ ! -e "$R/etc/nos/tls" ]'
check "/etc/nithronos removed" '[ ! -e "$R/etc/nithronos" ]'
check "state moved to /var/lib/nos" '[ -f "$R/var/lib/nos/deps-reported" ] && [ ! -e "$R/var/lib/nithronos" ]'
check "nftables file renamed" '[ -f "$R/etc/nftables.d/nos.nft" ] && [ ! -e "$R/etc/nftables.d/nithronos.nft" ]'
check "Caddyfile paths" 'grep -q "tls /etc/ssl/nos/cert.pem /etc/ssl/nos/key.pem" "$R/etc/caddy/Caddyfile" && grep -q "/usr/share/nos/web" "$R/etc/caddy/Caddyfile"'
check "ISO unit renamed and rewritten" 'grep -q "ExecStart=/usr/local/bin/nos-ensure-services.sh" "$R/etc/systemd/system/nos-ensure-services.service" && [ ! -e "$R/etc/systemd/system/nithronos-ensure-services.service" ]'
check "ISO script renamed and rewritten" 'grep -q "nos-certgen.service" "$R/usr/local/bin/nos-ensure-services.sh" && [ ! -e "$R/usr/local/bin/nithronos-ensure-services.sh" ]'
check "packaged replacement wins" '[ ! -e "$R/etc/systemd/system/nithronos-certgen.service" ] && [ ! -e "$R/etc/systemd/system/nos-certgen.service" ]'
check "enablement follows renamed ISO unit" '[ "$(readlink "$w/nos-ensure-services.service")" = /etc/systemd/system/nos-ensure-services.service ]'
check "enablement follows packaged unit" '[ "$(readlink "$w/nos-certgen.service")" = /lib/systemd/system/nos-certgen.service ] && [ "$(readlink "$w/nos-deps-report.service")" = /lib/systemd/system/nos-deps-report.service ]'
check "old enablement links removed" '! ls "$w" | grep -q nithronos'
check "unowned helper moved and rewritten" 'grep -q "TLS_DIR=/etc/ssl/nos" "$R/usr/lib/nos/nos-tls-gen.sh" && [ ! -e "$R/usr/lib/nithronos" ]'
check "unit paths rewritten" 'grep -q "ExecStart=/usr/lib/nos/nos-tls-gen.sh" "$R/etc/systemd/system/nos-tls-gen.service"'
check "os-release" 'grep -qx "ID=nos" "$R/etc/os-release" && grep -qx "NAME=\"nOS\"" "$R/etc/os-release" && grep -q "github.com/Nithron/nOS\"" "$R/etc/os-release"'
check "issue" 'grep -q "nOS 0.9.5" "$R/etc/issue"'
check "apt source and keyring" 'grep -q "signed-by=/usr/share/keyrings/nos-archive-keyring.gpg" "$R/etc/apt/sources.list.d/nos.list" && [ -f "$R/usr/share/keyrings/nos-archive-keyring.gpg" ] && [ ! -e "$R/etc/apt/sources.list.d/nithronos.list" ]'
check "old issue link removed" '[ ! -L "$R/etc/issue.d/50-nithronos-setup.issue" ]'
check "second run changes nothing" '[ ! -s "$R.second" ]'
check "nothing old left" '! (cd "$R" && find . -name "*nithronos*" | grep -q .)'
rm -f "$R.second"

# Both old and new present: the new one wins, the old one stays.
R2="$(mktemp -d)"
trap 'rm -rf "$R" "$R2"' EXIT
mkdir -p "$R2/etc/nithronos/tls" "$R2/etc/ssl/nos"
echo OLD >"$R2/etc/nithronos/tls/cert.pem"
echo NEW >"$R2/etc/ssl/nos/cert.pem"
NOS_ROOT="$R2" sh "$here/migrate-from-nithronos.sh" >/dev/null
check "existing new cert kept" '[ "$(cat "$R2/etc/ssl/nos/cert.pem")" = NEW ]'
check "old cert left in place, not moved into /etc/nos" '[ "$(cat "$R2/etc/nithronos/tls/cert.pem")" = OLD ] && [ ! -e "$R2/etc/nos/tls" ]'

exit $fail
