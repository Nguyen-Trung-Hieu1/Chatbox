package httpdelivery

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestLoggingWritesStructuredAccessLog(t *testing.T) {
	var output bytes.Buffer
	previous := structuredLogger.Writer()
	structuredLogger.SetOutput(&output)
	t.Cleanup(func() { structuredLogger.SetOutput(previous) })

	handler := requestLogging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requestID(r) == "" {
			t.Fatal("request ID was not added to the request context")
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/login", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.10, 10.42.0.1")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Header().Get("X-Request-ID") == "" {
		t.Fatal("X-Request-ID response header was not set")
	}

	line, err := bufio.NewReader(&output).ReadBytes('\n')
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	var entry map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(line), &entry); err != nil {
		t.Fatalf("log is not valid JSON: %v", err)
	}
	if entry["event"] != "http_request" || entry["method"] != "POST" || entry["path"] != "/api/login" {
		t.Fatalf("unexpected access log: %#v", entry)
	}
	if entry["status"] != float64(http.StatusCreated) || entry["client_ip"] != "203.0.113.10" {
		t.Fatalf("unexpected request metadata: %#v", entry)
	}
}

func TestResponseRecorderPreservesFlusher(t *testing.T) {
	res := httptest.NewRecorder()
	recorder := &responseRecorder{ResponseWriter: res}
	recorder.Flush()
	if recorder.status != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.status)
	}
}
