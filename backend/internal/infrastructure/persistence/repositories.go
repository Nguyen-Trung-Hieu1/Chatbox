package persistence

import (
	"backend/internal/domain"
	"context"
	"errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

type userDoc struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	Username  string             `bson:"username"`
	Password  string             `bson:"password"`
	CreatedAt time.Time          `bson:"created_at"`
}
type sessionDoc struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	UserID    primitive.ObjectID `bson:"user_id"`
	TokenHash string             `bson:"token_hash"`
	ExpiresAt time.Time          `bson:"expires_at"`
	CreatedAt time.Time          `bson:"created_at"`
}
type conversationDoc struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	OwnerID   primitive.ObjectID `bson:"owner_id"`
	Title     string             `bson:"title"`
	Summary   string             `bson:"summary,omitempty"`
	CreatedAt time.Time          `bson:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at"`
}
type messageDoc struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"`
	ConversationID primitive.ObjectID `bson:"conversation_id"`
	Role           string             `bson:"role"`
	Content        string             `bson:"content"`
	Summarized     bool               `bson:"summarized"`
	CreatedAt      time.Time          `bson:"created_at"`
}

func oid(s string) (primitive.ObjectID, error) {
	id, e := primitive.ObjectIDFromHex(s)
	if e != nil {
		return id, domain.ErrInvalidID
	}
	return id, nil
}
func mapErr(e error) error {
	if errors.Is(e, mongo.ErrNoDocuments) {
		return domain.ErrNotFound
	}
	if mongo.IsDuplicateKeyError(e) {
		return domain.ErrDuplicate
	}
	return e
}
func userEntity(d userDoc) *domain.User {
	return &domain.User{ID: d.ID.Hex(), Username: d.Username, Password: d.Password, CreatedAt: d.CreatedAt}
}
func conversationEntity(d conversationDoc) *domain.Conversation {
	return &domain.Conversation{ID: d.ID.Hex(), OwnerID: d.OwnerID.Hex(), Title: d.Title, Summary: d.Summary, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt}
}
func messageEntity(d messageDoc) domain.Message {
	return domain.Message{ID: d.ID.Hex(), ConversationID: d.ConversationID.Hex(), Role: d.Role, Content: d.Content, Summarized: d.Summarized, CreatedAt: d.CreatedAt}
}

type UserRepository struct{ db *Mongo }

func NewUserRepository(db *Mongo) *UserRepository { return &UserRepository{db} }
func (r *UserRepository) Create(ctx context.Context, u *domain.User) error {
	d := userDoc{ID: primitive.NewObjectID(), Username: u.Username, Password: u.Password, CreatedAt: u.CreatedAt}
	_, e := r.db.Users.InsertOne(ctx, d)
	if e == nil {
		u.ID = d.ID.Hex()
	}
	return mapErr(e)
}
func (r *UserRepository) ByUsername(ctx context.Context, v string) (*domain.User, error) {
	var d userDoc
	e := r.db.Users.FindOne(ctx, bson.M{"username": v}).Decode(&d)
	return userEntity(d), mapErr(e)
}
func (r *UserRepository) ByID(ctx context.Context, v string) (*domain.User, error) {
	id, e := oid(v)
	if e != nil {
		return nil, e
	}
	var d userDoc
	e = r.db.Users.FindOne(ctx, bson.M{"_id": id}).Decode(&d)
	return userEntity(d), mapErr(e)
}

type SessionRepository struct{ db *Mongo }

func NewSessionRepository(db *Mongo) *SessionRepository { return &SessionRepository{db} }
func (r *SessionRepository) Create(ctx context.Context, s *domain.Session) error {
	uid, e := oid(s.UserID)
	if e != nil {
		return e
	}
	d := sessionDoc{ID: primitive.NewObjectID(), UserID: uid, TokenHash: s.TokenHash, ExpiresAt: s.ExpiresAt, CreatedAt: s.CreatedAt}
	_, e = r.db.Sessions.InsertOne(ctx, d)
	if e == nil {
		s.ID = d.ID.Hex()
	}
	return mapErr(e)
}
func (r *SessionRepository) ActiveByHash(ctx context.Context, h string) (*domain.Session, error) {
	var d sessionDoc
	e := r.db.Sessions.FindOne(ctx, bson.M{"token_hash": h, "expires_at": bson.M{"$gt": time.Now().UTC()}}).Decode(&d)
	return &domain.Session{ID: d.ID.Hex(), UserID: d.UserID.Hex(), TokenHash: d.TokenHash, ExpiresAt: d.ExpiresAt, CreatedAt: d.CreatedAt}, mapErr(e)
}
func (r *SessionRepository) DeleteByHash(ctx context.Context, h string) error {
	res, e := r.db.Sessions.DeleteOne(ctx, bson.M{"token_hash": h})
	if e == nil && res.DeletedCount == 0 {
		return domain.ErrNotFound
	}
	return mapErr(e)
}

type ConversationRepository struct{ db *Mongo }

