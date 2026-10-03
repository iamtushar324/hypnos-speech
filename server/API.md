# Hypnos Speech API implementation contract

One Go process embeds `web/` and serves the administration application and speech APIs.
Production origin: https://speech.tusharbhardwaj.space. Private Tailscale ingress.
Module: hypnos.local/speech. The server/ directory owns go.mod/go.sum and cmd/speechd; runtime credentials and deployment configuration remain outside this repository.

## Authentication boundary

Package internal/auth exposes New(Config) (*Client,error), Config{SecretKey,
PublishableKey, AllowedGoogleEmail string, AuthorizedParties []string},
Client.VerifyOwner(context.Context, string) (Owner,error), Owner{ID,Email,Name string},
Client.PublishableKey() string, Client.FrontendAPI() string.
VerifyOwner accepts only a valid origin-bound Clerk JWT, verified owner primary
email and same verified Google account, failing closed. No development bypass.
The frontend obtains a fresh JWT from Clerk.session.getToken() and uses
Authorization: Bearer on every API request. Public /api/config is the only
unauthenticated API apart from /healthz. Never put credentials in URL parameters.
Device API keys authorize speech only, never administration routes.

## Gateway interface

Package internal/gateway exposes New(Config) (*Server,error),
Config{StateDir string, MasterKey []byte, PublicURL string, DictionPublicURL string, ClerkPublishableKey string,
ClerkFrontendAPI string, GroqAPIKey string, OpenRouterAPIKey string, VerifyOwner func(context.Context,string)
(auth.Owner,error)}. Server implements http.Handler and exposes Close() error,
MintDeviceKey(name string) (string,error) for an owner-operated local CLI.
Encrypted durable state contains transactional app configuration/history/device
capabilities only; ClickHouse remains the existing long-term analytics system.
AES-256-GCM encryption using a separately mounted master key, atomic writes with
fsync, private files. Do not log request bodies or provider errors containing
secrets/transcripts. Preserve audio artifacts; no automatic cleanup policy.

## JSON API (snake_case throughout)

- GET /api/config: {service_name,public_url,clerk:{publishable_key,frontend_api},diction:{endpoint}}
- GET /api/me: {id,email,name}
- GET /api/dashboard: {stats:{requests,stt_requests,tts_requests,errors,avg_latency_ms,
  estimated_cost_usd},providers:[Provider],recent:[History],system:{version,uptime_seconds}}
- GET /api/models and /v1/models: {data:[Model]}; Model={id,provider,kind,
  name,description,configured,streaming,languages:[string]}. Exact configured flags.
- GET/PUT /api/settings: {default_stt_model,default_tts_model,default_voice,language,
  save_history_text,default_profile_id,cleanup_enabled,cleanup_model,cleanup_prompt,
  cleanup_timeout_seconds}. Defaults Groq Turbo/Orpheus/Troy, language auto.
  Cleanup starts disabled until configured; default model openai/gpt-6-sol, timeout
  10 seconds. Prompt <=32,000 UTF-8 bytes, model <=160 characters, timeout 2..30 seconds.
  Handy's `${output}` placeholder references raw user data without injecting it into
  trusted system instructions. The configured prompt's Markdown formatting is retained.
  Preserve the complete settings object when editing defaults or cleanup separately.
- GET /api/providers: {data:[Provider]}; Provider={id,name,configured,has_api_key,
  endpoint,region,stream_deployment,batch_deployment,verified_at,last_error}.
- PUT /api/providers/{groq|microsoft|openrouter}: {api_key (optional; write-only),endpoint,region,
  stream_deployment,batch_deployment}. Never send stored keys back. Empty omitted key
  preserves credentials; explicit clear_api_key=true removes configured key.
  OpenRouter uses https://openrouter.ai/api/v1; its credential test performs GET /key
  without inference. Provider keys are never returned to the browser or device.
- POST /api/providers/{id}/test: credential/read-only probe where supported;
  {ok,message,verified_at}. Never falsely mark Microsoft verified without provider evidence.
- GET /api/dictionary: {data:[{id,word,replacement,notes}]}
- GET /api/snippets: {data:[{id,trigger,text}]}
- GET /api/profiles: {data:[{id,name,description,language,stt_model,tts_model,voice}]}
- POST collection creates; PUT /api/{collection}/{id} updates; DELETE item removes
  transactional configuration only. Dictionary matches whole words, case insensitive;
  snippets expand exact normalized transcripts to avoid accidental prose replacement.
- GET /api/history?limit=50&q=: {data:[History]}; History={id,kind,model,provider,
  created_at,status,latency_ms,duration_seconds,input_chars,text,estimated_cost_usd,
  error,voice,audio_available}. No raw secrets/errors. Pagination bounded.
  STT also includes raw_text, cleanup_status (off/applied/fallback/skipped_snippet),
  cleanup_model, cleanup_latency_ms, cleanup_error (sanitized), cleanup_cost_usd
  (nullable when unknown), cleanup_input_tokens and cleanup_output_tokens. Raw and
  final text both honor save_history_text. Old history retains its original fields.
  Estimated total includes known cleanup cost alongside the STT estimate.
