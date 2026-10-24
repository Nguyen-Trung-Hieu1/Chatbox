# Chatbox backend

Backend follows Clean Architecture. Dependencies always point inward:

- `internal/domain`: entities, business errors, and ports (interfaces)
- `internal/usecase`: application rules for authentication and chat
- `internal/infrastructure`: MongoDB and AI implementations of domain ports
- `internal/delivery/http`: HTTP handlers, routing, cookies, middleware and JSON
- `internal/config`: environment configuration
- `cmd/api`: composition root; the only place that wires concrete adapters

Run the API with `go run ./cmd/api`.

Copy `.env.example` values into the process environment before starting. The
application intentionally does not load `.env` files or contain API secrets.

## API flow

All frontend requests must use cookies (`credentials: "include"`).

1. `POST /api/register` with `{ "username": "...", "password": "..." }`
2. `POST /api/login` with the same shape; returns an HttpOnly browser-session
   cookie, which has no persistent `Expires`/`Max-Age` value
3. `GET /api/me` validates the current session
4. `POST /api/conversations` creates a conversation and returns its ObjectID
5. `GET /api/conversations` lists only conversations owned by the current user
6. `POST /api/chat` with `{ "conversation_id": "ObjectID", "content": "..." }`
7. `GET /api/history?id=ObjectID` returns owned conversation history
8. `POST /api/logout` revokes the server-side session and clears the cookie

MongoDB has a TTL index on `sessions.expires_at`, while middleware also rejects
expired sessions immediately. `AI_TOKEN_BUDGET` controls approximate input
context size. Older unsummarized messages are incrementally folded into the
conversation's rolling summary.

## Stage 1 plans and usage

`GET /api/entitlement` returns the account plan, remaining internal credits and
allowed models. `GET /api/usage` returns the most recent 1,000 chat and summary
usage records. `POST /api/chat` accepts an optional `model` from that allowed
list. The backend checks the model and atomically reserves estimated maximum
credits before calling 9Router. It then settles the reservation using reported
usage when available, or an explicitly marked estimate. Failed calls with no
content or usage are refunded. Credits are internal units, not a monetary price.

The built-in catalog offers four models to Free accounts and ten to Paid
accounts. `AI_FREE_MODELS_JSON` and `AI_PAID_MODELS_JSON` can override either
list with model objects containing `id`, optional `display_name`, `input_rate`,
`output_rate` and `max_output_tokens`. `display_name` changes only the selector;
9Router always receives the full `id`. Rates are trial credits per 1,000 tokens,
not verified provider prices. New accounts get
`FREE_INITIAL_CREDITS` (default 1,000) when a wallet is created and at each
two-hour reset. Existing users receive a wallet on first access. Upgrading to
Paid sets the balance to 2,000 credits; Paid resets to 2,000 every two hours.
The next reset is applied on the first request after its deadline.

## Admin usage dashboard

An account needs `is_admin: true` in the MongoDB `users` collection to open
`/admin`. For a fresh local database, set `ADMIN_BOOTSTRAP_USERNAME` and
`ADMIN_BOOTSTRAP_PASSWORD` in the private `backend/.env` file. On startup the
backend creates that account with a bcrypt password hash and admin permission.
An existing non-admin username is never promoted automatically. Alternatively,
grant an existing account through an operator session in `mongosh`:

```javascript
db.users.updateOne({ username: "admin" }, { $set: { is_admin: true } })
```

The admin API returns user plans and daily Web/API token usage. Dates use
Vietnam time (UTC+7). The existing wallet stores only its 1,000 most recent
usage records per user, so older activity is not included in the report.

An operator can grant test access from the backend directory with:

```
go run ./cmd/manage-account -username alice -plan paid -add-credits 10000
```

This command uses the same MongoDB environment as the API and is never exposed
as an HTTP endpoint. Payment integration and automatic periodic grants are not
part of stage 1.

## API keys for IDEs (text chat)

Sign in to Chatbox, open **API keys** in the sidebar, create a named key and
copy it immediately. Only its SHA-256 hash is stored; the full key cannot be
retrieved later. Keys can be revoked from the same panel. A revoked key stops
authenticating new requests. The panel also shows the current API base URL.

Configure an IDE extension that supports an OpenAI-compatible Chat Completions
provider with the Chatbox base URL (`http://localhost:8080/v1` locally), the
`cb_...` API key, and a model ID returned by `GET /v1/models`. The key is sent
as `Authorization: Bearer cb_...`. The following example uses a Free model:

When using the bundled frontend reverse proxy, the URL shown in the web panel
has the form `https://your-chatbox-domain/backend/v1`.

```powershell
$env:CHATBOX_API_KEY = "cb_..."
curl.exe http://localhost:8080/v1/chat/completions -H "Authorization: Bearer $env:CHATBOX_API_KEY" -H "Content-Type: application/json" -d '{"model":"ag/gemini-3-flash","messages":[{"role":"user","content":"Hello"}]}'
```

`POST /v1/chat/completions` supports plain text messages with `system`,
`developer`, `user`, or `assistant` roles, optional `max_tokens` or
`max_completion_tokens`, and `stream: true` using OpenAI-style SSE chunks.
`stream_options.include_usage` includes a final usage chunk. Tool calls,
multimodal content, JSON response formatting, and multiple choices are not yet
supported. IDE requests are not saved as Chatbox conversations. Their usage is
recorded with `kind: "api"` in the same wallet and can be read from
`GET /api/usage` after signing in on the web.

Continue Edit uses `POST /v1/completions`. Chatbox accepts a text `prompt`,
optional `max_tokens`, and `stream: true`; it returns OpenAI-style text
completion chunks. This endpoint uses the same API key authentication, model
policy, and wallet as Chat Completions. The prompt is forwarded to 9Router as
a user message because the upstream connection uses Chat Completions. Prompt
arrays, suffix insertion, echo, and multiple choices are not supported.

For VS Code with the Continue extension, a local `config.yaml` entry can use:

```yaml
name: Chatbox
version: 1.0.0
schema: v1
models:
  - name: Chatbox Gemini Flash
    provider: openai
    model: ag/gemini-3-flash
    apiBase: http://localhost:8080/v1
    apiKey: ${{ secrets.CHATBOX_API_KEY }}
    useResponsesApi: false
    roles:
      - chat
      - edit
      - apply
```

Replace `apiBase` with the URL shown in the Chatbox panel when using a deployed
server. This supports text chat and Continue Edit/Apply requests; Continue Agent features
requiring tool calls need a later API extension.

On Windows, Continue's local configuration is
`%USERPROFILE%\.continue\config.yaml`. Put the API key in
`%USERPROFILE%\.continue\.env` as `CHATBOX_API_KEY=cb_...` (no quotes), then
select the local config in Continue. Keep this `.env` file outside the project
and do not commit it. Reload the VS Code window after adding the secret.

The web's cookie-based `/api/chat` and the key-based `/v1/chat/completions`
share model policies and billing. An IDE request is rejected before 9Router is
called if the model is not allowed or the wallet cannot cover the reservation.

Existing message documents from the earlier schema used string conversation
IDs and have no owning conversation record. They are deliberately not exposed;
they need an explicit owner-aware migration before they can be accessed.
