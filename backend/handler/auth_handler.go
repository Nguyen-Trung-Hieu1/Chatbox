package handler

import (
	"net/http"
	"time"

	"backend/middleware"
	"backend/service"
)

type AuthHandler struct {
	auth       *service.AuthService
	cookieName string
	secure     bool
}

func NewAuthHandler(a *service.AuthService, name string, secure bool) *AuthHandler {
	return &AuthHandler{a, name, secure}
}

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if err := decodeJSON(r, &in); err != nil {
		writeJSON(w, 400, map[string]string{"error": "dữ liệu không hợp lệ"})
		return
	}
	if err := h.auth.Register(r.Context(), in.Username, in.Password); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 201, map[string]string{"message": "Đăng ký thành công!"})
}
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if err := decodeJSON(r, &in); err != nil {
		writeJSON(w, 400, map[string]string{"error": "dữ liệu không hợp lệ"})
		return
	}
	token, _, u, err := h.auth.Login(r.Context(), in.Username, in.Password)
	if err != nil {
		writeError(w, err)
		return
	}
	// No Expires or MaxAge: this is a browser-session cookie and disappears
	// when the browser session ends. MongoDB still enforces the server-side TTL.
	http.SetCookie(w, &http.Cookie{Name: h.cookieName, Value: token, Path: "/", HttpOnly: true, Secure: h.secure, SameSite: http.SameSiteLaxMode})
	writeJSON(w, 200, map[string]any{"message": "Đăng nhập thành công!", "user": map[string]string{"id": u.ID.Hex(), "username": u.Username}})
}
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(h.cookieName); err == nil {
		_ = h.auth.Logout(r.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: h.cookieName, Value: "", Path: "/", HttpOnly: true, Secure: h.secure, SameSite: http.SameSiteLaxMode, Expires: time.Unix(0, 0), MaxAge: -1})
	writeJSON(w, 200, map[string]string{"message": "Đăng xuất thành công!"})
}
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFromContext(r.Context())
	writeJSON(w, 200, map[string]string{"id": u.ID.Hex(), "username": u.Username})
}
