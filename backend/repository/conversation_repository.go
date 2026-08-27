package repository

import (
	"context"
	"time"

	"backend/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ConversationRepository struct{ db *Mongo }

func NewConversationRepository(db *Mongo) *ConversationRepository { return &ConversationRepository{db} }
func (r *ConversationRepository) Create(ctx context.Context, c *models.Conversation) error {
	_, err := r.db.Conversations.InsertOne(ctx, c)
	return err
}
func (r *ConversationRepository) OwnedByID(ctx context.Context, id, ownerID primitive.ObjectID) (*models.Conversation, error) {
	var c models.Conversation
	err := r.db.Conversations.FindOne(ctx, bson.M{"_id": id, "owner_id": ownerID}).Decode(&c)
	return &c, err
}
func (r *ConversationRepository) ListOwned(ctx context.Context, ownerID primitive.ObjectID) ([]models.Conversation, error) {
	cur, err := r.db.Conversations.Find(ctx, bson.M{"owner_id": ownerID}, options.Find().SetSort(bson.D{{Key: "updated_at", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []models.Conversation
	err = cur.All(ctx, &out)
	return out, err
}
func (r *ConversationRepository) UpdateActivity(ctx context.Context, id primitive.ObjectID, title string) error {
	set := bson.M{"updated_at": time.Now().UTC()}
	if title != "" {
		set["title"] = title
	}
	_, err := r.db.Conversations.UpdateByID(ctx, id, bson.M{"$set": set})
	return err
}
func (r *ConversationRepository) UpdateSummary(ctx context.Context, id primitive.ObjectID, summary string) error {
	_, err := r.db.Conversations.UpdateByID(ctx, id, bson.M{"$set": bson.M{"summary": summary}})
	return err
}
func (r *ConversationRepository) Rename(ctx context.Context, id primitive.ObjectID, title string) error {
	_, err := r.db.Conversations.UpdateByID(ctx, id, bson.M{"$set": bson.M{"title": title, "updated_at": time.Now().UTC()}})
	return err
}
func (r *ConversationRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.db.Conversations.DeleteOne(ctx, bson.M{"_id": id})
	return err
}
