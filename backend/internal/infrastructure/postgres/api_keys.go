package postgres

import (
	"backend/internal/domain"
	"context"

	"github.com/jackc/pgx/v5"
)

const keyColumns = `id,user_id,name,prefix,hash,created_at,last_used_at,revoked_at`

func scanKey(row pgx.Row) (*domain.APIKey, error) {
	k := &domain.APIKey{}
	err := row.Scan(&k.ID, &k.UserID, &k.Name, &k.Prefix, &k.Hash, &k.CreatedAt, &k.LastUsedAt, &k.RevokedAt)
	return k, mapErr(err)
}

func (s *Store) CreateAPIKey(ctx context.Context, k *domain.APIKey) error {
	if err := validID(k.UserID); err != nil {
		return err
	}
	k.ID = newID()
	_, err := s.pool.Exec(ctx, `INSERT INTO chatbox.api_keys (id,user_id,name,prefix,hash,created_at) VALUES ($1,$2,$3,$4,$5,$6)`,
		k.ID, k.UserID, k.Name, k.Prefix, k.Hash, k.CreatedAt)
	return mapErr(err)
}

type APIKeyRepository struct{ *Store }

func (r APIKeyRepository) Create(ctx context.Context, k *domain.APIKey) error {
	return r.Store.CreateAPIKey(ctx, k)
}

func (s *Store) ListKeys(ctx context.Context, user string) ([]domain.APIKey, error) {
	if err := validID(user); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT `+keyColumns+` FROM chatbox.api_keys WHERE user_id=$1 ORDER BY created_at DESC LIMIT 100`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.APIKey{}
	for rows.Next() {
		k, err := scanKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *k)
	}
	return out, rows.Err()
}

func (r APIKeyRepository) List(ctx context.Context, user string) ([]domain.APIKey, error) {
	return r.Store.ListKeys(ctx, user)
}

func (s *Store) ActiveByHash(ctx context.Context, hash string) (*domain.APIKey, error) {
	return scanKey(s.pool.QueryRow(ctx, `SELECT `+keyColumns+` FROM chatbox.api_keys WHERE hash=$1 AND revoked_at IS NULL`, hash))
}

func (s *Store) Revoke(ctx context.Context, user, id string) error {
	if err := validID(user); err != nil {
		return err
	}
	if err := validID(id); err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `UPDATE chatbox.api_keys SET revoked_at=now() WHERE id=$1 AND user_id=$2 AND revoked_at IS NULL`, id, user)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) Touch(ctx context.Context, id string) error {
	if err := validID(id); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `UPDATE chatbox.api_keys SET last_used_at=now() WHERE id=$1`, id)
	return err
}
