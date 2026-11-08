package postgres

import (
	"backend/internal/domain"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Store struct{ pool *pgxpool.Pool }

func New(ctx context.Context, host, database, user, password string) (*Store, error) {
	if host == "" || database == "" || user == "" || password == "" {
		return nil, errors.New("PostgreSQL connection settings are incomplete")
	}
	cfg, err := pgxpool.ParseConfig("host=" + host + " port=5432 dbname=" + database + " sslmode=disable")
	if err != nil {
		return nil, err
	}
	cfg.ConnConfig.User, cfg.ConnConfig.Password = user, password
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

func validID(id string) error {
	if _, err := primitive.ObjectIDFromHex(id); err != nil {
		return domain.ErrInvalidID
	}
	return nil
}

func newID() string { return primitive.NewObjectID().Hex() }

func mapErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return domain.ErrDuplicate
	}
	return err
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func scanUser(row pgx.Row) (*domain.User, error) {
	u := &domain.User{}
	var sub *string
	err := row.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.FirstName, &u.LastName, &sub, &u.IsAdmin, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	if sub != nil {
		u.KeycloakSub = *sub
	}
	return u, nil
}

const userColumns = `id, username, display_name, email, first_name, last_name, keycloak_sub, is_admin, created_at`

func (s *Store) Create(ctx context.Context, u *domain.User) error {
	u.ID = newID()
	_, err := s.pool.Exec(ctx, `INSERT INTO chatbox.users (id,username,created_at,is_admin) VALUES ($1,$2,$3,$4)`, u.ID, u.Username, u.CreatedAt, u.IsAdmin)
	return mapErr(err)
}

func (s *Store) ByUsername(ctx context.Context, name string) (*domain.User, error) {
	u, err := scanUser(s.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM chatbox.users WHERE username=$1`, name))
	return u, mapErr(err)
}

func (s *Store) ByID(ctx context.Context, id string) (*domain.User, error) {
	if err := validID(id); err != nil {
		return nil, err
	}
	u, err := scanUser(s.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM chatbox.users WHERE id=$1`, id))
	return u, mapErr(err)
}

func (s *Store) UpsertKeycloak(ctx context.Context, sub, username, displayName, email, firstName, lastName string) (*domain.User, error) {
	if sub == "" {
		return nil, domain.ErrInvalidCredentials
	}
	username, displayName = strings.TrimSpace(username), strings.TrimSpace(displayName)
	if chars := []rune(displayName); len(chars) > 80 {
		displayName = string(chars[:80])
	}
	if displayName == "" {
		displayName = username
	}
	name := "kc-" + sub
	for attempt := 0; attempt < 2; attempt++ {
		id := newID()
		if attempt == 1 {
			name = "kc-" + id
		}
		row := s.pool.QueryRow(ctx, `INSERT INTO chatbox.users
            (id,username,display_name,email,first_name,last_name,keycloak_sub,is_admin,created_at)
            VALUES ($1,$2,$3,$4,$5,$6,$7,false,$8)
            ON CONFLICT (keycloak_sub) DO UPDATE SET
              display_name=EXCLUDED.display_name,email=EXCLUDED.email,
              first_name=EXCLUDED.first_name,last_name=EXCLUDED.last_name
            RETURNING `+userColumns,
			id, name, displayName, strings.TrimSpace(email), strings.TrimSpace(firstName), strings.TrimSpace(lastName), sub, time.Now().UTC())
		u, err := scanUser(row)
		if err == nil {
			return u, nil
		}
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23505" || pgErr.ConstraintName != "users_username_key" || attempt == 1 {
			return nil, mapErr(err)
		}
	}
	return nil, fmt.Errorf("could not allocate Keycloak username")
}

// Legacy server sessions are disabled when Keycloak roles govern authentication.
type DisabledSessions struct{}

func (DisabledSessions) ActiveByHash(context.Context, string) (*domain.Session, error) {
	return nil, domain.ErrUnauthorized
}
func (DisabledSessions) DeleteByHash(context.Context, string) error { return nil }
func (DisabledSessions) Create(context.Context, *domain.Session) error {
	return domain.ErrUnauthorized
}
