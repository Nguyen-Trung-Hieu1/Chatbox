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

Existing message documents from the earlier schema used string conversation
IDs and have no owning conversation record. They are deliberately not exposed;
they need an explicit owner-aware migration before they can be accessed.
