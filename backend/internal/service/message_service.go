package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"hash"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"hookforward/backend/internal/domain"
	"hookforward/backend/internal/repository"
	"hookforward/backend/internal/ws"
)

type MessageService struct {
	messages *repository.MessageRepository
	clients  *repository.ClientRepository
	hub      *ws.Hub
}

type MessageView struct {
	ID               string     `json:"id"`
	ClientID         string     `json:"clientId"`
	Source           string     `json:"source"`
	SourceLabel      string     `json:"sourceLabel"`
	EventType        string     `json:"eventType"`
	HTTPMethod       string     `json:"httpMethod"`
	RequestPath      string     `json:"requestPath"`
	QueryString      string     `json:"queryString"`
	DeliveryStatus   string     `json:"deliveryStatus"`
	SignatureValid   bool       `json:"signatureValid"`
	HeadersJSON      string     `json:"headersJson"`
	PayloadJSON      string     `json:"payloadJson"`
	DeliveryAttempts int        `json:"deliveryAttempts"`
	LastError        string     `json:"lastError"`
	ReceivedAt       time.Time  `json:"receivedAt"`
	DeliveredAt      *time.Time `json:"deliveredAt"`
}

func NewMessageService(messages *repository.MessageRepository, clients *repository.ClientRepository, hub *ws.Hub) *MessageService {
	return &MessageService{
		messages: messages,
		clients:  clients,
		hub:      hub,
	}
}

func (s *MessageService) IngestWebhook(ctx context.Context, webhookToken string, req *http.Request, body []byte) (MessageView, error) {
	client, err := s.clients.FindByWebhookToken(ctx, webhookToken)
	if err != nil {
		return MessageView{}, err
	}

	signatureValid := verifyWebhookSignature(client, req, body)

	now := time.Now().UTC()
	message := domain.Message{
		ID:               "msg_" + newMessageID(8),
		UserID:           client.UserID,
		ClientID:         client.ID,
		Source:           detectSource(req),
		SourceLabel:      detectSourceLabel(req),
		EventType:        detectEventType(client, req),
		HTTPMethod:       req.Method,
		RequestPath:      req.URL.Path,
		QueryString:      req.URL.RawQuery,
		DeliveryStatus:   "received",
		SignatureValid:   signatureValid,
		HeadersJSON:      buildHeadersJSON(req.Header),
		PayloadJSON:      buildPayloadJSON(body),
		DeliveryAttempts: 0,
		LastError:        "",
		ReceivedAt:       now,
		DeliveredAt:      nil,
		CreatedAt:        now,
	}

	if err := s.messages.Insert(ctx, message); err != nil {
		return MessageView{}, err
	}

	if client.VerifySignature && !signatureValid {
		failedAt := time.Now().UTC()
		if err := s.messages.UpdateDelivery(ctx, message.ID, "validation_failed", "signature validation failed", &failedAt, 0); err != nil {
			log.Printf("[message] failed to update delivery status for %s: %v", message.ID, err)
		}
		message.DeliveryStatus = "validation_failed"
		message.LastError = "signature validation failed"
		message.DeliveredAt = &failedAt
		return messageViewFromDomain(message), nil
	}

	return s.deliverMessage(ctx, client, message, 1), nil
}

func (s *MessageService) Redeliver(ctx context.Context, userID string, messageID string) (MessageView, error) {
	message, err := s.messages.FindByIDAndUserID(ctx, messageID, userID)
	if err != nil {
		return MessageView{}, err
	}

	client, err := s.clients.FindByID(ctx, message.ClientID)
	if err != nil {
		return MessageView{}, err
	}

	nextAttempt := message.DeliveryAttempts + 1
	return s.deliverMessage(ctx, client, message, nextAttempt), nil
}

func (s *MessageService) GetByID(ctx context.Context, userID string, messageID string) (MessageView, error) {
	item, err := s.messages.FindByIDAndUserID(ctx, messageID, userID)
	if err != nil {
		return MessageView{}, err
	}

	return messageViewFromDomain(item), nil
}

