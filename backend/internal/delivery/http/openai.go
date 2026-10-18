package httpdelivery

import (
	"backend/internal/domain"
	"backend/internal/usecase"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func (s *Server) manageKeys(w http.ResponseWriter, r *http.Request) {
	owner := user(r.Context()).ID
	switch r.Method {
	case http.MethodGet:
		keys, err := s.keys.List(r.Context(), owner)
		if err != nil {
			fail(w, err)
			return
		}
		jsonOut(w, 200, keys)
	case http.MethodPost:
		var in struct {
			Name string `json:"name"`
		}
		if decode(r, &in) != nil {
			jsonOut(w, 400, map[string]string{"error": "dữ liệu không hợp lệ"})
			return
		}
		key, raw, err := s.keys.Create(r.Context(), owner, in.Name)
		if err != nil {
			fail(w, err)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		jsonOut(w, 201, map[string]any{"key": raw, "item": key})
	case http.MethodDelete:
		if err := s.keys.Revoke(r.Context(), owner, r.URL.Query().Get("id")); err != nil {
			fail(w, err)
			return
		}
		w.WriteHeader(204)
	default:
		w.WriteHeader(405)
	}
}

func openAIError(w http.ResponseWriter, err error) {
	status, code := 500, "server_error"
	switch {
	case errors.Is(err, domain.ErrUnauthorized):
		status, code = 401, "invalid_api_key"
	case errors.Is(err, domain.ErrModelForbidden):
		status, code = 403, "model_not_allowed"
	case errors.Is(err, domain.ErrQuotaExceeded):
		status, code = 402, "insufficient_credits"
	case errors.Is(err, usecase.ErrInvalidAPIMessages), errors.Is(err, domain.ErrInvalidContent):
		status, code = 400, "invalid_request"
	}
	message := err.Error()
	if status == 500 {
		message = "Lỗi máy chủ"
	}
	jsonOut(w, status, map[string]any{"error": map[string]any{"message": message, "type": code, "code": code}})
}

func (s *Server) apiAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			openAIError(w, domain.ErrUnauthorized)
			return
		}
		u, err := s.keys.Authenticate(r.Context(), parts[1])
		if err != nil {
			openAIError(w, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, u)))
	})
}

func (s *Server) openAIModels(w http.ResponseWriter, r *http.Request) {
	e, err := s.chat.Entitlement(r.Context(), user(r.Context()).ID)
	if err != nil {
		openAIError(w, err)
		return
	}
	items := make([]map[string]any, len(e.Models))
	for i, model := range e.Models {
		items[i] = map[string]any{"id": model.ID, "object": "model", "owned_by": "chatbox"}
	}
	jsonOut(w, 200, map[string]any{"object": "list", "data": items})
}

