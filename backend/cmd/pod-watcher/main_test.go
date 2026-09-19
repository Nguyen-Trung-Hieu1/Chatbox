package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestPodEventsNotifyOnce(t *testing.T) {
	var messages []string
	w := &watcher{
		telegram: &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Host != "api.telegram.org" || req.URL.Path != "/bottest-token/sendMessage" {
				t.Fatalf("unexpected Telegram request: %s", req.URL)
			}
			if err := req.ParseForm(); err != nil {
				t.Fatal(err)
			}
			messages = append(messages, req.Form.Get("text"))
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}, nil
		})},
		botToken: "test-token", chatID: "123", namespace: "chatbox1",
		known: make(map[string]pod), notified: make(map[string]bool),
	}
	var item pod
	item.Metadata.Name, item.Metadata.UID = "test-pod", "pod-uid"
	w.handle(context.Background(), "ADDED", item)
	if len(messages) != 0 {
		t.Fatal("creating a Pod must not trigger an alert")
	}
	w.handle(context.Background(), "DELETED", item)
	w.handle(context.Background(), "DELETED", item)
	if len(messages) != 1 || !strings.Contains(messages[0], "Event: deleted") {
		t.Fatalf("expected one deletion alert, got %v", messages)
	}
	item.Metadata.Name, item.Metadata.UID = "failed-pod", "failed-uid"
	item.Status.Phase = "Failed"
	w.handle(context.Background(), "MODIFIED", item)
	w.handle(context.Background(), "MODIFIED", item)
	if len(messages) != 2 || !strings.Contains(messages[1], "Event: stopped: Failed") {
		t.Fatalf("expected one failed Pod alert, got %v", messages)
	}
	item.Metadata.Name, item.Metadata.UID = "ready-pod", "ready-uid"
	item.Status.Phase = "Running"
	item.Status.Conditions = nil
	item.Status.Conditions = append(item.Status.Conditions, struct {
		Type   string `json:"type"`
		Status string `json:"status"`
	}{Type: "Ready", Status: "True"})
	w.handle(context.Background(), "ADDED", item)
	item.Status.Conditions = append([]struct {
		Type   string `json:"type"`
		Status string `json:"status"`
	}(nil), item.Status.Conditions...)
	item.Status.Conditions[0].Status = "False"
	w.handle(context.Background(), "MODIFIED", item)
	w.handle(context.Background(), "MODIFIED", item)
	if len(messages) != 3 || !strings.Contains(messages[2], "Event: stopped: NotReady") {
		t.Fatalf("expected one NotReady alert, got %v", messages)
	}
}