func (s *MessageService) RecoverPendingByClientID(ctx context.Context, clientID string) error {
	items, err := s.messages.ListPendingByClientID(ctx, clientID, 100)
	if err != nil {
		return err
	}

	for _, message := range items {
		client, getErr := s.clients.FindByID(ctx, message.ClientID)
		if getErr != nil {
			return getErr
		}

		nextAttempt := message.DeliveryAttempts
		if nextAttempt <= 0 {
			nextAttempt = 1
		}
		if message.DeliveryStatus == "delivery_failed" || message.DeliveryStatus == "received" {
			nextAttempt = message.DeliveryAttempts + 1
		}

		s.deliverMessage(ctx, client, message, nextAttempt)
	}

	return nil
}

func (s *MessageService) deliverMessage(ctx context.Context, client domain.Client, message domain.Message, attempt int) MessageView {
	if err := s.messages.UpdateDelivery(ctx, message.ID, "delivering", "", nil, attempt); err != nil {
		log.Printf("[message] failed to mark %s as delivering: %v", message.ID, err)
	}
	message.DeliveryStatus = "delivering"
	message.DeliveryAttempts = attempt

	if s.hub == nil {
		now := time.Now().UTC()
		if err := s.messages.UpdateDelivery(ctx, message.ID, "delivery_failed", "realtime delivery unavailable", &now, attempt); err != nil {
			log.Printf("[message] failed to mark %s as delivery_failed: %v", message.ID, err)
		}
		message.DeliveryStatus = "delivery_failed"
		message.LastError = "realtime delivery unavailable"
		message.DeliveredAt = &now
		return messageViewFromDomain(message)
	}

	if err := s.hub.Deliver(ctx, client, message); err != nil {
		now := time.Now().UTC()
		lastError := err.Error()
		if errors.Is(err, ws.ErrClientOffline) {
			lastError = "client offline"
		} else if errors.Is(err, ws.ErrAckTimeout) {
			lastError = "ack timeout"
		}
		log.Printf("[message] delivery failed for %s (attempt %d): %s", message.ID, attempt, lastError)
		if dbErr := s.messages.UpdateDelivery(ctx, message.ID, "delivery_failed", lastError, &now, attempt); dbErr != nil {
			log.Printf("[message] failed to update delivery status for %s: %v", message.ID, dbErr)
		}
		message.DeliveryStatus = "delivery_failed"
		message.LastError = lastError
		message.DeliveredAt = &now
		return messageViewFromDomain(message)
	}

	deliveredAt := time.Now().UTC()
	if err := s.messages.UpdateDelivery(ctx, message.ID, "delivered", "", &deliveredAt, attempt); err != nil {
		log.Printf("[message] failed to mark %s as delivered: %v", message.ID, err)
	}
	message.DeliveryStatus = "delivered"
	message.LastError = ""
	message.DeliveredAt = &deliveredAt

	return messageViewFromDomain(message)
}

func (s *MessageService) ListByUserID(ctx context.Context, userID string) ([]MessageView, error) {
	items, err := s.messages.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := make([]MessageView, 0, len(items))
	for _, item := range items {
		result = append(result, messageViewFromDomain(item))
	}
	return result, nil
}

func (s *MessageService) ListByUserIDAndClientID(ctx context.Context, userID string, clientID string) ([]MessageView, error) {
	items, err := s.messages.ListByUserIDAndClientID(ctx, userID, clientID)
	if err != nil {
		return nil, err
	}

	result := make([]MessageView, 0, len(items))
	for _, item := range items {
		result = append(result, messageViewFromDomain(item))
	}
	return result, nil
}

