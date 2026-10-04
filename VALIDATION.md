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

- The live gateway is built from the monorepo's `server/` at server-source commit `d5ef296`. Daemon, administration API and Diction share version 0.2.0.
- A disposable upgrade rehearsal created encrypted state with the previous binary, then opened it with Vokiri across two restart cycles. Authentication scope, single-writer locking and persistence passed without provider inference.
- Production health and anonymous-admin rejection passed. Web title, sign-in branding and favicon were inspected in the browser. The service catalog points to the running Vokiri deployment.
- Production encrypted state and retained audio remained byte-identical to their pre-deployment fingerprints. Verified backups and the previous image are retained privately for rollback.
- GitHub repository, default branch, documentation, issue template and contribution guidance use Vokiri. Bare `make` selects the Vokiri build.
