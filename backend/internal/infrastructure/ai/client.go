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
	Stream              bool               `json:"stream"`
	Model               string             `json:"model"`
	Messages            []domain.AIMessage `json:"messages"`
	MaxCompletionTokens int                `json:"max_completion_tokens"`
	StreamOptions       *struct {
		IncludeUsage bool `json:"include_usage"`
	} `json:"stream_options,omitempty"`
}
type response struct {
	Choices []struct {
		Message domain.AIMessage `json:"message"`
		Delta   domain.AIMessage `json:"delta"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		InputTokens      int `json:"input_tokens"`
		OutputTokens     int `json:"output_tokens"`
	} `json:"usage"`
}

func New(endpoint, key, model string) *Client { return &Client{endpoint, key, model, &http.Client{}} }
func (s *Client) newRequest(ctx context.Context, stream bool, model string, messages []domain.AIMessage, maxOutput int) (*http.Request, error) {
	if model == "" {
		model = s.model
	}
	r := request{Stream: stream, Model: model, Messages: messages, MaxCompletionTokens: maxOutput}
	if stream {
		r.StreamOptions = &struct {
			IncludeUsage bool `json:"include_usage"`
		}{true}
	}
	body, e := json.Marshal(r)
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
func usage(v *response) domain.AIUsage {
	if v.Usage == nil {
		return domain.AIUsage{}
	}
	in, out := v.Usage.PromptTokens, v.Usage.CompletionTokens
	if in == 0 {
		in = v.Usage.InputTokens
	}
	if out == 0 {
		out = v.Usage.OutputTokens
	}
	return domain.AIUsage{InputTokens: in, OutputTokens: out, Reported: in > 0 || out > 0}
}
func (s *Client) Complete(ctx context.Context, model string, messages []domain.AIMessage, maxOutput int) (domain.AIResult, error) {
	req, e := s.newRequest(ctx, false, model, messages, maxOutput)
	if e != nil {
		return domain.AIResult{}, e
	}
	resp, e := s.http.Do(req)
	if e != nil {
		return domain.AIResult{}, e
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return domain.AIResult{}, fmt.Errorf("AI trả về %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	var out response
	if e = json.NewDecoder(resp.Body).Decode(&out); e != nil {
		return domain.AIResult{}, e
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return domain.AIResult{}, errors.New("AI không trả về nội dung")
	}
	return domain.AIResult{Content: out.Choices[0].Message.Content, Usage: usage(&out)}, nil
}
func (s *Client) Stream(ctx context.Context, model string, messages []domain.AIMessage, maxOutput int, emit func(string) error) (domain.AIResult, error) {
	req, e := s.newRequest(ctx, true, model, messages, maxOutput)
	if e != nil {
		return domain.AIResult{}, e
	}
	resp, e := s.http.Do(req)
	if e != nil {
		return domain.AIResult{}, e
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return domain.AIResult{}, fmt.Errorf("AI trả về %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	var answer strings.Builder
	var measured domain.AIUsage
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
			return domain.AIResult{Content: answer.String(), Usage: measured}, fmt.Errorf("dữ liệu stream AI không hợp lệ: %w", e)
		}
		if u := usage(&chunk); u.Reported {
			measured = u
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta.Content
		if delta != "" {
			answer.WriteString(delta)
			if e = emit(delta); e != nil {
				return domain.AIResult{Content: answer.String(), Usage: measured}, e
			}
		}
	}
	if e = scan.Err(); e != nil {
		return domain.AIResult{Content: answer.String(), Usage: measured}, e
	}
	if strings.TrimSpace(answer.String()) == "" {
		return domain.AIResult{Usage: measured}, errors.New("AI không trả về nội dung")
	}
	return domain.AIResult{Content: answer.String(), Usage: measured}, nil
}
