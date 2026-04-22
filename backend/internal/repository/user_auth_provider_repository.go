package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"

	"hookforward/backend/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrUserAuthProviderNotFound = errors.New("user auth provider not found")

type UserAuthProviderRepository struct {
	db *pgxpool.Pool
}

func NewUserAuthProviderRepository(db *pgxpool.Pool) *UserAuthProviderRepository {
	return &UserAuthProviderRepository{db: db}
}

func (r *UserAuthProviderRepository) FindByProviderAndProviderUserID(ctx context.Context, provider string, providerUserID string) (domain.UserAuthProvider, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, user_id, provider, provider_user_id, provider_username, provider_email,
		       access_token_encrypted, refresh_token_encrypted, token_expires_at, scope,
		       profile_json::text, created_at, updated_at
		FROM user_auth_providers
		WHERE provider = $1 AND provider_user_id = $2 AND is_deleted = FALSE
	`, provider, providerUserID)

	item, err := scanUserAuthProvider(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.UserAuthProvider{}, ErrUserAuthProviderNotFound
		}
		return domain.UserAuthProvider{}, err
	}

	return item, nil
}

func (r *UserAuthProviderRepository) Upsert(ctx context.Context, item domain.UserAuthProvider) error {
	profileJSON := item.ProfileJSON
	if len(profileJSON) == 0 {
		profileJSON = []byte("{}")
	}
	if !json.Valid(profileJSON) {
		profileJSON = []byte("{}")
	}

	_, err := r.db.Exec(ctx, `
		INSERT INTO user_auth_providers (
			id, user_id, provider, provider_user_id, provider_username, provider_email,
			access_token_encrypted, refresh_token_encrypted, token_expires_at, scope,
			profile_json, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10,
			$11::jsonb, $12, $13
		)
		ON CONFLICT (provider, provider_user_id) DO UPDATE
		SET user_id = EXCLUDED.user_id,
		    provider_username = EXCLUDED.provider_username,
		    provider_email = EXCLUDED.provider_email,
		    access_token_encrypted = EXCLUDED.access_token_encrypted,
		    refresh_token_encrypted = EXCLUDED.refresh_token_encrypted,
		    token_expires_at = EXCLUDED.token_expires_at,
		    scope = EXCLUDED.scope,
		    profile_json = EXCLUDED.profile_json,
		    updated_at = EXCLUDED.updated_at,
		    is_deleted = FALSE,
		    deleted_at = NULL
	`, item.ID, item.UserID, item.Provider, item.ProviderUserID, item.ProviderUsername, item.ProviderEmail, item.AccessTokenEncrypted, item.RefreshTokenEncrypted, item.TokenExpiresAt, item.Scope, string(profileJSON), item.CreatedAt, item.UpdatedAt)

	return err
}

func scanUserAuthProvider(row interface {
	Scan(dest ...any) error
}) (domain.UserAuthProvider, error) {
	var item domain.UserAuthProvider
	var profileJSON string

	err := row.Scan(
		&item.ID,
		&item.UserID,
		&item.Provider,
		&item.ProviderUserID,
		&item.ProviderUsername,
		&item.ProviderEmail,
		&item.AccessTokenEncrypted,
		&item.RefreshTokenEncrypted,
		&item.TokenExpiresAt,
		&item.Scope,
		&profileJSON,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return domain.UserAuthProvider{}, err
	}

	item.ProfileJSON = []byte(profileJSON)
	return item, nil
}

func NewUserAuthProviderID() string {
	return "uap_" + newIDSuffix(8)
}

func newIDSuffix(length int) string {
	if length <= 0 {
		length = 8
	}
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "fallback"
	}
	return hex.EncodeToString(buf)
}
