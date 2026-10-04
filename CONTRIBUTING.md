# Contributing to Vokiri

Vokiri is a GPLv3 fork of [VoiceInk](https://github.com/Beingpax/VoiceInk) with its own clients and speech server. Submit Vokiri issues and changes to [iamtushar324/vokiri](https://github.com/iamtushar324/vokiri).

Client code belongs in `apps/<platform>/`; the current Mac app is in `apps/macos/`. Server, web console, and protocol adapter changes belong in `server/`. Keep private deployment manifests, credentials, recordings, personal prompts, and runtime state outside this public repository.

Read [AGENTS.md](AGENTS.md) and [the Mac guide](VOKIRI.md). Work from the `vokiri` branch. Run `swift test` for client core changes, the Vokiri Release build for native UI changes, and `go test -race ./...` plus `go vet ./...` in `server/` for gateway changes. Check `server/web/app.js` with `node --check` after editing it.

Preserve the final-text contract: the Mac must save and deliver the server response exactly, without local rewriting or automatic retries. Changes must preserve cancellation and private recording retention. Describe any live dictation behavior that has not been tested.

Keep upstream native files and type names intact where possible. Our build selects native primitives and generates asserted adapters; broad source renames make reviewed upstream merges harder. Retain copyright and license notices. The stock VoiceInk project remains available for comparison, while our own app identity, keys, data and updater behavior stay independent.

Use existing compatibility identifiers for installed client settings and encrypted server state. Changing a product label must never orphan the Keychain item or invalidate encrypted history. Review [server setup and upgrade notes](server/README.md) before modifying persisted formats.
