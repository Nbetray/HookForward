package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"hookforward/backend/internal/auth"
	"hookforward/backend/internal/bootstrap"
	"hookforward/backend/internal/config"
	httpserver "hookforward/backend/internal/http"
	"hookforward/backend/internal/mailer"
	"hookforward/backend/internal/repository"
	"hookforward/backend/internal/service"
	"hookforward/backend/internal/verification"
	"hookforward/backend/internal/ws"
)

func main() {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := repository.OpenPostgres(ctx, cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("open postgres: %v", err)
	}
	defer db.Close()

	if err := repository.RunMigrations(ctx, db, cfg.MigrationsDir); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	verifyStore := verification.NewStore(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err := verifyStore.Ping(ctx); err != nil {
		log.Fatalf("ping redis: %v", err)
	}

	users := repository.NewUserRepository(db)
	userAuthProviders := repository.NewUserAuthProviderRepository(db)
	clients := repository.NewClientRepository(db)
	messages := repository.NewMessageRepository(db)

	if err := bootstrap.EnsureAdmin(ctx, cfg, users); err != nil {
		log.Fatalf("ensure admin: %v", err)
	}

	tokenIssuer := auth.NewTokenIssuer(cfg.JWTSecret)
	realtimeHub := ws.NewHub()
	emailSender := mailer.NewSMTPSender(cfg)

	server := httpserver.NewServer(cfg, httpserver.ServerDependencies{
		Auth:        service.NewAuthService(users, userAuthProviders, cfg.JWTSecret, verifyStore, emailSender, cfg),
		Users:       service.NewUserService(users),
		Clients:     service.NewClientService(clients, cfg.PublicBaseURL, realtimeHub),
		Messages:    service.NewMessageService(messages, clients, realtimeHub),
		Tokens:      tokenIssuer,
		Realtime:    realtimeHub,
		MessageRepo: messages,
	})

	go func() {
		log.Printf("server listening on %s", cfg.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-ctx.Done()

	log.Println("shutting down...")
	realtimeHub.Shutdown()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}