- GET /api/history/{id}/audio: authenticated audio playback/download.
- GET /api/keys: {data:[{id,name,prefix,created_at,last_used_at,revoked}]}
- POST /api/keys: {name} => {id,name,prefix,token,diction?:{endpoint,pairing_uri,qr_data_url}};
  token and credential-bearing `diction://pair?url=...&key=...` link revealed once.
  QR is a PNG data URL. Neither pairing payload nor token appears in GET /api/keys.
- DELETE /api/keys/{id}: revokes (retains audit/history).
- POST /v1/audio/transcriptions: OpenAI multipart file/model/language/prompt/profile_id,
  response_format=json|text; returns {text,model,provider,latency_ms,id} or plain text.
- POST /v1/audio/speech: {model,input,voice,response_format,profile_id}; streams/returns
  audio/wav. Support Groq 200-char sentence chunks and safe WAV concatenation.
  Optional X-Speech-ID and X-Speech-Latency-Ms headers identify stored history.
- WS /v1/audio/stream: native clients can supply Authorization. Browser first sends
  {type:"auth",token:"JWT or device API key"}; authenticate within five seconds.
  Next {type:"start",model:"mai-transcribe-2-streaming",language:"auto",profile_id:""}.
  Binary frames are 16kHz mono signed little-endian PCM16. Stop with {action:"done"}
  or {type:"commit"}. Sends {type:"ready"}, {type:"partial",text},
  {type:"final",text,model,provider,id}, {type:"error",message}.
  Diction uses the separate capture adapter below; this native protocol requires start/auth.
  Microsoft adapter must genuinely forward audio while recording. Groq batch fallback
  explicitly reports mode:"batch" in ready, and returns final after done; no fake partials.

## Diction capture adapter

Configured through `SPEECH_DICTION_PUBLIC_URL` / `--diction-url`. Main mounts the
adapter beneath `/diction/`; Hypnos Tailscale Serve on 8444 maps its root to that
backend prefix. Client URL is `https://hypnos.husky-canopus.ts.net:8444`.
Clerk admin authorization remains bound to the canonical speech origin.

- GET /: service and endpoint discovery; GET /health: status/version.
- GET /v1/models: Diction/OpenAI model list, grouped provider availability and
  truthful capabilities (`pairing:true`, LLM/edit actions and rotation false).
  These discovery routes are public only within the private tailnet.
- GET /v1/auth/key with a valid `spk_` bearer: {key,rotation:false}.
- POST /v1/audio/transcriptions with bearer: multipart model aliases and dialect
  language normalization, then the shared encrypted-history transcription pipeline.
- WS /v1/audio/stream with bearer in the upgrade request, query model/language/codec.
  Accepts an optional context object, binary PCM16 16kHz mono, then {action:"done"}.
  No auth/start frames. Final is exactly {text,mode:"transcribe"}, then normal close.
  `codec=pcm` and PCM aliases are supported. `codec=opus` works only when the client
  offers `diction.opus.v1`; the server declines it, triggering Diction's PCM fallback.
  Explicit Opus without that negotiation returns 415. Unsupported editing intents
  fail rather than inserting an unprocessed editing instruction.
  Default/legacy Parakeet/Whisper aliases route to the advertised Groq Turbo model.
  Groq still runs batch inference after done. Never advertise unavailable models.

Non-Diction paths under the adapter return 404. Device keys are revocable,
speech-only, retained as hashes; clear pairing credentials are never logged.

Use a fresh, deliberately recorded sample for live validation; never bulk-upload history.

## Transcription cleanup

All successful HTTP/native WebSocket/Diction transcription finals use the optional
shared cleanup stage. Exact snippets bypass the model. Enabled cleanup sends raw
transcript plus every saved dictionary alias, canonical spelling and note in inert
user JSON. The model chooses contextually supported corrections; successful output
is returned directly without deterministic pre/post replacements. Cleanup off or
failed uses exact whole-word dictionary replacements on the original transcript.
Separate system instructions and user data are sent without tools or web plugins.
The complete encoded context, cleanup concurrency and total time are bounded;
oversized context falls back without silently dropping dictionary rows.
Native responses include metadata; Diction still receives exactly {text,mode:"transcribe"}.
Cleanup does not advertise or implement Diction's selected-text editing endpoints.

## Frontend

Web files are vanilla ES modules/CSS, embedded by Go; no build dependency required.
Render a polished responsive admin panel with overview, STT/TTS studio, vocabulary,
snippets, profiles, history, providers/settings and device keys. Signed-out page has
Google login through existing Clerk JS instance; never fake logged-in data. Load
Clerk JS from frontend_api npm path (same proven Toolyard integration). Use mounted
Clerk sign-in or redirect Google authentication, own-origin return URLs.
All real data/API and saving/error/loading states. Browser mic capture uses MediaRecorder
for batch multipart; Microsoft streaming uses AudioContext downsampled PCM16 in small
chunks (no unsupported claim of batch being streaming). Responsive keyboard accessible.
Never include infrastructure implementation jargon in ordinary product flows.
