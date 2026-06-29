# Warband Vault

Warband Vault is a local-first campaign and character manager for miniature wargames, skirmish games, and tabletop role-playing games. It stores campaign data on the user’s machine, exports readable JSON, generates printable HTML rosters, and uses signed release metadata for secure self-updates.

Screenshot placeholders:

- Main campaign roster: `docs/screenshots/main-window.png`
- Character editor: `docs/screenshots/character-editor.png`
- Update dialog: `docs/screenshots/update-dialog.png`

## Supported Platforms

- Windows amd64, distributed as a portable ZIP.
- macOS arm64 and amd64, distributed as ZIP files containing an ad-hoc-signed `.app`.
- Linux amd64, distributed as a tarball.

Linux builds target Ubuntu 22.04 or a compatible glibc baseline for the first release. Package-manager-managed Linux installations should disable direct self-replacement and use their package manager instead.

## Architecture

```text
warband-vault-launcher
        |
        v
state/current.json -> versions/vX.Y.Z/warband-vault
                                  |
                                  +-> SQLite user database under user config dir
                                  +-> signed update manifest check
                                  +-> warband-vault-updater for staged installs
```

Core packages under `internal` do not import UI packages. Domain validation, persistence, exports, configuration, logging, and updater security are testable without Fyne.

## Repository Layout

```text
cmd/                  application, launcher, updater, release tooling
internal/             domain, persistence, config, logging, update logic
ui/                   Fyne desktop presentation
migrations/           embedded SQLite migrations
assets/               embedded update public key and app assets
packaging/            platform package metadata
scripts/              local packaging helpers
.github/workflows/    CI and release automation
```

## Local Development

Install Go 1.25.x. On macOS with Homebrew:

```sh
brew install go@1.25
export PATH="/opt/homebrew/opt/go@1.25/bin:$PATH"
```

Common commands:

```sh
make test
make race
make build
make smoke-test
```

Native Fyne prerequisites vary by platform. Linux development generally needs OpenGL, X11, and xkbcommon development packages:

```sh
sudo apt-get install gcc libgl1-mesa-dev xorg-dev libxkbcommon-dev
```

## Data Location

By default, user data is stored below:

```text
<os.UserConfigDir>/WarbandVault/
├── warband-vault.db
├── config.json
├── backups/
├── exports/
└── logs/
```

Use `--data-dir <path>` for tests and portable development.

## Features

- Create, edit, archive, and delete campaigns.
- Create, edit, and delete campaign characters.
- Track role, level, experience, health, movement, armor, equipment, traits, injuries, notes, treasury, and custom fields.
- Create the fictional “The Blackwater Expedition” example campaign on demand.
- Export and import versioned JSON.
- Generate standalone print-friendly HTML rosters.
- Show version, commit, build date, and channel.
- Check for signed updates manually, with optional startup checks controlled by settings.

## Export Format

Campaign exports use a versioned envelope:

```json
{
  "schema_version": 1,
  "exported_at": "2026-06-28T18:00:00Z",
  "campaign": {}
}
```

Imports are size-limited, schema-checked, validated before insertion, and committed transactionally. ID collisions are assigned new random IDs.

## Update Protocol

Warband Vault does not overwrite the running executable. The launcher starts the version named by `state/current.json`, and the updater stages a new version directory before atomically switching state.

Update verification order:

1. Download `update-manifest.json` with strict size and timeout limits.
2. Download `update-manifest.json.sig`.
3. Verify the Ed25519 signature over the exact raw manifest bytes.
4. Decode and validate the manifest.
5. Reject wrong app IDs, unsupported schemas, invalid versions, downgrades, missing platform entries, and non-HTTPS production URLs.
6. Download the selected package to a `.partial` file.
7. Verify expected byte size and SHA-256.
8. Extract into staging with path traversal, symlink, hardlink, device file, file count, and expanded-size protections.
9. Locate the selected version payload inside either a direct version archive, a full `WarbandVault` install root, or the macOS `.app` resource layout.
10. Atomically switch `state/current.json` and preserve `previous.json`.

