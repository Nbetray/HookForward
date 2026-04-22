package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	AppName            string
	Env                string
	Addr               string
	PublicBaseURL      string
	FrontendBaseURL    string
	AllowedOrigins     string
	MigrationsDir      string
	PostgresDSN        string
	RedisAddr          string
	RedisPassword      string
	RedisDB            int
	JWTSecret          string
	SMTPHost           string
	SMTPPort           int
	SMTPUsername       string
	SMTPPassword       string
	SMTPFromEmail      string
	SMTPFromName       string
	AdminEmail         string
	AdminPassword      string
	GitHubClientID     string
	GitHubClientSecret string
}

func Load() Config {
	loadDotEnv()

	cfg := Config{
		AppName:            getEnv("APP_NAME", "hookforward"),
		Env:                getEnv("APP_ENV", "development"),
		Addr:               getEnv("APP_ADDR", ":8080"),
		PublicBaseURL:      getEnv("APP_PUBLIC_BASE_URL", "http://localhost:8080"),
		FrontendBaseURL:    getEnv("APP_FRONTEND_BASE_URL", "http://localhost:5173"),
		AllowedOrigins:     getEnv("APP_ALLOWED_ORIGINS", "http://localhost:5173"),
		MigrationsDir:      getEnv("APP_MIGRATIONS_DIR", "./migrations"),
		PostgresDSN:        getEnv("POSTGRES_DSN", ""),
		RedisAddr:          getEnv("REDIS_ADDR", ""),
		RedisPassword:      getEnv("REDIS_PASSWORD", ""),
		RedisDB:            getEnvInt("REDIS_DB", 0),
		JWTSecret:          getEnv("JWT_SECRET", "change-me"),
		SMTPHost:           getEnv("SMTP_HOST", ""),
		SMTPPort:           getEnvInt("SMTP_PORT", 587),
		SMTPUsername:       getEnv("SMTP_USERNAME", ""),
		SMTPPassword:       getEnv("SMTP_PASSWORD", ""),
		SMTPFromEmail:      getEnv("SMTP_FROM_EMAIL", ""),
		SMTPFromName:       getEnv("SMTP_FROM_NAME", ""),
		AdminEmail:         getEnv("ADMIN_EMAIL", "admin@example.com"),
		AdminPassword:      getEnv("ADMIN_PASSWORD", "change-me"),
		GitHubClientID:     getEnv("GITHUB_CLIENT_ID", ""),
		GitHubClientSecret: getEnv("GITHUB_CLIENT_SECRET", ""),
	}

	cfg.Validate()
	return cfg
}

func (c Config) Validate() {
	if c.Env != "development" && (c.JWTSecret == "change-me" || len(c.JWTSecret) < 16) {
		panic("JWT_SECRET must be set to a strong value (>=16 chars) in non-development environments")
	}
	if c.Env != "development" && c.AdminPassword == "change-me" {
		panic("ADMIN_PASSWORD must be changed in non-development environments")
	}
}

func loadDotEnv() {
	for _, candidate := range []string{
		".env",
		filepath.Join("..", ".env"),
		filepath.Join("..", "..", ".env"),
	} {
		if loadEnvFile(candidate) == nil {
			return
		}
	}
}

func loadEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" {
			continue
		}

		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, value)
		}
	}

	return scanner.Err()
}

func getEnv(key string, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}

	return fallback
}

func getEnvInt(key string, fallback int) int {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}
