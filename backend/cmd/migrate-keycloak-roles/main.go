package main

import (
	"backend/internal/config"
	"backend/internal/infrastructure/keycloak"
	"backend/internal/infrastructure/persistence"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	apply := flag.Bool("apply", false, "write roles to Keycloak; default is preview only")
	flag.Parse()
	cfg, err := config.Load(".env", "backend/.env")
	if err != nil {
		log.Fatal(err)
	}
	if cfg.KeycloakAdminClientSecret == "" {
		log.Fatal("KEYCLOAK_ADMIN_CLIENT_SECRET is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	db, err := persistence.NewMongo(ctx, cfg.MongoURI, cfg.MongoDatabase)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Client.Disconnect(context.Background())
	manager := keycloak.NewAdmin(cfg.KeycloakIssuer, cfg.KeycloakAdminClientID, cfg.KeycloakAdminClientSecret)
	cur, err := db.Users.Find(ctx, bson.M{"keycloak_sub": bson.M{"$type": "string"}}, options.Find().SetProjection(bson.M{"keycloak_sub": 1, "is_admin": 1}))
	if err != nil {
		log.Fatal(err)
	}
	defer cur.Close(ctx)
	var users []struct {
		ID    primitive.ObjectID `bson:"_id"`
		Sub   string             `bson:"keycloak_sub"`
		Admin bool               `bson:"is_admin"`
	}
	if err := cur.All(ctx, &users); err != nil {
		log.Fatal(err)
	}
	for _, u := range users {
		var wallet struct {
			Plan      string     `bson:"plan"`
			PaidUntil *time.Time `bson:"paid_until"`
		}
		err := db.Wallets.FindOne(ctx, bson.M{"_id": u.ID}).Decode(&wallet)
		if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			log.Fatal(err)
		}
		plan := "free"
		if wallet.Plan == "paid" && wallet.PaidUntil != nil && wallet.PaidUntil.After(time.Now()) {
			plan = "paid"
		}
		fmt.Printf("user=%s plan=%s admin=%t\n", u.ID.Hex(), plan, u.Admin)
		if !*apply {
			continue
		}
		if err := manager.SetPlan(ctx, u.Sub, plan); err != nil {
			log.Fatalf("user %s: %v", u.ID.Hex(), err)
		}
		if u.Admin {
			if err := manager.AssignRole(ctx, u.Sub, "chatbox-admin"); err != nil {
				log.Fatalf("user %s admin: %v", u.ID.Hex(), err)
			}
		}
	}
	if !*apply {
		fmt.Println("Preview only. Re-run with -apply to assign roles.")
	}
}
