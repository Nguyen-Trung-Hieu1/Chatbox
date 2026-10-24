package httpdelivery

import (
	"backend/internal/domain"
	"context"
	"net/http"
)

type AdminUseCase interface {
	Users(context.Context) ([]domain.AdminAccount, error)
	Usage(context.Context, string) (*domain.AdminUsageReport, error)
	SetPlan(context.Context, string, string) (*domain.AdminAccount, error)
}

func (s *Server) adminSetPlan(w http.ResponseWriter, r *http.Request) {
	var in domain.AdminPlanChange
	if decode(r, &in) != nil {
		jsonOut(w, 400, map[string]string{"error": "dữ liệu không hợp lệ"})
		return
	}
	account, err := s.admin.SetPlan(r.Context(), in.UserID, in.Plan)
	if err != nil {
		fail(w, err)
		return
	}
	jsonOut(w, 200, account)
}

func (s *Server) adminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !user(r.Context()).IsAdmin {
			fail(w, domain.ErrForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) adminUsers(w http.ResponseWriter, r *http.Request) {
	items, err := s.admin.Users(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	jsonOut(w, 200, items)
}

func (s *Server) adminUsage(w http.ResponseWriter, r *http.Request) {
	report, err := s.admin.Usage(r.Context(), r.URL.Query().Get("user_id"))
	if err != nil {
		fail(w, err)
		return
	}
	jsonOut(w, 200, report)
}
