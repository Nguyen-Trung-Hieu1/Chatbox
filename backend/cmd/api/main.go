package main

import (
	"backend/internal/config"
	httpdelivery "backend/internal/delivery/http"
	"backend/internal/domain"
	"backend/internal/infrastructure/ai"
	"backend/internal/infrastructure/persistence"
	"backend/internal/usecase"
	"context"
	"log"
	"net/http"
	"time"
)

func main() {
	cfg, err := config.Load(".env", "backend/.env")
	if err != nil {
		log.Fatal("Không thể đọc cấu hình: ", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := persistence.NewMongo(ctx, cfg.MongoURI, cfg.MongoDatabase)
	if err != nil {
		log.Fatal("Kết nối MongoDB thất bại: ", err)
	}
	defer db.Client.Disconnect(context.Background())
	auth := usecase.NewAuth(persistence.NewUserRepository(db), persistence.NewSessionRepository(db), cfg.SessionTTL)
	convert := func(models []config.Model) []domain.ModelPolicy {
		out := make([]domain.ModelPolicy, len(models))
		for i, m := range models {
			out[i] = domain.ModelPolicy{ID: m.ID, DisplayName: m.DisplayName, InputRate: m.InputRate, OutputRate: m.OutputRate, MaxOutputTokens: m.MaxOutputTokens}
		}
		return out
	}
	chat := usecase.NewChat(persistence.NewConversationRepository(db), persistence.NewMessageRepository(db), ai.New(cfg.AIEndpoint, cfg.AIKey, cfg.AIModel), cfg.TokenBudget, persistence.NewBillingRepository(db), convert(cfg.FreeModels), convert(cfg.PaidModels), cfg.FreeCredits)
	keys := usecase.NewAPIKeys(persistence.NewAPIKeyRepository(db), persistence.NewUserRepository(db))
	transport := httpdelivery.New(auth, chat, keys, cfg.CookieName, cfg.CookieSecure)
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: transport.Handler(cfg.CORSOrigin), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 2 * time.Minute, IdleTimeout: 60 * time.Second}
	log.Printf("Backend đang chạy tại %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}
