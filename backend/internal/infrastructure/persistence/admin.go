package persistence

import (
	"backend/internal/domain"
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

type AdminRepository struct{ db *Mongo }

func NewAdminRepository(db *Mongo) *AdminRepository { return &AdminRepository{db} }

func (r *AdminRepository) ListAccounts(ctx context.Context) ([]domain.AdminAccount, error) {
	cur, err := r.db.Users.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetProjection(bson.M{"username": 1}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var users []struct {
		ID       primitive.ObjectID `bson:"_id"`
		Username string             `bson:"username"`
	}
	if err = cur.All(ctx, &users); err != nil {
		return nil, err
	}
	items := make([]domain.AdminAccount, len(users))
	if len(users) == 0 {
		return items, nil
	}
	ids := make([]primitive.ObjectID, len(users))
	positions := make(map[primitive.ObjectID]int, len(users))
	for i, u := range users {
		ids[i], positions[u.ID] = u.ID, i
		items[i] = domain.AdminAccount{ID: u.ID.Hex(), Username: u.Username, Plan: "free"}
	}
	wallets, err := r.db.Wallets.Find(ctx, bson.M{"_id": bson.M{"$in": ids}}, options.Find().SetProjection(bson.M{"plan": 1, "paid_until": 1, "balance": 1}))
	if err != nil {
		return nil, err
	}
	defer wallets.Close(ctx)
	for wallets.Next(ctx) {
		var w struct {
			ID        primitive.ObjectID `bson:"_id"`
			Plan      string             `bson:"plan"`
			PaidUntil *time.Time         `bson:"paid_until"`
			Balance   int64              `bson:"balance"`
		}
		if err = wallets.Decode(&w); err != nil {
			return nil, err
		}
		i := positions[w.ID]
		items[i].Plan, items[i].PaidUntil, items[i].Balance = w.Plan, w.PaidUntil, w.Balance
	}
	return items, wallets.Err()
}

func (r *AdminRepository) Usage(ctx context.Context, userID string) ([]domain.UsageRecord, error) {
	id, err := oid(userID)
	if err != nil {
		return nil, err
	}
	if err = r.db.Users.FindOne(ctx, bson.M{"_id": id}, options.FindOne().SetProjection(bson.M{"_id": 1})).Err(); err != nil {
		return nil, mapErr(err)
	}
	var wallet struct {
		Usage []domain.UsageRecord `bson:"usage"`
	}
	err = r.db.Wallets.FindOne(ctx, bson.M{"_id": id}, options.FindOne().SetProjection(bson.M{"usage": 1})).Decode(&wallet)
	if err == mongo.ErrNoDocuments {
		return []domain.UsageRecord{}, nil
	}
	if err != nil {
		return nil, mapErr(err)
	}
	return wallet.Usage, nil
}

func (r *AdminRepository) SetPlan(ctx context.Context, userID, plan string, now time.Time, credits int64) (*domain.AdminAccount, error) {
	id, err := oid(userID)
	if err != nil {
		return nil, err
	}
	var u struct {
		Username string `bson:"username"`
	}
	if err := r.db.Users.FindOne(ctx, bson.M{"_id": id}, options.FindOne().SetProjection(bson.M{"username": 1})).Decode(&u); err != nil {
		return nil, mapErr(err)
	}
	update := bson.M{"$set": bson.M{
		"plan": plan, "balance": credits, "credit_reset_at": now.Add(2 * time.Hour),
	}}
	account := &domain.AdminAccount{ID: userID, Username: u.Username, Plan: plan, Balance: credits}
	if plan == "paid" {
		until := now.Add(48 * time.Hour)
		update["$set"].(bson.M)["paid_until"] = until
		account.PaidUntil = &until
	} else {
		update["$unset"] = bson.M{"paid_until": ""}
	}
	if _, err := r.db.Wallets.UpdateOne(ctx, bson.M{"_id": id}, update, options.Update().SetUpsert(true)); err != nil {
		return nil, mapErr(err)
	}
	return account, nil
}
