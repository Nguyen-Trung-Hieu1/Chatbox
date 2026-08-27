package repository

import (
	"context"
	"time"

	"backend/models"
	"go.mongodb.org/mongo-driver/bson"
)

type SessionRepository struct{ db *Mongo }

func NewSessionRepository(db *Mongo) *SessionRepository { return &SessionRepository{db} }
func (r *SessionRepository) Create(ctx context.Context, s *models.Session) error {
	_, err := r.db.Sessions.InsertOne(ctx, s)
	return err
}
func (r *SessionRepository) ActiveByHash(ctx context.Context, hash string) (*models.Session, error) {
	var s models.Session
	err := r.db.Sessions.FindOne(ctx, bson.M{"token_hash": hash, "expires_at": bson.M{"$gt": time.Now().UTC()}}).Decode(&s)
	return &s, err
}
func (r *SessionRepository) DeleteByHash(ctx context.Context, hash string) error {
	_, err := r.db.Sessions.DeleteOne(ctx, bson.M{"token_hash": hash})
	return err
}
