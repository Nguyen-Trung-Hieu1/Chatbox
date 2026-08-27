package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"backend/models"
)

type AIService struct {
	endpoint, apiKey, model string
	client                  *http.Client
}
type aiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type aiRequest struct {
	Stream   bool        `json:"stream"`
	Model    string      `json:"model"`
	Messages []aiMessage `json:"messages"`
}
type aiResponse struct {
	Choices []struct {
		Message aiMessage `json:"message"`
	} `json:"choices"`
}
type aiStreamResponse struct {
	Choices []struct {
		Delta aiMessage `json:"delta"`
	} `json:"choices"`
}

func NewAIService(endpoint, key, model string) *AIService {
	return &AIService{endpoint, key, model, &http.Client{}}
}
func (s *AIService) Complete(ctx context.Context, messages []aiMessage) (string, error) {
	body, _ := json.Marshal(aiRequest{false, s.model, messages})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.apiKey)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("AI trả về %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	var out aiResponse
	if err = json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return "", errors.New("AI không trả về nội dung")
	}
	return out.Choices[0].Message.Content, nil
}
func (s *AIService) Stream(ctx context.Context, messages []aiMessage, emit func(string) error) (string, error) {
	body, _ := json.Marshal(aiRequest{true, s.model, messages})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.apiKey)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("AI trả về %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	var answer strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var chunk aiStreamResponse
		if err = json.Unmarshal([]byte(data), &chunk); err != nil {
			return answer.String(), fmt.Errorf("dữ liệu stream AI không hợp lệ: %w", err)
		}
		if len(chunk.Choices) == 0 || chunk.Choices[0].Delta.Content == "" {
			continue
		}
		delta := chunk.Choices[0].Delta.Content
		answer.WriteString(delta)
		if err = emit(delta); err != nil {
			return answer.String(), err
		}
	}
	if err = scanner.Err(); err != nil {
		return answer.String(), err
	}
	if strings.TrimSpace(answer.String()) == "" {
		return "", errors.New("AI không trả về nội dung")
	}
	return answer.String(), nil
}
func estimateTokens(text string) int { n := utf8.RuneCountInString(text); return (n+3)/4 + 4 }
func toAI(messages []models.Message) []aiMessage {
	out := make([]aiMessage, 0, len(messages))
	for _, m := range messages {
		out = append(out, aiMessage{m.Role, m.Content})
	}
	return out
}
