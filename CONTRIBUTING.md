# Contributing to nOS

Thanks for your interest in contributing! We welcome issues, docs, code, tests, and ideas.

## Ground Rules
- Be respectful and constructive.
- Security-impacting changes: follow `SECURITY.md`.
- All contributions are made under the **Nithron Community License (NCL)** and its contribution clause (see `LICENSE`).

## How to Contribute
1. **Discuss**: Open an issue for bugs/features before large changes.
2. **Fork & Branch**: `feat/<short>`, `fix/<short>`, or `docs/<short>`.
3. **Conventional Commits**: `feat:`, `fix:`, `docs:`, `chore:`, `refactor:`, `test:`.
4. **Format & Lint**  
   - Go: `go fmt ./...` / `go vet ./...`  
   - Web: `npm run lint` (ESLint)  
5. **Tests**: Add unit/integration tests where applicable.
6. **PR**: Link the issue, describe motivation, risks, and testing.

## Developer Certificate of Origin (DCO)
By contributing, you certify the DCO:
Signed-off-by: Your Name you@example.com

markdown
Copy
Edit
Add this line to each commit (`git commit -s`). This confirms you have the right to contribute the code.

## Code Style & Structure
- Backend: Go (`/backend/nosd`, `/agent/nos-agent`), REST/gRPC, OpenAPI doc.
- Web: React + TypeScript (`/web`), generated client from OpenAPI.
- Packaging: Debian `.deb`, ISO build profiles under `/packaging`.

## CLA / Relicensing
Per `LICENSE` §4, contributions grant Nithron rights to relicense to keep the project healthy (e.g., to harmonize with third-party licenses).

## Contact
General: **hello@nithron.com**
