// Migrate the five active Chatbox collections from MongoDB to PostgreSQL.
// The source is read-only. By default this command only previews source counts.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type userDoc struct {
	ID          primitive.ObjectID `bson:"_id"`
	Username    string             `bson:"username"`
	DisplayName string             `bson:"display_name"`
	Email       string             `bson:"email"`
	FirstName   string             `bson:"first_name"`
	LastName    string             `bson:"last_name"`
	KeycloakSub string             `bson:"keycloak_sub"`
	IsAdmin     bool               `bson:"is_admin"`
	CreatedAt   time.Time          `bson:"created_at"`
}

type conversationDoc struct {
	ID        primitive.ObjectID `bson:"_id"`
	OwnerID   primitive.ObjectID `bson:"owner_id"`
	Title     string             `bson:"title"`
	Summary   string             `bson:"summary"`
	CreatedAt time.Time          `bson:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at"`
}

type messageDoc struct {
	ID             primitive.ObjectID `bson:"_id"`
	ConversationID primitive.ObjectID `bson:"conversation_id"`
	Role           string             `bson:"role"`
	Content        string             `bson:"content"`
	Summarized     bool               `bson:"summarized"`
	CreatedAt      time.Time          `bson:"created_at"`
}

type walletDoc struct {
	ID            primitive.ObjectID   `bson:"_id"`
	Plan          string               `bson:"plan"`
	PlanSource    string               `bson:"plan_source"`
	Balance       int64                `bson:"balance"`
	PaidUntil     *time.Time           `bson:"paid_until"`
	CreditResetAt *time.Time           `bson:"credit_reset_at"`
	Usage         []domain.UsageRecord `bson:"usage"`
}

type apiKeyDoc struct {
	ID         primitive.ObjectID `bson:"_id"`
	UserID     primitive.ObjectID `bson:"user_id"`
	Name       string             `bson:"name"`
	Prefix     string             `bson:"prefix"`
	Hash       string             `bson:"hash"`
	CreatedAt  time.Time          `bson:"created_at"`
	LastUsedAt *time.Time         `bson:"last_used_at"`
	RevokedAt  *time.Time         `bson:"revoked_at"`
}

var sourceNames = []string{"users", "conversations", "messages", "wallets", "api_keys"}
var targetNames = []string{"users", "conversations", "messages", "wallets", "usage_records", "api_keys"}

func main() {
	apply := flag.Bool("apply", false, "write to PostgreSQL; default is preview only")
	flag.Parse()
	if err := run(*apply); err != nil {
		log.Fatal(err)
	}
}

func run(apply bool) error {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		return errors.New("MONGO_URI is required")
	}
	database := os.Getenv("MONGO_DATABASE")
	if database == "" {
		database = "Chatbox1"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return fmt.Errorf("connect MongoDB: %w", err)
	}
	defer mongoClient.Disconnect(context.Background())
	if err := mongoClient.Ping(ctx, nil); err != nil {
		return fmt.Errorf("ping MongoDB: %w", err)
	}
	source := mongoClient.Database(database)
	expected := make(map[string]int64, len(sourceNames))
	for _, name := range sourceNames {
		count, err := source.Collection(name).CountDocuments(ctx, bson.D{})
		if err != nil {
			return fmt.Errorf("count MongoDB %s: %w", name, err)
		}
		expected[name] = count
		fmt.Printf("MongoDB %-14s %d\n", name, count)
	}
	if !apply {
		fmt.Println("Preview only. Re-run with -apply to write to PostgreSQL.")
		return nil
	}
	pgConfig, err := pgx.ParseConfig("host=app-postgres.database.svc.cluster.local port=5432 dbname=app_db sslmode=disable")
	if err != nil {
		return err
	}
	pgConfig.User, pgConfig.Password = os.Getenv("PGUSER"), os.Getenv("PGPASSWORD")
	if pgConfig.User == "" || pgConfig.Password == "" {
		return errors.New("PGUSER and PGPASSWORD are required")
	}
	pg, err := pgx.ConnectConfig(ctx, pgConfig)
	if err != nil {
		return fmt.Errorf("connect PostgreSQL: %w", err)
	}
	defer pg.Close(context.Background())
	tx, err := pg.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	for _, name := range targetNames {
		var count int64
		if err := tx.QueryRow(ctx, "SELECT count(*) FROM chatbox."+name).Scan(&count); err != nil {
			return fmt.Errorf("count PostgreSQL %s: %w", name, err)
		}
		if count != 0 {
			return fmt.Errorf("target chatbox.%s already contains %d rows; refusing to overwrite", name, count)
		}
	}
	actual := make(map[string]int64, len(targetNames))
	if actual["users"], err = copyUsers(ctx, source.Collection("users"), tx); err != nil {
		return err
	}
	if actual["conversations"], err = copyConversations(ctx, source.Collection("conversations"), tx); err != nil {
		return err
	}
	if actual["messages"], err = copyMessages(ctx, source.Collection("messages"), tx); err != nil {
		return err
	}
	if actual["wallets"], actual["usage_records"], err = copyWallets(ctx, source.Collection("wallets"), tx); err != nil {
		return err
	}
	if actual["api_keys"], err = copyAPIKeys(ctx, source.Collection("api_keys"), tx); err != nil {
		return err
	}
	for _, name := range sourceNames {
		if actual[name] != expected[name] {
			return fmt.Errorf("count changed while migrating %s: expected %d, copied %d", name, expected[name], actual[name])
		}
	}
	for _, name := range targetNames {
		var count int64
		if err := tx.QueryRow(ctx, "SELECT count(*) FROM chatbox."+name).Scan(&count); err != nil {
			return err
		}
		if count != actual[name] {
			return fmt.Errorf("PostgreSQL %s has %d rows, expected %d", name, count, actual[name])
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit PostgreSQL: %w", err)
	}
	for _, name := range targetNames {
		fmt.Printf("PostgreSQL %-14s %d\n", name, actual[name])
	}
	return nil
}

