package usecase

import (
	"backend/internal/domain"
	"context"
	"testing"
)

type memoryAPIKeys struct{ items map[string]*domain.APIKey }

func (m *memoryAPIKeys) Create(_ context.Context, key *domain.APIKey) error {
	key.ID = "key-1"
	m.items[key.Hash] = key
	return nil
}
func (m *memoryAPIKeys) List(context.Context, string) ([]domain.APIKey, error) { return nil, nil }
func (m *memoryAPIKeys) ActiveByHash(_ context.Context, hash string) (*domain.APIKey, error) {
	key := m.items[hash]
	if key == nil || key.RevokedAt != nil {
		return nil, domain.ErrNotFound
	}
	return key, nil
}
func (m *memoryAPIKeys) Revoke(context.Context, string, string) error { return nil }
func (m *memoryAPIKeys) Touch(context.Context, string) error          { return nil }

type apiKeyUsers struct{ domain.UserRepository }

func (apiKeyUsers) ByID(_ context.Context, id string) (*domain.User, error) {
	return &domain.User{ID: id}, nil
}

func TestAPIKeyIsUniqueAndStoredAsHash(t *testing.T) {
	repo := &memoryAPIKeys{items: map[string]*domain.APIKey{}}
	svc := NewAPIKeys(repo, apiKeyUsers{})
	first, raw, err := svc.Create(context.Background(), "owner", "VS Code")
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 46 || first.Hash == raw || first.Hash == "" {
		t.Fatalf("unsafe key: %#v", first)
	}
	if first.Prefix != raw[:11] {
		t.Fatal("wrong prefix")
	}
	_, second, err := svc.Create(context.Background(), "owner", "IDE")
	if err != nil || second == raw {
		t.Fatal("keys should be unique")
	}
	u, err := svc.Authenticate(context.Background(), raw)
	if err != nil || u.ID != "owner" {
		t.Fatalf("authentication failed: %v", err)
	}
	if _, err = svc.Authenticate(context.Background(), "cb_wrong"); err != domain.ErrUnauthorized {
		t.Fatalf("invalid key accepted: %v", err)
	}
	if _, _, err = svc.Create(context.Background(), "owner", "  "); err != domain.ErrInvalidAPIKeyName {
		t.Fatalf("empty name accepted: %v", err)
	}
}
