package usecase

import (
	"backend/internal/domain"
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

var ErrInvalidAPIMessages = errors.New("messages không hợp lệ; chỉ hỗ trợ nội dung text với role system, developer, user hoặc assistant")

// APIGenerate uses the same model policy, wallet reservation and settlement as web chat.
// It deliberately does not create a Chatbox conversation for IDE requests.
func (s *Chat) APIGenerate(ctx context.Context, owner, model string, messages []domain.AIMessage, maxOutput int, stream bool, emit func(string) error) (domain.AIResult, error) {
	if len(messages) == 0 || len(messages) > 100 {
		return domain.AIResult{}, ErrInvalidAPIMessages
	}
	characters := 0
	for _, message := range messages {
		if message.Role != "system" && message.Role != "developer" && message.Role != "user" && message.Role != "assistant" {
			return domain.AIResult{}, ErrInvalidAPIMessages
		}
		if strings.TrimSpace(message.Content) == "" {
			return domain.AIResult{}, ErrInvalidAPIMessages
		}
		characters += utf8.RuneCountInString(message.Content)
	}
	if characters > 100000 {
		return domain.AIResult{}, domain.ErrInvalidContent
	}
	p, err := s.policy(ctx, owner, model)
	if err != nil {
		return domain.AIResult{}, err
	}
	if maxOutput < 0 {
		return domain.AIResult{}, domain.ErrInvalidContent
	}
	if maxOutput == 0 || maxOutput > p.MaxOutputTokens {
		maxOutput = p.MaxOutputTokens
	}
	reserve := credits(p, estimatedUsage(messages, "").InputTokens*4, maxOutput)
	if err = s.billing.Reserve(ctx, owner, reserve); err != nil {
		return domain.AIResult{}, err
	}
	var result domain.AIResult
	if stream {
		result, err = s.ai.Stream(ctx, p.ID, messages, maxOutput, emit)
	} else {
		result, err = s.ai.Complete(ctx, p.ID, messages, maxOutput)
	}
	status := "completed"
	if err != nil {
		status = "interrupted"
		if result.Content == "" && !result.Usage.Reported {
			status = "failed"
		}
	}
	if chargeErr := s.charge(owner, p, "api", reserve, result, messages, status); chargeErr != nil {
		return result, chargeErr
	}
	result.Usage = billableUsage(result, messages, status)
	return result, err
}
