package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Mongo struct {
	Client        *mongo.Client
	Users         *mongo.Collection
	Sessions      *mongo.Collection
	Conversations *mongo.Collection
	Messages      *mongo.Collection
}

func NewMongo(ctx context.Context, uri, database string) (*Mongo, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}
	db := client.Database(database)
	m := &Mongo{client, db.Collection("users"), db.Collection("sessions"), db.Collection("conversations"), db.Collection("messages")}
	if err := m.ensureIndexes(ctx); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Mongo) ensureIndexes(ctx context.Context) error {
	unique := true
	if _, err := m.Users.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "username", Value: 1}}, Options: &options.IndexOptions{Unique: &unique}}); err != nil {
		return err
	}
	if _, err := m.Sessions.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "token_hash", Value: 1}}, Options: &options.IndexOptions{Unique: &unique}},
		{Keys: bson.D{{Key: "expires_at", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
	}); err != nil {
		return err
	}
	if _, err := m.Conversations.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "owner_id", Value: 1}, {Key: "updated_at", Value: -1}}}); err != nil {
		return err
	}
	_, err := m.Messages.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "conversation_id", Value: 1}, {Key: "created_at", Value: 1}}})
	return err
}

func DBContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}
