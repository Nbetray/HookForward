package repository

import (
	"context"
	"errors"
	"time"

	"hookforward/backend/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrClientNotFound = errors.New("client not found")

const clientColumns = `id, user_id, name, client_id, client_secret, webhook_token, webhook_secret,
	verify_signature, signature_header, signature_algorithm, event_type_header,
	webhook_url, status, last_connected_at, created_at, updated_at`

type ClientRepository struct {
	db *pgxpool.Pool
}

func NewClientRepository(db *pgxpool.Pool) *ClientRepository {
	return &ClientRepository{db: db}
}

func (r *ClientRepository) ListByUserID(ctx context.Context, userID string) ([]domain.Client, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+clientColumns+`
		FROM clients
		WHERE user_id = $1 AND is_deleted = FALSE
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	clients := make([]domain.Client, 0)
	for rows.Next() {
		client, scanErr := scanClient(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		clients = append(clients, client)
	}

	return clients, rows.Err()
}

func (r *ClientRepository) FindByIDAndUserID(ctx context.Context, id string, userID string) (domain.Client, error) {
	return r.findOne(ctx, `
		SELECT `+clientColumns+`
		FROM clients
		WHERE id = $1 AND user_id = $2 AND is_deleted = FALSE
	`, id, userID)
}

func (r *ClientRepository) FindByID(ctx context.Context, id string) (domain.Client, error) {
	return r.findOne(ctx, `
		SELECT `+clientColumns+`
		FROM clients
		WHERE id = $1 AND is_deleted = FALSE
	`, id)
}

func (r *ClientRepository) findOne(ctx context.Context, query string, args ...any) (domain.Client, error) {
	row := r.db.QueryRow(ctx, `
	`+query, args...)

	client, err := scanClient(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Client{}, ErrClientNotFound
		}
		return domain.Client{}, err
	}

	return client, nil
}

func (r *ClientRepository) Insert(ctx context.Context, client domain.Client) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO clients (
			id, user_id, name, client_id, client_secret, webhook_token, webhook_secret,
			verify_signature, signature_header, signature_algorithm, event_type_header,
			webhook_url, status, last_connected_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13, $14, $15, $16
		)
	`,
		client.ID,
		client.UserID,
		client.Name,
		client.ClientID,
		client.ClientSecret,
		client.WebhookToken,
		client.WebhookSecret,
		client.VerifySignature,
		client.SignatureHeader,
		client.SignatureAlgorithm,
		client.EventTypeHeader,
		client.WebhookURL,
		client.Status,
		client.LastConnected,
		client.CreatedAt,
		client.UpdatedAt,
	)

	return err
}

func (r *ClientRepository) FindByWebhookToken(ctx context.Context, token string) (domain.Client, error) {
	row := r.db.QueryRow(ctx, `
		SELECT `+clientColumns+`
		FROM clients
		WHERE webhook_token = $1 AND is_deleted = FALSE
	`, token)

	client, err := scanClient(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Client{}, ErrClientNotFound
		}
		return domain.Client{}, err
	}

	return client, nil
}

func (r *ClientRepository) FindByClientCredentials(ctx context.Context, clientID string, clientSecret string) (domain.Client, error) {
	row := r.db.QueryRow(ctx, `
		SELECT `+clientColumns+`
		FROM clients
		WHERE client_id = $1 AND client_secret = $2 AND is_deleted = FALSE
	`, clientID, clientSecret)

	client, err := scanClient(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Client{}, ErrClientNotFound
		}
		return domain.Client{}, err
	}

	return client, nil
}

func scanClient(row interface {
	Scan(dest ...any) error
}) (domain.Client, error) {
	var client domain.Client
	err := row.Scan(
		&client.ID,
		&client.UserID,
		&client.Name,
		&client.ClientID,
		&client.ClientSecret,
		&client.WebhookToken,
		&client.WebhookSecret,
		&client.VerifySignature,
		&client.SignatureHeader,
		&client.SignatureAlgorithm,
		&client.EventTypeHeader,
		&client.WebhookURL,
		&client.Status,
		&client.LastConnected,
		&client.CreatedAt,
		&client.UpdatedAt,
	)
	if err != nil {
		return domain.Client{}, err
	}

	return client, nil
}

func (r *ClientRepository) TouchLastConnected(ctx context.Context, id string, at time.Time) error {
	_, err := r.db.Exec(ctx, `
		UPDATE clients
		SET last_connected_at = $2, updated_at = $2
		WHERE id = $1 AND is_deleted = FALSE
	`, id, at)
	return err
}

func (r *ClientRepository) SoftDelete(ctx context.Context, id string, userID string, at time.Time) error {
	_, err := r.db.Exec(ctx, `
		UPDATE clients
		SET is_deleted = TRUE, deleted_at = $3, updated_at = $3
		WHERE id = $1 AND user_id = $2 AND is_deleted = FALSE
	`, id, userID, at)
	return err
}

func (r *ClientRepository) UpdateSecuritySettings(ctx context.Context, id string, userID string, verifySignature bool, at time.Time) error {
	commandTag, err := r.db.Exec(ctx, `
		UPDATE clients
		SET verify_signature = $3,
		    updated_at = $4
		WHERE id = $1 AND user_id = $2 AND is_deleted = FALSE
	`, id, userID, verifySignature, at)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return ErrClientNotFound
	}
	return nil
}

func (r *ClientRepository) UpdateCustomHeaders(ctx context.Context, id string, userID string, signatureHeader string, signatureAlgorithm string, eventTypeHeader string, at time.Time) error {
	commandTag, err := r.db.Exec(ctx, `
		UPDATE clients
		SET signature_header = $3,
		    signature_algorithm = $4,
		    event_type_header = $5,
		    updated_at = $6
		WHERE id = $1 AND user_id = $2 AND is_deleted = FALSE
	`, id, userID, signatureHeader, signatureAlgorithm, eventTypeHeader, at)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return ErrClientNotFound
	}
	return nil
}
