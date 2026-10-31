package persistence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type KeycloakSession struct {
	AccessToken  string    `bson:"access_token"`
	RefreshToken string    `bson:"refresh_token"`
	IDToken      string    `bson:"id_token"`
	AccessExpiry time.Time `bson:"access_expiry"`
}

type keycloakSessionDoc struct {
	TokenHash string          `bson:"token_hash"`
	Tokens    KeycloakSession `bson:"tokens"`
	ExpiresAt time.Time       `bson:"expires_at"`
}

type KeycloakSessionStore struct{ collection *mongo.Collection }

func NewKeycloakSessionStore(ctx context.Context, db *Mongo) (*KeycloakSessionStore, error) {
	collection := db.Client.Database(db.Users.Database().Name()).Collection("keycloak_sessions")
	_, err := collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "token_hash", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "expires_at", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
	})
	if err != nil {
		return nil, err
	}
	return &KeycloakSessionStore{collection: collection}, nil
}

func sessionHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func (s *KeycloakSessionStore) Create(ctx context.Context, token string, value KeycloakSession, expiresAt time.Time) error {
	_, err := s.collection.InsertOne(ctx, keycloakSessionDoc{TokenHash: sessionHash(token), Tokens: value, ExpiresAt: expiresAt})
	return err
}

func (s *KeycloakSessionStore) Get(ctx context.Context, token string) (KeycloakSession, error) {
	var doc keycloakSessionDoc
	err := s.collection.FindOne(ctx, bson.M{"token_hash": sessionHash(token), "expires_at": bson.M{"$gt": time.Now().UTC()}}).Decode(&doc)
	if err != nil {
		return KeycloakSession{}, err
	}
	return doc.Tokens, nil
}

func (s *KeycloakSessionStore) Update(ctx context.Context, token string, value KeycloakSession) error {
	result, err := s.collection.UpdateOne(ctx, bson.M{"token_hash": sessionHash(token), "expires_at": bson.M{"$gt": time.Now().UTC()}}, bson.M{"$set": bson.M{"tokens": value}})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (s *KeycloakSessionStore) Delete(ctx context.Context, token string) error {
	_, err := s.collection.DeleteOne(ctx, bson.M{"token_hash": sessionHash(token)})
	return err
}
