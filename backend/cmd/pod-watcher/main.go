package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const serviceAccountDir = "/var/run/secrets/kubernetes.io/serviceaccount"

type pod struct {
	Metadata struct {
		Name              string `json:"name"`
		UID               string `json:"uid"`
		DeletionTimestamp string `json:"deletionTimestamp"`
	} `json:"metadata"`
	Status struct {
		Phase      string `json:"phase"`
		Reason     string `json:"reason"`
		Conditions []struct {
			Type   string `json:"type"`
			Status string `json:"status"`
		} `json:"conditions"`
		ContainerStatuses []struct {
			Name  string `json:"name"`
			State struct {
				Waiting *struct {
					Reason string `json:"reason"`
				} `json:"waiting"`
				Terminated *struct {
					ExitCode int    `json:"exitCode"`
					Reason   string `json:"reason"`
				} `json:"terminated"`
			} `json:"state"`
		} `json:"containerStatuses"`
	} `json:"status"`
}

type podList struct {
	Metadata struct {
		ResourceVersion string `json:"resourceVersion"`
	} `json:"metadata"`
	Items []pod `json:"items"`
}

type watchEvent struct {
	Type   string          `json:"type"`
	Object json.RawMessage `json:"object"`
}

type watcher struct {
	client    *http.Client
	telegram  *http.Client
	kubeURL   string
	token     string
	botToken  string
	chatID    string
	namespace string
	known     map[string]pod
	notified  map[string]bool
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	w, err := newWatcher()
	if err != nil {
		log.Fatal(err)
	}
	for ctx.Err() == nil {
		if err := w.run(ctx); err != nil && ctx.Err() == nil {
			log.Printf("watch interrupted: %v", err)
		}
		select {
		case <-ctx.Done():
		case <-time.After(5 * time.Second):
		}
	}
}

func newWatcher() (*watcher, error) {
	token, err := os.ReadFile(serviceAccountDir + "/token")
	if err != nil {
		return nil, fmt.Errorf("read Kubernetes token: %w", err)
	}
	ca, err := os.ReadFile(serviceAccountDir + "/ca.crt")
	if err != nil {
		return nil, fmt.Errorf("read Kubernetes CA: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(ca) {
		return nil, errors.New("invalid Kubernetes CA")
	}
	botToken, err := os.ReadFile(os.Getenv("TELEGRAM_BOT_TOKEN_FILE"))
	if err != nil {
		return nil, fmt.Errorf("read Telegram bot token: %w", err)
	}
	chatID := os.Getenv("TELEGRAM_CHAT_ID")
	namespace := os.Getenv("WATCH_NAMESPACE")
	if chatID == "" || namespace == "" || strings.TrimSpace(string(botToken)) == "" {
		return nil, errors.New("TELEGRAM_CHAT_ID, WATCH_NAMESPACE and bot token are required")
	}
	host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
	if host == "" || port == "" {
		return nil, errors.New("Kubernetes service address is unavailable")
	}
	return &watcher{
		client:   &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}}},
		telegram: &http.Client{Timeout: 10 * time.Second},
		kubeURL:  fmt.Sprintf("https://%s:%s/api/v1/namespaces/%s/pods", host, port, url.PathEscape(namespace)),
		token:    strings.TrimSpace(string(token)), botToken: strings.TrimSpace(string(botToken)),
		chatID: chatID, namespace: namespace, known: make(map[string]pod), notified: make(map[string]bool),
	}, nil
}

func (w *watcher) request(ctx context.Context, target string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+w.token)
	return w.client.Do(req)
}

