# Hypnos Speech for macOS

A personal batch capture client based on [VoiceInk](https://github.com/Beingpax/VoiceInk) v2.21, commit **`640b0c8c36ee74d9e76d930f40e2c04bb739b4b3`**. GPLv3 and Pax's upstream attribution are retained. The root repository remains a genuine VoiceInk fork; `server/` contains the reviewed gateway source snapshot. Nothing from the personal vault is published.

## Setup

1. Use macOS 15+, keep Tailscale connected, and build as described below.
2. Open `~/Applications/Hypnos Speech.app`. In Settings, confirm the HTTPS endpoint and model.
3. Sign in to `https://speech.tusharbhardwaj.space/#keys`, create a **dedicated Mac speech-only device key**, and enter it directly into the app's **secure device key field**. Save settings. Never put a key in chat, source, screenshots, shell commands, or documentation. Blank replacement input preserves the saved key.
4. Click **Grant Microphone** and **Grant Accessibility**. In macOS System Settings → Privacy & Security, enable Hypnos Speech for those permissions. Accessibility is used for the hotkey and Command-V paste. Screen Recording permission is not needed.
5. Default hotkey: **Control–Option–Space**, independent of Handy's right-side modifiers. Press to start/stop. Enable **Push to talk** to hold/release, or record another shortcut using VoiceInk's shortcut recorder. After granting permissions, use **Refresh hotkey** if needed.
6. Focus a disposable plain-text document and make a short fresh recording. Stop, wait for Hypnos processing, and compare the document against the exact saved text. Never upload existing app history to test this integration.

Default endpoint: `https://speech.tusharbhardwaj.space/v1/audio/transcriptions`. Default model: `whisper-large-v3-turbo`. Profile ID is optional. Language is omitted for automatic detection. Hypnos controls dictionary, snippets, and cleanup.

Microphone recordings are WAV, mono 16 kHz PCM16, using VoiceInk's native Core Audio recorder. Upload begins only after capture has stopped. No partial transcript or streaming recognition is displayed.

## Build and signing

Install Xcode and its Command Line Tools, Git and Python 3. Read upstream [BUILDING.md](BUILDING.md) and [Makefile](Makefile). The Hypnos build follows upstream's Release + LocalBuild signing route but generates an ignored project with our own product/bundle identity:

```sh
./scripts/build-hypnos.sh
open "$HOME/Applications/Hypnos Speech.app"
```

The sole available Apple Development signing identity is selected automatically. Override with `LOCAL_CODESIGN_IDENTITY` when several exist. For supported ad-hoc signing:

```sh
LOCAL_CODESIGN_IDENTITY=- ./scripts/build-hypnos.sh
```

Ad-hoc rebuilds may require granting permissions again. There is no security disabling or signing purchase. The script verifies the bundle signature before installing. It only copies to `~/Applications/Hypnos Speech.app`; it does not overwrite Handy or VoiceInk.

The generated Hypnos target compiles only VoiceInk's native capture/shortcut/Keychain/paste/UI primitives, our capture wrapper, and Swift Atomics **1.3.0**. Local-inference frameworks, model managers, Sparkle and the XPC inference helper are not linked or initialized. No model download or whisper.cpp build is required. This preserves compatibility with the installed Xcode 26.3 despite upstream's newer local-model package requirements.

Build output: `.hypnos-native-build/Build/Products/Release/Hypnos Speech.app`. Generated project: `HypnosSpeech.xcodeproj`; generated native adapters: `.hypnos-native/` (both ignored). Regenerate after upstream updates. Upstream `make local` still builds stock VoiceInk.

## Separation and recording storage

- Bundle identifier/preferences: `space.hypnos.speech.mac`.
- Keychain service: `space.hypnos.speech.mac`, account `speech-device-key`, non-syncing local Keychain. Keys never enter preferences or history.
- Data: `~/Library/Application Support/space.hypnos.speech.mac/Recordings/`.
- Per recording: UUID-named `.wav` and JSON metadata with status, timestamp, safe error, and exact final text. Directories are 0700 and files 0600. This is private on-disk storage, not additional application-level encryption; use macOS/FileVault for disk encryption.
- No automatic audio/history deletion. Remove selected files explicitly in Finder when desired. Failed, canceled, and interrupted sessions are retained. Launch marks interrupted sessions failed and never uploads them automatically.

Only WAV audio, model, `response_format=json`, and optional profile ID are uploaded. No screen, selected text, clipboard, or local dictionary context is sent. The clipboard is used locally for paste/restore. The app keeps only the target application's process identity, not its document contents. Optional server cleanup can send fresh transcript/dictionary context to a configured provider.

A successful response's `text` is validated for nonempty content without trimming its value. Unknown JSON metadata is accepted. The text is saved and sent to CursorPaster unchanged: no bracket filter, paragraph formatter, whitespace cleanup, replacements, snippets, local AI, licensing prefix, or trailing space. Auto-learn is disabled for this paste path.

**Cancel** invalidates delivery immediately and cancels the upload task. A late final cannot paste; the paste-delay guard also checks cancellation. Already posted keyboard events cannot be undone. An interrupted network request may have been processed on the server. **Retry** explicitly makes a new request; there is no automatic retry. A saved successful result uses **Paste saved text** without re-uploading. The app reports a posted paste command; actual insertion depends on the destination app and must be checked in the document.

HTTP authentication failures, non-2xx statuses, unavailable network/Tailscale, 90-second request/120-second resource deadlines, empty text, malformed responses, storage errors, and paste permission failures have safe messages. Provider error bodies are never logged/displayed. Redirects are refused rather than forwarding audio or credentials to another origin.

## Focused patches and upstream updates

Our implementation is additive in `VoiceInk/Hypnos/`, `HypnosSpeech/`, `HypnosTests/`, and `scripts/*hypnos*`. Only two upstream Swift files have hooks:

1. `KeychainService.swift`: separate namespace under `HYPNOS_SPEECH`.
2. `CursorPaster.swift`: optional auto-learn suppression and cancellation guard, with original defaults preserved.

The generator selects native source files and makes small, asserted adapters in ignored build sources: removes local auto-learn, upstream preference migrations, and model-mode lookups, routes shortcut validation alerts locally, and extracts recording indicator widgets. Assertions stop the build if those upstream interfaces change. Upstream files remain intact.

The Hypnos app bypasses the upstream transcription pipeline entirely; its explicit final-output mode uses a separate tested workflow from response to history to paste. Native CoreAudioRecorder, ShortcutMonitor/ShortcutRecorder, MiniRecorderPanel/RecorderStatusDisplay, KeychainService, and CursorPaster are reused. Settings do not expose upstream model downloads, local cleanup, screen capture, or update controls.

Upstream `LOCAL_BUILD` alone does not fully guard the v2.21 updater initializer. Our entry point never constructs it, and our own Info.plist has no Sparkle feed or signing key. Stock upstream binary updates therefore cannot replace this app.

Keep upstream separate from our development/default branch:

```sh
git remote -v
git fetch upstream --tags
git switch hypnos-speech
git merge <reviewed-upstream-tag-or-commit>
swift test
./scripts/build-hypnos.sh
```

Use a merge, preserving both histories; do not rename upstream source folders or broadly rebrand Swift symbols. Review changes to the two hooks, recorder/paste contracts, and generator assertions. Keep our app identity and final-output invariants. Do not push our branch to Beingpax/VoiceInk. Push only reviewed, credential-free source to `origin` (the personal public fork). The backend has no VoiceInk overlap and should merge independently.

## Validation and rollback

Run `swift test` for multipart/auth, URLSession upload/errors, exact UTF-8 output through history and delivery, cancellation including late finals/paste delay, private failure retention, explicit retry, empty/malformed responses, and interrupted recovery. Backend checks live in [server/README.md](server/README.md). These are mock/unit checks and do not claim real microphone/hotkey/server/paste verification. See [VALIDATION.md](VALIDATION.md) for the actual results of this build.

Quit Hypnos Speech and remove only `~/Applications/Hypnos Speech.app` to uninstall. The recordings and Keychain item remain for recovery. Remove the key through Settings and revoke the Mac key in the server UI if retiring the device. Restore an older built Hypnos app or revert our commits and rebuild for rollback. Handy, VoiceInk, and their preferences/Keychain/data are independent.
