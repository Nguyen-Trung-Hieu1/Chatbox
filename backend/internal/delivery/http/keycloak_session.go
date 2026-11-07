package httpdelivery

import (
	"backend/internal/domain"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const refreshCookieLifetime = 30 * 24 * time.Hour

type keycloakTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

func (s *Server) requestTokens(ctx context.Context, values url.Values) (keycloakTokenResponse, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.keycloak.issuer+"/protocol/openid-connect/token", strings.NewReader(values.Encode()))
	if err != nil {
		return keycloakTokenResponse{}, 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.keycloak.client.Do(req)
	if err != nil {
		return keycloakTokenResponse{}, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return keycloakTokenResponse{}, resp.StatusCode, errors.New("Keycloak rejected token request")
	}
	var tokens keycloakTokenResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&tokens); err != nil {
		return tokens, resp.StatusCode, err
	}
	if tokens.AccessToken == "" || tokens.ExpiresIn <= 0 {
		return tokens, resp.StatusCode, errors.New("Keycloak returned incomplete tokens")
	}
	return tokens, resp.StatusCode, nil
}

func (s *Server) refreshCookieName() string {
	if s.refreshCookie != "" {
		return s.refreshCookie
	}
	return "chatbox_refresh"
}

func (s *Server) setRefreshCookie(w http.ResponseWriter, value string) {
	sameSite := http.SameSiteLaxMode
	if s.secure {
		sameSite = http.SameSiteNoneMode
	}
	http.SetCookie(w, &http.Cookie{
		Name: s.refreshCookieName(), Value: value, Path: "/api/auth", HttpOnly: true,
		Secure: s.secure, SameSite: sameSite, MaxAge: int(refreshCookieLifetime.Seconds()),
	})
}

func (s *Server) clearRefreshCookie(w http.ResponseWriter) {
	sameSite := http.SameSiteLaxMode
	if s.secure {
		sameSite = http.SameSiteNoneMode
	}
	http.SetCookie(w, &http.Cookie{
		Name: s.refreshCookieName(), Value: "", Path: "/api/auth", HttpOnly: true,
		Secure: s.secure, SameSite: sameSite, MaxAge: -1,
	})
}

func (s *Server) clearLegacySessionCookie(w http.ResponseWriter) {
	if s.cookie == "" {
		return
	}
	http.SetCookie(w, &http.Cookie{Name: s.cookie, Value: "", Path: "/", HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteLaxMode, MaxAge: -1})
}

func (s *Server) exchangeKeycloakCode(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	if s.keycloak == nil {
		jsonOut(w, 503, map[string]string{"error": "Keycloak chưa được cấu hình"})
		return
	}
	var in struct {
		Code        string `json:"code"`
		Verifier    string `json:"code_verifier"`
		Nonce       string `json:"nonce"`
		RedirectURI string `json:"redirect_uri"`
	}
	if decode(r, &in) != nil || in.Code == "" || len(in.Code) > 4096 || len(in.Verifier) < 43 || len(in.Verifier) > 128 || len(in.Nonce) < 16 || len(in.Nonce) > 128 || in.RedirectURI != s.origin+"/auth/callback" {
		jsonOut(w, 400, map[string]string{"error": "Yêu cầu đăng nhập không hợp lệ"})
		return
	}
	values := url.Values{"grant_type": {"authorization_code"}, "client_id": {s.keycloak.clientID}, "redirect_uri": {in.RedirectURI}, "code": {in.Code}, "code_verifier": {in.Verifier}}
	tokens, _, err := s.requestTokens(r.Context(), values)
	if err != nil || tokens.RefreshToken == "" {
		jsonOut(w, 401, map[string]string{"error": "Không thể hoàn tất đăng nhập Keycloak"})
		return
	}
	identity, err := s.keycloak.verifyIDToken(tokens.IDToken, in.Nonce)
	if err != nil {
		fail(w, domain.ErrUnauthorized)
		return
	}
	accessIdentity, err := s.keycloak.verifyAccessToken(tokens.AccessToken)
	if err != nil || accessIdentity.Subject != identity.Subject {
		fail(w, domain.ErrUnauthorized)
		return
	}
	u, err := s.auth.ResolveKeycloak(r.Context(), identity.Subject, identity.Username, identity.DisplayName, identity.Email, identity.FirstName, identity.LastName)
	if err != nil {
		fail(w, err)
		return
	}
	if err := s.syncKeycloakRole(r.Context(), u); err != nil {
		fail(w, err)
		return
	}
	s.clearLegacySessionCookie(w)
	s.setRefreshCookie(w, tokens.RefreshToken)
	jsonOut(w, 200, map[string]any{"user": u, "access_token": tokens.AccessToken, "expires_in": tokens.ExpiresIn})
}

func (s *Server) refreshKeycloakToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	if s.keycloak == nil {
		jsonOut(w, 503, map[string]string{"error": "Keycloak chưa được cấu hình"})
		return
	}
	cookie, err := r.Cookie(s.refreshCookieName())
	if err != nil || cookie.Value == "" {
		fail(w, domain.ErrUnauthorized)
		return
	}
	values := url.Values{"grant_type": {"refresh_token"}, "client_id": {s.keycloak.clientID}, "refresh_token": {cookie.Value}}
	tokens, status, err := s.requestTokens(r.Context(), values)
	if err != nil {
		if status >= 400 && status < 500 {
			s.clearRefreshCookie(w)
		}
		fail(w, domain.ErrUnauthorized)
		return
	}
	if tokens.RefreshToken != "" {
		// Persist rotation before downstream role/profile reads can fail.
		s.setRefreshCookie(w, tokens.RefreshToken)
	}
	identity, err := s.keycloak.verifyAccessToken(tokens.AccessToken)
	if err != nil {
		s.clearRefreshCookie(w)
		fail(w, domain.ErrUnauthorized)
		return
	}
	u, err := s.auth.ResolveKeycloak(r.Context(), identity.Subject, identity.Username, identity.DisplayName, identity.Email, identity.FirstName, identity.LastName)
	if err != nil {
		fail(w, err)
		return
	}
	if err := s.syncKeycloakRole(r.Context(), u); err != nil {
		fail(w, err)
		return
	}
	jsonOut(w, http.StatusOK, map[string]any{"user": u, "access_token": tokens.AccessToken, "expires_in": tokens.ExpiresIn})
}

func (s *Server) logoutKeycloakSession(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(s.refreshCookieName()); err == nil && cookie.Value != "" && s.keycloak != nil {
		values := url.Values{"client_id": {s.keycloak.clientID}, "token": {cookie.Value}, "token_type_hint": {"refresh_token"}}
		req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, s.keycloak.issuer+"/protocol/openid-connect/revoke", strings.NewReader(values.Encode()))
		if err == nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			if resp, err := s.keycloak.client.Do(req); err == nil {
				resp.Body.Close()
			}
		}
	}
	s.clearRefreshCookie(w)
	s.clearLegacySessionCookie(w)
	if s.keycloak == nil {
		jsonOut(w, 200, map[string]string{"logout_url": s.origin + "/"})
		return
	}
	logout, _ := url.Parse(s.keycloak.issuer + "/protocol/openid-connect/logout")
	logout.RawQuery = url.Values{"client_id": {s.keycloak.clientID}, "post_logout_redirect_uri": {s.origin + "/"}}.Encode()
	jsonOut(w, 200, map[string]string{"logout_url": logout.String()})
}
