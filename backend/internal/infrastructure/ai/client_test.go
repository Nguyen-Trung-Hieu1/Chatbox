package ai

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"backend/internal/domain"
)

func TestStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, content := range []string{"Xin ", "chào"} {
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", content)
			w.(http.Flusher).Flush()
		}
		fmt.Fprint(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":12,\"completion_tokens\":3}}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	client := New(server.URL, "", "test")
	var deltas strings.Builder
	answer, err := client.Stream(context.Background(), "test", []domain.AIMessage{{Role: "user", Content: "hello"}}, 128, func(delta string) error {
		deltas.WriteString(delta)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if answer.Content != "Xin chào" || deltas.String() != answer.Content {
		t.Fatalf("unexpected stream result: answer=%q deltas=%q", answer.Content, deltas.String())
	}
	if !answer.Usage.Reported || answer.Usage.InputTokens != 12 || answer.Usage.OutputTokens != 3 {
		t.Fatalf("unexpected usage: %+v", answer.Usage)
	}
}
