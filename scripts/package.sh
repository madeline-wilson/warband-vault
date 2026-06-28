#!/usr/bin/env bash
set -euo pipefail

GO_BIN="${GO:-go}"
VERSION="${VERSION:-v0.1.0}"
COMMIT="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo unknown)}"
BUILD_DATE="${BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
CHANNEL="${CHANNEL:-development}"
OUTDIR="${OUTDIR:-dist/packages}"
GOOS_VALUE="${GOOS:-$("${GO_BIN}" env GOOS)}"
GOARCH_VALUE="${GOARCH:-$("${GO_BIN}" env GOARCH)}"
EXE=""
if [[ "${GOOS_VALUE}" == "windows" ]]; then
  EXE=".exe"
fi

ldflags="-s -w -X warband-vault/internal/buildinfo.Version=${VERSION} -X warband-vault/internal/buildinfo.Commit=${COMMIT} -X warband-vault/internal/buildinfo.BuildDate=${BUILD_DATE} -X warband-vault/internal/buildinfo.Channel=${CHANNEL}"
package_name="WarbandVault-${GOOS_VALUE}-${GOARCH_VALUE}"
work="dist/work/${package_name}"
install_root="${work}/WarbandVault"
version_dir="${install_root}/versions/${VERSION}"

rm -rf "${work}"
mkdir -p "${version_dir}" "${install_root}/state" "${install_root}/downloads" "${install_root}/logs" "${OUTDIR}"

"${GO_BIN}" build -trimpath -ldflags "${ldflags}" -o "${install_root}/warband-vault-launcher${EXE}" ./cmd/launcher
"${GO_BIN}" build -trimpath -ldflags "${ldflags}" -o "${version_dir}/warband-vault${EXE}" ./cmd/warband-vault
"${GO_BIN}" build -trimpath -ldflags "${ldflags}" -o "${version_dir}/warband-vault-updater${EXE}" ./cmd/updater

cat > "${install_root}/state/current.json" <<JSON
{
  "version": "${VERSION}",
  "relative_executable": "versions/${VERSION}/warband-vault${EXE}"
}
JSON

cp README.md LICENSE "${install_root}/"
if [[ -f assets/icon.png ]]; then
  cp assets/icon.png "${install_root}/"
fi
if [[ "${GOOS_VALUE}" == "linux" ]]; then
  mkdir -p "${install_root}/share/applications" "${install_root}/share/icons/hicolor/256x256/apps"
  cp packaging/linux/warband-vault.desktop "${install_root}/share/applications/"
  if [[ -f assets/icon.png ]]; then
    cp assets/icon.png "${install_root}/share/icons/hicolor/256x256/apps/warband-vault.png"
  fi
  tar -C "${work}" -czf "${OUTDIR}/${package_name}.tar.gz" WarbandVault
elif [[ "${GOOS_VALUE}" == "windows" ]]; then
  if command -v zip >/dev/null 2>&1; then
    (cd "${work}" && zip -qr "../../packages/${package_name}.zip" WarbandVault)
  else
    powershell.exe -NoProfile -Command "Compress-Archive -Path '$(cygpath -w "${install_root}")' -DestinationPath '$(cygpath -w "${OUTDIR}/${package_name}.zip")' -Force"
  fi
elif [[ "${GOOS_VALUE}" == "darwin" ]]; then
  app_root="${work}/Warband Vault.app"
  mkdir -p "${app_root}/Contents/MacOS" "${app_root}/Contents/Resources"
  cp "${version_dir}/warband-vault" "${app_root}/Contents/MacOS/Warband Vault"
  cp packaging/macos/Info.plist "${app_root}/Contents/Info.plist"
  if [[ -f assets/icon.png ]]; then
    cp assets/icon.png "${app_root}/Contents/Resources/icon.png"
  fi
  if command -v codesign >/dev/null 2>&1; then
    codesign --force --deep --sign - "${app_root}"
  fi
  (cd "${work}" && zip -qr "../../packages/${package_name}.zip" "Warband Vault.app")
else
  echo "unsupported GOOS ${GOOS_VALUE}" >&2
  exit 1
fi
