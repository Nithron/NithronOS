# nOS v1.0 Release Checklist

This checklist must be completed before the v1.0 release can be published.

## Automated Checks ✅

These are verified automatically by CI:

- [x] All unit tests passing (Go + Web)
- [x] All integration tests passing
- [x] All E2E tests passing (Playwright)
- [x] ISO smoke test passing (< 90s boot)
- [x] Storage E2E tests passing
- [x] Upgrade tests passing (N-1 → N)
- [x] Static analysis clean (golangci-lint, ESLint)
- [x] No security vulnerabilities (dependabot)
- [x] Code coverage > 70%
- [x] Documentation builds without errors
- [x] All packages signed with GPG
- [x] SBOMs generated for all artifacts
- [x] Checksums generated (SHA256)

## Performance Targets ✅

Verified by CI metrics:

- [x] Boot time < 90 seconds
- [x] ISO size < 2 GB
- [x] Idle memory usage < 512 MB
- [x] API p95 response time < 100ms
- [x] Web UI initial load < 3 seconds
- [x] Dashboard refresh < 1 second

## Manual Verification Required 🔍

These items must be manually verified before release:

### UI/UX Polish

- [ ] Favicon appears on all pages
- [ ] Logo displays correctly (light/dark mode)
- [ ] 404 page is styled and helpful
- [ ] About page shows correct version
- [ ] No placeholder text visible anywhere
- [ ] All forms have proper validation
- [ ] Error messages are user-friendly
- [ ] Loading states are smooth
- [ ] Animations are performant
- [ ] Mobile responsive (tablet minimum)

### Functionality

- [ ] First-boot flow works end-to-end
- [ ] OTP displays clearly on console
- [ ] Setup wizard completes successfully
- [ ] All navigation links work
- [ ] Search functionality works
- [ ] Filters and sorting work
- [ ] Data export works (CSV/JSON)
- [ ] File uploads work correctly
- [ ] WebSocket connections stable
- [ ] SSE events stream properly

### Security

- [ ] Default credentials documented
- [ ] No debug endpoints exposed
- [ ] HTTPS redirect works
- [ ] CSP headers present
- [ ] Rate limiting active
- [ ] Session timeout works
- [ ] 2FA enrollment works
- [ ] Password policies enforced
- [ ] Audit logs capture all events
- [ ] No sensitive data in logs

### Storage

- [ ] Pool creation works
- [ ] Snapshot creation/deletion works
- [ ] Scrub can be initiated
- [ ] SMART data displays
- [ ] Disk health alerts work
- [ ] Space usage accurate
- [ ] Mount options correct
- [ ] Quotas can be set

### Applications

- [ ] App catalog loads
- [ ] Sample apps install correctly
- [ ] App lifecycle works (start/stop/restart)
- [ ] App logs stream
- [ ] App updates work
- [ ] App data persists
- [ ] App backups work
- [ ] Reverse proxy works

### Backup & Recovery

- [ ] Scheduled backups run
- [ ] Manual backups work
- [ ] Retention policy applies
- [ ] Replication works (SSH)
- [ ] Restore wizard works
- [ ] File-level restore works
- [ ] Cloud sync works (if configured)

### Monitoring

- [ ] Metrics collect properly
- [ ] Charts render correctly
- [ ] Alerts fire when expected
- [ ] Notifications deliver
- [ ] Historical data retained
- [ ] Export works

### Networking

- [ ] Firewall rules apply
- [ ] WireGuard setup works
- [ ] Let's Encrypt works
- [ ] Port forwarding works
- [ ] DNS resolution works

### Documentation

- [ ] Installation guide complete
- [ ] User guide complete
- [ ] Admin guide complete
- [ ] API reference complete
- [ ] CLI reference complete
- [ ] Troubleshooting guide complete
- [ ] All screenshots current
- [ ] All examples work
- [ ] No broken links
- [ ] Search works

### Packaging

- [ ] Debian packages install cleanly
- [ ] Dependencies resolved
- [ ] Services start automatically
- [ ] Uninstall removes cleanly
- [ ] Upgrade path works
- [ ] ISO boots on various hardware
- [ ] ISO installer works

### Accessibility

- [ ] Keyboard navigation works
- [ ] Screen reader compatible
- [ ] Color contrast sufficient
- [ ] Focus indicators visible
- [ ] Alt text on images

## Release Artifacts ✅

Ensure all artifacts are present:

- [x] `nos-1.0.0-amd64.iso`
- [x] `nos-1.0.0-amd64.iso.sig`
- [x] `nos-1.0.0-amd64.iso.sha256`
- [x] `nosd_1.0.0_amd64.deb`
- [x] `nos-agent_1.0.0_amd64.deb`
- [x] `nos-web_1.0.0_amd64.deb`
- [x] `nosctl_1.0.0_linux_amd64.tar.gz`
- [x] `sbom-packages.spdx.json`
- [x] `sbom-iso.spdx.json`
- [x] Release notes
- [x] Changelog

## Marketing & Communication

- [ ] Website updated
- [ ] Blog post drafted
- [ ] Social media posts prepared
- [ ] Email announcement ready
- [ ] Demo video recorded
- [ ] Screenshots updated

## Legal & Compliance

- [ ] License files present
- [ ] Copyright headers correct
- [ ] Third-party licenses documented
- [ ] Export compliance checked
- [ ] Privacy policy updated
- [ ] Terms of service updated

## Post-Release

- [ ] Docker images pushed
- [ ] Homebrew formula updated
- [ ] AUR package updated
- [ ] Documentation site live
- [ ] Demo environment updated
- [ ] Support channels monitored

---

## Sign-off

Release v1.0 is approved by:

- [ ] Project Lead: _________________ Date: _______
- [ ] Tech Lead: ___________________ Date: _______
- [ ] QA Lead: _____________________ Date: _______
- [ ] Security Lead: ________________ Date: _______

## Notes

Add any additional notes or concerns here:

```
[Notes section - to be filled in during release]
```

---

**DO NOT RELEASE** until all automated checks pass and all manual items are verified!
