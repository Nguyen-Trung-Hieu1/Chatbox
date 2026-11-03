package usecase

import (
	"backend/internal/domain"
	"context"
	"errors"
	"testing"
	"time"
)

type adminRepoStub struct {
	plan    string
	credits int64
}

type roleRepoStub struct {
	adminRepoStub
	plan string
}

func (*roleRepoStub) KeycloakSub(context.Context, string) (string, error) { return "kc-user", nil }
func (r *roleRepoStub) SyncRolePlan(_ context.Context, _ string, plan string, _ int64) error {
	r.plan = plan
	return nil
}
func (r *roleRepoStub) SetRolePlan(_ context.Context, id, plan string, credits int64) (*domain.AdminAccount, error) {
	r.plan = plan
	return &domain.AdminAccount{ID: id, Plan: plan, Balance: credits}, nil
}

type roleManagerStub struct{ roles map[string]bool }

func (r *roleManagerStub) Roles(context.Context, string) (map[string]bool, error) {
	return r.roles, nil
}
func (r *roleManagerStub) SetPlan(_ context.Context, _ string, plan string) error {
	r.roles["chatbox-paid"] = plan == "paid"
	return nil
}

func TestKeycloakPaidOverridesInheritedFree(t *testing.T) {
	repo := &roleRepoStub{}
	admin := NewAdmin(repo, 1000)
	admin.UseKeycloakRoles(&roleManagerStub{roles: map[string]bool{"chatbox-free": true, "chatbox-paid": true, "chatbox-admin": true}})
	u := &domain.User{ID: "local", KeycloakSub: "kc-user"}
	if err := admin.SyncUser(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	if repo.plan != "paid" || !u.IsAdmin {
		t.Fatalf("plan=%s admin=%t", repo.plan, u.IsAdmin)
	}
}

func (*adminRepoStub) ListAccounts(context.Context) ([]domain.AdminAccount, error) { return nil, nil }
func (*adminRepoStub) Usage(context.Context, string) ([]domain.UsageRecord, error) { return nil, nil }
func (r *adminRepoStub) SetPlan(_ context.Context, id, plan string, _ time.Time, credits int64) (*domain.AdminAccount, error) {
	r.plan, r.credits = plan, credits
	return &domain.AdminAccount{ID: id, Plan: plan, Balance: credits}, nil
}

func TestAdminSetPlanValidatesAndSetsCredits(t *testing.T) {
	repo := &adminRepoStub{}
	admin := NewAdmin(repo, 1000)
	if _, err := admin.SetPlan(context.Background(), "id", "invalid"); !errors.Is(err, domain.ErrInvalidPlan) {
		t.Fatalf("expected invalid plan, got %v", err)
	}
	for _, tc := range []struct {
		plan    string
		credits int64
	}{{"paid", 2000}, {"free", 1000}} {
		account, err := admin.SetPlan(context.Background(), "id", tc.plan)
		if err != nil || account.Balance != tc.credits || repo.plan != tc.plan || repo.credits != tc.credits {
			t.Fatalf("plan=%s account=%+v err=%v repo=%+v", tc.plan, account, err, repo)
		}
	}
}

func TestSummarizeUsageSeparatesWebAndAPIByVietnamDate(t *testing.T) {
	records := []domain.UsageRecord{
		{Model: "model-a", Kind: "chat", InputTokens: 100, OutputTokens: 20, CreatedAt: time.Date(2026, 10, 20, 18, 0, 0, 0, time.UTC)},
		{Model: "model-b", Kind: "api", InputTokens: 30, OutputTokens: 10, CreatedAt: time.Date(2026, 10, 20, 19, 0, 0, 0, time.UTC)},
		{Model: "model-a", Kind: "summary", InputTokens: 5, OutputTokens: 5, CreatedAt: time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)},
	}
	report := summarizeUsage(records)
	if len(report.Days) != 2 || report.Days[0].Date != "2026-10-21" || report.Days[0].WebInput != 100 || report.Days[0].WebOutput != 20 || report.Days[0].APIInput != 30 || report.Days[0].APIOutput != 10 {
		t.Fatalf("unexpected daily usage: %+v", report.Days)
	}
	if report.TopModel == nil || report.TopModel.ID != "model-a" || report.TopModel.Tokens != 130 {
		t.Fatalf("unexpected top model: %+v", report.TopModel)
	}
}