type completionRequest struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"messages"`
	Stream              bool `json:"stream"`
	MaxTokens           int  `json:"max_tokens"`
	MaxCompletionTokens int  `json:"max_completion_tokens"`
	StreamOptions       struct {
		IncludeUsage bool `json:"include_usage"`
	} `json:"stream_options"`
	Tools          json.RawMessage `json:"tools"`
	ToolChoice     json.RawMessage `json:"tool_choice"`
	ResponseFormat json.RawMessage `json:"response_format"`
	N              int             `json:"n"`
}

func completionID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
	}
	return "chatcmpl-" + hex.EncodeToString(b)
}

func (s *Server) openAIChat(w http.ResponseWriter, r *http.Request) {
	var in completionRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&in); err != nil {
		openAIError(w, usecase.ErrInvalidAPIMessages)
		return
	}
	if in.Model == "" || len(in.Tools) > 0 || len(in.ToolChoice) > 0 || len(in.ResponseFormat) > 0 || in.N > 1 || in.MaxTokens < 0 || in.MaxCompletionTokens < 0 {
		openAIError(w, usecase.ErrInvalidAPIMessages)
		return
	}
	messages := make([]domain.AIMessage, len(in.Messages))
	for i, m := range in.Messages {
		if len(m.Content) == 0 || m.Content[0] != '"' {
			openAIError(w, usecase.ErrInvalidAPIMessages)
			return
		}
		if err := json.Unmarshal(m.Content, &messages[i].Content); err != nil {
			openAIError(w, usecase.ErrInvalidAPIMessages)
			return
		}
		messages[i].Role = m.Role
	}
	maxOutput := in.MaxCompletionTokens
	if maxOutput == 0 {
		maxOutput = in.MaxTokens
	}
	id, created := completionID(), time.Now().Unix()
	started := false
	writeSSE := func(payload any) error {
		if !started {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("X-Accel-Buffering", "no")
			started = true
		}
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		if _, err = fmt.Fprintf(w, "data: %s\n\n", encoded); err != nil {
			return err
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		return nil
	}
	first := true
	emit := func(delta string) error {
		part := map[string]any{"content": delta}
		if first {
			part["role"] = "assistant"
			first = false
		}
		return writeSSE(map[string]any{"id": id, "object": "chat.completion.chunk", "created": created, "model": in.Model, "choices": []any{map[string]any{"index": 0, "delta": part, "finish_reason": nil}}})
	}
	result, err := s.chat.APIGenerate(r.Context(), user(r.Context()).ID, in.Model, messages, maxOutput, in.Stream, emit)
	if err != nil {
		if !started {
			openAIError(w, err)
			return
		}
		encoded, _ := json.Marshal(map[string]any{"error": map[string]any{"message": err.Error(), "type": "stream_error"}})
		_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", encoded)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		return
	}
	u := map[string]int{"prompt_tokens": result.Usage.InputTokens, "completion_tokens": result.Usage.OutputTokens, "total_tokens": result.Usage.InputTokens + result.Usage.OutputTokens}
	if !in.Stream {
		jsonOut(w, 200, map[string]any{"id": id, "object": "chat.completion", "created": created, "model": in.Model, "choices": []any{map[string]any{"index": 0, "message": map[string]string{"role": "assistant", "content": result.Content}, "finish_reason": "stop"}}, "usage": u})
		return
	}
	_ = writeSSE(map[string]any{"id": id, "object": "chat.completion.chunk", "created": created, "model": in.Model, "choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "stop"}}})
	if in.StreamOptions.IncludeUsage {
		_ = writeSSE(map[string]any{"id": id, "object": "chat.completion.chunk", "created": created, "model": in.Model, "choices": []any{}, "usage": u})
	}
	_, _ = io.WriteString(w, "data: [DONE]\n\n")
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

type textCompletionRequest struct {
	Model     string          `json:"model"`
	Prompt    json.RawMessage `json:"prompt"`
	Stream    bool            `json:"stream"`
	MaxTokens int             `json:"max_tokens"`
	N         int             `json:"n"`
	Echo      bool            `json:"echo"`
	Suffix    string          `json:"suffix"`
}

// Continue Edit uses the legacy text Completions endpoint. The prompt is sent
// through the same account policy and billing path as Chat Completions.
func (s *Server) openAICompletions(w http.ResponseWriter, r *http.Request) {
	var in textCompletionRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&in); err != nil {
		openAIError(w, usecase.ErrInvalidAPIMessages)
		return
	}
	var prompt string
	if in.Model == "" || len(in.Prompt) == 0 || json.Unmarshal(in.Prompt, &prompt) != nil || strings.TrimSpace(prompt) == "" || in.MaxTokens < 0 || in.N > 1 || in.Echo || in.Suffix != "" {
		openAIError(w, usecase.ErrInvalidAPIMessages)
		return
	}
	id, created := completionID(), time.Now().Unix()
	started := false
	writeSSE := func(payload any) error {
		if !started {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("X-Accel-Buffering", "no")
			started = true
		}
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		if _, err = fmt.Fprintf(w, "data: %s\n\n", encoded); err != nil {
			return err
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		return nil
	}
	emit := func(delta string) error {
		return writeSSE(map[string]any{"id": id, "object": "text_completion", "created": created, "model": in.Model, "choices": []any{map[string]any{"text": delta, "index": 0, "finish_reason": nil}}})
	}
	messages := []domain.AIMessage{{Role: "user", Content: prompt}}
	result, err := s.chat.APIGenerate(r.Context(), user(r.Context()).ID, in.Model, messages, in.MaxTokens, in.Stream, emit)
	if err != nil {
		if !started {
			openAIError(w, err)
			return
		}
		encoded, _ := json.Marshal(map[string]any{"error": map[string]any{"message": err.Error(), "type": "stream_error"}})
		_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", encoded)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		return
	}
	u := map[string]int{"prompt_tokens": result.Usage.InputTokens, "completion_tokens": result.Usage.OutputTokens, "total_tokens": result.Usage.InputTokens + result.Usage.OutputTokens}
	if !in.Stream {
		jsonOut(w, 200, map[string]any{"id": id, "object": "text_completion", "created": created, "model": in.Model, "choices": []any{map[string]any{"text": result.Content, "index": 0, "finish_reason": "stop"}}, "usage": u})
		return
	}
	_ = writeSSE(map[string]any{"id": id, "object": "text_completion", "created": created, "model": in.Model, "choices": []any{map[string]any{"text": "", "index": 0, "finish_reason": "stop"}}})
	_, _ = io.WriteString(w, "data: [DONE]\n\n")
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}
