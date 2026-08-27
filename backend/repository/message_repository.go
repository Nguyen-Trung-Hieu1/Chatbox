package repository

import (
	"context"

	"backend/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MessageRepository struct{ db *Mongo }

func NewMessageRepository(db *Mongo) *MessageRepository { return &MessageRepository{db} }
func (r *MessageRepository) Create(ctx context.Context, m *models.Message) error {
	_, err := r.db.Messages.InsertOne(ctx, m)
	return err
}
func (r *MessageRepository) List(ctx context.Context, conversationID primitive.ObjectID) ([]models.Message, error) {
	cur, err := r.db.Messages.Find(ctx, bson.M{"conversation_id": conversationID}, options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}, {Key: "_id", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []models.Message
	err = cur.All(ctx, &out)
	return out, err
}
func (r *MessageRepository) Unsummarized(ctx context.Context, conversationID primitive.ObjectID) ([]models.Message, error) {
	cur, err := r.db.Messages.Find(ctx, bson.M{"conversation_id": conversationID, "summarized": bson.M{"$ne": true}}, options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}, {Key: "_id", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []models.Message
	err = cur.All(ctx, &out)
	return out, err
}
func (r *MessageRepository) MarkSummarized(ctx context.Context, ids []primitive.ObjectID) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.db.Messages.UpdateMany(ctx, bson.M{"_id": bson.M{"$in": ids}}, bson.M{"$set": bson.M{"summarized": true}})
	return err
}
func (r *MessageRepository) DeleteByConversation(ctx context.Context, conversationID primitive.ObjectID) error {
	_, err := r.db.Messages.DeleteMany(ctx, bson.M{"conversation_id": conversationID})
	return err
}
