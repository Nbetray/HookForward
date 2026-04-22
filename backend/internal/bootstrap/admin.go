package bootstrap

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"hookforward/backend/internal/auth"
	"hookforward/backend/internal/config"
	"hookforward/backend/internal/domain"
	"hookforward/backend/internal/repository"
)

func EnsureAdmin(ctx context.Context, cfg config.Config, users *repository.UserRepository) error {
	email := strings.ToLower(strings.TrimSpace(cfg.AdminEmail))

	admin, err := users.FindByEmail(ctx, email)
	if err == nil {
		if admin.Role != "admin" {
			return errors.New("configured admin email already exists with non-admin role")
		}
		return nil
	}
	if !errors.Is(err, repository.ErrUserNotFound) {
		return err
	}

	hashedPassword, err := auth.HashPassword(cfg.AdminPassword)
	if err != nil {
		return err
	}

	now := time.Now().UTC()

	return users.Insert(ctx, domain.User{
		ID:            newID("usr"),
		Email:         email,
		EmailVerified: true,
		PasswordHash:  hashedPassword,
		AuthSource:    "email",
		DisplayName:   "Administrator",
		AvatarURL:     "",
		Role:          "admin",
		Status:        "active",
		LastLoginAt:   nil,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
}

func newID(prefix string) string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return prefix + "_" + hex.EncodeToString(buf)
}
