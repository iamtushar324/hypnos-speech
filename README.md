# Vokiri

**Your voice, across your apps.** A lean native Mac dictation client and a self-hosted speech server, together in one monorepo.

Press a shortcut, speak, and stop. Vokiri sends the fresh recording to your server and pastes its final text exactly. The server manages transcription models, dictionary, snippets, profiles, cleanup, device keys, and encrypted history.

## Repository

| Path | Purpose |
| --- | --- |
| `apps/macos/` | Native Swift client, resources, and core tests |
| `server/` | Go speech gateway, embedded web administration, and API adapters |
| `scripts/` | Mac build, installation, and artwork generation |
| `VoiceInk/`, `VoiceInk.xcodeproj/` | Preserved upstream source for reviewed merges |

Future first-party clients belong under `apps/<platform>/` and use the same server API. The existing Diction iPhone integration is a server adapter; the Diction app itself is a separate project.

## Get started

```sh
git clone https://github.com/iamtushar324/vokiri.git
cd vokiri
./scripts/build-vokiri.sh
open "$HOME/Applications/Vokiri.app"
```

Requires macOS 15+, Xcode and Python 3. Configure a dedicated speech-only device key in the app. Default shortcut: **Control–Option–Space**. [Mac setup and privacy details](VOKIRI.md).

Build and test both components:

```sh
swift test
cd server
go test -race ./...
go vet ./...
go build -o vokirid ./cmd/vokirid
```

The server requires Go 1.26+, private persistent storage, authentication and provider configuration. See [server setup](server/README.md) and the [API contract](server/API.md). The [Dockerfile](server/Dockerfile) uses `server/` as its build context. Building does not deploy automatically.

## Design

- Native recording, hotkeys, Keychain, recording indicator and paste; no local model downloads or inference frameworks.
- Only fresh recorded audio and speech request fields leave the Mac. No clipboard, selection or screen uploads.
- The returned text is final: no local rewriting, dictionary replacements or hidden enhancement.
- Failed and canceled recordings stay local for explicit retry. Saved text can be pasted again without re-upload.
- Runtime keys, recordings, personal prompts and deployment state stay outside this public repository.

Vokiri replaces the earlier Hypnos Speech product name. Existing Mac security/storage identifiers, server encryption formats, environment variables and endpoints remain compatible so the rename preserves keys and history. See [migration details](VOKIRI.md#upgrading-from-hypnos-speech) and [validation](VALIDATION.md).

## Upstream and license

Vokiri is a genuine fork of [VoiceInk by Pax](https://github.com/Beingpax/VoiceInk), based on **v2.21** (`640b0c8`). The original native components, copyright and [GPLv3 license](LICENSE) are retained. Vokiri development uses the `vokiri` branch. The stock VoiceInk target remains available for upstream review; use the Vokiri build script for this app.

The server was initially imported from a separately maintained gateway at `ff81e39`; its maintained source now lives in `server/`. Private deployment history was not imported. Read [contribution guidance](CONTRIBUTING.md) before changing either component.
