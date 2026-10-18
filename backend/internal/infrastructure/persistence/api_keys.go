package persistence

import (
	"backend/internal/domain"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type apiKeyDoc struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"`
	UserID     primitive.ObjectID `bson:"user_id"`
	Name       string             `bson:"name"`
	Prefix     string             `bson:"prefix"`
	Hash       string             `bson:"hash"`
	CreatedAt  time.Time          `bson:"created_at"`
	LastUsedAt *time.Time         `bson:"last_used_at,omitempty"`
	RevokedAt  *time.Time         `bson:"revoked_at,omitempty"`
}

type APIKeyRepository struct{ db *Mongo }

func NewAPIKeyRepository(db *Mongo) *APIKeyRepository { return &APIKeyRepository{db} }

func apiKeyEntity(d apiKeyDoc) *domain.APIKey {
	return &domain.APIKey{ID: d.ID.Hex(), UserID: d.UserID.Hex(), Name: d.Name, Prefix: d.Prefix, Hash: d.Hash, CreatedAt: d.CreatedAt, LastUsedAt: d.LastUsedAt, RevokedAt: d.RevokedAt}
}

func (r *APIKeyRepository) Create(ctx context.Context, key *domain.APIKey) error {
	uid, err := oid(key.UserID)
	if err != nil {
		return err
	}
	d := apiKeyDoc{ID: primitive.NewObjectID(), UserID: uid, Name: key.Name, Prefix: key.Prefix, Hash: key.Hash, CreatedAt: key.CreatedAt}
	if _, err = r.db.APIKeys.InsertOne(ctx, d); err != nil {
		return mapErr(err)
	}
	key.ID = d.ID.Hex()
	return nil
}

func (r *APIKeyRepository) List(ctx context.Context, user string) ([]domain.APIKey, error) {
	uid, err := oid(user)
	if err != nil {
		return nil, err
	}
	cur, err := r.db.APIKeys.Find(ctx, bson.M{"user_id": uid}, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(100))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var docs []apiKeyDoc
	if err = cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	keys := make([]domain.APIKey, len(docs))
	for i, d := range docs {
		keys[i] = *apiKeyEntity(d)
	}
	return keys, nil
}

func (r *APIKeyRepository) ActiveByHash(ctx context.Context, hash string) (*domain.APIKey, error) {
	var d apiKeyDoc
	err := r.db.APIKeys.FindOne(ctx, bson.M{"hash": hash, "revoked_at": bson.M{"$exists": false}}).Decode(&d)
	if err != nil {
		return nil, mapErr(err)
	}
	return apiKeyEntity(d), nil
}

func (r *APIKeyRepository) Revoke(ctx context.Context, user, id string) error {
	uid, err := oid(user)
	if err != nil {
		return err
	}
	keyID, err := oid(id)
	if err != nil {
		return err
	}
	res, err := r.db.APIKeys.UpdateOne(ctx, bson.M{"_id": keyID, "user_id": uid, "revoked_at": bson.M{"$exists": false}}, bson.M{"$set": bson.M{"revoked_at": time.Now().UTC()}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *APIKeyRepository) Touch(ctx context.Context, id string) error {
	keyID, err := oid(id)
	if err != nil {
		return err
	}
	_, err = r.db.APIKeys.UpdateOne(ctx, bson.M{"_id": keyID}, bson.M{"$set": bson.M{"last_used_at": time.Now().UTC()}})
	return err
}