func (w *watcher) run(ctx context.Context) error {
	listCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	resp, err := w.request(listCtx, w.kubeURL)
	if err != nil {
		cancel()
		return err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		cancel()
		return fmt.Errorf("list pods: HTTP %d", resp.StatusCode)
	}
	var list podList
	err = json.NewDecoder(resp.Body).Decode(&list)
	resp.Body.Close()
	cancel()
	if err != nil {
		return err
	}
	current := make(map[string]pod, len(list.Items))
	for _, item := range list.Items {
		current[item.Metadata.UID] = item
	}
	// A relist after a broken watch also catches Pods deleted while disconnected.
	for uid, old := range w.known {
		if now, exists := current[uid]; !exists {
			w.notify(ctx, old, "deleted")
		} else if reason := stoppedReason(old, now); reason != "" {
			w.notify(ctx, now, "stopped: "+reason)
		}
	}
	w.known = current
	log.Printf("watching %d Pods in %s", len(current), w.namespace)

	query := url.Values{"watch": {"1"}, "resourceVersion": {list.Metadata.ResourceVersion}, "timeoutSeconds": {"300"}}
	resp, err = w.request(ctx, w.kubeURL+"?"+query.Encode())
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("watch pods: HTTP %d", resp.StatusCode)
	}
	decoder := json.NewDecoder(resp.Body)
	for ctx.Err() == nil {
		var event watchEvent
		if err := decoder.Decode(&event); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if event.Type == "ERROR" {
			return errors.New("Kubernetes watch expired or failed")
		}
		if event.Type == "BOOKMARK" {
			continue
		}
		var item pod
		if err := json.Unmarshal(event.Object, &item); err != nil || item.Metadata.UID == "" {
			continue
		}
		w.handle(ctx, event.Type, item)
	}
	return ctx.Err()
}

func (w *watcher) handle(ctx context.Context, eventType string, item pod) {
	uid := item.Metadata.UID
	if eventType == "DELETED" || item.Metadata.DeletionTimestamp != "" {
		w.notify(ctx, item, "deleted")
		delete(w.known, uid)
		return
	}
	previous := w.known[uid]
	w.known[uid] = item
	if reason := stoppedReason(previous, item); reason != "" {
		w.notify(ctx, item, "stopped: "+reason)
	}
}

func ready(item pod) bool {
	for _, condition := range item.Status.Conditions {
		if condition.Type == "Ready" && condition.Status == "True" {
			return true
		}
	}
	return false
}

func stoppedReason(previous, current pod) string {
	if current.Status.Phase == "Failed" || current.Status.Phase == "Unknown" {
		return current.Status.Phase
	}
	for _, container := range current.Status.ContainerStatuses {
		if waiting := container.State.Waiting; waiting != nil {
			switch waiting.Reason {
			case "CrashLoopBackOff", "ImagePullBackOff", "ErrImagePull":
				return container.Name + " " + waiting.Reason
			}
		}
		if terminated := container.State.Terminated; terminated != nil && terminated.ExitCode != 0 {
			return fmt.Sprintf("%s exited with code %d", container.Name, terminated.ExitCode)
		}
	}
	if ready(previous) && !ready(current) {
		return "NotReady"
	}
	return ""
}

func (w *watcher) notify(ctx context.Context, item pod, reason string) {
	key := item.Metadata.UID + ":stopped"
	if reason == "deleted" {
		key = item.Metadata.UID + ":deleted"
	}
	if w.notified[key] {
		return
	}
	message := fmt.Sprintf("Chatbox Pod alert\nNamespace: %s\nPod: %s\nEvent: %s\nTime: %s", w.namespace, item.Metadata.Name, reason, time.Now().Format(time.RFC3339))
	if item.Status.Reason != "" {
		message += "\nReason: " + item.Status.Reason
	}
	form := url.Values{"chat_id": {w.chatID}, "text": {message}}
	requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, "https://api.telegram.org/bot"+w.botToken+"/sendMessage", strings.NewReader(form.Encode()))
	if err != nil {
		log.Printf("prepare Telegram notification: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := w.telegram.Do(req)
	if err != nil {
		log.Printf("send Telegram notification for %s: %v", item.Metadata.Name, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("send Telegram notification for %s: HTTP %d", item.Metadata.Name, resp.StatusCode)
		return
	}
	w.notified[key] = true
	log.Printf("notified %s for Pod %s", reason, item.Metadata.Name)
}
