package ws

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"hookforward/backend/internal/domain"

	"github.com/gorilla/websocket"
)

var (
	ErrClientOffline = errors.New("client offline")
	ErrAckTimeout    = errors.New("ack timeout")
)

const (
	readTimeout  = 90 * time.Second
	pingInterval = 30 * time.Second
	writeTimeout = 10 * time.Second
)

type AckResult struct {
	Success bool
	Error   string
}

type outboundMessage struct {
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

type inboundMessage struct {
	Type         string `json:"type"`
	MessageID    string `json:"message_id"`
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error"`
}

type clientConnection struct {
	client  domain.Client
	conn    *websocket.Conn
	writeMu sync.Mutex
	mu      sync.Mutex
	pending map[string]chan AckResult
	closed  chan struct{}
}

type Hub struct {
	upgrader websocket.Upgrader
	mu       sync.RWMutex
	clients  map[string]*clientConnection
}

func NewHub() *Hub {
	return &Hub{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(_ *http.Request) bool { return true },
		},
		clients: make(map[string]*clientConnection),
	}
}

func (h *Hub) Upgrade(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	return h.upgrader.Upgrade(w, r, nil)
}

func (h *Hub) Serve(client domain.Client, conn *websocket.Conn, onReady func(context.Context)) {
	cc := &clientConnection{
		client:  client,
		conn:    conn,
		pending: make(map[string]chan AckResult),
		closed:  make(chan struct{}),
	}

	h.register(cc)
	defer h.unregister(client.ID, cc)

	_ = cc.writeJSON(map[string]any{
		"type":      "auth_ok",
		"client_id": client.ClientID,
	})

	if onReady != nil {
		go onReady(context.Background())
	}

	conn.SetReadLimit(1 << 20)
	conn.SetReadDeadline(time.Now().Add(readTimeout))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(readTimeout))
		return nil
	})
	conn.SetPingHandler(func(appData string) error {
		conn.SetReadDeadline(time.Now().Add(readTimeout))
		cc.writeMu.Lock()
		defer cc.writeMu.Unlock()
		conn.SetWriteDeadline(time.Now().Add(writeTimeout))
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(writeTimeout))
	})

	done := make(chan struct{})
	defer close(done)

	go func() {
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-cc.closed:
				return
			case <-ticker.C:
				cc.writeMu.Lock()
				conn.SetWriteDeadline(time.Now().Add(writeTimeout))
				err := conn.WriteControl(websocket.PingMessage, []byte("server-ping"), time.Now().Add(writeTimeout))
				cc.writeMu.Unlock()
				if err != nil {
					return
				}
			}
		}
	}()

	for {
		var msg inboundMessage
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}

		if msg.Type == "ack" && msg.MessageID != "" {
			cc.resolveAck(msg.MessageID, AckResult{Success: msg.Success, Error: msg.ErrorMessage})
		}
	}
}

func (h *Hub) Deliver(ctx context.Context, client domain.Client, message domain.Message) error {
	h.mu.RLock()
	cc := h.clients[client.ID]
	h.mu.RUnlock()
	if cc == nil {
		return ErrClientOffline
	}

	ackCh := make(chan AckResult, 1)
	cc.addPending(message.ID, ackCh)
	defer cc.removePending(message.ID)

	err := cc.writeJSON(outboundMessage{
		Type:       "webhook_message",
		MessageID:  message.ID,
		Event:      message.EventType,
		Method:     message.HTTPMethod,
		Path:       message.RequestPath,
		Query:      message.QueryString,
		Source:     message.Source,
		Headers:    json.RawMessage(message.HeadersJSON),
		Payload:    json.RawMessage(message.PayloadJSON),
		ReceivedAt: message.ReceivedAt,
	})
	if err != nil {
		return err
	}

	select {
	case ack := <-ackCh:
		if ack.Success {
			return nil
		}
		if ack.Error == "" {
			return errors.New("ack failed")
		}
		return errors.New(ack.Error)
	case <-time.After(12 * time.Second):
		return ErrAckTimeout
	case <-ctx.Done():
		return ctx.Err()
	case <-cc.closed:
		return ErrClientOffline
	}
}

func (h *Hub) register(cc *clientConnection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if existing := h.clients[cc.client.ID]; existing != nil {
		existing.close()
	}
	h.clients[cc.client.ID] = cc
}

func (h *Hub) unregister(clientInternalID string, cc *clientConnection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if existing := h.clients[clientInternalID]; existing == cc {
		delete(h.clients, clientInternalID)
	}
	cc.close()
}

func (c *clientConnection) writeJSON(payload any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	return c.conn.WriteJSON(payload)
}

func (c *clientConnection) addPending(messageID string, ackCh chan AckResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pending[messageID] = ackCh
}

func (c *clientConnection) removePending(messageID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.pending, messageID)
}

func (c *clientConnection) resolveAck(messageID string, result AckResult) {
	c.mu.Lock()
	ackCh := c.pending[messageID]
	c.mu.Unlock()

	if ackCh != nil {
		select {
		case ackCh <- result:
		default:
		}
	}
}

func (c *clientConnection) close() {
	select {
	case <-c.closed:
		return
	default:
		close(c.closed)
	}
	_ = c.conn.Close()
}

func (h *Hub) IsOnline(clientInternalID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.clients[clientInternalID]
	return ok
}

func (h *Hub) Shutdown() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, cc := range h.clients {
		cc.close()
		delete(h.clients, id)
	}
}
