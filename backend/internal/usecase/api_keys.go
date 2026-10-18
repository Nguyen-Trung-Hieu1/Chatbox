package usecase

import (
	"backend/internal/domain"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

type APIKeys struct {
	repo  domain.APIKeyRepository
	users domain.UserRepository
}

func NewAPIKeys(repo domain.APIKeyRepository, users domain.UserRepository) *APIKeys {
	return &APIKeys{repo: repo, users: users}
}

func apiKeyHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (s *APIKeys) Create(ctx context.Context, user, name string) (*domain.APIKey, string, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < 1 || n > 80 {
		return nil, "", domain.ErrInvalidAPIKeyName
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return nil, "", err
	}
	raw := "cb_" + base64.RawURLEncoding.EncodeToString(random)
	key := &domain.APIKey{UserID: user, Name: name, Prefix: raw[:11], Hash: apiKeyHash(raw), CreatedAt: time.Now().UTC()}
	if err := s.repo.Create(ctx, key); err != nil {
		return nil, "", err
	}
	return key, raw, nil
}

func (s *APIKeys) List(ctx context.Context, user string) ([]domain.APIKey, error) {
	return s.repo.List(ctx, user)
}

func (s *APIKeys) Revoke(ctx context.Context, user, id string) error {
	return s.repo.Revoke(ctx, user, id)
}

func (s *APIKeys) Authenticate(ctx context.Context, raw string) (*domain.User, error) {
	if !strings.HasPrefix(raw, "cb_") || len(raw) != 46 {
		return nil, domain.ErrUnauthorized
	}
	key, err := s.repo.ActiveByHash(ctx, apiKeyHash(raw))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrUnauthorized
		}
		return nil, err
	}
	u, err := s.users.ByID(ctx, key.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrUnauthorized
		}
		return nil, err
	}
	_ = s.repo.Touch(ctx, key.ID)
	return u, nil
}
