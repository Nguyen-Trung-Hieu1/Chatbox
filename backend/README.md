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

## Keycloak session flow

The frontend starts authorization code + PKCE login and receives the code at
`/auth/callback`. It sends the code, verifier, and nonce to
`POST /api/auth/exchange`. The backend exchanges the code with Keycloak,
validates the ID and access tokens, and returns the access token in the
response. The frontend keeps the access token in memory and sends it as a
Bearer token. The backend sets the refresh token in an HttpOnly cookie named
by `REFRESH_COOKIE_NAME` (default `chatbox_refresh`) with path `/api/auth`.
Frontend JavaScript cannot read that cookie. `POST /api/auth/refresh` uses the
cookie to get a new access token and updates the cookie when Keycloak rotates
the refresh token. On page reload, the frontend calls this endpoint to restore
its in-memory access token. `POST /api/auth/logout` revokes the refresh token
and clears the cookie.

Neither user access nor refresh tokens are stored in the Chatbox backend
database. The refresh cookie lasts up to 30 days, but Keycloak's SSO idle/max
settings can end the login sooner. `COOKIE_SECURE=true` is required on HTTPS
deployments. The backend accepts state-changing `/api/` requests only when
their Origin matches `CORS_ORIGIN`, including refresh and logout requests.

`AI_TOKEN_BUDGET` controls approximate input
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

### Keycloak-backed roles and plans

Create realm roles `chatbox-free`, `chatbox-paid`, and `chatbox-admin` in the
`chatbox` realm. `chatbox-free` may be a default role. Paid takes precedence
when a user has both Free and Paid (because default roles are inherited).
Give the `chatbox-backend-admin` confidential client's service account only
the `realm-management` `manage-users` role. Keep its secret on the backend.

Before enabling role-backed plans, set `KEYCLOAK_ADMIN_CLIENT_ID` and
`KEYCLOAK_ADMIN_CLIENT_SECRET` in the private backend environment. After the
new backend image is deployed with `KEYCLOAK_ROLES_ENABLED=false`, preview the
one-time migration in K3s with
`sudo k3s kubectl -n chatbox1 exec deployment/chatbox-backend -- /app/migrate-keycloak-roles`.
Review the preview, then run the same command with `-apply`. Locally the
equivalents are `go run ./cmd/migrate-keycloak-roles` and
`go run ./cmd/migrate-keycloak-roles -apply`. It assigns each existing
Keycloak-linked user a role matching the current, unexpired MongoDB plan and
copies the legacy admin flag to `chatbox-admin`; it does not remove an admin
role already assigned in Keycloak. Review the preview before applying.
Set `KEYCLOAK_ROLES_ENABLED=true` and restart the API only after the migration
completes. With this flag off, the existing MongoDB plan behavior remains
active even if the secret is configured.

When enabled, the backend reads effective realm roles on authenticated requests,
uses `chatbox-paid` for model access, and uses `chatbox-admin` for the admin API.
The admin plan action changes the Keycloak role and then updates the MongoDB
credit wallet. A user with only `chatbox-free` gets Free models. API-key calls
also recheck the owner's Keycloak roles. A Keycloak outage fails these requests
closed. Keycloak-backed Paid roles do not use the former 48-hour local expiry.
The Admin API client secret must never be placed in frontend configuration.
Enabling Keycloak roles also disables the legacy password login and legacy
session cookies so the old MongoDB admin flag cannot bypass Keycloak roles.

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
