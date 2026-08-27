package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"time"

	"backend/models"
	"backend/repository"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	users    *repository.UserRepository
	sessions *repository.SessionRepository
	ttl      time.Duration
}

func NewAuthService(u *repository.UserRepository, s *repository.SessionRepository, ttl time.Duration) *AuthService {
	return &AuthService{u, s, ttl}
}
func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func (s *AuthService) Register(ctx context.Context, username, password string) error {
	username = strings.TrimSpace(username)
	if username == "" || len(password) < 8 {
		return ErrInvalidCredentials
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u := &models.User{ID: primitive.NewObjectID(), Username: username, Password: string(hash), CreatedAt: time.Now().UTC()}
	if err = s.users.Create(ctx, u); mongo.IsDuplicateKeyError(err) {
		return ErrUsernameExists
	}
	return err
}
func (s *AuthService) Login(ctx context.Context, username, password string) (string, time.Time, *models.User, error) {
	u, err := s.users.ByUsername(ctx, strings.TrimSpace(username))
	if err != nil || bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)) != nil {
		return "", time.Time{}, nil, ErrInvalidCredentials
	}
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", time.Time{}, nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	expires := time.Now().UTC().Add(s.ttl)
	err = s.sessions.Create(ctx, &models.Session{ID: primitive.NewObjectID(), UserID: u.ID, TokenHash: tokenHash(token), ExpiresAt: expires, CreatedAt: time.Now().UTC()})
	return token, expires, u, err
}
func (s *AuthService) Authenticate(ctx context.Context, token string) (*models.User, error) {
	if token == "" {
		return nil, ErrUnauthorized
	}
	session, err := s.sessions.ActiveByHash(ctx, tokenHash(token))
	if err != nil {
		return nil, ErrUnauthorized
	}
	u, err := s.users.ByID(ctx, session.UserID)
	if err != nil {
		return nil, ErrUnauthorized
	}
	return u, nil
}
func (s *AuthService) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	err := s.sessions.DeleteByHash(ctx, tokenHash(token))
	if err == mongo.ErrNoDocuments {
		return nil
	}
	return err
}
