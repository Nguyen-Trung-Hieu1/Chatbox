package main

import (
	"testing"
	"time"

	"backend/internal/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestWalletUsageBSONRoundTrip(t *testing.T) {
	when := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	original := walletDoc{
		ID:      primitive.NewObjectID(),
		Plan:    "paid",
		Balance: 1770,
		Usage: []domain.UsageRecord{{
			Model: "ag/gemini-3-flash", Kind: "chat", InputTokens: 12,
			OutputTokens: 34, Credits: 7, Reported: true, Status: "completed", CreatedAt: when,
		}},
	}
	raw, err := bson.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var decoded walletDoc
	if err := bson.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Usage) != 1 || decoded.Usage[0].InputTokens != 12 ||
		decoded.Usage[0].OutputTokens != 34 || decoded.Usage[0].Credits != 7 ||
		!decoded.Usage[0].CreatedAt.Equal(when) {
		t.Fatalf("wallet usage changed during BSON decode: %+v", decoded.Usage)
	}
}
