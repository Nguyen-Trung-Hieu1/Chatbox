package usecase

import (
	"backend/internal/domain"
	"context"
	"errors"
	"testing"
)

type billingStub struct {
	plan    string
	balance int64
}

func (b *billingStub) Ensure(context.Context, string, int64) error    { return nil }
func (b *billingStub) Balance(context.Context, string) (int64, error) { return b.balance, nil }
func (b *billingStub) Plan(context.Context, string) (string, error)   { return b.plan, nil }
func (b *billingStub) Reserve(context.Context, string, int64) error   { return nil }
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
	if p, err := s.policy(context.Background(), "user", "premium"); err != nil || p.ID != "premium" {
		t.Fatalf("paid premium access: %v", err)
	}
	if got := credits(paid, 1000, 1000); got != 15 {
		t.Fatalf("credits = %d", got)
	}
}