func NewConversationRepository(db *Mongo) *ConversationRepository { return &ConversationRepository{db} }
func (r *ConversationRepository) Create(ctx context.Context, c *domain.Conversation) error {
	owner, e := oid(c.OwnerID)
	if e != nil {
		return e
	}
	d := conversationDoc{ID: primitive.NewObjectID(), OwnerID: owner, Title: c.Title, Summary: c.Summary, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt}
	_, e = r.db.Conversations.InsertOne(ctx, d)
	if e == nil {
		c.ID = d.ID.Hex()
	}
	return mapErr(e)
}
func (r *ConversationRepository) OwnedByID(ctx context.Context, idv, ownerv string) (*domain.Conversation, error) {
	id, e := oid(idv)
	if e != nil {
		return nil, e
	}
	owner, e := oid(ownerv)
	if e != nil {
		return nil, e
	}
	var d conversationDoc
	e = r.db.Conversations.FindOne(ctx, bson.M{"_id": id, "owner_id": owner}).Decode(&d)
	return conversationEntity(d), mapErr(e)
}
func (r *ConversationRepository) ListOwned(ctx context.Context, v string) ([]domain.Conversation, error) {
	owner, e := oid(v)
	if e != nil {
		return nil, e
	}
	cur, e := r.db.Conversations.Find(ctx, bson.M{"owner_id": owner}, options.Find().SetSort(bson.D{{Key: "updated_at", Value: -1}}))
	if e != nil {
		return nil, e
	}
	defer cur.Close(ctx)
	var docs []conversationDoc
	if e = cur.All(ctx, &docs); e != nil {
		return nil, e
	}
	out := make([]domain.Conversation, len(docs))
	for i, d := range docs {
		out[i] = *conversationEntity(d)
	}
	return out, nil
}
func (r *ConversationRepository) UpdateActivity(ctx context.Context, v, title string) error {
	id, e := oid(v)
	if e != nil {
		return e
	}
	set := bson.M{"updated_at": time.Now().UTC()}
	if title != "" {
		set["title"] = title
	}
	_, e = r.db.Conversations.UpdateByID(ctx, id, bson.M{"$set": set})
	return e
}
func (r *ConversationRepository) UpdateSummary(ctx context.Context, v, summary string) error {
	id, e := oid(v)
	if e != nil {
		return e
	}
	_, e = r.db.Conversations.UpdateByID(ctx, id, bson.M{"$set": bson.M{"summary": summary}})
	return e
}
func (r *ConversationRepository) Rename(ctx context.Context, v, title string) error {
	id, e := oid(v)
	if e != nil {
		return e
	}
	_, e = r.db.Conversations.UpdateByID(ctx, id, bson.M{"$set": bson.M{"title": title, "updated_at": time.Now().UTC()}})
	return e
}
func (r *ConversationRepository) Delete(ctx context.Context, v string) error {
	id, e := oid(v)
	if e != nil {
		return e
	}
	_, e = r.db.Conversations.DeleteOne(ctx, bson.M{"_id": id})
	return e
}

type MessageRepository struct{ db *Mongo }

func NewMessageRepository(db *Mongo) *MessageRepository { return &MessageRepository{db} }
func (r *MessageRepository) Create(ctx context.Context, m *domain.Message) error {
	cid, e := oid(m.ConversationID)
	if e != nil {
		return e
	}
	d := messageDoc{ID: primitive.NewObjectID(), ConversationID: cid, Role: m.Role, Content: m.Content, Summarized: m.Summarized, CreatedAt: m.CreatedAt}
	_, e = r.db.Messages.InsertOne(ctx, d)
	if e == nil {
		m.ID = d.ID.Hex()
	}
	return e
}
func (r *MessageRepository) query(ctx context.Context, filter any) ([]domain.Message, error) {
	cur, e := r.db.Messages.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}, {Key: "_id", Value: 1}}))
	if e != nil {
		return nil, e
	}
	defer cur.Close(ctx)
	var docs []messageDoc
	if e = cur.All(ctx, &docs); e != nil {
		return nil, e
	}
	out := make([]domain.Message, len(docs))
	for i, d := range docs {
		out[i] = messageEntity(d)
	}
	return out, nil
}
func (r *MessageRepository) List(ctx context.Context, v string) ([]domain.Message, error) {
	id, e := oid(v)
	if e != nil {
		return nil, e
	}
	return r.query(ctx, bson.M{"conversation_id": id})
}
func (r *MessageRepository) Unsummarized(ctx context.Context, v string) ([]domain.Message, error) {
	id, e := oid(v)
	if e != nil {
		return nil, e
	}
	return r.query(ctx, bson.M{"conversation_id": id, "summarized": bson.M{"$ne": true}})
}
func (r *MessageRepository) MarkSummarized(ctx context.Context, values []string) error {
	ids := make([]primitive.ObjectID, 0, len(values))
	for _, v := range values {
		id, e := oid(v)
		if e != nil {
			return e
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil
	}
	_, e := r.db.Messages.UpdateMany(ctx, bson.M{"_id": bson.M{"$in": ids}}, bson.M{"$set": bson.M{"summarized": true}})
	return e
}
func (r *MessageRepository) DeleteByConversation(ctx context.Context, v string) error {
	id, e := oid(v)
	if e != nil {
		return e
	}
	_, e = r.db.Messages.DeleteMany(ctx, bson.M{"conversation_id": id})
	return e
}
