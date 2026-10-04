#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/.."
mode="${1:-}"
case "$mode" in
  ""|--build-only|--install-only) ;;
  *) printf 'Usage: %s [--build-only|--install-only]\n' "$0" >&2; exit 2 ;;
esac
app=".vokiri-native-build/Build/Products/Release/Vokiri.app"
installed="$HOME/Applications/Vokiri.app"
legacy="$HOME/Applications/Hypnos Speech.app"
previous="$installed"
if [[ ! -d "$previous" && -d "$legacy" ]]; then previous="$legacy"; fi
app_running() { pgrep -x 'Vokiri' >/dev/null || pgrep -x 'Hypnos Speech' >/dev/null; }
if [[ "$mode" != --install-only ]]; then
# Same Release/local signing route as make local, with a generated project to keep upstream untouched.
python3 scripts/prepare-vokiri-project.py
make check
identity="${LOCAL_CODESIGN_IDENTITY:-}"
identity_file=".vokiri-native/signing-identity"
if [[ -z "$identity" ]]; then
  if [[ -f "$identity_file" ]]; then
    identity=$(cat "$identity_file")
  else
    identities=$(security find-identity -v -p codesigning | awk '/"Apple Development: / {print $2}')
    count=$(printf '%s\n' "$identities" | awk 'NF {n++} END {print n+0}')
    if [[ "$count" == 1 ]]; then
      identity="$identities"
    elif [[ -d "$previous" ]] && codesign -dv "$previous" 2>&1 | awk '/^Authority=/{found=1} END {exit !found}'; then
      printf 'Choose LOCAL_CODESIGN_IDENTITY explicitly; refusing to silently change the installed developer-signed app to ad-hoc signing.\n' >&2
      exit 1
    else
      identity="-"
    fi
  fi
fi
if [[ "$identity" != - ]] && ! security find-identity -v -p codesigning | awk -v wanted="$identity" '$2 == wanted {found=1} END {exit !found}'; then
  printf 'The saved signing identity is unavailable. Choose LOCAL_CODESIGN_IDENTITY explicitly; app permissions may need refreshing if it changes.\n' >&2
  exit 1
fi
# -xcconfig overrides target/command defaults, so put the chosen identity last in an ignored config.
printf '#include "../apps/macos/Resources/LocalBuild.xcconfig"\nCODE_SIGN_IDENTITY = %s\n' "$identity" > .vokiri-native/Signing.xcconfig
xcodebuild -project Vokiri.xcodeproj -scheme "Vokiri" -configuration Release \
  -derivedDataPath .vokiri-native-build -xcconfig .vokiri-native/Signing.xcconfig \
  CODE_SIGN_IDENTITY="$identity" CODE_SIGNING_ALLOWED=YES CODE_SIGN_STYLE=Manual DEVELOPMENT_TEAM="" \
  CODE_SIGN_ENTITLEMENTS="$PWD/apps/macos/Resources/Vokiri.entitlements" \
  SWIFT_ACTIVE_COMPILATION_CONDITIONS='$(inherited) LOCAL_BUILD VOKIRI' \
  -skipPackagePluginValidation -skipMacroValidation build
test -d "$app"
codesign --verify --deep --strict "$app"
printf '%s\n' "$identity" > "$identity_file"
fi
if [[ "$mode" == --build-only ]]; then
  printf 'Built and verified: %s\nQuit Vokiri, then run %s --install-only.\n' "$app" "$0"
  exit 0
fi
test -d "$app"
codesign --verify --deep --strict "$app"
if app_running; then
  printf 'Build is ready. Quit Vokiri (or Hypnos Speech) before installation, then run %s --install-only. The running app has not been overwritten.\n' "$0" >&2
  exit 1
fi
# Stage a complete verified bundle; never merge files over a running/signed installation.
mkdir -p "$HOME/Applications"
cache="$HOME/Library/Caches/space.hypnos.speech.mac/BuildInstalls"
mkdir -p "$cache"
stage=$(mktemp -d "$cache/install.XXXXXX")
ditto "$app" "$stage/Vokiri.app"
codesign --verify --deep --strict "$stage/Vokiri.app"
if app_running; then
  printf 'Vokiri reopened during staging. Quit it and rerun --install-only.\n' >&2
  exit 1
fi
if [[ -d "$previous" ]]; then
  before=$(codesign -dr - "$previous" 2>&1 | sed -n '/^designated =>/p')
  after=$(codesign -dr - "$stage/Vokiri.app" 2>&1 | sed -n '/^designated =>/p')
  if [[ "$before" != "$after" ]]; then
    printf 'Signing requirement changed. Refresh the existing Accessibility entry for the newly installed app.\n' >&2
  fi
  mv "$previous" "$stage/previous.bundle"
fi
if ! mv "$stage/Vokiri.app" "$installed"; then
  if [[ -d "$stage/previous.bundle" ]]; then mv "$stage/previous.bundle" "$previous"; fi
  exit 1
fi
codesign --verify --deep --strict "$installed"
# Retire any remaining old-name installation only after the new bundle verifies.
if [[ -d "$legacy" ]]; then mv "$legacy" "$stage/legacy.bundle"; fi
printf 'Installed: %s\nPrevious bundle (if any): %s/previous.bundle\n' "$installed" "$stage"
