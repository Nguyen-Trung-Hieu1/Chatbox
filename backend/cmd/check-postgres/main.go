// Read-only smoke check of the PostgreSQL repositories before switching the API.
package main

import (
	"backend/internal/infrastructure/postgres"
	"context"
	"log"
	"os"
	"time"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	host, database := os.Getenv("PGHOST"), os.Getenv("PGDATABASE")
	if host == "" {
		host = "app-postgres.database.svc.cluster.local"
	}
	if database == "" {
		database = "app_db"
	}
	s, err := postgres.New(ctx, host, database, os.Getenv("PGUSER"), os.Getenv("PGPASSWORD"))
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()
	accounts, err := s.ListAccounts(ctx)
	if err != nil {
		log.Fatal(err)
	}
	var conversations, messages, keys, usage int
	for _, account := range accounts {
		u, err := s.ByID(ctx, account.ID)
		if err != nil {
			log.Fatal(err)
		}
		if _, err := s.ByUsername(ctx, u.Username); err != nil {
			log.Fatal(err)
		}
		if _, err := s.KeycloakSub(ctx, u.ID); err != nil {
			log.Fatal(err)
		}
		if _, err := s.Balance(ctx, u.ID); err != nil {
			log.Fatal(err)
		}
		history, err := s.History(ctx, u.ID)
		if err != nil {
			log.Fatal(err)
		}
		usage += len(history)
		userConversations, err := s.ListOwned(ctx, u.ID)
		if err != nil {
			log.Fatal(err)
		}
		conversations += len(userConversations)
		for _, c := range userConversations {
			if _, err := s.OwnedByID(ctx, c.ID, u.ID); err != nil {
				log.Fatal(err)
			}
			conversationMessages, err := s.List(ctx, c.ID)
			if err != nil {
				log.Fatal(err)
			}
			messages += len(conversationMessages)
		}
		userKeys, err := (postgres.APIKeyRepository{Store: s}).List(ctx, u.ID)
		if err != nil {
			log.Fatal(err)
		}
		keys += len(userKeys)
	}
	log.Printf("PostgreSQL repositories OK: users=%d conversations=%d messages=%d usage=%d api_keys=%d", len(accounts), conversations, messages, usage, keys)
}
