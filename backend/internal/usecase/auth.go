package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"backend/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

type Auth struct {
	users    domain.UserRepository
	sessions domain.SessionRepository
	ttl      time.Duration
}

func NewAuth(u domain.UserRepository, s domain.SessionRepository, ttl time.Duration) *Auth {
	return &Auth{u, s, ttl}
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (a *Auth) Register(ctx context.Context, username, password string) error {
	username = strings.TrimSpace(username)
	if username == "" || len(password) < 8 {
		return domain.ErrInvalidCredentials
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	err = a.users.Create(ctx, &domain.User{Username: username, Password: string(hash), CreatedAt: time.Now().UTC()})
	if errors.Is(err, domain.ErrDuplicate) {
		return domain.ErrUsernameExists
	}
	return err
}

func (a *Auth) Login(ctx context.Context, username, password string) (string, *domain.User, error) {
	u, err := a.users.ByUsername(ctx, strings.TrimSpace(username))
	if err != nil || bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)) != nil {
		return "", nil, domain.ErrInvalidCredentials
	}
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	now := time.Now().UTC()
	err = a.sessions.Create(ctx, &domain.Session{UserID: u.ID, TokenHash: tokenHash(token), ExpiresAt: now.Add(a.ttl), CreatedAt: now})
	return token, u, err
}

func (a *Auth) Authenticate(ctx context.Context, token string) (*domain.User, error) {
	if token == "" {
		return nil, domain.ErrUnauthorized
	}
	s, err := a.sessions.ActiveByHash(ctx, tokenHash(token))
	if err != nil {
		return nil, domain.ErrUnauthorized
	}
	u, err := a.users.ByID(ctx, s.UserID)
	if err != nil {
		return nil, domain.ErrUnauthorized
	}
	return u, nil
}

func (a *Auth) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	err := a.sessions.DeleteByHash(ctx, tokenHash(token))
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	return err
}
