package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	MongoURI, MongoDatabase, AIEndpoint, AIKey, AIModel, HTTPAddr, CORSOrigin, CookieName string
	CookieSecure                                                                          bool
	SessionTTL                                                                            time.Duration
	TokenBudget                                                                           int
}

func Load(files ...string) (Config, error) {
	for _, path := range files {
		if err := loadFile(path); err != nil {
			return Config{}, err
		}
	}
	return Config{value("MONGO_URI", "mongodb://localhost:27017"), value("MONGO_DATABASE", "Chatbox1"), value("AI_ENDPOINT", "http://localhost:20127/v1/chat/completions"), os.Getenv("AI_API_KEY"), value("AI_MODEL", "cx/gpt-5.6-terra"), value("HTTP_ADDR", ":8080"), value("CORS_ORIGIN", "http://localhost:3000"), value("SESSION_COOKIE_NAME", "chatbox_session"), strings.EqualFold(value("COOKIE_SECURE", "false"), "true"), time.Duration(integer("SESSION_TTL_HOURS", 24)) * time.Hour, integer("AI_TOKEN_BUDGET", 6000)}, nil
}
func value(k, f string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return f
}
func integer(k string, f int) int {
	v, e := strconv.Atoi(os.Getenv(k))
	if e != nil {
		return f
	}
	return v
}
func loadFile(path string) error {
	f, e := os.Open(path)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if ok {
			k = strings.TrimSpace(k)
			v = strings.Trim(strings.TrimSpace(v), "\"'")
			if _, exists := os.LookupEnv(k); k != "" && !exists {
				if e = os.Setenv(k, v); e != nil {
					return e
				}
			}
		}
	}
	return s.Err()
}
