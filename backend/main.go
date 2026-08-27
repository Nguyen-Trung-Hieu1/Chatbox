package main

import (
	"bufio"
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"backend/handler"
	"backend/middleware"
	"backend/repository"
	"backend/service"
)

func loadEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), "\"'")
		if key != "" {
			if _, exists := os.LookupEnv(key); !exists {
				if err := os.Setenv(key, value); err != nil {
					return err
				}
			}
		}
	}
	return scanner.Err()
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return value
}
func withCORS(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Add("Vary", "Origin")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func method(method string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		h(w, r)
	}
}

func main() {
	if err := loadEnvFile(".env"); err != nil {
		log.Fatal("Không thể đọc file .env: ", err)
	}
	if err := loadEnvFile("backend/.env"); err != nil {
		log.Fatal("Không thể đọc file backend/.env: ", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := repository.NewMongo(ctx, env("MONGO_URI", "mongodb://localhost:27017"), env("MONGO_DATABASE", "Chatbox1"))
	if err != nil {
		log.Fatal("Kết nối MongoDB thất bại: ", err)
	}
	defer db.Client.Disconnect(context.Background())
	ttl := time.Duration(envInt("SESSION_TTL_HOURS", 24)) * time.Hour
	authService := service.NewAuthService(repository.NewUserRepository(db), repository.NewSessionRepository(db), ttl)
	aiService := service.NewAIService(env("AI_ENDPOINT", "http://localhost:20127/v1/chat/completions"), os.Getenv("AI_API_KEY"), env("AI_MODEL", "cx/gpt-5.6-terra"))
	chatService := service.NewChatService(repository.NewConversationRepository(db), repository.NewMessageRepository(db), aiService, envInt("AI_TOKEN_BUDGET", 6000))
	cookieName := env("SESSION_COOKIE_NAME", "chatbox_session")
	cookieSecure := strings.EqualFold(env("COOKIE_SECURE", "false"), "true")
	authHandler := handler.NewAuthHandler(authService, cookieName, cookieSecure)
	chatHandler := handler.NewChatHandler(chatService)
	requireAuth := middleware.Auth(authService, cookieName)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/register", method(http.MethodPost, authHandler.Register))
	mux.HandleFunc("/api/login", method(http.MethodPost, authHandler.Login))
	mux.HandleFunc("/api/logout", method(http.MethodPost, authHandler.Logout))
	mux.Handle("/api/me", requireAuth(method(http.MethodGet, authHandler.Me)))
	mux.Handle("/api/conversations", requireAuth(http.HandlerFunc(chatHandler.Conversations)))
	mux.Handle("/api/history", requireAuth(method(http.MethodGet, chatHandler.History)))
	mux.Handle("/api/chat", requireAuth(method(http.MethodPost, chatHandler.Chat)))

	server := &http.Server{Addr: env("HTTP_ADDR", ":8080"), Handler: withCORS(env("CORS_ORIGIN", "http://localhost:3000"), mux), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 2 * time.Minute, IdleTimeout: 60 * time.Second}
	log.Printf("Backend đang chạy tại %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}
