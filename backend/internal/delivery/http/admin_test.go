package httpdelivery

import (
	"backend/internal/domain"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type adminStub struct{}

func (adminStub) Users(context.Context) ([]domain.AdminAccount, error) {
	return []domain.AdminAccount{}, nil
}
func (adminStub) Usage(context.Context, string) (*domain.AdminUsageReport, error) {
	return &domain.AdminUsageReport{Days: []domain.DailyUsage{}}, nil
}
func (adminStub) SetPlan(_ context.Context, id, plan string) (*domain.AdminAccount, error) {
	return &domain.AdminAccount{ID: id, Plan: plan}, nil
}

func TestAdminOnlyRejectsNonAdmin(t *testing.T) {
	s := &Server{admin: adminStub{}}
	for _, tc := range []struct {
		admin  bool
		status int
	}{{false, 403}, {true, 200}} {
		r := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
		r = r.WithContext(context.WithValue(r.Context(), userKey, &domain.User{IsAdmin: tc.admin}))
		w := httptest.NewRecorder()
		s.adminOnly(method("GET", s.adminUsers)).ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("admin=%t: got %d, want %d", tc.admin, w.Code, tc.status)
		}
	}
}

func TestAdminSetPlanRequiresAdmin(t *testing.T) {
	s := &Server{admin: adminStub{}}
	for _, tc := range []struct {
		admin  bool
		status int
	}{{false, 403}, {true, 200}} {
		r := httptest.NewRequest(http.MethodPatch, "/api/admin/plan", strings.NewReader(`{"user_id":"abc","plan":"paid"}`))
		r = r.WithContext(context.WithValue(r.Context(), userKey, &domain.User{IsAdmin: tc.admin}))
		w := httptest.NewRecorder()
		s.adminOnly(method("PATCH", s.adminSetPlan)).ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("admin=%t: got %d, want %d", tc.admin, w.Code, tc.status)
		}
	}
}
