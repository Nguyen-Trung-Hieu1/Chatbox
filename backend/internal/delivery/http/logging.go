package httpdelivery

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

type requestIDKeyType int

const requestIDKey requestIDKeyType = 0

var structuredLogger = log.New(os.Stdout, "", 0)

type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *responseRecorder) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseRecorder) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytes += n
	return n, err
}

func (w *responseRecorder) Flush() {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func newRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return time.Now().UTC().Format("20060102T150405.000000000")
	}
	return hex.EncodeToString(b)
}

func requestID(r *http.Request) string {
	id, _ := r.Context().Value(requestIDKey).(string)
	return id
}

func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		return strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func writeLog(fields map[string]any) {
	fields["timestamp"] = time.Now().UTC().Format(time.RFC3339Nano)
	b, err := json.Marshal(fields)
	if err == nil {
		structuredLogger.Print(string(b))
	}
}

func logEvent(r *http.Request, level, event string, extra map[string]any) {
	fields := map[string]any{
		"level":      level,
		"event":      event,
		"request_id": requestID(r),
	}
	for key, value := range extra {
		fields[key] = value
	}
	writeLog(fields)
}

func requestLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		id := newRequestID()
		w.Header().Set("X-Request-ID", id)
		r = r.WithContext(context.WithValue(r.Context(), requestIDKey, id))
		recorder := &responseRecorder{ResponseWriter: w}

		next.ServeHTTP(recorder, r)
		if recorder.status == 0 {
			recorder.status = http.StatusOK
		}

		level := "info"
		if recorder.status >= 500 {
			level = "error"
		} else if recorder.status >= 400 {
			level = "warn"
		}
		writeLog(map[string]any{
			"level":       level,
			"event":       "http_request",
			"request_id":  id,
			"method":      r.Method,
			"path":        r.URL.Path,
			"status":      recorder.status,
			"duration_ms": time.Since(started).Milliseconds(),
			"bytes":       recorder.bytes,
			"client_ip":   clientIP(r),
		})
	})
}
