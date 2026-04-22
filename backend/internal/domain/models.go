package domain

import "time"

type User struct {
	ID            string
	Email         string
	EmailVerified bool
	PasswordHash  string
	AuthSource    string
	DisplayName   string
	AvatarURL     string
	Role          string
	Status        string
	LastLoginAt   *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type UserAuthProvider struct {
	ID                    string
	UserID                string
	Provider              string
	ProviderUserID        string
	ProviderUsername      string
	ProviderEmail         string
	AccessTokenEncrypted  string
	RefreshTokenEncrypted string
	TokenExpiresAt        *time.Time
	Scope                 string
	ProfileJSON           []byte
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type Client struct {
	ID              string
	UserID          string
	Name            string
	ClientID        string
	ClientSecret    string
	WebhookToken    string
	WebhookSecret   string
	VerifySignature    bool
	SignatureHeader    string
	SignatureAlgorithm string
	EventTypeHeader    string
	WebhookURL      string
	Status          string
	LastConnected   *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Message struct {
	ID               string
	UserID           string
	ClientID         string
	Source           string
	SourceLabel      string
	EventType        string
	HTTPMethod       string
	RequestPath      string
	QueryString      string
	DeliveryStatus   string
	SignatureValid   bool
	HeadersJSON      []byte
	PayloadJSON      []byte
	DeliveryAttempts int
	LastError        string
	ReceivedAt       time.Time
	DeliveredAt      *time.Time
	CreatedAt        time.Time
}