The immediately previous version is retained for rollback. User campaign data is not stored in the install directory and is not modified by binary rollback.

The standalone updater can perform the same install flow used by the GUI:

```sh
warband-vault-updater \
  --manifest-url https://example.com/update-manifest.json \
  --install-root /path/to/WarbandVault \
  --current-version v0.1.0 \
  --install \
  --restart
```

## Signing Keys

Generate a release keypair:

```sh
go run ./cmd/release-keygen
```

Store the private key as the GitHub Actions secret `UPDATE_SIGNING_PRIVATE_KEY_B64`. Never commit it. Embed the public key in `assets/update_public_key.txt`.

Key rotation is a future concern and should require an authenticated trust transition.

## Release Process

Tagged releases matching `v*` run `.github/workflows/release.yml`.

The workflow builds native packages, signs macOS bundles ad hoc, computes checksums, generates `update-manifest.json`, signs the raw manifest and checksum bytes with Ed25519, and uploads packages plus signed metadata to a draft GitHub Release.

GitHub Releases act as the static manifest and artifact host. No custom server, paid hosting, paid GitHub feature, Apple Developer membership, or commercial code-signing certificate is required.

## Platform Packaging Notes

Windows: The portable ZIP contains the launcher, an initial version directory, state bootstrap, and first-run notes. The binaries are not Authenticode signed, so SmartScreen may warn.

macOS: The ZIP contains a complete `.app` signed with an ad-hoc signature. `Contents/MacOS/Warband Vault` is the stable launcher, and mutable versions live under `Contents/Resources/WarbandVault/versions`. It is not Apple-notarized and is not signed with Developer ID. Users may need to control-click and choose Open the first time. Do not disable Gatekeeper globally.

Linux: The tarball contains the launcher, versioned app directory, desktop file, icon when present, and instructions. AppImage is a future stretch goal.

## Threat Model

Implemented protections:

- Ed25519 release authentication for manifest metadata.
- SHA-256 integrity checks for package artifacts.
- Rejection of unsigned, malformed, downgraded, or wrong-application manifests.
- Rejection of archive paths that escape staging directories.
- Transactional state switching and a single updater lock.
- User data stored outside the application install directory.

Not protected in this zero-cost release:

- Compromise of the release signing private key.
- Compromise of a maintainer’s GitHub account.
- A malicious binary signed with the legitimate release key.
- Operating-system warnings caused by lack of commercial signing.
- A fully compromised local administrator or root account.

Ed25519 signatures authenticate Warband Vault release metadata. SHA-256 detects artifact corruption. Neither is the same as Windows publisher signing, Apple Developer ID signing, or Apple notarization.

## CI

CI runs formatting, `go vet`, Staticcheck, `go test`, race tests, `govulncheck`, native command builds, and a Linux GUI smoke test under Xvfb where practical.

## Known Limitations

- macOS packaging is ad-hoc signed; notarization is out of scope for zero-cost distribution.
- AppImage, `.deb`, RPM, Flatpak, and package-manager update integration are not implemented yet.
- The embedded public update key is a placeholder until a real release key is generated.
- Health-marker rollback plumbing is implemented in the updater core; full cross-platform process supervision should be exercised on each OS in CI before a public release.

## Assumptions

1. “Seamless replacement” permits restarting the application.
2. A custom application server is not required.
3. GitHub Releases may serve static manifests and artifacts.
4. Separate packages may be built for each OS and architecture.
5. Updates require user approval by default.
6. The application is installed in a user-writable directory.
7. The immediately previous version is retained for rollback.
8. Campaign data is local-only in the first version.
9. Windows and macOS builds are not commercially signed.
10. macOS builds are not notarized.
11. Ed25519 signatures authenticate update artifacts independently of OS publisher identity.
12. Package-manager-managed Linux installations will not be overwritten by the internal updater.
13. The launcher is updated only through manual installation when the protocol requires it.
