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
}
type ChatUseCase interface {
	CreateConversation(context.Context, string, string) (*domain.Conversation, error)
	List(context.Context, string) ([]domain.Conversation, error)
	History(context.Context, string, string) ([]domain.Message, error)
	Rename(context.Context, string, string, string) (*domain.Conversation, error)
	Delete(context.Context, string, string) error
	SendStream(context.Context, string, string, string, func(string) error) (*domain.Message, error)
}
type Server struct {
	auth   AuthUseCase
	chat   ChatUseCase
	cookie string
	secure bool
}

func New(auth *usecase.Auth, chat *usecase.Chat, cookie string, secure bool) *Server {
	return &Server{auth, chat, cookie, secure}
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
	mux.HandleFunc("/api/logout", method("POST", s.logout))
	mux.Handle("/api/me", s.authn(method("GET", s.me)))
	mux.Handle("/api/conversations", s.authn(http.HandlerFunc(s.conversations)))
	mux.Handle("/api/history", s.authn(method("GET", s.history)))
	mux.Handle("/api/chat", s.authn(method("POST", s.send)))
	return cors(origin, mux)
}
func cors(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
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
	var in credentials
	if decode(r, &in) != nil {
		jsonOut(w, 400, map[string]string{"error": "dữ liệu không hợp lệ"})
		return
	}
	if e := s.auth.Register(r.Context(), in.Username, in.Password); e != nil {
		fail(w, e)
		return
	}
	jsonOut(w, 201, map[string]string{"message": "Đăng ký thành công!"})
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if decode(r, &in) != nil {
		jsonOut(w, 400, map[string]string{"error": "dữ liệu không hợp lệ"})
		return
	}
	token, u, e := s.auth.Login(r.Context(), in.Username, in.Password)
	if e != nil {
		fail(w, e)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: s.cookie, Value: token, Path: "/", HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteLaxMode})
	jsonOut(w, 200, map[string]any{"message": "Đăng nhập thành công!", "user": map[string]string{"id": u.ID, "username": u.Username}})
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, e := r.Cookie(s.cookie); e == nil {
		_ = s.auth.Logout(r.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: s.cookie, Value: "", Path: "/", HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteLaxMode, Expires: time.Unix(0, 0), MaxAge: -1})
	jsonOut(w, 200, map[string]string{"message": "Đăng xuất thành công!"})
}
func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	u := user(r.Context())
	jsonOut(w, 200, map[string]string{"id": u.ID, "username": u.Username})
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
			fail(w, e)
			return
		}
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
			fail(w, e)
			return
		}
		jsonOut(w, 200, c)
	case "DELETE":
		if e := s.chat.Delete(r.Context(), r.URL.Query().Get("id"), u.ID); e != nil {
			fail(w, e)
			return
		}
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
	}
	if decode(r, &in) != nil || strings.TrimSpace(in.Content) == "" {
		jsonOut(w, 400, map[string]string{"error": "nội dung không hợp lệ"})
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
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
	reply, e := s.chat.SendStream(r.Context(), in.ConversationID, in.Content, user(r.Context()).ID, emit)
	if e != nil {
		if r.Context().Err() == nil {
			_ = encoder.Encode(map[string]any{"type": "error", "error": e.Error()})
			flusher.Flush()
		}
		return
	}
	_ = encoder.Encode(map[string]any{"type": "done", "message": reply})
	flusher.Flush()
}
