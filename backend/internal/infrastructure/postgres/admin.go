package postgres

import (
	"backend/internal/domain"
	"context"
	"time"
)

func (s *Store) ListAccounts(ctx context.Context) ([]domain.AdminAccount, error) {
	rows, err := s.pool.Query(ctx, `SELECT u.id,u.username,COALESCE(w.plan,'free'),w.paid_until,COALESCE(w.balance,0)
        FROM chatbox.users u LEFT JOIN chatbox.wallets w ON w.user_id=u.id ORDER BY u.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.AdminAccount{}
	for rows.Next() {
		var a domain.AdminAccount
		if err := rows.Scan(&a.ID, &a.Username, &a.Plan, &a.PaidUntil, &a.Balance); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) Usage(ctx context.Context, user string) ([]domain.UsageRecord, error) {
	if _, err := s.ByID(ctx, user); err != nil {
		return nil, err
	}
	return s.History(ctx, user)
}

func (s *Store) KeycloakSub(ctx context.Context, user string) (string, error) {
	if err := validID(user); err != nil {
		return "", err
	}
	var sub *string
	err := s.pool.QueryRow(ctx, `SELECT keycloak_sub FROM chatbox.users WHERE id=$1`, user).Scan(&sub)
	if sub == nil {
		return "", mapErr(err)
	}
	return *sub, mapErr(err)
}

func (s *Store) SyncRolePlan(ctx context.Context, user, plan string, freeCredits int64) error {
	if err := validID(user); err != nil {
		return err
	}
	credits := freeCredits
	if plan == "paid" {
		credits = paidCredits
	}
	now := time.Now().UTC()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	_, err = tx.Exec(ctx, `INSERT INTO chatbox.wallets (user_id,plan,plan_source,balance,credit_reset_at)
        VALUES ($1,$2,'keycloak',$3,$4) ON CONFLICT (user_id) DO NOTHING`, user, plan, credits, now.Add(creditPeriod))
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE chatbox.wallets SET plan=$2,plan_source='keycloak',balance=$3,
        credit_reset_at=$4,paid_until=NULL WHERE user_id=$1 AND (plan IS DISTINCT FROM $2 OR plan_source IS DISTINCT FROM 'keycloak')`,
		user, plan, credits, now.Add(creditPeriod))
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE chatbox.wallets SET paid_until=NULL WHERE user_id=$1 AND plan_source='keycloak' AND paid_until IS NOT NULL`, user)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) SetRolePlan(ctx context.Context, user, plan string, credits int64) (*domain.AdminAccount, error) {
	u, err := s.ByID(ctx, user)
	if err != nil {
		return nil, err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO chatbox.wallets (user_id,plan,plan_source,balance,credit_reset_at)
        VALUES ($1,$2,'keycloak',$3,$4)
        ON CONFLICT (user_id) DO UPDATE SET plan=EXCLUDED.plan,plan_source='keycloak',
          balance=EXCLUDED.balance,credit_reset_at=EXCLUDED.credit_reset_at,paid_until=NULL`,
		user, plan, credits, time.Now().UTC().Add(creditPeriod))
	if err != nil {
		return nil, err
	}
	return &domain.AdminAccount{ID: user, Username: u.Username, Plan: plan, Balance: credits}, nil
}

func (s *Store) SetPlan(ctx context.Context, user, plan string, now time.Time, credits int64) (*domain.AdminAccount, error) {
	u, err := s.ByID(ctx, user)
	if err != nil {
		return nil, err
	}
	var until *time.Time
	if plan == "paid" {
		until = ptrTime(now.Add(48 * time.Hour))
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO chatbox.wallets (user_id,plan,balance,paid_until,credit_reset_at)
        VALUES ($1,$2,$3,$4,$5)
        ON CONFLICT (user_id) DO UPDATE SET plan=EXCLUDED.plan,balance=EXCLUDED.balance,
          paid_until=EXCLUDED.paid_until,credit_reset_at=EXCLUDED.credit_reset_at`,
		user, plan, credits, until, now.Add(creditPeriod))
	if err != nil {
		return nil, err
	}
	return &domain.AdminAccount{ID: user, Username: u.Username, Plan: plan, PaidUntil: until, Balance: credits}, nil
}

var _ interface {
	ListAccounts(context.Context) ([]domain.AdminAccount, error)
	Usage(context.Context, string) ([]domain.UsageRecord, error)
	SetPlan(context.Context, string, string, time.Time, int64) (*domain.AdminAccount, error)
} = (*Store)(nil)
