package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"hookforward/backend/internal/domain"
	"hookforward/backend/internal/repository"
	"hookforward/backend/internal/ws"
)

type ClientService struct {
	clients       *repository.ClientRepository
	publicBaseURL string
	hub           *ws.Hub
}

type ClientView struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	ClientID        string     `json:"clientId"`
	ClientSecret    string     `json:"clientSecret,omitempty"`
	WebhookToken    string     `json:"webhookToken"`
	WebhookSecret   string     `json:"webhookSecret,omitempty"`
	VerifySignature    bool       `json:"verifySignature"`
	SignatureHeader    string     `json:"signatureHeader"`
	SignatureAlgorithm string     `json:"signatureAlgorithm"`
	EventTypeHeader    string     `json:"eventTypeHeader"`
	WebhookURL      string     `json:"webhookUrl"`
	WSEndpoint      string     `json:"wsEndpoint"`
	Status          string     `json:"status"`
	Online          bool       `json:"online"`
	LastConnected   *time.Time `json:"lastConnectedAt"`
	CreatedAt       time.Time  `json:"createdAt"`
}

func NewClientService(clients *repository.ClientRepository, publicBaseURL string, hub *ws.Hub) *ClientService {
	return &ClientService{
		clients:       clients,
		publicBaseURL: strings.TrimSuffix(publicBaseURL, "/"),
		hub:           hub,
	}
}

func (s *ClientService) ListByUserID(ctx context.Context, userID string) ([]ClientView, error) {
	items, err := s.clients.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := make([]ClientView, 0, len(items))
	for _, item := range items {
		result = append(result, s.clientView(item, false))
	}
	return result, nil
}

func (s *ClientService) GetByID(ctx context.Context, userID string, id string) (ClientView, error) {
	client, err := s.clients.FindByIDAndUserID(ctx, id, userID)
	if err != nil {
		return ClientView{}, err
	}
	return s.clientView(client, true), nil
}

func (s *ClientService) AuthenticateRealtimeClient(ctx context.Context, clientID string, clientSecret string) (domain.Client, error) {
	client, err := s.clients.FindByClientCredentials(ctx, strings.TrimSpace(clientID), strings.TrimSpace(clientSecret))
	if err != nil {
		return domain.Client{}, err
	}
	if client.Status != "active" {
		return domain.Client{}, errors.New("client disabled")
	}
	return client, nil
}

func (s *ClientService) MarkConnected(ctx context.Context, id string, at time.Time) error {
	return s.clients.TouchLastConnected(ctx, id, at)
}

func (s *ClientService) Delete(ctx context.Context, userID string, id string, messages *repository.MessageRepository) error {
	now := time.Now().UTC()
	if err := s.clients.SoftDelete(ctx, id, userID, now); err != nil {
		return err
	}
	if messages != nil {
		if err := messages.SoftDeleteByUserAndClientID(ctx, userID, id, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *ClientService) UpdateSecuritySettings(ctx context.Context, userID string, id string, verifySignature bool) (ClientView, error) {
	if err := s.clients.UpdateSecuritySettings(ctx, id, userID, verifySignature, time.Now().UTC()); err != nil {
		return ClientView{}, err
	}
	client, err := s.clients.FindByIDAndUserID(ctx, id, userID)
	if err != nil {
		return ClientView{}, err
	}
	return s.clientView(client, true), nil
}

func (s *ClientService) UpdateCustomHeaders(ctx context.Context, userID string, id string, signatureHeader string, signatureAlgorithm string, eventTypeHeader string) (ClientView, error) {
	if err := s.clients.UpdateCustomHeaders(ctx, id, userID, strings.TrimSpace(signatureHeader), strings.TrimSpace(signatureAlgorithm), strings.TrimSpace(eventTypeHeader), time.Now().UTC()); err != nil {
		return ClientView{}, err
	}
	client, err := s.clients.FindByIDAndUserID(ctx, id, userID)
	if err != nil {
		return ClientView{}, err
	}
	return s.clientView(client, true), nil
}

func (s *ClientService) Create(ctx context.Context, userID string, name string) (ClientView, error) {
	now := time.Now().UTC()
	clientToken := newSecret(8)
	clientSecret := newSecret(16)
	webhookToken := newSecret(12)
	webhookSecret := newSecret(16)

	client := domain.Client{
		ID:              "cli_" + newSecret(8),
		UserID:          userID,
		Name:            strings.TrimSpace(name),
		ClientID:        "client_" + clientToken,
		ClientSecret:    clientSecret,
		WebhookToken:    webhookToken,
		WebhookSecret:   webhookSecret,
		VerifySignature: false,
		WebhookURL:      s.publicBaseURL + "/webhook/incoming/" + webhookToken,
		Status:          "active",
		LastConnected:   nil,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := s.clients.Insert(ctx, client); err != nil {
		return ClientView{}, err
	}

	return s.clientView(client, true), nil
}

func (s *ClientService) clientView(client domain.Client, includeSecrets bool) ClientView {
	online := s.hub != nil && s.hub.IsOnline(client.ID)
	return clientViewFromDomain(client, includeSecrets, online)
}

func clientViewFromDomain(client domain.Client, includeSecrets bool, online bool) ClientView {
	view := ClientView{
		ID:              client.ID,
		Name:            client.Name,
		ClientID:        client.ClientID,
		WebhookToken:    client.WebhookToken,
		VerifySignature:    client.VerifySignature,
		SignatureHeader:    client.SignatureHeader,
		SignatureAlgorithm: client.SignatureAlgorithm,
		EventTypeHeader:    client.EventTypeHeader,
		WebhookURL:      client.WebhookURL,
		WSEndpoint:      toWSEndpoint(client.WebhookURL, client.WebhookToken),
		Status:          client.Status,
		Online:          online,
		LastConnected:   client.LastConnected,
		CreatedAt:       client.CreatedAt,
	}

	if includeSecrets {
		view.ClientSecret = client.ClientSecret
		view.WebhookSecret = client.WebhookSecret
	}

	return view
}

func toWSEndpoint(webhookURL, webhookToken string) string {
	ep := strings.Replace(webhookURL, "/webhook/incoming/"+webhookToken, "/ws/connect", 1)
	if strings.HasPrefix(ep, "https://") {
		return "wss://" + strings.TrimPrefix(ep, "https://")
	}
	if strings.HasPrefix(ep, "http://") {
		return "ws://" + strings.TrimPrefix(ep, "http://")
	}
	return ep
}

func newSecret(size int) string {
	buf := make([]byte, size)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
