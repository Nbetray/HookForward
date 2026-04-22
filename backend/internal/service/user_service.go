package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"hookforward/backend/internal/domain"
	"hookforward/backend/internal/repository"
)

var (
	ErrAdminOnly         = errors.New("admin only")
	ErrInvalidStatus     = errors.New("invalid user status")
	ErrCannotDisableSelf = errors.New("cannot disable current admin")
)

type UserService struct {
	users *repository.UserRepository
}

func NewUserService(users *repository.UserRepository) *UserService {
	return &UserService{users: users}
}

func (s *UserService) GetByID(ctx context.Context, userID string) (UserView, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return UserView{}, err
	}
	return userViewFromDomain(user), nil
}

func (s *UserService) ListAll(ctx context.Context) ([]UserView, error) {
	users, err := s.users.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]UserView, 0, len(users))
	for _, user := range users {
		items = append(items, userViewFromDomain(user))
	}
	return items, nil
}

func (s *UserService) UpdateStatus(ctx context.Context, actorUserID string, targetUserID string, status string) (UserView, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	if status != "active" && status != "disabled" {
		return UserView{}, ErrInvalidStatus
	}
	if actorUserID == targetUserID && status == "disabled" {
		return UserView{}, ErrCannotDisableSelf
	}

	if err := s.users.UpdateStatusByID(ctx, targetUserID, status, time.Now().UTC()); err != nil {
		return UserView{}, err
	}

	user, err := s.users.FindByID(ctx, targetUserID)
	if err != nil {
		return UserView{}, err
	}
	return userViewFromDomain(user), nil
}

func userViewFromDomain(user domain.User) UserView {
	return UserView{
		ID:            user.ID,
		Email:         user.Email,
		EmailVerified: user.EmailVerified,
		AuthSource:    user.AuthSource,
		DisplayName:   user.DisplayName,
		AvatarURL:     user.AvatarURL,
		Role:          user.Role,
		Status:        user.Status,
		LastLoginAt:   user.LastLoginAt,
	}
}
