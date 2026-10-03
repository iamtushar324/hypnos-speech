#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/.."
# Same Release/local signing route as make local, with a generated project to keep upstream untouched.
python3 scripts/prepare-hypnos-project.py
make check
identity="${LOCAL_CODESIGN_IDENTITY:-}"
if [[ -z "$identity" ]]; then
  identities=$(security find-identity -v -p codesigning | awk '/"Apple Development: / {print $2}')
  count=$(printf '%s\n' "$identities" | awk 'NF {n++} END {print n+0}')
  if [[ "$count" == 1 ]]; then identity="$identities"; else identity="-"; fi
fi
# -xcconfig overrides target/command defaults, so put the chosen identity last in an ignored config.
printf '#include "../HypnosSpeech/LocalBuild.xcconfig"\nCODE_SIGN_IDENTITY = %s\n' "$identity" > .hypnos-native/Signing.xcconfig
xcodebuild -project HypnosSpeech.xcodeproj -scheme "Hypnos Speech" -configuration Release \
  -derivedDataPath .hypnos-native-build -xcconfig .hypnos-native/Signing.xcconfig \
  CODE_SIGN_IDENTITY="$identity" CODE_SIGNING_ALLOWED=YES CODE_SIGN_STYLE=Manual DEVELOPMENT_TEAM="" \
  CODE_SIGN_ENTITLEMENTS="$PWD/HypnosSpeech/HypnosSpeech.entitlements" \
  SWIFT_ACTIVE_COMPILATION_CONDITIONS='$(inherited) LOCAL_BUILD HYPNOS_SPEECH' \
  -skipPackagePluginValidation -skipMacroValidation build
app=".hypnos-native-build/Build/Products/Release/Hypnos Speech.app"
test -d "$app"
codesign --verify --deep --strict "$app"
# Only our own app path is replaced. No VoiceInk/Handy path is touched.
mkdir -p "$HOME/Applications"
ditto "$app" "$HOME/Applications/Hypnos Speech.app"
printf 'Built: %s\n' "$HOME/Applications/Hypnos Speech.app"
