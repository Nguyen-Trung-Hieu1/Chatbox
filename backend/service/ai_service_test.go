package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAIServiceStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		for _, content := range []string{"Xin ", "chào"} {
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", content)
			flusher.Flush()
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	service := NewAIService(server.URL, "", "test")
	var deltas strings.Builder
	answer, err := service.Stream(context.Background(), []aiMessage{{Role: "user", Content: "hello"}}, func(delta string) error {
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
