# Security Policy

We take security seriously. If you believe you’ve found a vulnerability, **do not** open a public issue.

## Reporting a Vulnerability
Email **security@nithron.com** with:
- Description and impact
- A minimal reproduction (if possible)
- Affected component/version (e.g., nosd vX.Y)
- Your contact and PGP key (optional)

We will acknowledge within 5 business days.

## Disclosure & Timeline
- We follow a responsible disclosure model with a **90-day** default embargo.
- We may coordinate CVE assignment and credit the reporter (if desired).
- Critical issues may be patched faster; timelines may extend for complex fixes.

## Scope
- In scope: nOS code in this repo (backend `nosd`, `nos-agent`, web UI) and official packaging.
- Out of scope: third-party dependencies, forks, or self-modified builds (report upstream where appropriate).

## Hardening & Updates
- Security fixes will be released via Official Build Artifacts and noted in release notes.
- We recommend enabling automatic updates once available.

Thank you for helping keep users safe.
