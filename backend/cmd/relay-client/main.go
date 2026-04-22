package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hookforward/backend/pkg/realtimeclient"
)

func main() {
	var (
		wsURL        = flag.String("ws", "", "websocket endpoint, e.g. ws://127.0.0.1:8080/ws/connect")
		clientID     = flag.String("client-id", "", "client id")
		clientSecret = flag.String("client-secret", "", "client secret")
	)
	flag.Parse()

	logger := log.New(os.Stdout, "[relay-client] ", log.LstdFlags)

	client, err := realtimeclient.New(realtimeclient.Options{
		WSEndpoint:   *wsURL,
		ClientID:     *clientID,
		ClientSecret: *clientSecret,
		Logger:       logger,
		OnMessage: func(_ context.Context, message realtimeclient.Message) error {
			prettyPayload, _ := json.MarshalIndent(struct {
				ID      string          `json:"id"`
				Event   string          `json:"event"`
				Method  string          `json:"method"`
				Path    string          `json:"path"`
				Source  string          `json:"source"`
				Headers json.RawMessage `json:"headers"`
				Payload json.RawMessage `json:"payload"`
			}{
				ID:      message.MessageID,
				Event:   message.Event,
				Method:  message.Method,
				Path:    message.Path,
				Source:  message.Source,
				Headers: message.Headers,
				Payload: message.Payload,
			}, "", "  ")
			logger.Printf("message payload:\n%s", string(prettyPayload))
			return nil
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Keep the sample client reconnecting until the user stops it.
	for {
		err = client.Run(ctx)
		if err == nil || err == context.Canceled {
			return
		}

		logger.Printf("run loop exited: %v", err)

		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
		}
	}
}
