package httpdelivery

import (
	"backend/internal/domain"
	"backend/internal/usecase"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

type AuthUseCase interface {
	Register(context.Context, string, string) error
	Login(context.Context, string, string) (string, *domain.User, error)
	Authenticate(context.Context, string) (*domain.User, error)
	Logout(context.Context, string) error
	LoginKeycloak(context.Context, string, string, string) (string, *domain.User, error)
}
type ChatUseCase interface {
	CreateConversation(context.Context, string, string) (*domain.Conversation, error)
	List(context.Context, string) ([]domain.Conversation, error)
	History(context.Context, string, string) ([]domain.Message, error)
	Rename(context.Context, string, string, string) (*domain.Conversation, error)
	Delete(context.Context, string, string) error
	SendStream(context.Context, string, string, string, string, func(string) error) (*domain.Message, error)
	Entitlement(context.Context, string) (*domain.Entitlement, error)
	UsageHistory(context.Context, string) ([]domain.UsageRecord, error)
	APIGenerate(context.Context, string, string, []domain.AIMessage, int, bool, func(string) error) (domain.AIResult, error)
}
type APIKeyUseCase interface {
	Create(context.Context, string, string) (*domain.APIKey, string, error)
	List(context.Context, string) ([]domain.APIKey, error)
	Revoke(context.Context, string, string) error
	Authenticate(context.Context, string) (*domain.User, error)
}
type Server struct {
	auth     AuthUseCase
	chat     ChatUseCase
	keys     APIKeyUseCase
	admin    AdminUseCase
	cookie   string
	secure   bool
	keycloak *keycloakVerifier
}

func New(auth *usecase.Auth, chat *usecase.Chat, keys *usecase.APIKeys, admin AdminUseCase, cookie string, secure bool, keycloakConfig ...string) *Server {
	s := &Server{auth: auth, chat: chat, keys: keys, admin: admin, cookie: cookie, secure: secure}
	if len(keycloakConfig) == 2 {
		s.keycloak = newKeycloakVerifier(keycloakConfig[0], keycloakConfig[1])
	}
	return s
}

type contextKey int

const userKey contextKey = 0

func user(ctx context.Context) *domain.User { u, _ := ctx.Value(userKey).(*domain.User); return u }
func jsonOut(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, e error) {
	status := 500
	switch {
	case errors.Is(e, domain.ErrInvalidCredentials), errors.Is(e, domain.ErrUsernameExists), errors.Is(e, domain.ErrInvalidID), errors.Is(e, domain.ErrInvalidTitle):
		status = 400
	case errors.Is(e, domain.ErrUnauthorized):
		status = 401
	case errors.Is(e, domain.ErrForbidden):
		status = 403
	case errors.Is(e, domain.ErrModelForbidden):
		status = 403
	case errors.Is(e, domain.ErrQuotaExceeded):
		status = 402
	case errors.Is(e, domain.ErrNotFound):
		status = 404
	case errors.Is(e, domain.ErrInvalidContent), errors.Is(e, domain.ErrInvalidAPIKeyName), errors.Is(e, domain.ErrInvalidPlan):
		status = 400
	}
	jsonOut(w, status, map[string]string{"error": e.Error()})
}
func decode(r *http.Request, v any) error {
	return json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(v)
}
func method(m string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != m {
			w.WriteHeader(405)
			return
		}
		h(w, r)
	}
}
func (s *Server) authn(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, e := r.Cookie(s.cookie)
		if e != nil {
			fail(w, domain.ErrUnauthorized)
			return
		}
		u, e := s.auth.Authenticate(r.Context(), c.Value)
		if e != nil {
			fail(w, domain.ErrUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, u)))
	})
}
func (s *Server) Handler(origin string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/register", method("POST", s.register))
	mux.HandleFunc("/api/login", method("POST", s.login))
	mux.HandleFunc("/api/keycloak/login", method("POST", s.keycloakLogin))
	mux.HandleFunc("/api/logout", method("POST", s.logout))
	mux.Handle("/api/me", s.authn(method("GET", s.me)))
	mux.Handle("/api/conversations", s.authn(http.HandlerFunc(s.conversations)))
	mux.Handle("/api/history", s.authn(method("GET", s.history)))
	mux.Handle("/api/chat", s.authn(method("POST", s.send)))
	mux.Handle("/api/entitlement", s.authn(method("GET", s.entitlement)))
	mux.Handle("/api/usage", s.authn(method("GET", s.usage)))
	mux.Handle("/api/keys", s.authn(http.HandlerFunc(s.manageKeys)))
	mux.Handle("/api/admin/users", s.authn(s.adminOnly(method("GET", s.adminUsers))))
	mux.Handle("/api/admin/usage", s.authn(s.adminOnly(method("GET", s.adminUsage))))
	mux.Handle("/api/admin/plan", s.authn(s.adminOnly(method("PATCH", s.adminSetPlan))))
	mux.Handle("/v1/models", s.apiAuth(method("GET", s.openAIModels)))
	mux.Handle("/v1/chat/completions", s.apiAuth(method("POST", s.openAIChat)))
	mux.Handle("/v1/completions", s.apiAuth(method("POST", s.openAICompletions)))
	return requestLogging(cors(origin, mux))
}
func cors(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Add("Vary", "Origin")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	if s.keycloak != nil {
		jsonOut(w, http.StatusGone, map[string]string{"error": "Đăng ký đã chuyển sang Keycloak"})
		return
	}
	var in credentials
	if decode(r, &in) != nil {
		logEvent(r, "warn", "register_failed", map[string]any{"reason": "invalid_input"})
		jsonOut(w, 400, map[string]string{"error": "dữ liệu không hợp lệ"})
		return
	}
	if e := s.auth.Register(r.Context(), in.Username, in.Password); e != nil {
		logEvent(r, "warn", "register_failed", map[string]any{"reason": errorCode(e)})
		fail(w, e)
		return
	}
	logEvent(r, "info", "register_success", nil)
	jsonOut(w, 201, map[string]string{"message": "Đăng ký thành công!"})
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if decode(r, &in) != nil {
		logEvent(r, "warn", "login_failed", map[string]any{"reason": "invalid_input"})
		jsonOut(w, 400, map[string]string{"error": "dữ liệu không hợp lệ"})
		return
	}
	token, u, e := s.auth.Login(r.Context(), in.Username, in.Password)
	if e != nil {
		logEvent(r, "warn", "login_failed", map[string]any{"reason": errorCode(e)})
		fail(w, e)
		return
	}
	logEvent(r, "info", "login_success", map[string]any{"user_id": u.ID})
	http.SetCookie(w, &http.Cookie{Name: s.cookie, Value: token, Path: "/", HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteLaxMode})
	jsonOut(w, 200, map[string]any{"message": "Đăng nhập thành công!", "user": map[string]any{"id": u.ID, "username": u.Username, "is_admin": u.IsAdmin}})
}

