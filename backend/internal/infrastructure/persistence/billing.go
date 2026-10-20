package persistence

import (
	"backend/internal/domain"
	"context"
	"errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

type BillingRepository struct{ db *Mongo }

const paidCredits int64 = 2000
const creditPeriod = 2 * time.Hour

func NewBillingRepository(db *Mongo) *BillingRepository { return &BillingRepository{db} }

func (r *BillingRepository) Ensure(ctx context.Context, user string, initial int64) error {
	id, err := oid(user)
	if err != nil {
		return err
	}
	_, err = r.db.Wallets.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$setOnInsert": bson.M{"plan": "free", "balance": initial, "credit_reset_at": time.Now().UTC().Add(creditPeriod), "usage": bson.A{}}}, options.Update().SetUpsert(true))
	return err
}
func (r *BillingRepository) Balance(ctx context.Context, user string) (int64, error) {
	id, err := oid(user)
	if err != nil {
		return 0, err
	}
	var d struct {
		Balance int64 `bson:"balance"`
	}
	err = r.db.Wallets.FindOne(ctx, bson.M{"_id": id}).Decode(&d)
	return d.Balance, mapErr(err)
}
func (r *BillingRepository) Plan(ctx context.Context, user string, freeCredits int64) (string, *time.Time, *time.Time, error) {
	id, err := oid(user)
	if err != nil {
		return "", nil, nil, err
	}
	now := time.Now().UTC()
	_, err = r.db.Wallets.UpdateOne(ctx,
		bson.M{"_id": id, "plan": "paid", "$or": bson.A{
			bson.M{"paid_until": bson.M{"$lte": now}},
			bson.M{"paid_until": bson.M{"$exists": false}},
		}},
		bson.M{"$set": bson.M{"plan": "free", "balance": freeCredits, "credit_reset_at": now.Add(creditPeriod)}, "$unset": bson.M{"paid_until": ""}},
	)
	if err != nil {
		return "", nil, nil, err
	}
	_, err = r.db.Wallets.UpdateOne(ctx,
		bson.M{"_id": id, "plan": "paid", "paid_until": bson.M{"$gt": now}, "$or": bson.A{
			bson.M{"credit_reset_at": bson.M{"$lte": now}},
			bson.M{"credit_reset_at": bson.M{"$exists": false}},
		}},
		bson.M{"$set": bson.M{"balance": paidCredits, "credit_reset_at": now.Add(creditPeriod)}},
	)
	if err != nil {
		return "", nil, nil, err
	}
	_, err = r.db.Wallets.UpdateOne(ctx,
		bson.M{"_id": id, "plan": "free", "$or": bson.A{
			bson.M{"credit_reset_at": bson.M{"$lte": now}},
			bson.M{"credit_reset_at": bson.M{"$exists": false}},
		}},
		bson.M{"$set": bson.M{"balance": freeCredits, "credit_reset_at": now.Add(creditPeriod)}},
	)
	if err != nil {
		return "", nil, nil, err
	}
	var d struct {
		Plan          string     `bson:"plan"`
		PaidUntil     *time.Time `bson:"paid_until"`
		CreditResetAt *time.Time `bson:"credit_reset_at"`
	}
	err = r.db.Wallets.FindOne(ctx, bson.M{"_id": id}).Decode(&d)
	return d.Plan, d.PaidUntil, d.CreditResetAt, mapErr(err)
}
func (r *BillingRepository) ActivatePaid(ctx context.Context, user string, now, until time.Time) error {
	id, err := oid(user)
	if err != nil {
		return err
	}
	result, err := r.db.Wallets.UpdateOne(ctx,
		bson.M{"_id": id, "$or": bson.A{
			bson.M{"plan": bson.M{"$ne": "paid"}},
			bson.M{"paid_until": bson.M{"$lte": now}},
			bson.M{"paid_until": bson.M{"$exists": false}},
		}},
		bson.M{"$set": bson.M{"plan": "paid", "paid_until": until, "balance": paidCredits, "credit_reset_at": now.Add(creditPeriod)}},
	)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return domain.ErrForbidden
	}
	return nil
}
func (r *BillingRepository) Reserve(ctx context.Context, user string, amount int64) error {
	if amount < 0 {
		return errors.New("invalid reserve")
	}
	if amount == 0 {
		return nil
	}
	id, err := oid(user)
	if err != nil {
		return err
	}
	result, err := r.db.Wallets.UpdateOne(ctx, bson.M{"_id": id, "balance": bson.M{"$gte": amount}}, bson.M{"$inc": bson.M{"balance": -amount}})
	if err != nil {
		return err
	}
	if result.ModifiedCount == 0 {
		return domain.ErrQuotaExceeded
	}
	return nil
}
func (r *BillingRepository) Settle(ctx context.Context, user string, reserved, spent int64, record domain.UsageRecord) error {
	id, err := oid(user)
	if err != nil {
		return err
	}
	if spent < 0 {
		spent = 0
	}
	if spent > reserved {
		spent = reserved
	}
	record.Credits = spent
	result, err := r.db.Wallets.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$inc": bson.M{"balance": reserved - spent}, "$push": bson.M{"usage": bson.M{"$each": bson.A{record}, "$slice": -1000}}})
	if err != nil {
		return err
	}
	if result.ModifiedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}
func (r *BillingRepository) History(ctx context.Context, user string) ([]domain.UsageRecord, error) {
	id, err := oid(user)
	if err != nil {
		return nil, err
	}
	var d struct {
		Usage []domain.UsageRecord `bson:"usage"`
	}
	err = r.db.Wallets.FindOne(ctx, bson.M{"_id": id}).Decode(&d)
	return d.Usage, mapErr(err)
}
