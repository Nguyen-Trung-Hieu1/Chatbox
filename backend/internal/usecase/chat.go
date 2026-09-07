package usecase

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"backend/internal/domain"
)

const assistantSystemPrompt = `Bạn là trợ lý của Chatbox. Mặc định luôn trả lời bằng tiếng Việt tự nhiên, đầy đủ dấu và nhất quán; chỉ đổi ngôn ngữ hoặc viết không dấu khi người dùng yêu cầu rõ ràng. Trình bày bằng Markdown dễ đọc. Mọi đoạn mã nhiều dòng phải đặt trong fenced code block có ghi ngôn ngữ phù hợp.`

type Chat struct {
	conversations domain.ConversationRepository
	messages      domain.MessageRepository
	ai            domain.AIClient
	tokenBudget   int
}

func NewChat(c domain.ConversationRepository, m domain.MessageRepository, ai domain.AIClient, budget int) *Chat {
	return &Chat{c, m, ai, budget}
}

func (s *Chat) CreateConversation(ctx context.Context, owner, title string) (*domain.Conversation, error) {
	now := time.Now().UTC()
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Cuộc trò chuyện mới"
	}
	c := &domain.Conversation{OwnerID: owner, Title: title, CreatedAt: now, UpdatedAt: now}
	return c, s.conversations.Create(ctx, c)
}
func (s *Chat) List(ctx context.Context, owner string) ([]domain.Conversation, error) {
	return s.conversations.ListOwned(ctx, owner)
}
func (s *Chat) owned(ctx context.Context, id, owner string) (*domain.Conversation, error) {
	if len(id) != 24 {
		return nil, domain.ErrInvalidID
	}
	c, err := s.conversations.OwnedByID(ctx, id, owner)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, domain.ErrForbidden
	}
	return c, err
}
func (s *Chat) History(ctx context.Context, id, owner string) ([]domain.Message, error) {
	c, err := s.owned(ctx, id, owner)
	if err != nil {
		return nil, err
	}
	return s.messages.List(ctx, c.ID)
}
func (s *Chat) Rename(ctx context.Context, id, title, owner string) (*domain.Conversation, error) {
	c, err := s.owned(ctx, id, owner)
	if err != nil {
		return nil, err
	}
	title = strings.TrimSpace(title)
	if len([]rune(title)) == 0 || len([]rune(title)) > 100 {
		return nil, domain.ErrInvalidTitle
	}
	if err = s.conversations.Rename(ctx, c.ID, title); err != nil {
		return nil, err
	}
	c.Title = title
	c.UpdatedAt = time.Now().UTC()
	return c, nil
}
func (s *Chat) Delete(ctx context.Context, id, owner string) error {
	c, err := s.owned(ctx, id, owner)
	if err != nil {
		return err
	}
	if err = s.messages.DeleteByConversation(ctx, c.ID); err != nil {
		return err
	}
	return s.conversations.Delete(ctx, c.ID)
}
func (s *Chat) SendStream(ctx context.Context, id, content, owner string, emit func(string) error) (*domain.Message, error) {
	c, err := s.owned(ctx, id, owner)
	if err != nil {
		return nil, err
	}
	content = strings.TrimSpace(content)
	now := time.Now().UTC()
	u := &domain.Message{ConversationID: c.ID, Role: "user", Content: content, CreatedAt: now}
	if err = s.messages.Create(ctx, u); err != nil {
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
	reply := &domain.Message{ConversationID: c.ID, Role: "assistant", Content: answer, CreatedAt: time.Now().UTC()}
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
func (s *Chat) buildContext(ctx context.Context, c *domain.Conversation) ([]domain.AIMessage, error) {
	msgs, err := s.messages.Unsummarized(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	c.Summary = truncate(c.Summary, max(128, s.tokenBudget/3))
	used := tokens(c.Summary)
	start := len(msgs)
	for start > 0 {
		t := tokens(msgs[start-1].Content)
		if start < len(msgs) && used+t > s.tokenBudget {
			break
		}
		used += t
		start--
	}
	if start > 0 {
		end, batch := 0, 0
		for end < start {
			t := tokens(msgs[end].Content)
			if end > 0 && batch+t > max(256, s.tokenBudget/2) {
				break
			}
			batch += t
			end++
		}
		old := msgs[:end]
		prompt := []domain.AIMessage{{Role: "system", Content: "Tóm tắt ngắn gọn hội thoại, giữ lại sự kiện, yêu cầu và quyết định quan trọng."}, {Role: "user", Content: "Tóm tắt hiện tại:\n" + c.Summary + "\n\nCác tin nhắn mới cần gộp:\n" + format(old)}}
		if summary, e := s.ai.Complete(ctx, prompt); e == nil {
			ids := make([]string, len(old))
			for i, m := range old {
				ids[i] = m.ID
			}
			if e = s.conversations.UpdateSummary(ctx, c.ID, summary); e == nil {
				_ = s.messages.MarkSummarized(ctx, ids)
				c.Summary = summary
			}
		}
	}
	out := []domain.AIMessage{{Role: "system", Content: assistantSystemPrompt}}
	if c.Summary != "" {
		out = append(out, domain.AIMessage{Role: "system", Content: "Tóm tắt hội thoại trước đó:\n" + c.Summary})
	}
	for _, m := range msgs[start:] {
		out = append(out, domain.AIMessage{Role: m.Role, Content: m.Content})
	}
	return out, nil
}
func format(ms []domain.Message) string {
	var b strings.Builder
	for _, m := range ms {
		b.WriteString(m.Role + ": " + m.Content + "\n")
	}
	return b.String()
}
func tokens(v string) int { return (utf8.RuneCountInString(v)+3)/4 + 4 }
func truncate(v string, b int) string {
	r := []rune(v)
	n := b * 4
	if len(r) <= n {
		return v
	}
	return string(r[len(r)-n:])
}
