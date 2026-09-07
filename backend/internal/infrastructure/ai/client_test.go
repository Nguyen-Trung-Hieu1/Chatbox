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
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	client := New(server.URL, "", "test")
	var deltas strings.Builder
	answer, err := client.Stream(context.Background(), []domain.AIMessage{{Role: "user", Content: "hello"}}, func(delta string) error {
		deltas.WriteString(delta)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if answer != "Xin chào" || deltas.String() != answer {
		t.Fatalf("unexpected stream result: answer=%q deltas=%q", answer, deltas.String())
	}
}
