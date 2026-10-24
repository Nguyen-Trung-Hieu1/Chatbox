package usecase

import (
	"backend/internal/domain"
	"context"
	"golang.org/x/crypto/bcrypt"
	"testing"
)

type bootstrapUsers struct {
	domain.UserRepository
	user *domain.User
}

func (r *bootstrapUsers) ByUsername(_ context.Context, username string) (*domain.User, error) {
	if r.user == nil || r.user.Username != username {
		return nil, domain.ErrNotFound
	}
	return r.user, nil
}
func (r *bootstrapUsers) Create(_ context.Context, user *domain.User) error {
	r.user = user
	return nil
}

func TestBootstrapAdminCreatesHashedAccountOnce(t *testing.T) {
	repo := &bootstrapUsers{}
	auth := NewAuth(repo, nil, 0)
	if err := auth.BootstrapAdmin(context.Background(), "admin", "123456789"); err != nil {
		t.Fatal(err)
	}
	if repo.user == nil || !repo.user.IsAdmin || bcrypt.CompareHashAndPassword([]byte(repo.user.Password), []byte("123456789")) != nil {
		t.Fatal("admin was not stored with a bcrypt password hash")
	}
	firstHash := repo.user.Password
	if err := auth.BootstrapAdmin(context.Background(), "admin", "123456789"); err != nil {
		t.Fatal(err)
	}
	if repo.user.Password != firstHash {
		t.Fatal("restart changed the admin password")
	}
}

func TestBootstrapAdminDoesNotPromoteExistingUser(t *testing.T) {
	repo := &bootstrapUsers{user: &domain.User{Username: "admin", Password: "existing"}}
	auth := NewAuth(repo, nil, 0)
	if err := auth.BootstrapAdmin(context.Background(), "admin", "123456789"); err == nil {
		t.Fatal("existing account was promoted")
	}
	if repo.user.IsAdmin {
		t.Fatal("existing user gained admin access")
	}
}
