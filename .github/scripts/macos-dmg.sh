#!/usr/bin/env bash
# Builds the macOS .dmg for the GitHub release, in place of
# `giode build -release dmg`.
#
# Giode CLI v0.1.35-alpha.1 signs Developer ID builds with the hardened
# runtime and only Go's entitlements; without device.audio-input macOS then
# never lets NoiseFree record. So this builds the signed app without
# notarizing it, signs it again with assets/macos/developer-id.entitlements,
# and then notarizes and staples the app and the disk image itself.
#
# Without MACOS_CERT_BASE64 it builds an unsigned .dmg, as giode would.
# Uses GIODE_MACOS_IDENTITY (else the keychain's Developer ID Application
# identity), GIODE_NOTARY_ID, GIODE_NOTARY_TEAM_ID (else the identity's
# team) and GIODE_NOTARY_PASS.
set -euo pipefail

if [ -z "${MACOS_CERT_BASE64:-}" ]; then
  giode build dmg
  exit 0
fi

identity="${GIODE_MACOS_IDENTITY:-$(security find-identity -v -p codesigning | sed -n 's/.*"\(Developer ID Application: [^"]*\)".*/\1/p' | head -n 1)}"
if [ -z "$identity" ]; then
  echo "::error::no Developer ID Application identity in the keychain" >&2
  exit 1
fi
team="${GIODE_NOTARY_TEAM_ID:-$(printf '%s' "$identity" | sed -n 's/.*(\([A-Z0-9]*\))$/\1/p')}"

# Signed, not notarized: signing again would void the ticket.
env -u GIODE_NOTARY_ID -u GIODE_NOTARY_TEAM_ID -u GIODE_NOTARY_PASS \
  GIODE_MACOS_IDENTITY="$identity" giode build -release macos
app="$(ls -d dist/macos/*.app | head -n 1)"
name="$(basename "$app" .app)"
codesign --force --deep --options runtime --timestamp \
  --entitlements assets/macos/developer-id.entitlements --sign "$identity" "$app"
codesign --verify --strict --deep --verbose=2 "$app"
codesign -d --entitlements - "$app" | grep -q com.apple.security.device.audio-input

notarize() {
  if [ -z "${GIODE_NOTARY_ID:-}" ]; then
    echo "::warning::GIODE_NOTARY_ID isn't set: $1 is signed but not notarized, so macOS won't open it once downloaded"
    return 0
  fi
  result="$(xcrun notarytool submit "$1" --apple-id "$GIODE_NOTARY_ID" --team-id "$team" \
    --password "$GIODE_NOTARY_PASS" --wait --output-format json)"
  echo "$result"
  echo "$result" | grep -q '"status" *: *"Accepted"'
  xcrun stapler staple "$2"
}

ditto -c -k --keepParent "$app" "$RUNNER_TEMP/app.zip"
notarize "$RUNNER_TEMP/app.zip" "$app"
rm -f "$RUNNER_TEMP/app.zip"

version="$(printf '%s' "${GIODE_VERSION#v}" | cut -d. -f1-3)"   # v1.4.2.7 is version 1.4.2, build 7
if [ -z "$version" ]; then
  version="$(sed -n 's/^version *= *"\([^"]*\)".*/\1/p' giode.toml | head -n 1)"
fi
dmg="dist/macos/$name-$version-universal.dmg"
stage="$(mktemp -d)"
ditto "$app" "$stage/$name.app"
ln -s /Applications "$stage/Applications"
rm -f "$dmg"
# hdiutil create fails now and then on GitHub's runners; try again.
for attempt in 1 2 3 4 5; do
  if hdiutil create -volname "$name" -srcfolder "$stage" -ov -fs HFS+ -format UDZO "$dmg"; then break; fi
  if [ "$attempt" = 5 ]; then exit 1; fi
  sleep 5
done
rm -rf "$stage"
codesign --force --sign "$identity" "$dmg"
notarize "$dmg" "$dmg"
