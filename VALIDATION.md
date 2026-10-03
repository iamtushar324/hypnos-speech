# Validation — 2026-10-04

Base: VoiceInk v2.21, `640b0c8c36ee74d9e76d930f40e2c04bb739b4b3`.

- Mac: Apple Silicon arm64, macOS 26.5.2; Xcode 26.3; Command Line Tools installed. Tailscale reports Running and Hypnos reachable.
- Capture-only Release app builds successfully. Installed at `~/Applications/Hypnos Speech.app` (3.4 MB), signed with the available Apple Development certificate; strict bundle signature verification passed. Bundle identifier is `space.hypnos.speech.mac`. Linked dependencies contain system frameworks and static Swift Atomics, with no local inference or Sparkle updater.
- App launch/process verified. Native UI automation is unavailable in this session (computer-use native pipe closes before returning the app state); visual review is not claimed.
- `swift test`: **11 tests passed**, including multipart/bearer authentication, approved-field boundaries, real URLSession with mock transport, exact UTF-8 preservation through history/paste callback, late-response cancellation, cancellation during paste delay, failed-audio retention, explicit retry, invalid/empty responses, and private storage/interrupted recovery. These are mock/unit tests.
- Backend `go test -race ./...`, `go vet ./...`, and static build passed on both this Mac and Hypnos Linux amd64. JavaScript syntax check passed on this Mac. Hypnos checkout: `/srv/hypnos/personal/projects/hypnos-speech`; binary: `build/speechd` within that checkout. Task-local Go 1.26.3 archive was verified against the official published SHA-256.
- Existing canonical gateway health returns `{"ok":true}`; unauthenticated `POST /v1/audio/transcriptions` returns HTTP 401. No test recording has been uploaded to the gateway. The existing production deployment and encrypted settings/state are unchanged; the public repository's backend was built, not deployed over production.
- Source/commit secret scanning passed for the backend import. The Mac patches receive a separate staged scan before publication. The import excludes all private backend history, deployment files, runtime credentials/state/audio, personal dictionary and cleanup prompt.
- Actual microphone recording, native hotkey events, exact paste into a document, and a live authenticated Hypnos dictation remain **pending** until app permissions and a dedicated Mac key are entered through the secure settings UI.

## Remaining acceptance steps

1. Open the installed app. Create a dedicated Mac device key in the authenticated `https://speech.tusharbhardwaj.space/#keys` UI, enter it directly in the app's secure key field, and Save settings.
2. Grant Microphone and Accessibility using the app's controls / macOS Privacy & Security. Refresh hotkey.
3. Run **Run local validation**. It records fresh microphone audio for two seconds, exercises the native global event tap with synthetic key events, pastes a multiline Markdown/code/Unicode fixture into a disposable native text document, and checks cancellation before paste. It makes no server request. Its private report is `~/Library/Application Support/space.hypnos.speech.mac/Validation/latest.json`.
4. Focus a disposable plain-text document. Use Control–Option–Space (or the configured push-to-talk shortcut), speak a short sample specifically for this test, and stop. Check that the saved server text and inserted document match exactly. This is the live acceptance test; mock/local checks cannot substitute for it.
