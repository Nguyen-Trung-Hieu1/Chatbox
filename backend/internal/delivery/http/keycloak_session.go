package httpdelivery

import (
	"backend/internal/domain"
	"backend/internal/infrastructure/persistence"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const sessionLifetime = 30 * 24 * time.Hour

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
	if tokens.AccessToken == "" || tokens.RefreshToken == "" || tokens.ExpiresIn <= 0 {
		return tokens, resp.StatusCode, errors.New("Keycloak returned incomplete tokens")
	}
	return tokens, resp.StatusCode, nil
}

func (s *Server) exchangeKeycloakCode(w http.ResponseWriter, r *http.Request) {
	if s.keycloak == nil || s.sessions == nil {
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
	if err != nil {
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
	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		fail(w, err)
		return
	}
	sessionID := base64.RawURLEncoding.EncodeToString(secret)
	value := persistence.KeycloakSession{AccessToken: tokens.AccessToken, RefreshToken: tokens.RefreshToken, IDToken: tokens.IDToken, AccessExpiry: time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second)}
	if err = s.sessions.Create(r.Context(), sessionID, value, time.Now().Add(sessionLifetime)); err != nil {
		fail(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: s.cookie, Value: sessionID, Path: "/", HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteLaxMode, MaxAge: int(sessionLifetime.Seconds())})
	jsonOut(w, 200, map[string]any{"user": u})
}

func (s *Server) activeSession(ctx context.Context, sessionID string) (persistence.KeycloakSession, error) {
	if s.sessions == nil {
		return persistence.KeycloakSession{}, domain.ErrUnauthorized
	}
	// Serialize refreshes so a rotating refresh token cannot be spent twice by concurrent requests.
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	value, err := s.sessions.Get(ctx, sessionID)
	if err != nil {
		return value, err
	}
	if time.Until(value.AccessExpiry) > 30*time.Second {
		return value, nil
	}
	tokens, status, err := s.requestTokens(ctx, url.Values{"grant_type": {"refresh_token"}, "client_id": {s.keycloak.clientID}, "refresh_token": {value.RefreshToken}})
	if err != nil {
		if status >= 400 && status < 500 {
			_ = s.sessions.Delete(ctx, sessionID)
		}
		return value, err
	}
	value.AccessToken = tokens.AccessToken
	value.RefreshToken = tokens.RefreshToken
	if tokens.IDToken != "" {
		value.IDToken = tokens.IDToken
	}
	value.AccessExpiry = time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second)
	if err := s.sessions.Update(ctx, sessionID, value); err != nil {
		return value, err
	}
	return value, nil
}

func (s *Server) logoutKeycloakSession(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(s.cookie); err == nil && s.sessions != nil {
		if value, err := s.sessions.Get(r.Context(), c.Value); err == nil && s.keycloak != nil {
			values := url.Values{"client_id": {s.keycloak.clientID}, "token": {value.RefreshToken}, "token_type_hint": {"refresh_token"}}
			req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, s.keycloak.issuer+"/protocol/openid-connect/revoke", strings.NewReader(values.Encode()))
			if err == nil {
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				if resp, err := s.keycloak.client.Do(req); err == nil {
					resp.Body.Close()
				}
			}
		}
		_ = s.sessions.Delete(r.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: s.cookie, Value: "", Path: "/", HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	if s.keycloak == nil {
		jsonOut(w, 200, map[string]string{"logout_url": s.origin + "/"})
		return
	}
	logout, _ := url.Parse(s.keycloak.issuer + "/protocol/openid-connect/logout")
	logout.RawQuery = url.Values{"client_id": {s.keycloak.clientID}, "post_logout_redirect_uri": {s.origin + "/"}}.Encode()
	jsonOut(w, 200, map[string]string{"logout_url": logout.String()})
}
