package repository

import (
	"context"
	"errors"
	"time"

	"hookforward/backend/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrUserNotFound = errors.New("user not found")

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (domain.User, error) {
	return r.scanUser(ctx, `
		SELECT id, email, email_verified, password_hash, auth_source, display_name, avatar_url,
		       role, status, last_login_at, created_at, updated_at
		FROM users
		WHERE email = $1 AND is_deleted = FALSE
	`, email)
}

func (r *UserRepository) FindByID(ctx context.Context, id string) (domain.User, error) {
	return r.scanUser(ctx, `
		SELECT id, email, email_verified, password_hash, auth_source, display_name, avatar_url,
		       role, status, last_login_at, created_at, updated_at
		FROM users
		WHERE id = $1 AND is_deleted = FALSE
	`, id)
}

func (r *UserRepository) ListAll(ctx context.Context) ([]domain.User, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, email, email_verified, password_hash, auth_source, display_name, avatar_url,
		       role, status, last_login_at, created_at, updated_at
		FROM users
		WHERE is_deleted = FALSE
		ORDER BY created_at DESC, email ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.User, 0)
	for rows.Next() {
		var user domain.User
		if err := rows.Scan(
			&user.ID,
			&user.Email,
			&user.EmailVerified,
			&user.PasswordHash,
			&user.AuthSource,
			&user.DisplayName,
			&user.AvatarURL,
			&user.Role,
			&user.Status,
			&user.LastLoginAt,
			&user.CreatedAt,
			&user.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, user)
	}

	return items, rows.Err()
}

func (r *UserRepository) scanUser(ctx context.Context, query string, arg string) (domain.User, error) {
	row := r.db.QueryRow(ctx, `
	`+query, arg)

	var user domain.User
	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.EmailVerified,
		&user.PasswordHash,
		&user.AuthSource,
		&user.DisplayName,
		&user.AvatarURL,
		&user.Role,
		&user.Status,
		&user.LastLoginAt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, ErrUserNotFound
		}
		return domain.User{}, err
	}

	return user, nil
}

func (r *UserRepository) Insert(ctx context.Context, user domain.User) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO users (
			id, email, email_verified, password_hash, auth_source, display_name, avatar_url,
			role, status, last_login_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12
		)
	`,
		user.ID,
		user.Email,
		user.EmailVerified,
		user.PasswordHash,
		user.AuthSource,
		user.DisplayName,
		user.AvatarURL,
		user.Role,
		user.Status,
		user.LastLoginAt,
		user.CreatedAt,
		user.UpdatedAt,
	)

	return err
}

func (r *UserRepository) TouchLastLogin(ctx context.Context, userID string, at time.Time) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users
		SET last_login_at = $2, updated_at = $2
		WHERE id = $1 AND is_deleted = FALSE
	`, userID, at)

	return err
}

func (r *UserRepository) UpdatePasswordByEmail(ctx context.Context, email string, passwordHash string, at time.Time) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users
		SET password_hash = $2,
		    email_verified = TRUE,
		    updated_at = $3
		WHERE email = $1 AND is_deleted = FALSE
	`, email, passwordHash, at)

	return err
}

func (r *UserRepository) UpdateStatusByID(ctx context.Context, userID string, status string, at time.Time) error {
	commandTag, err := r.db.Exec(ctx, `
		UPDATE users
		SET status = $2,
		    updated_at = $3
		WHERE id = $1 AND is_deleted = FALSE
	`, userID, status, at)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}
