# Agent Notes

## Project State

Warband Vault is implemented as a Go/Fyne desktop app in this repo. The original tiny self-update sample was replaced with a production-oriented skeleton:

- `cmd/warband-vault`: main Fyne desktop app with `--version`, `--smoke-test`, `--data-dir`, and `--no-update-check`.
- `cmd/launcher`: small launcher that reads `state/current.json` and only starts relative executables inside the install root.
- `cmd/updater`: signed-manifest checker, package downloader, and staging helper.
- `cmd/manifest-tool`: create/sign/verify release manifests.
- `cmd/release-keygen`: Ed25519 release key generator.
- `internal/*`: domain, validation, config, logging, SQLite persistence, migrations, export, update security, platform helpers.
- `ui/main_window.go`: main desktop UI.

The repo was not a Git repository when work started.

## Toolchain

The user approved installing whatever was needed. Installed:

- Homebrew `go` 1.26.4, globally linked by brew.
- Homebrew `go@1.25` 1.25.11, keg-only and used for this repo.

Use this path for spec-aligned work:

```sh
export PATH="/opt/homebrew/opt/go@1.25/bin:$PATH"
go version
```

`go.mod` currently says `go 1.25.0`, which is Go 1.25.x as requested.

Static analysis tools were installed into the Go bin dir:

```sh
go install honnef.co/go/tools/cmd/staticcheck@v0.7.0
go install golang.org/x/vuln/cmd/govulncheck@v1.5.0
```

Use:

```sh
export PATH="/opt/homebrew/opt/go@1.25/bin:$PATH:/Users/mads/go/bin"
```

## Verification Commands

These passed after the latest UI refresh fix:

```sh
go test ./...
go test -race -count=1 ./...
go vet ./...
staticcheck ./...
govulncheck ./...
go build ./cmd/warband-vault
go run ./cmd/warband-vault --smoke-test --data-dir /private/tmp/warband-vault-smoke-refresh-fix
```

`go build ./cmd/warband-vault` emits a macOS linker warning about duplicate `-lobjc`; it still exits successfully.

Local package verification also passed earlier:

```sh
GO=/opt/homebrew/opt/go@1.25/bin/go VERSION=v0.1.0 CHANNEL=development ./scripts/package.sh
```

This produced `dist/packages/WarbandVault-darwin-arm64.zip`. `dist/` is ignored.

## Important UI Lesson

Bug reported by user: saving a campaign did not appear to save or did not show automatically.

Root cause was in `ui/main_window.go`: actions saved successfully, then called `loadCampaigns()` / `loadCampaign()` from inside an already-running `m.run(...)`. `m.run` drops new operations while `busy == true`, so UI refreshes were silently skipped. Startup had the same shape: `loadCampaigns()` selected the first item while still busy, so the roster load could be skipped.

Fix:

- Added snapshot/apply helpers:
  - `campaignSnapshot(ctx, selectedID)`
  - `applySnapshot(campaigns, selected)`
  - `applySelectedCampaign(selected)`
  - `selectCampaignInList(id)`
- Added `m.suppress` so programmatic list selection does not recursively trigger `OnSelected`.
- Save/import/example/delete flows now fetch fresh data inside the same background operation and apply it directly with `fyne.Do`.

If adding future UI operations, do not call `m.run` from inside another `m.run`; fetch what is needed in the current goroutine and update widgets once on the UI thread.

## Persistence Notes

SQLite uses `modernc.org/sqlite` and opens with foreign keys enabled:

```go
sql.Open("sqlite", paths.Database+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
```

Migrations live in top-level `migrations/` and are embedded by the `migrations` package, because `go:embed` cannot reach parent directories from `internal/migration`.

User data defaults to:

```text
<os.UserConfigDir>/WarbandVault/
```

Use `--data-dir` for development and tests.

## Update Security Notes

Implemented core update primitives:

- Ed25519 detached signature verification over raw manifest bytes.
- SHA-256 package verification.
- HTTPS required except explicit/local test allowance.
- Bounded downloader with `.partial` files.
- Safe ZIP/tar.gz extraction with traversal, absolute path, Windows drive path, symlink, duplicate path, file count, and expanded-size protections.
- Relative-only launcher state.
- Update lock.
- Atomic current/previous state handling plus `RollbackToPrevious`.

Known caveat: `assets/update_public_key.txt` is a placeholder. Before any real release, run `cmd/release-keygen`, store the private key in `UPDATE_SIGNING_PRIVATE_KEY_B64`, and replace the embedded public key.

## Dependency/Vulnerability Note

`govulncheck` originally flagged reachable vulnerabilities in `golang.org/x/image` via Fyne. Fixed by upgrading `golang.org/x/image` to `v0.43.0`, which also moved `golang.org/x/mod` to `v0.36.0` and `golang.org/x/text` to `v0.38.0`.

Current `govulncheck ./...` reports no vulnerabilities affecting reachable code.

## Packaging/CI Notes

Added:

- `Makefile`
- `FyneApp.toml`
- `scripts/package.sh`
- `.github/workflows/ci.yml`
- `.github/workflows/release.yml`
- `packaging/linux/warband-vault.desktop`
- `packaging/macos/Info.plist`
- `packaging/windows/README.txt`

The release workflow is scaffolded but should be exercised in a real GitHub repo before trusting it. macOS packaging is ad-hoc signed and not notarized.

## Known Gaps

- No real release signing public key embedded yet.
- Full updater process supervision and macOS bundle replacement should be tested on actual Windows/macOS/Linux release runners.
- `assets/icon.png` is a simple generated placeholder, not final branding.
- A local `warband-vault` binary may exist at repo root from `go build ./cmd/warband-vault`; it is a generated artifact.
