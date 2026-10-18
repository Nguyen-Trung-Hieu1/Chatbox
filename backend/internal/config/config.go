package config

import (
	"bufio"
	"encoding/json"
	"fmt"
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
	FreeCredits                                                                           int64
	FreeModels, PaidModels                                                                []Model
}

type Model struct {
	ID              string `json:"id"`
	DisplayName     string `json:"display_name"`
	InputRate       int64  `json:"input_rate"`
	OutputRate      int64  `json:"output_rate"`
	MaxOutputTokens int    `json:"max_output_tokens"`
}

func Load(files ...string) (Config, error) {
	for _, path := range files {
		if err := loadFile(path); err != nil {
			return Config{}, err
		}
	}
	c := Config{MongoURI: value("MONGO_URI", "mongodb://localhost:27017"), MongoDatabase: value("MONGO_DATABASE", "Chatbox1"), AIEndpoint: value("AI_ENDPOINT", "http://localhost:20127/v1/chat/completions"), AIKey: os.Getenv("AI_API_KEY"), AIModel: value("AI_MODEL", "ag/claude-opus-4-6-thinking"), HTTPAddr: value("HTTP_ADDR", ":8080"), CORSOrigin: value("CORS_ORIGIN", "http://localhost:3000"), CookieName: value("SESSION_COOKIE_NAME", "chatbox_session"), CookieSecure: strings.EqualFold(value("COOKIE_SECURE", "false"), "true"), SessionTTL: time.Duration(integer("SESSION_TTL_HOURS", 24)) * time.Hour, TokenBudget: integer("AI_TOKEN_BUDGET", 6000), FreeCredits: int64(integer("FREE_INITIAL_CREDITS", 100))}
	c.FreeModels = []Model{
		{ID: "ag/gemini-3-flash", DisplayName: "gemini-3-flash", InputRate: 1, OutputRate: 2, MaxOutputTokens: 1024},
		{ID: "ag/gemini-3.5-flash-low", DisplayName: "gemini-3.5-flash-low", InputRate: 1, OutputRate: 2, MaxOutputTokens: 1024},
		{ID: "ag/gemini-3.5-flash-extra-low", DisplayName: "gemini-3.5-flash-extra-low", InputRate: 1, OutputRate: 2, MaxOutputTokens: 1024},
		{ID: "ag/gpt-oss-120b-medium", DisplayName: "gpt-oss-120b", InputRate: 1, OutputRate: 2, MaxOutputTokens: 1024},
	}
	c.PaidModels = append(append([]Model{}, c.FreeModels...),
		Model{ID: "ag/gemini-3-flash-agent", DisplayName: "gemini-3-flash-agent", InputRate: 2, OutputRate: 4, MaxOutputTokens: 1024},
		Model{ID: "ag/claude-sonnet-4-6", DisplayName: "claude-sonnet-4-6", InputRate: 4, OutputRate: 8, MaxOutputTokens: 1024},
		Model{ID: "ag/claude-opus-4-6-thinking", DisplayName: "claude-opus-4-6-thinking", InputRate: 8, OutputRate: 16, MaxOutputTokens: 1024},
	)
	if raw := os.Getenv("AI_FREE_MODELS_JSON"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &c.FreeModels); err != nil {
			return Config{}, err
		}
	}
	if raw := os.Getenv("AI_PAID_MODELS_JSON"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &c.PaidModels); err != nil {
			return Config{}, err
		}
	}
	if len(c.FreeModels) == 0 || len(c.PaidModels) == 0 || c.FreeCredits < 0 {
		return Config{}, fmt.Errorf("cấu hình gói AI không hợp lệ")
	}
	for _, group := range [][]Model{c.FreeModels, c.PaidModels} {
		seen := map[string]bool{}
		for _, m := range group {
			if m.ID == "" || m.InputRate < 0 || m.OutputRate < 0 || m.MaxOutputTokens <= 0 {
				return Config{}, fmt.Errorf("cấu hình model AI không hợp lệ")
			}
			if seen[m.ID] {
				return Config{}, fmt.Errorf("model AI bị trùng: %s", m.ID)
			}
			seen[m.ID] = true
		}
	}
	return c, nil
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
