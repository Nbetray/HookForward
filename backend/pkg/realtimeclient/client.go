package realtimeclient

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type Message struct {
	Type       string          `json:"type"`
	MessageID  string          `json:"message_id"`
	Event      string          `json:"event"`
	Method     string          `json:"method"`
	Path       string          `json:"path"`
	Query      string          `json:"query"`
	Source     string          `json:"source"`
	Headers    json.RawMessage `json:"headers"`
	Payload    json.RawMessage `json:"payload"`
	ReceivedAt time.Time       `json:"received_at"`
}

type Options struct {
	WSEndpoint       string
	ClientID         string
	ClientSecret     string
	ReconnectDelay   time.Duration
	HandshakeTimeout time.Duration
	Logger           *log.Logger
	OnMessage        func(context.Context, Message) error
}

type Client struct {
	opts   Options
	dialer websocket.Dialer
}

func New(opts Options) (*Client, error) {
	if strings.TrimSpace(opts.WSEndpoint) == "" {
		return nil, errors.New("ws endpoint is required")
	}
	if strings.TrimSpace(opts.ClientID) == "" {
		return nil, errors.New("client id is required")
	}
	if strings.TrimSpace(opts.ClientSecret) == "" {
		return nil, errors.New("client secret is required")
	}
	if opts.ReconnectDelay <= 0 {
		opts.ReconnectDelay = 3 * time.Second
	}
	if opts.HandshakeTimeout <= 0 {
		opts.HandshakeTimeout = 10 * time.Second
	}

	return &Client{
		opts: opts,
		dialer: websocket.Dialer{
			HandshakeTimeout: opts.HandshakeTimeout,
		},
	}, nil
}

func (c *Client) Run(ctx context.Context) error {
	for {
		err := c.runOnce(ctx)
		if err == nil || errors.Is(err, context.Canceled) {
			return err
		}

		c.logf("connection closed: %v", err)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(c.opts.ReconnectDelay):
		}
	}
}

func (c *Client) runOnce(ctx context.Context) error {
	conn, _, err := c.dialer.DialContext(ctx, c.opts.WSEndpoint, http.Header{})
	if err != nil {
		return err
	}
	defer conn.Close()

	c.logf("connected to %s", c.opts.WSEndpoint)

	if err := conn.WriteJSON(map[string]string{
		"type":          "auth",
		"client_id":     c.opts.ClientID,
		"client_secret": c.opts.ClientSecret,
	}); err != nil {
		return err
	}

	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	var authResp map[string]any
	if err := conn.ReadJSON(&authResp); err != nil {
		return err
	}

	if authResp["type"] != "auth_ok" {
		return errors.New("server rejected websocket auth")
	}

	conn.SetReadDeadline(time.Time{})
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})

	done := make(chan struct{})
	defer close(done)

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(5*time.Second))
			}
		}
	}()

	for {
		var message Message
		if err := conn.ReadJSON(&message); err != nil {
			return err
		}

		if message.Type != "webhook_message" {
			continue
		}

		c.logf("received message %s event=%s method=%s path=%s", message.MessageID, message.Event, message.Method, message.Path)

		ack := map[string]any{
			"type":       "ack",
			"message_id": message.MessageID,
			"success":    true,
		}

		if c.opts.OnMessage != nil {
			if err := c.opts.OnMessage(ctx, message); err != nil {
				ack["success"] = false
				ack["error"] = err.Error()
			}
		}

		if err := conn.WriteJSON(ack); err != nil {
			return err
		}
	}
}

func (c *Client) logf(format string, args ...any) {
	if c.opts.Logger != nil {
		c.opts.Logger.Printf(format, args...)
	}
}
