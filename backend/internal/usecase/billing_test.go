package usecase

import (
	"backend/internal/domain"
	"context"
	"errors"
	"testing"
	"time"
)

type billingStub struct {
	plan          string
	paidUntil     *time.Time
	creditResetAt *time.Time
	balance       int64
}

func (b *billingStub) Ensure(context.Context, string, int64) error    { return nil }
func (b *billingStub) Balance(context.Context, string) (int64, error) { return b.balance, nil }
func (b *billingStub) Plan(context.Context, string, int64) (string, *time.Time, *time.Time, error) {
	return b.plan, b.paidUntil, b.creditResetAt, nil
}
func (b *billingStub) ActivatePaid(_ context.Context, _ string, now, until time.Time) error {
	if until.Sub(now) != 48*time.Hour {
		return errors.New("wrong paid duration")
	}
	b.plan = "paid"
	b.paidUntil = &until
	resetAt := now.Add(2 * time.Hour)
	b.creditResetAt = &resetAt
	b.balance = 2000
	return nil
}
func (b *billingStub) Reserve(context.Context, string, int64) error { return nil }
func (b *billingStub) Settle(context.Context, string, int64, int64, domain.UsageRecord) error {
	return nil
}
func (b *billingStub) History(context.Context, string) ([]domain.UsageRecord, error) { return nil, nil }

func TestModelEntitlement(t *testing.T) {
	free := domain.ModelPolicy{ID: "basic", InputRate: 1, OutputRate: 2, MaxOutputTokens: 100}
	paid := domain.ModelPolicy{ID: "premium", InputRate: 5, OutputRate: 10, MaxOutputTokens: 100}
	b := &billingStub{plan: "free", balance: 100}
	s := NewChat(nil, nil, nil, 6000, b, []domain.ModelPolicy{free}, []domain.ModelPolicy{free, paid}, 100)
	if _, err := s.policy(context.Background(), "user", "premium"); !errors.Is(err, domain.ErrModelForbidden) {
		t.Fatalf("free premium access: %v", err)
	}
	b.plan = "paid"
	until := time.Now().Add(time.Hour)
	b.paidUntil = &until
	if p, err := s.policy(context.Background(), "user", "premium"); err != nil || p.ID != "premium" {
		t.Fatalf("paid premium access: %v", err)
	}
	if got := credits(paid, 1000, 1000); got != 15 {
		t.Fatalf("credits = %d", got)
	}
	past := time.Now().Add(-time.Minute)
	b.paidUntil = &past
	if _, err := s.policy(context.Background(), "user", "premium"); !errors.Is(err, domain.ErrModelForbidden) {
		t.Fatalf("expired paid plan still allowed premium: %v", err)
	}
}

func TestUpgradeGrantsPaidForTwoDays(t *testing.T) {
	b := &billingStub{plan: "free", balance: 100}
	s := NewChat(nil, nil, nil, 100, b, nil, nil, 100)
	e, err := s.Upgrade(context.Background(), "user")
	if err != nil || e.Plan != "paid" || e.PaidUntil == nil || e.CreditResetAt == nil || e.Balance != 2000 {
		t.Fatalf("upgrade failed: entitlement=%+v err=%v", e, err)
	}
}
