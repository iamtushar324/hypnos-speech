# Vokiri gateway

The Go gateway embeds the responsive administration UI and serves batch speech APIs, device-key management, dictionary/snippets, profiles, encrypted history, and optional server-side cleanup. The Mac client uses `POST /v1/audio/transcriptions`; Groq transcription produces final text after recording stops.

This is the maintained Vokiri server, initially imported from the existing gateway at commit `ff81e39`. Future server changes belong here alongside the clients, not in a separate source repository. Private Git history, deployment manifests, personal cleanup prompt/dictionary, recordings, state, credentials, and machine-specific operational scripts are excluded. Configure your own prompt in the administration UI; [a generic example](prompts/example-cleanup.txt) is included. Existing encrypted deployment state is not changed by this source import.

## Build and test

From this directory, with Go 1.26+:

```sh
go test -race ./...
go vet ./...
node --check web/app.js
CGO_ENABLED=0 go build -trimpath -o /tmp/vokirid ./cmd/vokirid
```

On Linux/Hypnos, build this same `server/` directory in a checkout of the repository. No macOS dependencies are involved. A [Dockerfile](Dockerfile) is provided; build with `server/` as the context. Do not bake configuration, state, or credentials into an image.

## Runtime setup

Use a private network/Tailscale ingress with HTTPS. The personal origin is `https://speech.tusharbhardwaj.space`; deployments set `SPEECH_PUBLIC_URL`. Set the owner explicitly: `SPEECH_OWNER_EMAIL` has no default and authentication fails closed without it.

| Setting | Purpose |
| --- | --- |
| `SPEECH_PUBLIC_URL` | HTTPS administration origin / Clerk authorized party |
| `SPEECH_OWNER_EMAIL` | Verified Google account allowed to administer the deployment |
| `SPEECH_CLERK_PUBLISHABLE_KEY_FILE` | Clerk publishable key file |
| `SPEECH_CLERK_SECRET_KEY_FILE` | Clerk secret key file |
| `SPEECH_MASTER_KEY_FILE` | Exactly 32 binary bytes for AES-256-GCM state encryption |
| `SPEECH_STATE_DIR` | Private durable state directory; default `/state` |
| `SPEECH_ADDR` | Listen address; default `127.0.0.1:8680` |
| `SPEECH_GROQ_API_KEY_FILE` | Groq credential file, or configure through admin |
| `SPEECH_OPENROUTER_API_KEY_FILE` | Optional cleanup credential file, or configure through admin |

Equivalent `SPEECH_*_KEY` environment variables are supported; protected files avoid exposing credentials in process environments. Default paths are under `/credentials`. Keep files mode 0600, outside Git. Back up the state master key separately: losing it makes retained encrypted state unreadable. Never print secrets in logs or pass them in command arguments.

Clerk must enable Google authentication with the exact admin origin. There is no development authentication bypass. Device keys are hashed, revocable, and authorize speech only. Create a dedicated Mac key in the authenticated `/#keys` UI and enter its one-time reveal directly into the Mac client's secure settings field.

Cleanup starts disabled. Enable it in Providers & settings. The server sends new transcript text and saved dictionary context to the selected cleanup provider; existing history is not automatically reprocessed. Groq/OpenRouter processing and retention follow their policies and your configuration. History/audio are encrypted in private durable state; there is no automatic deletion policy.

Diction is an existing optional iPhone adapter with a separate endpoint. The Mac uses the canonical multipart endpoint, never Diction pairing. TTS and streaming remain existing server capabilities and are not part of this Mac release. Microsoft adapters need Azure configuration before use.

See [API.md](API.md) for schemas and authentication boundaries. Personal runtime deployment and recovery procedures remain private, outside this repository.

## Rollback

Builds do not deploy automatically. Keep the previous binary/image and retained state/master key. Reverting source does not revoke device keys or remove history. Revoke a lost Mac key through admin; do not delete production state to roll back a client build.

## Upgrade compatibility

The Vokiri rename preserves the `SPEECH_*` configuration contract, API routes, device key hashes, serialized state and audio formats. The AES-GCM associated-data identifiers retain their original `hypnos-speech-*` values: changing them would make existing encrypted state unreadable. Keep your existing state directory, master key, origin and Clerk configuration when deploying this source. The binary is now `vokirid`; update the service executable/image entrypoint, not the data.

For Linux hosts using local containerd, `scripts/build-image.py` packages a static Linux/amd64 binary and the host CA bundle into an OCI archive without a registry push. Production manifests, runtime keys and backups remain deployment-owned.

## Silence handling

Before transcription, the server skips supported WAV captures with no sustained audible signal. This prevents the speech model from inventing words for a silent mic open/close. No transcription/cleanup call or server history entry is made. Native Mac WAV and Diction PCM captures are covered; see [the exact formats, responses, and limitations](API.md#silent-captures-server-021). Client capture controls and text handling remain unchanged.
