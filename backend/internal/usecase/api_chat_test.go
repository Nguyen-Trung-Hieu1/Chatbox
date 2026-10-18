package usecase

import (
	"backend/internal/domain"
	"context"
	"errors"
	"testing"
)

type quotaBilling struct {
	billingStub
	reserved int64
	spent    int64
	record   domain.UsageRecord
}

func (b *quotaBilling) Reserve(_ context.Context, _ string, amount int64) error {
	if b.balance < amount {
		return domain.ErrQuotaExceeded
	}
	b.balance -= amount
	b.reserved = amount
	return nil
}
func (b *quotaBilling) Settle(_ context.Context, _ string, reserved, spent int64, record domain.UsageRecord) error {
	b.balance += reserved - spent
	b.spent, b.record = spent, record
	return nil
}

type fakeAPIClient struct{ calls int }

func (a *fakeAPIClient) Complete(_ context.Context, _ string, _ []domain.AIMessage, _ int) (domain.AIResult, error) {
	a.calls++
	return domain.AIResult{Content: "answer", Usage: domain.AIUsage{InputTokens: 100, OutputTokens: 50, Reported: true}}, nil
}
func (a *fakeAPIClient) Stream(ctx context.Context, model string, messages []domain.AIMessage, max int, emit func(string) error) (domain.AIResult, error) {
	a.calls++
	if err := emit("answer"); err != nil {
		return domain.AIResult{Content: "answer"}, err
	}
	return domain.AIResult{Content: "answer", Usage: domain.AIUsage{InputTokens: 100, OutputTokens: 50, Reported: true}}, nil
}

func TestAPIGenerateChecksQuotaAndSettlesSharedWallet(t *testing.T) {
	p := domain.ModelPolicy{ID: "basic", InputRate: 1, OutputRate: 2, MaxOutputTokens: 100}
	b := &quotaBilling{billingStub: billingStub{plan: "free", balance: 0}}
	ai := &fakeAPIClient{}
	chat := NewChat(nil, nil, ai, 6000, b, []domain.ModelPolicy{p}, []domain.ModelPolicy{p}, 100)
	messages := []domain.AIMessage{{Role: "user", Content: "hello"}}
	if _, err := chat.APIGenerate(context.Background(), "owner", "basic", messages, 0, false, nil); !errors.Is(err, domain.ErrQuotaExceeded) || ai.calls != 0 {
		t.Fatalf("quota did not block AI: %v, calls=%d", err, ai.calls)
	}
	b.balance = 100
	result, err := chat.APIGenerate(context.Background(), "owner", "basic", messages, 0, false, nil)
	if err != nil || ai.calls != 1 || result.Content != "answer" || b.spent != 1 || b.balance != 99 || b.record.Kind != "api" {
		t.Fatalf("incorrect settlement: result=%+v err=%v billing=%+v calls=%d", result, err, b, ai.calls)
	}
}
