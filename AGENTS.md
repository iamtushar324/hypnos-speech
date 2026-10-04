# Vokiri

This repository is a public, GPLv3 VoiceInk fork and the maintained speech gateway in one monorepo. Never copy the personal vault, runtime credentials/state/audio, personal cleanup prompts/dictionaries, or private backend Git history into it.

- Upstream base: VoiceInk v2.21, `640b0c8c36ee74d9e76d930f40e2c04bb739b4b3`.
- Development/default branch: `vokiri`. Remote `upstream` is Beingpax/VoiceInk; `origin` is iamtushar324/vokiri.
- Keep native client work additive under `apps/macos/` and `scripts/*vokiri*`. Preserve upstream file/type names. Prefer explicit native-source selection/adapters to importing model managers.
- Build: `./scripts/build-vokiri.sh`. Generated project/adapters/build directories are ignored. Only install Vokiri.app.
- Test: `swift test`; server: `cd server && go test -race ./... && go vet ./...`.
- Vokiri text is final. Never trim, filter brackets, normalize paragraphs, append spaces, apply local dictionaries/snippets, or run local enhancement. The native paste path must keep cancellation guards and never run auto-learn.
- Upload only fresh recorded audio and approved fields. Never upload clipboard/selection/screen or historical recordings. No automatic retry.
- Never ask for keys in chat. User enters the dedicated speech-only key in the app's SecureField; Keychain only. Redact secret scans; do not log response error bodies.
- Preserve failed/canceled/interrupted recordings in private storage for explicit user retry. Keep saved successful text for re-paste without re-upload.
- Fetch/merge reviewed upstream versions; regenerate the native target and verify generator contract assertions, tests, signing, and manual dictation after merging.
- Production Vokiri state/deployment is separate. Source builds are not deployments. Do not claim live dictation, UI, hotkeys, or paste verified without evidence.

- Preserve compatibility identifiers: bundle/Keychain/storage `space.hypnos.speech.mac`, existing preference keys, `SPEECH_*` environment variables, and AES-GCM associated data constants. They keep installed clients and encrypted state readable; they are not product branding.
