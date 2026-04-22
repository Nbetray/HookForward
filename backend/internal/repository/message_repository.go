package repository

import (
	"context"
	"errors"
	"time"

	"hookforward/backend/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrMessageNotFound = errors.New("message not found")

type MessageRepository struct {
	db *pgxpool.Pool
}

func NewMessageRepository(db *pgxpool.Pool) *MessageRepository {
	return &MessageRepository{db: db}
}

func (r *MessageRepository) Insert(ctx context.Context, message domain.Message) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO messages (
			id, user_id, client_id, source, source_label, event_type, http_method, request_path,
			query_string, delivery_status, signature_valid, headers_json, payload_json,
			delivery_attempts, last_error, received_at, delivered_at, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12::jsonb, $13::jsonb,
			$14, $15, $16, $17, $18
		)
	`,
		message.ID,
		message.UserID,
		message.ClientID,
		message.Source,
		message.SourceLabel,
		message.EventType,
		message.HTTPMethod,
		message.RequestPath,
		message.QueryString,
		message.DeliveryStatus,
		message.SignatureValid,
		string(message.HeadersJSON),
		string(message.PayloadJSON),
		message.DeliveryAttempts,
		message.LastError,
		message.ReceivedAt,
		message.DeliveredAt,
		message.CreatedAt,
	)

	return err
}

func (r *MessageRepository) ListByUserID(ctx context.Context, userID string) ([]domain.Message, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, client_id, source, source_label, event_type, http_method, request_path,
		       query_string, delivery_status, signature_valid, headers_json::text, payload_json::text,
		       delivery_attempts, last_error, received_at, delivered_at, created_at
		FROM messages
		WHERE user_id = $1 AND is_deleted = FALSE
		ORDER BY created_at DESC
		LIMIT 100
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.Message
	for rows.Next() {
		item, scanErr := scanMessage(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

func (r *MessageRepository) ListByUserIDAndClientID(ctx context.Context, userID string, clientID string) ([]domain.Message, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, client_id, source, source_label, event_type, http_method, request_path,
		       query_string, delivery_status, signature_valid, headers_json::text, payload_json::text,
		       delivery_attempts, last_error, received_at, delivered_at, created_at
		FROM messages
		WHERE user_id = $1 AND client_id = $2 AND is_deleted = FALSE
		ORDER BY created_at DESC
		LIMIT 100
	`, userID, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.Message
	for rows.Next() {
		item, scanErr := scanMessage(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

func (r *MessageRepository) FindByIDAndUserID(ctx context.Context, id string, userID string) (domain.Message, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, user_id, client_id, source, source_label, event_type, http_method, request_path,
		       query_string, delivery_status, signature_valid, headers_json::text, payload_json::text,
		       delivery_attempts, last_error, received_at, delivered_at, created_at
		FROM messages
		WHERE id = $1 AND user_id = $2 AND is_deleted = FALSE
	`, id, userID)

	message, err := scanMessage(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Message{}, ErrMessageNotFound
		}
		return domain.Message{}, err
	}

	return message, nil
}

func (r *MessageRepository) FindByID(ctx context.Context, id string) (domain.Message, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, user_id, client_id, source, source_label, event_type, http_method, request_path,
		       query_string, delivery_status, signature_valid, headers_json::text, payload_json::text,
		       delivery_attempts, last_error, received_at, delivered_at, created_at
		FROM messages
		WHERE id = $1 AND is_deleted = FALSE
	`, id)

	message, err := scanMessage(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Message{}, ErrMessageNotFound
		}
		return domain.Message{}, err
	}

	return message, nil
}

func (r *MessageRepository) ListPendingByClientID(ctx context.Context, clientID string, limit int) ([]domain.Message, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, client_id, source, source_label, event_type, http_method, request_path,
		       query_string, delivery_status, signature_valid, headers_json::text, payload_json::text,
		       delivery_attempts, last_error, received_at, delivered_at, created_at
		FROM messages
		WHERE client_id = $1
		  AND delivery_status IN ('received', 'delivering', 'delivery_failed')
		  AND is_deleted = FALSE
		ORDER BY created_at ASC
		LIMIT $2
	`, clientID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.Message, 0)
	for rows.Next() {
		item, scanErr := scanMessage(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

func (r *MessageRepository) UpdateDelivery(ctx context.Context, messageID string, status string, lastError string, deliveredAt *time.Time, attempts int) error {
	_, err := r.db.Exec(ctx, `
		UPDATE messages
		SET delivery_status = $2,
		    last_error = $3,
		    delivered_at = $4,
		    delivery_attempts = $5
		WHERE id = $1 AND is_deleted = FALSE
	`, messageID, status, lastError, deliveredAt, attempts)

	return err
}

func (r *MessageRepository) SoftDeleteByUserAndClientID(ctx context.Context, userID string, clientID string, at time.Time) error {
	_, err := r.db.Exec(ctx, `
		UPDATE messages
		SET is_deleted = TRUE, deleted_at = $3
		WHERE user_id = $1 AND client_id = $2 AND is_deleted = FALSE
	`, userID, clientID, at)
	return err
}

type DailyMessageCount struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

type StatusCount struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

type DashboardStats struct {
	TotalClients  int                `json:"totalClients"`
	OnlineClients int                `json:"onlineClients"`
	TotalMessages int                `json:"totalMessages"`
	Delivered     int                `json:"delivered"`
	Failed        int                `json:"failed"`
	Pending       int                `json:"pending"`
	Daily         []DailyMessageCount `json:"daily"`
	ByStatus      []StatusCount       `json:"byStatus"`
}

func (r *MessageRepository) DashboardStats(ctx context.Context, userID string, days int) (DashboardStats, error) {
	var stats DashboardStats

	err := r.db.QueryRow(ctx, `
		SELECT
			COALESCE(COUNT(*), 0),
			COALESCE(COUNT(*) FILTER (WHERE delivery_status = 'delivered'), 0),
			COALESCE(COUNT(*) FILTER (WHERE delivery_status = 'delivery_failed'), 0),
			COALESCE(COUNT(*) FILTER (WHERE delivery_status IN ('received','delivering','queued')), 0)
		FROM messages WHERE user_id = $1 AND is_deleted = FALSE
	`, userID).Scan(&stats.TotalMessages, &stats.Delivered, &stats.Failed, &stats.Pending)
	if err != nil {
		return stats, err
	}

	err = r.db.QueryRow(ctx, `
		SELECT
			COALESCE(COUNT(*), 0),
			COALESCE(COUNT(*) FILTER (WHERE last_connected_at > NOW() - INTERVAL '5 minutes'), 0)
		FROM clients WHERE user_id = $1 AND is_deleted = FALSE
	`, userID).Scan(&stats.TotalClients, &stats.OnlineClients)
	if err != nil {
		return stats, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT d::date::text AS date, COALESCE(COUNT(m.id), 0) AS count
		FROM generate_series(
			CURRENT_DATE - ($2::int - 1) * INTERVAL '1 day',
			CURRENT_DATE,
			'1 day'
		) AS d
		LEFT JOIN messages m
			ON m.user_id = $1 AND m.is_deleted = FALSE AND m.created_at::date = d::date
		GROUP BY d
		ORDER BY d
	`, userID, days)
	if err != nil {
		return stats, err
	}
	defer rows.Close()

	for rows.Next() {
		var dc DailyMessageCount
		if err := rows.Scan(&dc.Date, &dc.Count); err != nil {
			return stats, err
		}
		stats.Daily = append(stats.Daily, dc)
	}

	statusRows, err := r.db.Query(ctx, `
		SELECT delivery_status, COUNT(*) AS count
		FROM messages WHERE user_id = $1 AND is_deleted = FALSE
		GROUP BY delivery_status ORDER BY count DESC
	`, userID)
	if err != nil {
		return stats, err
	}
	defer statusRows.Close()

	for statusRows.Next() {
		var sc StatusCount
		if err := statusRows.Scan(&sc.Status, &sc.Count); err != nil {
			return stats, err
		}
		stats.ByStatus = append(stats.ByStatus, sc)
	}

	return stats, nil
}

func scanMessage(row interface {
	Scan(dest ...any) error
}) (domain.Message, error) {
	var message domain.Message
	var headers string
	var payload string

	err := row.Scan(
		&message.ID,
		&message.UserID,
		&message.ClientID,
		&message.Source,
		&message.SourceLabel,
		&message.EventType,
		&message.HTTPMethod,
		&message.RequestPath,
		&message.QueryString,
		&message.DeliveryStatus,
		&message.SignatureValid,
		&headers,
		&payload,
		&message.DeliveryAttempts,
		&message.LastError,
		&message.ReceivedAt,
		&message.DeliveredAt,
		&message.CreatedAt,
	)
	if err != nil {
		return domain.Message{}, err
	}

	message.HeadersJSON = []byte(headers)
	message.PayloadJSON = []byte(payload)
	return message, nil
}
