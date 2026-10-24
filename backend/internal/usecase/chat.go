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
	conversations          domain.ConversationRepository
	messages               domain.MessageRepository
	ai                     domain.AIClient
	tokenBudget            int
	billing                domain.BillingRepository
	freeModels, paidModels []domain.ModelPolicy
	initialCredits         int64
}

func NewChat(c domain.ConversationRepository, m domain.MessageRepository, ai domain.AIClient, budget int, billing domain.BillingRepository, free, paid []domain.ModelPolicy, initial int64) *Chat {
	return &Chat{conversations: c, messages: m, ai: ai, tokenBudget: budget, billing: billing, freeModels: free, paidModels: paid, initialCredits: initial}
}

func (s *Chat) Entitlement(ctx context.Context, user string) (*domain.Entitlement, error) {
	if err := s.billing.Ensure(ctx, user, s.initialCredits); err != nil {
		return nil, err
	}
	plan, paidUntil, creditResetAt, err := s.billing.Plan(ctx, user, s.initialCredits)
	if err != nil {
		return nil, err
	}
	balance, err := s.billing.Balance(ctx, user)
	if err != nil {
		return nil, err
	}
	models := s.freeModels
	if plan == "paid" && paidUntil != nil && time.Now().Before(*paidUntil) {
		models = s.paidModels
	} else {
		plan, paidUntil = "free", nil
	}
	return &domain.Entitlement{Plan: plan, PaidUntil: paidUntil, CreditResetAt: creditResetAt, Balance: balance, Models: models}, nil
}
func (s *Chat) UsageHistory(ctx context.Context, user string) ([]domain.UsageRecord, error) {
	return s.billing.History(ctx, user)
}
func (s *Chat) policy(ctx context.Context, user, model string) (domain.ModelPolicy, error) {
	e, err := s.Entitlement(ctx, user)
	if err != nil {
		return domain.ModelPolicy{}, err
	}
	if model == "" && len(e.Models) > 0 {
		model = e.Models[0].ID
	}
	for _, p := range e.Models {
		if p.ID == model {
			return p, nil
		}
	}
	return domain.ModelPolicy{}, domain.ErrModelForbidden
}
func credits(p domain.ModelPolicy, in, out int) int64 {
	n := int64(in)*p.InputRate + int64(out)*p.OutputRate
	return (n + 999) / 1000
}
func estimatedUsage(messages []domain.AIMessage, output string) domain.AIUsage {
	in := 0
	for _, m := range messages {
		in += tokens(m.Content)
	}
	return domain.AIUsage{InputTokens: in, OutputTokens: tokens(output)}
}
func (s *Chat) charge(owner string, p domain.ModelPolicy, kind string, reserve int64, result domain.AIResult, messages []domain.AIMessage, status string) error {
	u := billableUsage(result, messages, status)
	spent := credits(p, u.InputTokens, u.OutputTokens)
	if spent > reserve {
		spent = reserve
	}
	settleCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.billing.Settle(settleCtx, owner, reserve, spent, domain.UsageRecord{UserID: owner, Model: p.ID, Kind: kind, InputTokens: u.InputTokens, OutputTokens: u.OutputTokens, Reported: u.Reported, Status: status, CreatedAt: time.Now().UTC()})
}

func billableUsage(result domain.AIResult, messages []domain.AIMessage, status string) domain.AIUsage {
	u := result.Usage
	if status == "interrupted" && u.Reported && tokens(result.Content) > u.OutputTokens {
		u.OutputTokens = tokens(result.Content)
		u.Reported = false
	}
	if status == "failed" && result.Content == "" && !u.Reported {
		u = domain.AIUsage{}
	} else if !u.Reported {
		u = estimatedUsage(messages, result.Content)
	}
	return u
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
func (s *Chat) SendStream(ctx context.Context, id, content, owner, model string, emit func(string) error) (*domain.Message, error) {
	c, err := s.owned(ctx, id, owner)
	if err != nil {
		return nil, err
	}
	content = strings.TrimSpace(content)
	if len([]rune(content)) > 20000 {
		return nil, domain.ErrInvalidContent
	}
	p, err := s.policy(ctx, owner, model)
	if err != nil {
		return nil, err
	}
	if p.MaxOutputTokens <= 0 {
		p.MaxOutputTokens = 1024
	}
	contextMessages, err := s.buildContext(ctx, c, owner, p)
	if err != nil {
		return nil, err
	}
	contextMessages = append(contextMessages, domain.AIMessage{Role: "user", Content: content})
	est := estimatedUsage(contextMessages, "")
	reserve := credits(p, est.InputTokens*4, p.MaxOutputTokens)
	if err = s.billing.Reserve(ctx, owner, reserve); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	u := &domain.Message{ConversationID: c.ID, Role: "user", Content: content, CreatedAt: now}
	if err = s.messages.Create(ctx, u); err != nil {
		_ = s.charge(owner, p, "chat", reserve, domain.AIResult{}, nil, "failed")
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
	result, streamErr := s.ai.Stream(ctx, p.ID, contextMessages, p.MaxOutputTokens, emit)
	status := "completed"
	if streamErr != nil {
		status = "interrupted"
		if result.Content == "" && !result.Usage.Reported {
			status = "failed"
		}
	}
	if chargeErr := s.charge(owner, p, "chat", reserve, result, contextMessages, status); chargeErr != nil {
		return nil, chargeErr
	}
	if strings.TrimSpace(result.Content) == "" {
		return nil, streamErr
	}
	reply := &domain.Message{ConversationID: c.ID, Role: "assistant", Content: result.Content, CreatedAt: time.Now().UTC()}
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
func (s *Chat) buildContext(ctx context.Context, c *domain.Conversation, owner string, p domain.ModelPolicy) ([]domain.AIMessage, error) {
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
		summaryReserve := credits(p, estimatedUsage(prompt, "").InputTokens*4, p.MaxOutputTokens)
		if e := s.billing.Reserve(ctx, owner, summaryReserve); e == nil {
			if summary, e := s.ai.Complete(ctx, p.ID, prompt, p.MaxOutputTokens); e == nil {
				if chargeErr := s.charge(owner, p, "summary", summaryReserve, summary, prompt, "completed"); chargeErr != nil {
					return nil, chargeErr
				}
				ids := make([]string, len(old))
				for i, m := range old {
					ids[i] = m.ID
				}
				if e = s.conversations.UpdateSummary(ctx, c.ID, summary.Content); e == nil {
					_ = s.messages.MarkSummarized(ctx, ids)
					c.Summary = summary.Content
				}
			} else {
				if chargeErr := s.charge(owner, p, "summary", summaryReserve, summary, prompt, "failed"); chargeErr != nil {
					return nil, chargeErr
				}
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
