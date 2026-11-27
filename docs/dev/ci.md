# CI

One workflow, `.github/workflows/ci.yml`. Every job fails the run when its
check fails; nothing is reported as passed that didn't run.

## Jobs

| Job | What it checks | When |
|-----|----------------|------|
| Lint & Static Analysis | version strings, tidy go.mod, golangci-lint, staticcheck and govulncheck for nosd and nos-agent; the web UI's lint, typecheck and dependency audit | always |
| Go modules (sync-core, nosctl) | build, lint and tests of the sync client and the CLI | always |
| Desktop client | the NithronSync desktop app builds on Linux, Windows and macOS | always |
| Mobile (Android) | the Android app's typecheck, lint, tests and a debug APK | always |
| Contract: FE-BE endpoint check | every API path the web UI calls is served; `docs/api/openapi.json` is current | always |
| Build, Test | nosd, nos-agent and the web UI: builds and unit tests, the NithronOS → nOS migration, shares against real Samba, nosd's systemd sandbox | always |
| E2E: sync, storage, every page | `scripts/live-ui/run.sh` against the real nosd and nos-agent in a container: NithronSync with the real sync daemon (`sync-e2e.sh`), a RAID1 pool created, checked and destroyed on loop devices (`storage-e2e.sh`), and Playwright opening every route of the web UI in the dark and the light theme, with a capture of each; every text must be at AA in both themes (`tests/e2e/ui/tests/live-walk.spec.ts`, `test-results/contrast-report.json`) | always |
| Build Documentation | `mkdocs build --strict` of `docs-site/` | always |
| Build Debian packages, Build ISO | the packages and the installer ISO | main, manual runs, releases, and pull requests that change the system (below) |
| ISO install test (bios, uefi) | `tests/smoke/iso-install-test.sh`: installs the ISO in QEMU, boots it, checks the setup code, nos-agent, WebDAV and that no unit failed; then `tests/upgrade/upgrade-test.sh` updates nosd through the API from a repository with a newer version and rolls the update back | same as the ISO |
| SBOMs and checksums, v1 Release Gate, Upload Release Assets | releases only; the gate needs every job above | tags |

"Changes the system" (the `Changed areas` job): a pull request touching
`packaging/`, `deploy/`, `installer/`, `tests/smoke/`, `tests/upgrade/` or the
workflow. Other pull requests skip the packages, the ISO and its install
test; main runs them on every push.

## Running the checks locally

```bash
# Unit tests
(cd backend/nosd && go test ./...)
(cd agent/nos-agent && go test ./...)
(cd web && npm run test:run)

# The live passes (docker, Go, Node; privileged containers for loop devices)
scripts/live-ui/run.sh                 # sync, storage and the page walk
ONLY=storage scripts/live-ui/run.sh    # one pass: sync or storage
KEEP=1 scripts/live-ui/run.sh          # leave it running; --stop removes it

# The install test, with update and rollback (KVM, QEMU; an ISO and the
# .debs from a CI run's artifacts)
tests/upgrade/next-version-repo.sh <debs folder> /tmp/repo    # needs dpkg-deb, apt-ftparchive
UPGRADE_REPO=/tmp/repo tests/smoke/iso-install-test.sh <iso> bios
```

## Tests and authentication

nosd's handler tests build the router without the sign-in, admin and CSRF
checks (`skipAuthChecks`, set in `TestMain`); tests of those checks call
`withRealAuth`. The variable is only reachable from the package's tests: no
environment variable or setting turns authentication off in a real nosd.

## Artifacts

Each run keeps the packages, the ISO, the web UI, the desktop builds, the
Android APK, the live passes' screenshots and report, and the install test's
serial logs.
