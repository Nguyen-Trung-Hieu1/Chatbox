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
