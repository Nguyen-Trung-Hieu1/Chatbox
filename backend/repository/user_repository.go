package repository

import (
	"context"

	"backend/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type UserRepository struct{ db *Mongo }

func NewUserRepository(db *Mongo) *UserRepository { return &UserRepository{db} }
func (r *UserRepository) Create(ctx context.Context, u *models.User) error {
	_, err := r.db.Users.InsertOne(ctx, u)
	return err
}
func (r *UserRepository) ByUsername(ctx context.Context, username string) (*models.User, error) {
	var u models.User
	err := r.db.Users.FindOne(ctx, bson.M{"username": username}).Decode(&u)
	return &u, err
}
func (r *UserRepository) ByID(ctx context.Context, id primitive.ObjectID) (*models.User, error) {
	var u models.User
	err := r.db.Users.FindOne(ctx, bson.M{"_id": id}).Decode(&u)
	return &u, err
}
func IsNotFound(err error) bool { return err == mongo.ErrNoDocuments }
