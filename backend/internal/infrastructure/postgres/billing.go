package postgres

import (
	"backend/internal/domain"
	"context"
	"errors"
	"time"
)

const paidCredits int64 = 2000
const creditPeriod = 2 * time.Hour

func (s *Store) Ensure(ctx context.Context, user string, initial int64) error {
	if err := validID(user); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO chatbox.wallets (user_id,plan,balance,credit_reset_at)
        VALUES ($1,'free',$2,$3) ON CONFLICT (user_id) DO NOTHING`,
		user, initial, time.Now().UTC().Add(creditPeriod))
	return err
}

func (s *Store) Balance(ctx context.Context, user string) (int64, error) {
	if err := validID(user); err != nil {
		return 0, err
	}
	var balance int64
	err := s.pool.QueryRow(ctx, `SELECT balance FROM chatbox.wallets WHERE user_id=$1`, user).Scan(&balance)
	return balance, mapErr(err)
}

func (s *Store) Plan(ctx context.Context, user string, freeCredits int64) (string, *time.Time, *time.Time, error) {
	if err := validID(user); err != nil {
		return "", nil, nil, err
	}
	now := time.Now().UTC()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", nil, nil, err
	}
	defer tx.Rollback(context.Background())
	var plan string
	var source *string
	var paidUntil, resetAt *time.Time
	err = tx.QueryRow(ctx, `SELECT plan,plan_source,paid_until,credit_reset_at FROM chatbox.wallets WHERE user_id=$1 FOR UPDATE`, user).Scan(&plan, &source, &paidUntil, &resetAt)
	if err != nil {
		return "", nil, nil, mapErr(err)
	}
	if plan == "paid" && (source == nil || *source != "keycloak") && (paidUntil == nil || !paidUntil.After(now)) {
		plan, paidUntil, resetAt = "free", nil, ptrTime(now.Add(creditPeriod))
		_, err = tx.Exec(ctx, `UPDATE chatbox.wallets SET plan='free',paid_until=NULL,balance=$2,credit_reset_at=$3 WHERE user_id=$1`, user, freeCredits, resetAt)
	} else if resetAt == nil || !resetAt.After(now) {
		credits := freeCredits
		if plan == "paid" {
			credits = paidCredits
		}
		resetAt = ptrTime(now.Add(creditPeriod))
		_, err = tx.Exec(ctx, `UPDATE chatbox.wallets SET balance=$2,credit_reset_at=$3 WHERE user_id=$1`, user, credits, resetAt)
	}
	if err != nil {
		return "", nil, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", nil, nil, err
	}
	return plan, paidUntil, resetAt, nil
}

func ptrTime(t time.Time) *time.Time { return &t }

func (s *Store) Reserve(ctx context.Context, user string, amount int64) error {
	if amount < 0 {
		return errors.New("invalid reserve")
	}
	if amount == 0 {
		return nil
	}
	if err := validID(user); err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `UPDATE chatbox.wallets SET balance=balance-$2 WHERE user_id=$1 AND balance >= $2`, user, amount)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrQuotaExceeded
	}
	return nil
}

func (s *Store) Settle(ctx context.Context, user string, reserved, spent int64, record domain.UsageRecord) error {
	if err := validID(user); err != nil {
		return err
	}
	if spent < 0 {
		spent = 0
	}
	if spent > reserved {
		spent = reserved
	}
	record.Credits = spent
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	var id string
	if err := tx.QueryRow(ctx, `SELECT user_id FROM chatbox.wallets WHERE user_id=$1 FOR UPDATE`, user).Scan(&id); err != nil {
		return mapErr(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE chatbox.wallets SET balance=balance+$2 WHERE user_id=$1`, user, reserved-spent); err != nil {
		return err
	}
	var position int64
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(position)+1,0) FROM chatbox.usage_records WHERE user_id=$1`, user).Scan(&position); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO chatbox.usage_records
        (user_id,position,model,kind,input_tokens,output_tokens,credits,reported,status,created_at)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		user, position, record.Model, record.Kind, record.InputTokens, record.OutputTokens,
		record.Credits, record.Reported, record.Status, record.CreatedAt)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM chatbox.usage_records WHERE user_id=$1 AND position < $2`, user, position-999); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) History(ctx context.Context, user string) ([]domain.UsageRecord, error) {
	if err := validID(user); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT model,kind,input_tokens,output_tokens,credits,reported,status,created_at
        FROM chatbox.usage_records WHERE user_id=$1 ORDER BY position`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.UsageRecord{}
	for rows.Next() {
		var r domain.UsageRecord
		if err := rows.Scan(&r.Model, &r.Kind, &r.InputTokens, &r.OutputTokens, &r.Credits, &r.Reported, &r.Status, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

var _ domain.BillingRepository = (*Store)(nil)
