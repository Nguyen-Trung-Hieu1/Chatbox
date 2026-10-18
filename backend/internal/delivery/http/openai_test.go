package httpdelivery

import (
	"backend/internal/domain"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeAPIChat struct {
	ChatUseCase
	model string
	err   error
}

type fakeKeyAuth struct{ APIKeyUseCase }

func (fakeKeyAuth) Authenticate(_ context.Context, raw string) (*domain.User, error) {
	if raw != "cb_valid" {
		return nil, domain.ErrUnauthorized
	}
	return &domain.User{ID: "owner"}, nil
}

func TestAPIAuthRequiresBearerKey(t *testing.T) {
	s := &Server{keys: fakeKeyAuth{}}
	h := s.apiAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user(r.Context()).ID != "owner" {
			t.Fatal("wrong owner")
		}
		w.WriteHeader(204)
	}))
	for _, test := range []struct {
		authorization string
		want          int
	}{{"", 401}, {"Bearer cb_wrong", 401}, {"Bearer cb_valid", 204}} {
		req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		req.Header.Set("Authorization", test.authorization)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != test.want {
			t.Fatalf("%q: got %d", test.authorization, w.Code)
		}
	}
}

func (f *fakeAPIChat) APIGenerate(_ context.Context, _, model string, messages []domain.AIMessage, _ int, stream bool, emit func(string) error) (domain.AIResult, error) {
	f.model = model
	if f.err != nil {
		return domain.AIResult{}, f.err
	}
	if len(messages) != 1 || messages[0].Content != "hello" {
		return domain.AIResult{}, errors.New("wrong messages")
	}
	if stream {
		if err := emit("world"); err != nil {
			return domain.AIResult{}, err
		}
	}
	return domain.AIResult{Content: "world", Usage: domain.AIUsage{InputTokens: 10, OutputTokens: 2, Reported: true}}, nil
}
func (f *fakeAPIChat) Entitlement(context.Context, string) (*domain.Entitlement, error) {
	return &domain.Entitlement{Plan: "free", Models: []domain.ModelPolicy{{ID: "ag/gemini-3-flash"}}}, nil
}

func TestOpenAIModelsListsAllowedIDs(t *testing.T) {
	s := &Server{chat: &fakeAPIChat{}}
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req = req.WithContext(context.WithValue(req.Context(), userKey, &domain.User{ID: "owner"}))
	w := httptest.NewRecorder()
	s.openAIModels(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"ag/gemini-3-flash"`) {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestOpenAIChatResponseAndStreaming(t *testing.T) {
	for _, stream := range []bool{false, true} {
		chat := &fakeAPIChat{}
		s := &Server{chat: chat}
		body := `{"model":"ag/gpt-oss-120b-medium","messages":[{"role":"user","content":"hello"}],"stream":false}`
		if stream {
			body = strings.Replace(body, `"stream":false`, `"stream":true`, 1)
		}
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
		req = req.WithContext(context.WithValue(req.Context(), userKey, &domain.User{ID: "owner"}))
		w := httptest.NewRecorder()
		s.openAIChat(w, req)
		if w.Code != 200 || chat.model != "ag/gpt-oss-120b-medium" {
			t.Fatalf("unexpected response: %d %s", w.Code, w.Body.String())
		}
		if stream {
			if !strings.Contains(w.Body.String(), `"content":"world"`) || !strings.Contains(w.Body.String(), "data: [DONE]") {
				t.Fatal(w.Body.String())
			}
		} else {
			var response struct {
				Object string `json:"object"`
				Usage  struct {
					Total int `json:"total_tokens"`
				} `json:"usage"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || response.Object != "chat.completion" || response.Usage.Total != 12 {
				t.Fatal(w.Body.String(), err)
			}
		}
	}
}

func TestOpenAIChatRejectsQuotaBeforeStreaming(t *testing.T) {
	s := &Server{chat: &fakeAPIChat{err: domain.ErrQuotaExceeded}}
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"ag/gpt-oss-120b-medium","messages":[{"role":"user","content":"hello"}],"stream":true}`))
	req = req.WithContext(context.WithValue(req.Context(), userKey, &domain.User{ID: "owner"}))
	w := httptest.NewRecorder()
	s.openAIChat(w, req)
	if w.Code != 402 || !strings.Contains(w.Body.String(), "insufficient_credits") {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestOpenAITextCompletionsForContinueEdit(t *testing.T) {
	for _, stream := range []bool{false, true} {
		chat := &fakeAPIChat{}
		s := &Server{chat: chat}
		body := `{"model":"ag/claude-sonnet-4-6","prompt":"hello","stream":false}`
		if stream {
			body = strings.Replace(body, `"stream":false`, `"stream":true`, 1)
		}
		req := httptest.NewRequest(http.MethodPost, "/v1/completions", strings.NewReader(body))
		req = req.WithContext(context.WithValue(req.Context(), userKey, &domain.User{ID: "owner"}))
		w := httptest.NewRecorder()
		s.openAICompletions(w, req)
		if w.Code != 200 || chat.model != "ag/claude-sonnet-4-6" {
			t.Fatalf("unexpected response: %d %s", w.Code, w.Body.String())
		}
		if stream {
			if !strings.Contains(w.Body.String(), `"object":"text_completion"`) || !strings.Contains(w.Body.String(), `"text":"world"`) || !strings.Contains(w.Body.String(), "data: [DONE]") {
				t.Fatal(w.Body.String())
			}
		} else {
			var response struct {
				Object  string `json:"object"`
				Choices []struct {
					Text string `json:"text"`
				} `json:"choices"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || response.Object != "text_completion" || len(response.Choices) != 1 || response.Choices[0].Text != "world" {
				t.Fatal(w.Body.String(), err)
			}
		}
	}
}

func TestOpenAITextCompletionsRejectsQuotaAndInvalidPrompt(t *testing.T) {
	for _, tc := range []struct {
		body string
		chat *fakeAPIChat
		want int
	}{
		{`{"model":"ag/claude-sonnet-4-6","prompt":"hello","stream":true}`, &fakeAPIChat{err: domain.ErrQuotaExceeded}, 402},
		{`{"model":"ag/claude-sonnet-4-6","prompt":"hello"}`, &fakeAPIChat{err: domain.ErrModelForbidden}, 403},
		{`{"model":"ag/claude-sonnet-4-6","prompt":["hello"]}`, &fakeAPIChat{}, 400},
	} {
		s := &Server{chat: tc.chat}
		req := httptest.NewRequest(http.MethodPost, "/v1/completions", strings.NewReader(tc.body))
		req = req.WithContext(context.WithValue(req.Context(), userKey, &domain.User{ID: "owner"}))
		w := httptest.NewRecorder()
		s.openAICompletions(w, req)
		if w.Code != tc.want {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}

func TestOpenAITextCompletionsRouteRequiresAPIKey(t *testing.T) {
	s := &Server{keys: fakeKeyAuth{}, chat: &fakeAPIChat{}}
	for _, tc := range []struct {
		key  string
		want int
	}{{"", 401}, {"Bearer cb_valid", 200}} {
		req := httptest.NewRequest(http.MethodPost, "/v1/completions", strings.NewReader(`{"model":"ag/claude-sonnet-4-6","prompt":"hello"}`))
		req.Header.Set("Authorization", tc.key)
		w := httptest.NewRecorder()
		s.Handler("").ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}
