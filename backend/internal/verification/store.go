package verification

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type Store struct {
	client *redis.Client
}

func NewStore(addr string, password string, db int) *Store {
	return &Store{
		client: redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
			DB:       db,
		}),
	}
}

func (s *Store) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

func (s *Store) SaveCode(ctx context.Context, purpose string, email string, code string, ttl time.Duration) error {
	return s.client.Set(ctx, codeKey(purpose, email), code, ttl).Err()
}

func (s *Store) LoadCode(ctx context.Context, purpose string, email string) (string, error) {
	return s.client.Get(ctx, codeKey(purpose, email)).Result()
}

func (s *Store) DeleteCode(ctx context.Context, purpose string, email string) error {
	return s.client.Del(ctx, codeKey(purpose, email)).Err()
}

func (s *Store) AllowSend(ctx context.Context, purpose string, email string, ttl time.Duration) (bool, error) {
	return s.client.SetNX(ctx, throttleKey(purpose, email), "1", ttl).Result()
}

func codeKey(purpose string, email string) string {
	return fmt.Sprintf("verification:%s:%s", purpose, normalizeEmail(email))
}

func throttleKey(purpose string, email string) string {
	return fmt.Sprintf("verification:%s:throttle:%s", purpose, normalizeEmail(email))
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
