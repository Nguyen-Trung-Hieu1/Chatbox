package main

import (
	"backend/internal/config"
	httpdelivery "backend/internal/delivery/http"
	"backend/internal/domain"
	"backend/internal/infrastructure/ai"
	"backend/internal/infrastructure/keycloak"
	"backend/internal/infrastructure/persistence"
	"backend/internal/infrastructure/postgres"
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
	var users domain.UserRepository
	var sessions domain.SessionRepository
	var conversations domain.ConversationRepository
	var messages domain.MessageRepository
	var billing domain.BillingRepository
	var keysRepo domain.APIKeyRepository
	var adminRepo usecase.AdminRepository
	if cfg.PostgresEnabled {
		pg, err := postgres.New(ctx, cfg.PGHost, cfg.PGDatabase, cfg.PGUser, cfg.PGPassword)
		if err != nil {
			log.Fatal("Kết nối PostgreSQL thất bại: ", err)
		}
		defer pg.Close()
		users, sessions = pg, postgres.DisabledSessions{}
		conversations, messages = postgres.ConversationRepository{Store: pg}, postgres.MessageRepository{Store: pg}
		billing, keysRepo, adminRepo = pg, postgres.APIKeyRepository{Store: pg}, pg
	} else {
		db, err := persistence.NewMongo(ctx, cfg.MongoURI, cfg.MongoDatabase)
		if err != nil {
			log.Fatal("Kết nối MongoDB thất bại: ", err)
		}
		defer db.Client.Disconnect(context.Background())
		users, sessions = persistence.NewUserRepository(db), persistence.NewSessionRepository(db)
		conversations, messages = persistence.NewConversationRepository(db), persistence.NewMessageRepository(db)
		billing, keysRepo, adminRepo = persistence.NewBillingRepository(db), persistence.NewAPIKeyRepository(db), persistence.NewAdminRepository(db)
	}
	auth := usecase.NewAuth(users, sessions, cfg.SessionTTL)
	if !cfg.PostgresEnabled && cfg.AdminBootstrapUsername != "" {
		if err := auth.BootstrapAdmin(ctx, cfg.AdminBootstrapUsername, cfg.AdminBootstrapPassword); err != nil {
			log.Fatal("Không thể khởi tạo tài khoản admin: ", err)
		}
	}
	convert := func(models []config.Model) []domain.ModelPolicy {
		out := make([]domain.ModelPolicy, len(models))
		for i, m := range models {
			out[i] = domain.ModelPolicy{ID: m.ID, DisplayName: m.DisplayName, InputRate: m.InputRate, OutputRate: m.OutputRate, MaxOutputTokens: m.MaxOutputTokens}
		}
		return out
	}
	chat := usecase.NewChat(conversations, messages, ai.New(cfg.AIEndpoint, cfg.AIKey, cfg.AIModel), cfg.TokenBudget, billing, convert(cfg.FreeModels), convert(cfg.PaidModels), cfg.FreeCredits)
	keys := usecase.NewAPIKeys(keysRepo, users)
	admin := usecase.NewAdmin(adminRepo, cfg.FreeCredits)
	if cfg.KeycloakRolesEnabled {
		admin.UseKeycloakRoles(keycloak.NewAdmin(cfg.KeycloakIssuer, cfg.KeycloakAdminClientID, cfg.KeycloakAdminClientSecret))
	}
	transport := httpdelivery.New(auth, chat, keys, admin, cfg.CookieName, cfg.RefreshCookieName, cfg.CookieSecure, cfg.KeycloakIssuer, cfg.KeycloakClientID)
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: transport.Handler(cfg.CORSOrigin), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 2 * time.Minute, IdleTimeout: 60 * time.Second}
	log.Printf("Backend đang chạy tại %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}