func (s *Server) keycloakLogin(w http.ResponseWriter, r *http.Request) {
	if s.keycloak == nil {
		jsonOut(w, 503, map[string]string{"error": "Keycloak chưa được cấu hình"})
		return
	}
	var in struct {
		IDToken string `json:"id_token"`
		Nonce   string `json:"nonce"`
	}
	if decode(r, &in) != nil {
		jsonOut(w, 400, map[string]string{"error": "dữ liệu không hợp lệ"})
		return
	}
	identity, err := s.keycloak.verifyIDToken(in.IDToken, in.Nonce)
	if err != nil {
		logEvent(r, "warn", "keycloak_login_failed", map[string]any{"reason": "invalid_token"})
		fail(w, domain.ErrUnauthorized)
		return
	}
	token, u, err := s.auth.LoginKeycloak(r.Context(), identity.Subject, identity.Username, identity.DisplayName)
	if err != nil {
		fail(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: s.cookie, Value: token, Path: "/", HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteLaxMode, MaxAge: 86400})
	jsonOut(w, 200, map[string]any{"user": map[string]any{"id": u.ID, "username": u.Username, "display_name": u.DisplayName, "is_admin": u.IsAdmin}})
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, e := r.Cookie(s.cookie); e == nil {
		_ = s.auth.Logout(r.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: s.cookie, Value: "", Path: "/", HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteLaxMode, Expires: time.Unix(0, 0), MaxAge: -1})
	logEvent(r, "info", "logout_success", nil)
	jsonOut(w, 200, map[string]string{"message": "Đăng xuất thành công!"})
}
func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	u := user(r.Context())
	jsonOut(w, 200, map[string]any{"id": u.ID, "username": u.Username, "display_name": u.DisplayName, "is_admin": u.IsAdmin})
}
func (s *Server) entitlement(w http.ResponseWriter, r *http.Request) {
	e, err := s.chat.Entitlement(r.Context(), user(r.Context()).ID)
	if err != nil {
		fail(w, err)
		return
	}
	jsonOut(w, 200, e)
}
func (s *Server) usage(w http.ResponseWriter, r *http.Request) {
	h, err := s.chat.UsageHistory(r.Context(), user(r.Context()).ID)
	if err != nil {
		fail(w, err)
		return
	}
	if h == nil {
		h = []domain.UsageRecord{}
	}
	jsonOut(w, 200, h)
}
func (s *Server) conversations(w http.ResponseWriter, r *http.Request) {
	u := user(r.Context())
	switch r.Method {
	case "GET":
		list, e := s.chat.List(r.Context(), u.ID)
		if e != nil {
			fail(w, e)
			return
		}
		if list == nil {
			list = []domain.Conversation{}
		}
		jsonOut(w, 200, list)
	case "POST":
		var in struct {
			Title string `json:"title"`
		}
		_ = decode(r, &in)
		c, e := s.chat.CreateConversation(r.Context(), u.ID, in.Title)
		if e != nil {
			logEvent(r, "error", "conversation_create_failed", map[string]any{"reason": errorCode(e)})
			fail(w, e)
			return
		}
		logEvent(r, "info", "conversation_created", map[string]any{"user_id": u.ID, "conversation_id": c.ID})
		jsonOut(w, 201, c)
	case "PATCH":
		var in struct {
			Title string `json:"title"`
		}
		if decode(r, &in) != nil {
			jsonOut(w, 400, map[string]string{"error": "dữ liệu không hợp lệ"})
			return
		}
		c, e := s.chat.Rename(r.Context(), r.URL.Query().Get("id"), in.Title, u.ID)
		if e != nil {
			logEvent(r, "warn", "conversation_rename_failed", map[string]any{"reason": errorCode(e)})
			fail(w, e)
			return
		}
		logEvent(r, "info", "conversation_renamed", map[string]any{"user_id": u.ID, "conversation_id": c.ID})
		jsonOut(w, 200, c)
	case "DELETE":
		conversationID := r.URL.Query().Get("id")
		if e := s.chat.Delete(r.Context(), conversationID, u.ID); e != nil {
			logEvent(r, "warn", "conversation_delete_failed", map[string]any{"reason": errorCode(e)})
			fail(w, e)
			return
		}
		logEvent(r, "info", "conversation_deleted", map[string]any{"user_id": u.ID, "conversation_id": conversationID})
		w.WriteHeader(204)
	default:
		w.WriteHeader(405)
	}
}
func (s *Server) history(w http.ResponseWriter, r *http.Request) {
	h, e := s.chat.History(r.Context(), r.URL.Query().Get("id"), user(r.Context()).ID)
	if e != nil {
		fail(w, e)
		return
	}
	if h == nil {
		h = []domain.Message{}
	}
	jsonOut(w, 200, h)
}
func (s *Server) send(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ConversationID string `json:"conversation_id"`
		Content        string `json:"content"`
		Model          string `json:"model"`
	}
	if decode(r, &in) != nil || strings.TrimSpace(in.Content) == "" {
		logEvent(r, "warn", "chat_failed", map[string]any{"reason": "invalid_input"})
		jsonOut(w, 400, map[string]string{"error": "nội dung không hợp lệ"})
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		logEvent(r, "error", "chat_failed", map[string]any{"reason": "streaming_unsupported"})
		jsonOut(w, 500, map[string]string{"error": "máy chủ không hỗ trợ streaming"})
		return
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	encoder := json.NewEncoder(w)
	emit := func(v string) error {
		if e := encoder.Encode(map[string]any{"type": "delta", "content": v}); e != nil {
			return e
		}
		flusher.Flush()
		return nil
	}
	reply, e := s.chat.SendStream(r.Context(), in.ConversationID, in.Content, user(r.Context()).ID, in.Model, emit)
	if e != nil {
		logEvent(r, "error", "chat_failed", map[string]any{"reason": errorCode(e), "conversation_id": in.ConversationID})
		if r.Context().Err() == nil {
			_ = encoder.Encode(map[string]any{"type": "error", "error": e.Error()})
			flusher.Flush()
		}
		return
	}
	logEvent(r, "info", "chat_completed", map[string]any{"user_id": user(r.Context()).ID, "conversation_id": in.ConversationID})
	_ = encoder.Encode(map[string]any{"type": "done", "message": reply})
	flusher.Flush()
}

func errorCode(e error) string {
	switch {
	case errors.Is(e, domain.ErrInvalidCredentials):
		return "invalid_credentials"
	case errors.Is(e, domain.ErrUsernameExists):
		return "username_exists"
	case errors.Is(e, domain.ErrUnauthorized):
		return "unauthorized"
	case errors.Is(e, domain.ErrForbidden):
		return "forbidden"
	case errors.Is(e, domain.ErrModelForbidden):
		return "model_forbidden"
	case errors.Is(e, domain.ErrQuotaExceeded):
		return "quota_exceeded"
	case errors.Is(e, domain.ErrInvalidID):
		return "invalid_id"
	case errors.Is(e, domain.ErrInvalidTitle):
		return "invalid_title"
	case errors.Is(e, domain.ErrNotFound):
		return "not_found"
	case errors.Is(e, domain.ErrDuplicate):
		return "duplicate"
	default:
		return "internal_error"
	}
}
