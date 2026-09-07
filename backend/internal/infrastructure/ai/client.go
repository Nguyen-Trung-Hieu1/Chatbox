package ai

import (
	"backend/internal/domain"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Client struct {
	endpoint, key, model string
	http                 *http.Client
}
type request struct {
	Stream   bool               `json:"stream"`
	Model    string             `json:"model"`
	Messages []domain.AIMessage `json:"messages"`
}
type response struct {
	Choices []struct {
		Message domain.AIMessage `json:"message"`
		Delta   domain.AIMessage `json:"delta"`
	} `json:"choices"`
}

func New(endpoint, key, model string) *Client { return &Client{endpoint, key, model, &http.Client{}} }
func (s *Client) newRequest(ctx context.Context, stream bool, messages []domain.AIMessage) (*http.Request, error) {
	body, e := json.Marshal(request{stream, s.model, messages})
	if e != nil {
		return nil, e
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if e != nil {
		return nil, e
	}
	req.Header.Set("Content-Type", "application/json")
	if s.key != "" {
		req.Header.Set("Authorization", "Bearer "+s.key)
	}
	return req, nil
}
func (s *Client) Complete(ctx context.Context, messages []domain.AIMessage) (string, error) {
	req, e := s.newRequest(ctx, false, messages)
	if e != nil {
		return "", e
	}
	resp, e := s.http.Do(req)
	if e != nil {
		return "", e
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("AI trả về %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	var out response
	if e = json.NewDecoder(resp.Body).Decode(&out); e != nil {
		return "", e
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return "", errors.New("AI không trả về nội dung")
	}
	return out.Choices[0].Message.Content, nil
}
func (s *Client) Stream(ctx context.Context, messages []domain.AIMessage, emit func(string) error) (string, error) {
	req, e := s.newRequest(ctx, true, messages)
	if e != nil {
		return "", e
	}
	resp, e := s.http.Do(req)
	if e != nil {
		return "", e
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("AI trả về %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	var answer strings.Builder
	scan := bufio.NewScanner(resp.Body)
	scan.Buffer(make([]byte, 64*1024), 1024*1024)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var chunk response
		if e = json.Unmarshal([]byte(data), &chunk); e != nil {
			return answer.String(), fmt.Errorf("dữ liệu stream AI không hợp lệ: %w", e)
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta.Content
		if delta != "" {
			answer.WriteString(delta)
			if e = emit(delta); e != nil {
				return answer.String(), e
			}
		}
	}
	if e = scan.Err(); e != nil {
		return answer.String(), e
	}
	if strings.TrimSpace(answer.String()) == "" {
		return "", errors.New("AI không trả về nội dung")
	}
	return answer.String(), nil
}
