package service

import (
	"context"
	"strings"
	"time"

	"backend/models"
	"backend/repository"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ChatService struct {
	conversations *repository.ConversationRepository
	messages      *repository.MessageRepository
	ai            *AIService
	tokenBudget   int
}

const assistantSystemPrompt = `Bạn là trợ lý của Chatbox. Mặc định luôn trả lời bằng tiếng Việt tự nhiên, đầy đủ dấu và nhất quán; chỉ đổi ngôn ngữ hoặc viết không dấu khi người dùng yêu cầu rõ ràng. Trình bày bằng Markdown dễ đọc. Mọi đoạn mã nhiều dòng phải đặt trong fenced code block có ghi ngôn ngữ phù hợp, ví dụ ba dấu backtick kèm js, go hoặc python. Không trộn phần giải thích vào bên trong code block.`

func NewChatService(c *repository.ConversationRepository, m *repository.MessageRepository, ai *AIService, budget int) *ChatService {
	return &ChatService{c, m, ai, budget}
}
func (s *ChatService) CreateConversation(ctx context.Context, owner primitive.ObjectID, title string) (*models.Conversation, error) {
	now := time.Now().UTC()
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Cuộc trò chuyện mới"
	}
	c := &models.Conversation{ID: primitive.NewObjectID(), OwnerID: owner, Title: title, CreatedAt: now, UpdatedAt: now}
	return c, s.conversations.Create(ctx, c)
}
func (s *ChatService) List(ctx context.Context, owner primitive.ObjectID) ([]models.Conversation, error) {
	return s.conversations.ListOwned(ctx, owner)
}
func (s *ChatService) owned(ctx context.Context, idText string, owner primitive.ObjectID) (*models.Conversation, error) {
	id, err := primitive.ObjectIDFromHex(idText)
	if err != nil {
		return nil, ErrInvalidID
	}
	c, err := s.conversations.OwnedByID(ctx, id, owner)
	if repository.IsNotFound(err) {
		return nil, ErrForbidden
	}
	return c, err
}
func (s *ChatService) History(ctx context.Context, idText string, owner primitive.ObjectID) ([]models.Message, error) {
	c, err := s.owned(ctx, idText, owner)
	if err != nil {
		return nil, err
	}
	return s.messages.List(ctx, c.ID)
}
func (s *ChatService) Rename(ctx context.Context, idText, title string, owner primitive.ObjectID) (*models.Conversation, error) {
	c, err := s.owned(ctx, idText, owner)
	if err != nil {
		return nil, err
	}
	title = strings.TrimSpace(title)
	if len([]rune(title)) == 0 || len([]rune(title)) > 100 {
		return nil, ErrInvalidTitle
	}
	if err = s.conversations.Rename(ctx, c.ID, title); err != nil {
		return nil, err
	}
	c.Title = title
	c.UpdatedAt = time.Now().UTC()
	return c, nil
}
func (s *ChatService) Delete(ctx context.Context, idText string, owner primitive.ObjectID) error {
	c, err := s.owned(ctx, idText, owner)
	if err != nil {
		return err
	}
	if err = s.messages.DeleteByConversation(ctx, c.ID); err != nil {
		return err
	}
	return s.conversations.Delete(ctx, c.ID)
}
func (s *ChatService) Send(ctx context.Context, idText, content string, owner primitive.ObjectID) (*models.Message, error) {
	c, err := s.owned(ctx, idText, owner)
	if err != nil {
		return nil, err
	}
	content = strings.TrimSpace(content)
	now := time.Now().UTC()
	user := &models.Message{ID: primitive.NewObjectID(), ConversationID: c.ID, Role: "user", Content: content, CreatedAt: now}
	if err = s.messages.Create(ctx, user); err != nil {
		return nil, err
	}
	title := ""
	if c.Title == "Cuộc trò chuyện mới" {
		title = content
		if len([]rune(title)) > 30 {
			title = string([]rune(title)[:30]) + "..."
		}
	}
	_ = s.conversations.UpdateActivity(ctx, c.ID, title)
	contextMessages, err := s.buildContext(ctx, c)
	if err != nil {
		return nil, err
	}
	answer, err := s.ai.Complete(ctx, contextMessages)
	if err != nil {
		return nil, err
	}
	reply := &models.Message{ID: primitive.NewObjectID(), ConversationID: c.ID, Role: "assistant", Content: answer, CreatedAt: time.Now().UTC()}
	if err = s.messages.Create(ctx, reply); err != nil {
		return nil, err
	}
	_ = s.conversations.UpdateActivity(ctx, c.ID, "")
	return reply, nil
}
func (s *ChatService) SendStream(ctx context.Context, idText, content string, owner primitive.ObjectID, emit func(string) error) (*models.Message, error) {
	c, err := s.owned(ctx, idText, owner)
	if err != nil {
		return nil, err
	}
	content = strings.TrimSpace(content)
	now := time.Now().UTC()
	user := &models.Message{ID: primitive.NewObjectID(), ConversationID: c.ID, Role: "user", Content: content, CreatedAt: now}
	if err = s.messages.Create(ctx, user); err != nil {
		return nil, err
	}
	title := ""
	if c.Title == "Cuộc trò chuyện mới" {
		title = content
		if len([]rune(title)) > 30 {
			title = string([]rune(title)[:30]) + "..."
		}
	}
	_ = s.conversations.UpdateActivity(ctx, c.ID, title)
	contextMessages, err := s.buildContext(ctx, c)
	if err != nil {
		return nil, err
	}
	answer, streamErr := s.ai.Stream(ctx, contextMessages, emit)
	if strings.TrimSpace(answer) == "" {
		return nil, streamErr
	}
	reply := &models.Message{ID: primitive.NewObjectID(), ConversationID: c.ID, Role: "assistant", Content: answer, CreatedAt: time.Now().UTC()}
	saveCtx := ctx
	var cancel context.CancelFunc
	if streamErr != nil || ctx.Err() != nil {
		saveCtx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
	}
	if err = s.messages.Create(saveCtx, reply); err != nil {
		return nil, err
	}
	_ = s.conversations.UpdateActivity(saveCtx, c.ID, "")
	return reply, streamErr
}
func (s *ChatService) buildContext(ctx context.Context, c *models.Conversation) ([]aiMessage, error) {
	msgs, err := s.messages.Unsummarized(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	// Keep the summary bounded so it can never crowd the newest user message out.
	c.Summary = truncateToTokens(c.Summary, max(128, s.tokenBudget/3))
	summaryTokens := estimateTokens(c.Summary)
	used := summaryTokens
	start := len(msgs)
	for start > 0 {
		t := estimateTokens(msgs[start-1].Content)
		if start < len(msgs) && used+t > s.tokenBudget {
			break
		}
		used += t
		start--
	}
	if start > 0 {
		// Summarize a bounded oldest batch. This keeps the summarization request
		// itself within budget even after a very long conversation.
		end := 0
		batchTokens := 0
		for end < start {
			t := estimateTokens(msgs[end].Content)
			if end > 0 && batchTokens+t > max(256, s.tokenBudget/2) {
				break
			}
			batchTokens += t
			end++
		}
		old := msgs[:end]
		prompt := []aiMessage{{"system", "Tóm tắt ngắn gọn hội thoại, giữ lại sự kiện, yêu cầu và quyết định quan trọng để dùng làm ngữ cảnh cho lượt sau."}, {"user", "Tóm tắt hiện tại:\n" + c.Summary + "\n\nCác tin nhắn mới cần gộp:\n" + formatMessages(old)}}
		summary, summaryErr := s.ai.Complete(ctx, prompt)
		if summaryErr == nil {
			ids := make([]primitive.ObjectID, 0, len(old))
			for _, m := range old {
				ids = append(ids, m.ID)
			}
			if err = s.conversations.UpdateSummary(ctx, c.ID, summary); err == nil {
				_ = s.messages.MarkSummarized(ctx, ids)
				c.Summary = summary
			}
		}
	}
	out := make([]aiMessage, 0, len(msgs)-start+2)
	out = append(out, aiMessage{"system", assistantSystemPrompt})
	if c.Summary != "" {
		out = append(out, aiMessage{"system", "Tóm tắt hội thoại trước đó:\n" + c.Summary})
	}
	out = append(out, toAI(msgs[start:])...)
	return out, nil
}
func formatMessages(messages []models.Message) string {
	var b strings.Builder
	for _, m := range messages {
		b.WriteString(m.Role)
		b.WriteString(": ")
		b.WriteString(m.Content)
		b.WriteByte('\n')
	}
	return b.String()
}

func truncateToTokens(text string, budget int) string {
	runes := []rune(text)
	limit := budget * 4
	if len(runes) <= limit {
		return text
	}
	return string(runes[len(runes)-limit:])
}