func forEach[T any](ctx context.Context, collection *mongo.Collection, insert func(T) error) (int64, error) {
	cur, err := collection.Find(ctx, bson.D{})
	if err != nil {
		return 0, err
	}
	defer cur.Close(ctx)
	var count int64
	for cur.Next(ctx) {
		var doc T
		if err := cur.Decode(&doc); err != nil {
			return count, err
		}
		if err := insert(doc); err != nil {
			return count, err
		}
		count++
	}
	return count, cur.Err()
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func copyUsers(ctx context.Context, col *mongo.Collection, tx pgx.Tx) (int64, error) {
	return forEach(ctx, col, func(d userDoc) error {
		_, err := tx.Exec(ctx, `INSERT INTO chatbox.users
            (id, username, display_name, email, first_name, last_name, keycloak_sub, is_admin, created_at)
            VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			d.ID.Hex(), d.Username, d.DisplayName, d.Email, d.FirstName, d.LastName,
			nullableString(d.KeycloakSub), d.IsAdmin, d.CreatedAt)
		return err
	})
}

func copyConversations(ctx context.Context, col *mongo.Collection, tx pgx.Tx) (int64, error) {
	return forEach(ctx, col, func(d conversationDoc) error {
		_, err := tx.Exec(ctx, `INSERT INTO chatbox.conversations
            (id, owner_id, title, summary, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6)`,
			d.ID.Hex(), d.OwnerID.Hex(), d.Title, d.Summary, d.CreatedAt, d.UpdatedAt)
		return err
	})
}

func copyMessages(ctx context.Context, col *mongo.Collection, tx pgx.Tx) (int64, error) {
	return forEach(ctx, col, func(d messageDoc) error {
		_, err := tx.Exec(ctx, `INSERT INTO chatbox.messages
            (id, conversation_id, role, content, summarized, created_at) VALUES ($1,$2,$3,$4,$5,$6)`,
			d.ID.Hex(), d.ConversationID.Hex(), d.Role, d.Content, d.Summarized, d.CreatedAt)
		return err
	})
}

func copyWallets(ctx context.Context, col *mongo.Collection, tx pgx.Tx) (int64, int64, error) {
	var usageCount int64
	count, err := forEach(ctx, col, func(d walletDoc) error {
		plan := d.Plan
		if plan == "" {
			plan = "free"
		}
		_, err := tx.Exec(ctx, `INSERT INTO chatbox.wallets
            (user_id, plan, plan_source, balance, paid_until, credit_reset_at) VALUES ($1,$2,$3,$4,$5,$6)`,
			d.ID.Hex(), plan, nullableString(d.PlanSource), d.Balance, d.PaidUntil, d.CreditResetAt)
		if err != nil {
			return err
		}
		for i, u := range d.Usage {
			_, err := tx.Exec(ctx, `INSERT INTO chatbox.usage_records
                (user_id, position, model, kind, input_tokens, output_tokens, credits, reported, status, created_at)
                VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
				d.ID.Hex(), i, u.Model, u.Kind, u.InputTokens, u.OutputTokens,
				u.Credits, u.Reported, u.Status, u.CreatedAt)
			if err != nil {
				return err
			}
			usageCount++
		}
		return nil
	})
	return count, usageCount, err
}

func copyAPIKeys(ctx context.Context, col *mongo.Collection, tx pgx.Tx) (int64, error) {
	return forEach(ctx, col, func(d apiKeyDoc) error {
		_, err := tx.Exec(ctx, `INSERT INTO chatbox.api_keys
            (id, user_id, name, prefix, hash, created_at, last_used_at, revoked_at)
            VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			d.ID.Hex(), d.UserID.Hex(), d.Name, d.Prefix, d.Hash,
			d.CreatedAt, d.LastUsedAt, d.RevokedAt)
		return err
	})
}
