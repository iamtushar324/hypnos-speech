# Vokiri validation — 2026-10-04

## Mac rebrand

- Swift core suite: **12 tests passed**, including exact final text, request boundaries, cancellation, retry, and retained recording recovery.
- `scripts/prepare-vokiri-project.py` contract assertions and Release build passed with Xcode 26.3. The target compiles 37 selected Swift sources and Swift Atomics 1.3.0.
- `Vokiri.app` version 0.2.0 built, signed, verified and installed. Its designated signing requirement matches the previously installed Hypnos Speech app.
- Native dashboard inspected: Vokiri title, menu, dashboard, and About label are present. Readiness confirms existing Microphone/Accessibility grants, saved Keychain device key and installed hotkey. The existing shortcut was preserved.
- Every existing recording/audio metadata file matched its pre-install SHA-256. The previous app bundle is retained by the installer for rollback.
- No fresh live transcription or microphone recording was performed as part of this rename. The core tests use synthetic data; readiness is not a claim of end-to-end dictation.

## Server source

- `go test -race ./...`, `go vet ./...`, JavaScript syntax check, and static `vokirid` build passed.
- The Go module is `github.com/iamtushar324/vokiri/server`. Web branding, service metadata and binary use Vokiri.
- Existing state/audio encryption AAD, serialized formats, device key handling, environment variables and API routes are unchanged.
- Initial server import differences were reviewed: the public source intentionally excludes the private owner default, personal prompt and private operational data.

Historical, pre-rebrand results and limitations are preserved in [the earlier validation record](docs/history/2026-10-03-hypnos-validation.md).

## Deployment check

- The initial rebrand deployed the monorepo's `server/` at server-source commit `d5ef296`, version 0.2.0. The subsequent server-only release is recorded below.
- A disposable upgrade rehearsal created encrypted state with the previous binary, then opened it with Vokiri across two restart cycles. Authentication scope, single-writer locking and persistence passed without provider inference.
- Production health and anonymous-admin rejection passed. Web title, sign-in branding and favicon were inspected in the browser. The service catalog points to the running Vokiri deployment.
- Production encrypted state and retained audio remained byte-identical to their pre-deployment fingerprints. Verified backups and the previous image are retained privately for rollback.
- GitHub repository, default branch, documentation, issue template and contribution guidance use Vokiri. Bare `make` selects the Vokiri build.

## Server silence guard — 0.2.1

- Server-source commit `0151f43` is deployed. All client sources and the installed Mac app remain unchanged.
- Go race tests and vet passed on macOS and Linux. Regressions cover silent/empty WAV, faint noise, DC offset, isolated clicks, quiet audible signals, a short signal after long silence, supported sample formats and opposite-phase stereo.
- Both multipart routes and the Groq/Diction batch WebSockets skip speech providers, cleanup, server audio retention and history for silent input. Tests preserve audible input bytes and a legitimate “Thank you.” response.
- A disposable upgrade rehearsal on the deployment host returned empty responses for synthetic one-second silence in 4.87–8.98 ms across two restart cycles. These are local HTTP round trips, not physical microphone or remote-network timings. No microphone recording was performed.
- Production health and anonymous-admin rejection passed, and the Diction adapter reports `vokiri/0.2.1`. Encrypted state and retained audio matched their pre-deployment fingerprints. Verified backups and the previous 0.2.0 image are retained privately.
- This is a conservative energy gate, not learned speech/noise classification. Louder background noise can pass. Unsupported/compressed audio and Microsoft's separate realtime path retain their existing behavior; see [the response contract and scope](server/API.md#silent-captures-server-021).
- Existing clients handle an empty result as before. The current Mac client shows its empty-response status and does not paste; this release introduces no client microphone timeout or new model dependency.
