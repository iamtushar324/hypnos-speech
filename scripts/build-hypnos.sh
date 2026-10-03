#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/.."
mode="${1:-}"
case "$mode" in
  ""|--build-only|--install-only) ;;
  *) printf 'Usage: %s [--build-only|--install-only]\n' "$0" >&2; exit 2 ;;
esac
app=".hypnos-native-build/Build/Products/Release/Hypnos Speech.app"
installed="$HOME/Applications/Hypnos Speech.app"
if [[ "$mode" != --install-only ]]; then
# Same Release/local signing route as make local, with a generated project to keep upstream untouched.
python3 scripts/prepare-hypnos-project.py
make check
identity="${LOCAL_CODESIGN_IDENTITY:-}"
identity_file=".hypnos-native/signing-identity"
if [[ -z "$identity" ]]; then
  if [[ -f "$identity_file" ]]; then
    identity=$(cat "$identity_file")
  else
    identities=$(security find-identity -v -p codesigning | awk '/"Apple Development: / {print $2}')
    count=$(printf '%s\n' "$identities" | awk 'NF {n++} END {print n+0}')
    if [[ "$count" == 1 ]]; then
      identity="$identities"
    elif [[ -d "$installed" ]] && codesign -dv "$installed" 2>&1 | awk '/^Authority=/{found=1} END {exit !found}'; then
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
printf '#include "../HypnosSpeech/LocalBuild.xcconfig"\nCODE_SIGN_IDENTITY = %s\n' "$identity" > .hypnos-native/Signing.xcconfig
xcodebuild -project HypnosSpeech.xcodeproj -scheme "Hypnos Speech" -configuration Release \
  -derivedDataPath .hypnos-native-build -xcconfig .hypnos-native/Signing.xcconfig \
  CODE_SIGN_IDENTITY="$identity" CODE_SIGNING_ALLOWED=YES CODE_SIGN_STYLE=Manual DEVELOPMENT_TEAM="" \
  CODE_SIGN_ENTITLEMENTS="$PWD/HypnosSpeech/HypnosSpeech.entitlements" \
  SWIFT_ACTIVE_COMPILATION_CONDITIONS='$(inherited) LOCAL_BUILD HYPNOS_SPEECH' \
  -skipPackagePluginValidation -skipMacroValidation build
test -d "$app"
codesign --verify --deep --strict "$app"
printf '%s\n' "$identity" > "$identity_file"
fi
if [[ "$mode" == --build-only ]]; then
  printf 'Built and verified: %s\nQuit Hypnos Speech, then run %s --install-only.\n' "$app" "$0"
  exit 0
fi
test -d "$app"
codesign --verify --deep --strict "$app"
if pgrep -x 'Hypnos Speech' >/dev/null; then
  printf 'Build is ready. Quit Hypnos Speech before installation, then run %s --install-only. The running app has not been overwritten.\n' "$0" >&2
  exit 1
fi
# Stage a complete verified bundle; never merge files over a running/signed installation.
mkdir -p "$HOME/Applications"
cache="$HOME/Library/Caches/space.hypnos.speech.mac/BuildInstalls"
mkdir -p "$cache"
stage=$(mktemp -d "$cache/install.XXXXXX")
ditto "$app" "$stage/Hypnos Speech.app"
codesign --verify --deep --strict "$stage/Hypnos Speech.app"
if pgrep -x 'Hypnos Speech' >/dev/null; then
  printf 'Hypnos Speech reopened during staging. Quit it and rerun --install-only.\n' >&2
  exit 1
fi
if [[ -d "$installed" ]]; then
  before=$(codesign -dr - "$installed" 2>&1 | sed -n '/^designated =>/p')
  after=$(codesign -dr - "$stage/Hypnos Speech.app" 2>&1 | sed -n '/^designated =>/p')
  if [[ "$before" != "$after" ]]; then
    printf 'Signing requirement changed. Refresh the existing Accessibility entry for the newly installed app.\n' >&2
  fi
  mv "$installed" "$stage/previous.bundle"
fi
if ! mv "$stage/Hypnos Speech.app" "$installed"; then
  if [[ -d "$stage/previous.bundle" ]]; then mv "$stage/previous.bundle" "$installed"; fi
  exit 1
fi
codesign --verify --deep --strict "$installed"
printf 'Installed: %s\nPrevious bundle (if any): %s/previous.bundle\n' "$installed" "$stage"
