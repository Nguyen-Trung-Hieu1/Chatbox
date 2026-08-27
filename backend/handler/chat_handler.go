package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"backend/middleware"
	"backend/models"
	"backend/service"
)

type ChatHandler struct{ chat *service.ChatService }

func NewChatHandler(s *service.ChatService) *ChatHandler { return &ChatHandler{s} }
func (h *ChatHandler) Conversations(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFromContext(r.Context())
	switch r.Method {
	case http.MethodGet:
		list, err := h.chat.List(r.Context(), u.ID)
		if err != nil {
			writeError(w, err)
			return
		}
		if list == nil {
			list = []models.Conversation{}
		}
		writeJSON(w, 200, list)
	case http.MethodPost:
		var in struct {
			Title string `json:"title"`
		}
		_ = decodeJSON(r, &in)
		c, err := h.chat.CreateConversation(r.Context(), u.ID, in.Title)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, 201, c)
	case http.MethodPatch:
		var in struct {
			Title string `json:"title"`
		}
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, 400, map[string]string{"error": "dữ liệu không hợp lệ"})
			return
		}
		c, err := h.chat.Rename(r.Context(), r.URL.Query().Get("id"), in.Title, u.ID)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, 200, c)
	case http.MethodDelete:
		if err := h.chat.Delete(r.Context(), r.URL.Query().Get("id"), u.ID); err != nil {
			writeError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
func (h *ChatHandler) History(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFromContext(r.Context())
	history, err := h.chat.History(r.Context(), r.URL.Query().Get("id"), u.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	if history == nil {
		history = []models.Message{}
	}
	writeJSON(w, 200, history)
}
func (h *ChatHandler) Chat(w http.ResponseWriter, r *http.Request) {
	u := middleware.UserFromContext(r.Context())
	var in struct {
		ConversationID string `json:"conversation_id"`
		Content        string `json:"content"`
	}
	if err := decodeJSON(r, &in); err != nil || strings.TrimSpace(in.Content) == "" {
		writeJSON(w, 400, map[string]string{"error": "nội dung không hợp lệ"})
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, 500, map[string]string{"error": "máy chủ không hỗ trợ streaming"})
		return
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	encoder := json.NewEncoder(w)
	emit := func(content string) error {
		if err := encoder.Encode(map[string]any{"type": "delta", "content": content}); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}
	reply, err := h.chat.SendStream(r.Context(), in.ConversationID, in.Content, u.ID, emit)
	if err != nil {
		if r.Context().Err() == nil {
			_ = encoder.Encode(map[string]any{"type": "error", "error": err.Error()})
			flusher.Flush()
		}
		return
	}
	_ = encoder.Encode(map[string]any{"type": "done", "message": reply})
	flusher.Flush()
}
