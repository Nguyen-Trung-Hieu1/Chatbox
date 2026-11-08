package postgres

import (
	"backend/internal/domain"
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

func scanConversation(row pgx.Row) (*domain.Conversation, error) {
	c := &domain.Conversation{}
	err := row.Scan(&c.ID, &c.OwnerID, &c.Title, &c.Summary, &c.CreatedAt, &c.UpdatedAt)
	return c, mapErr(err)
}

const conversationColumns = `id, owner_id, title, summary, created_at, updated_at`

func (s *Store) CreateConversation(ctx context.Context, c *domain.Conversation) error {
	if err := validID(c.OwnerID); err != nil {
		return err
	}
	c.ID = newID()
	_, err := s.pool.Exec(ctx, `INSERT INTO chatbox.conversations (id,owner_id,title,summary,created_at,updated_at) VALUES ($1,$2,$3,$4,$5,$6)`,
		c.ID, c.OwnerID, c.Title, c.Summary, c.CreatedAt, c.UpdatedAt)
	return mapErr(err)
}

func (s *Store) OwnedByID(ctx context.Context, id, owner string) (*domain.Conversation, error) {
	if err := validID(id); err != nil {
		return nil, err
	}
	if err := validID(owner); err != nil {
		return nil, err
	}
	return scanConversation(s.pool.QueryRow(ctx, `SELECT `+conversationColumns+` FROM chatbox.conversations WHERE id=$1 AND owner_id=$2`, id, owner))
}

func (s *Store) ListOwned(ctx context.Context, owner string) ([]domain.Conversation, error) {
	if err := validID(owner); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT `+conversationColumns+` FROM chatbox.conversations WHERE owner_id=$1 ORDER BY updated_at DESC`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Conversation{}
	for rows.Next() {
		c, err := scanConversation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

func (s *Store) UpdateActivity(ctx context.Context, id, title string) error {
	if err := validID(id); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `UPDATE chatbox.conversations SET updated_at=$2,title=CASE WHEN $3<>'' THEN $3 ELSE title END WHERE id=$1`, id, time.Now().UTC(), title)
	return err
}

func (s *Store) UpdateSummary(ctx context.Context, id, summary string) error {
	if err := validID(id); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `UPDATE chatbox.conversations SET summary=$2 WHERE id=$1`, id, summary)
	return err
}

func (s *Store) Rename(ctx context.Context, id, title string) error {
	if err := validID(id); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `UPDATE chatbox.conversations SET title=$2,updated_at=$3 WHERE id=$1`, id, title, time.Now().UTC())
	return err
}

func (s *Store) Delete(ctx context.Context, id string) error {
	if err := validID(id); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM chatbox.conversations WHERE id=$1`, id)
	return err
}

type ConversationRepository struct{ *Store }

func (r ConversationRepository) Create(ctx context.Context, c *domain.Conversation) error {
	return r.Store.CreateConversation(ctx, c)
}

func scanMessage(row pgx.Row) (domain.Message, error) {
	var m domain.Message
	err := row.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.Summarized, &m.CreatedAt)
	return m, err
}

const messageColumns = `id, conversation_id, role, content, summarized, created_at`

func (s *Store) CreateMessage(ctx context.Context, m *domain.Message) error {
	if err := validID(m.ConversationID); err != nil {
		return err
	}
	m.ID = newID()
	_, err := s.pool.Exec(ctx, `INSERT INTO chatbox.messages (id,conversation_id,role,content,summarized,created_at) VALUES ($1,$2,$3,$4,$5,$6)`,
		m.ID, m.ConversationID, m.Role, m.Content, m.Summarized, m.CreatedAt)
	return mapErr(err)
}

func (s *Store) messages(ctx context.Context, id string, unsummarized bool) ([]domain.Message, error) {
	if err := validID(id); err != nil {
		return nil, err
	}
	query := `SELECT ` + messageColumns + ` FROM chatbox.messages WHERE conversation_id=$1`
	if unsummarized {
		query += ` AND summarized=false`
	}
	query += ` ORDER BY created_at,id`
	rows, err := s.pool.Query(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Message{}
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) List(ctx context.Context, id string) ([]domain.Message, error) {
	return s.messages(ctx, id, false)
}
func (s *Store) Unsummarized(ctx context.Context, id string) ([]domain.Message, error) {
	return s.messages(ctx, id, true)
}
func (s *Store) MarkSummarized(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	for _, id := range ids {
		if err := validID(id); err != nil {
			return err
		}
	}
	_, err := s.pool.Exec(ctx, `UPDATE chatbox.messages SET summarized=true WHERE id=ANY($1)`, ids)
	return err
}
func (s *Store) DeleteByConversation(ctx context.Context, id string) error {
	if err := validID(id); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM chatbox.messages WHERE conversation_id=$1`, id)
	return err
}

type MessageRepository struct{ *Store }

func (r MessageRepository) Create(ctx context.Context, m *domain.Message) error {
	return r.Store.CreateMessage(ctx, m)
}