func messageViewFromDomain(item domain.Message) MessageView {
	return MessageView{
		ID:               item.ID,
		ClientID:         item.ClientID,
		Source:           item.Source,
		SourceLabel:      item.SourceLabel,
		EventType:        item.EventType,
		HTTPMethod:       item.HTTPMethod,
		RequestPath:      item.RequestPath,
		QueryString:      item.QueryString,
		DeliveryStatus:   item.DeliveryStatus,
		SignatureValid:   item.SignatureValid,
		HeadersJSON:      string(item.HeadersJSON),
		PayloadJSON:      string(item.PayloadJSON),
		DeliveryAttempts: item.DeliveryAttempts,
		LastError:        item.LastError,
		ReceivedAt:       item.ReceivedAt,
		DeliveredAt:      item.DeliveredAt,
	}
}

func buildHeadersJSON(headers http.Header) []byte {
	payload, err := json.Marshal(headers)
	if err != nil {
		return []byte("{}")
	}
	return payload
}

func buildPayloadJSON(body []byte) []byte {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return []byte("{}")
	}

	var raw json.RawMessage
	if err := json.Unmarshal(body, &raw); err == nil {
		return body
	}

	payload, err := json.Marshal(map[string]string{"raw": trimmed})
	if err != nil {
		return []byte("{}")
	}
	return payload
}

func detectSource(req *http.Request) string {
	if req.Header.Get("X-GitHub-Event") != "" || req.Header.Get("X-Hub-Signature-256") != "" {
		return "github"
	}
	return "custom"
}

func detectSourceLabel(req *http.Request) string {
	if source := req.Header.Get("X-Webhook-Source"); strings.TrimSpace(source) != "" {
		return strings.TrimSpace(source)
	}
	return detectSource(req)
}

func detectEventType(client domain.Client, req *http.Request) string {
	if h := strings.TrimSpace(client.EventTypeHeader); h != "" {
		if value := strings.TrimSpace(req.Header.Get(h)); value != "" {
			return value
		}
	}
	for _, header := range []string{"X-GitHub-Event", "X-Gitlab-Event", "X-Event-Type"} {
		if value := strings.TrimSpace(req.Header.Get(header)); value != "" {
			return value
		}
	}
	return "webhook"
}

func verifyWebhookSignature(client domain.Client, req *http.Request, body []byte) bool {
	if !client.VerifySignature {
		return true
	}

	secret := strings.TrimSpace(client.WebhookSecret)
	if secret == "" {
		return false
	}

	headers := []string{"X-Hub-Signature-256", "X-Webhook-Signature-256"}
	if h := strings.TrimSpace(client.SignatureHeader); h != "" {
		headers = append([]string{h}, headers...)
	}

	algo := strings.TrimSpace(client.SignatureAlgorithm)
	if algo == "" {
		algo = "hmac-sha256"
	}

	for _, header := range headers {
		value := strings.TrimSpace(req.Header.Get(header))
		if value == "" {
			continue
		}
		return verifySignature(algo, secret, body, value)
	}

	return false
}

func verifySignature(algorithm string, secret string, body []byte, provided string) bool {
	switch algorithm {
	case "hmac-sha1":
		return verifyHMAC(sha1.New, "sha1=", secret, body, provided)
	case "plain":
		return subtle.ConstantTimeCompare([]byte(secret), []byte(strings.TrimSpace(provided))) == 1
	default:
		return verifyHMAC(sha256.New, "sha256=", secret, body, provided)
	}
}

func verifyHMAC(hashFunc func() hash.Hash, prefix string, secret string, body []byte, provided string) bool {
	mac := hmac.New(hashFunc, []byte(secret))
	mac.Write(body)
	expectedBytes := mac.Sum(nil)

	candidate := strings.TrimSpace(provided)
	candidate = strings.TrimPrefix(candidate, prefix)
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return false
	}

	candidateBytes, err := hex.DecodeString(candidate)
	if err != nil {
		return false
	}

	return subtle.ConstantTimeCompare(expectedBytes, candidateBytes) == 1
}

func newMessageID(size int) string {
	buf := make([]byte, size)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
