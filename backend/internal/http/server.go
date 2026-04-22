package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"hookforward/backend/internal/auth"
	"hookforward/backend/internal/config"
	"hookforward/backend/internal/repository"
	"hookforward/backend/internal/service"
	"hookforward/backend/internal/ws"
)

type ServerDependencies struct {
	Auth        *service.AuthService
	Users       *service.UserService
	Clients     *service.ClientService
	Messages    *service.MessageService
	Tokens      *auth.TokenIssuer
	Realtime    *ws.Hub
	MessageRepo *repository.MessageRepository
}

func NewServer(cfg config.Config, deps ServerDependencies) *http.Server {
	authLimiter := newRateLimiter(20, 1*time.Minute)
	webhookLimiter := newRateLimiter(120, 1*time.Minute)

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "ok",
			"env":    cfg.Env,
		})
	})

	mux.HandleFunc("/api/v1/meta", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"appName":       cfg.AppName,
			"publicBaseUrl": cfg.PublicBaseURL,
			"features": []string{
				"auth-email",
				"oauth-github-ready",
				"webhook-incoming",
				"websocket-reconnect",
			},
		})
	})

	mux.HandleFunc("/api/v1/auth/login", withRateLimit(authLimiter, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		if deps.Auth == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "auth service unavailable"})
			return
		}

		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		result, err := deps.Auth.Login(r.Context(), req.Email, req.Password)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrInvalidCredentials):
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
			case errors.Is(err, service.ErrUserDisabled):
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "user disabled"})
			default:
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "login failed"})
			}
			return
		}

		writeJSON(w, http.StatusOK, result)
	}))

	mux.HandleFunc("/api/v1/auth/github/start", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		if deps.Auth == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "auth service unavailable"})
			return
		}

		authURL, _, err := deps.Auth.GitHubAuthURL(r.Context())
		if err != nil {
			switch {
			case errors.Is(err, service.ErrGitHubOAuthDisabled):
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "github oauth not configured"})
			default:
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to start github oauth"})
			}
			return
		}

		http.Redirect(w, r, authURL, http.StatusFound)
	})

	mux.HandleFunc("/api/v1/auth/github/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		if deps.Auth == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "auth service unavailable"})
			return
		}

		redirectToFrontend := func(path string, params map[string]string) {
			target, err := url.Parse(strings.TrimRight(cfg.FrontendBaseURL, "/") + path)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "invalid frontend redirect"})
				return
			}
			query := target.Query()
			for key, value := range params {
				query.Set(key, value)
			}
			target.RawQuery = query.Encode()
			http.Redirect(w, r, target.String(), http.StatusFound)
		}

		if oauthErr := strings.TrimSpace(r.URL.Query().Get("error")); oauthErr != "" {
			redirectToFrontend("/login", map[string]string{"oauth_error": oauthErr})
			return
		}

		result, err := deps.Auth.CompleteGitHubLogin(
			r.Context(),
			r.URL.Query().Get("state"),
			r.URL.Query().Get("code"),
		)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrGitHubOAuthDisabled):
				redirectToFrontend("/login", map[string]string{"oauth_error": "github_oauth_not_configured"})
			case errors.Is(err, service.ErrInvalidOAuthState):
				redirectToFrontend("/login", map[string]string{"oauth_error": "invalid_oauth_state"})
			case errors.Is(err, service.ErrUserDisabled):
				redirectToFrontend("/login", map[string]string{"oauth_error": "user_disabled"})
			default:
				log.Printf("github oauth error: %v", err)
				redirectToFrontend("/login", map[string]string{"oauth_error": "github_login_failed"})
			}
			return
		}

		redirectToFrontend("/auth/github/callback", map[string]string{
			"access_token": result.Token,
		})
	})

	mux.HandleFunc("/api/v1/auth/register/send-code", withRateLimit(authLimiter, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		if deps.Auth == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "auth service unavailable"})
			return
		}

		var req struct {
			Email string `json:"email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		result, err := deps.Auth.SendRegisterCode(r.Context(), req.Email)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrEmailAlreadyUsed):
				writeJSON(w, http.StatusConflict, map[string]string{"error": "email already used"})
			case errors.Is(err, service.ErrCodeSendTooFast):
				writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "code requested too frequently"})
			default:
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to send verification code"})
			}
			return
		}

		writeJSON(w, http.StatusOK, result)
	}))

	mux.HandleFunc("/api/v1/auth/register", withRateLimit(authLimiter, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		if deps.Auth == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "auth service unavailable"})
			return
		}

		var req struct {
			Email    string `json:"email"`
			Code     string `json:"code"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		result, err := deps.Auth.Register(r.Context(), req.Email, req.Code, req.Password)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrEmailAlreadyUsed):
				writeJSON(w, http.StatusConflict, map[string]string{"error": "email already used"})
			case errors.Is(err, service.ErrInvalidCode):
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid verification code"})
			case errors.Is(err, auth.ErrWeakPassword):
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": auth.ErrWeakPassword.Error()})
			default:
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "registration failed"})
			}
			return
		}

		writeJSON(w, http.StatusCreated, result)
	}))

	mux.HandleFunc("/api/v1/auth/password/send-code", withRateLimit(authLimiter, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		if deps.Auth == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "auth service unavailable"})
			return
		}

		var req struct {
			Email string `json:"email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		result, err := deps.Auth.SendResetCode(r.Context(), req.Email)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrCodeSendTooFast):
				writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "code requested too frequently"})
			default:
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to send reset code"})
			}
			return
		}

		writeJSON(w, http.StatusOK, result)
	}))

	mux.HandleFunc("/api/v1/auth/password/reset", withRateLimit(authLimiter, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		if deps.Auth == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "auth service unavailable"})
			return
		}

		var req struct {
			Email    string `json:"email"`
			Code     string `json:"code"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		if err := deps.Auth.ResetPassword(r.Context(), req.Email, req.Code, req.Password); err != nil {
			switch {
			case errors.Is(err, service.ErrInvalidCode):
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid verification code"})
			case errors.Is(err, auth.ErrWeakPassword):
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": auth.ErrWeakPassword.Error()})
			case errors.Is(err, repository.ErrUserNotFound):
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
			default:
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "password reset failed"})
			}
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"reset": true})
	}))

	mux.HandleFunc("/api/v1/dashboard/stats", requireAuth(deps.Tokens, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		claims, err := mustClaims(r)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing auth context"})
			return
		}
		if deps.MessageRepo == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "service unavailable"})
			return
		}
		stats, err := deps.MessageRepo.DashboardStats(r.Context(), claims.UserID, 7)
		if err != nil {
			log.Printf("dashboard stats error: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load stats"})
			return
		}
		writeJSON(w, http.StatusOK, stats)
	}))

	mux.HandleFunc("/api/v1/me", requireAuth(deps.Tokens, func(w http.ResponseWriter, r *http.Request) {
		claims, err := mustClaims(r)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing auth context"})
			return
		}
		if deps.Users == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "user service unavailable"})
			return
		}

		user, err := deps.Users.GetByID(r.Context(), claims.UserID)
		if err != nil {
			switch {
			case errors.Is(err, repository.ErrUserNotFound):
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
			default:
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load user"})
			}
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"user": user})
	}))

	mux.HandleFunc("/api/v1/admin/users", requireAdmin(deps.Tokens, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		if deps.Users == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "user service unavailable"})
			return
		}

		items, err := deps.Users.ListAll(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list users"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}))

	mux.HandleFunc("/api/v1/admin/users/", requireAdmin(deps.Tokens, func(w http.ResponseWriter, r *http.Request) {
		if deps.Users == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "user service unavailable"})
			return
		}
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}

		claims, err := mustClaims(r)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing auth context"})
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/users/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] != "status" {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "admin user route not found"})
			return
		}

		var req struct {
			Status string `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		user, err := deps.Users.UpdateStatus(r.Context(), claims.UserID, parts[0], req.Status)
		if err != nil {
			switch {
			case errors.Is(err, repository.ErrUserNotFound):
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
			case errors.Is(err, service.ErrInvalidStatus):
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid status"})
			case errors.Is(err, service.ErrCannotDisableSelf):
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cannot disable current admin"})
			default:
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update user status"})
			}
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"user": user})
	}))

	mux.HandleFunc("/api/v1/clients", requireAuth(deps.Tokens, func(w http.ResponseWriter, r *http.Request) {
		if deps.Clients == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "client service unavailable"})
			return
		}

		claims, err := mustClaims(r)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing auth context"})
			return
		}

		switch r.Method {
		case http.MethodGet:
			items, listErr := deps.Clients.ListByUserID(r.Context(), claims.UserID)
			if listErr != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list clients"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"items": items})
		case http.MethodPost:
			var req struct {
				Name string `json:"name"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
				return
			}
			if strings.TrimSpace(req.Name) == "" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
				return
			}

			client, createErr := deps.Clients.Create(r.Context(), claims.UserID, req.Name)
			if createErr != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create client"})
				return
			}
			writeJSON(w, http.StatusCreated, client)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
	}))

	mux.HandleFunc("/api/v1/messages", requireAuth(deps.Tokens, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		if deps.Messages == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "message service unavailable"})
			return
		}

		claims, err := mustClaims(r)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing auth context"})
			return
		}

		items, err := deps.Messages.ListByUserID(r.Context(), claims.UserID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list messages"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}))

	mux.HandleFunc("/api/v1/messages/", requireAuth(deps.Tokens, func(w http.ResponseWriter, r *http.Request) {
		if deps.Messages == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "message service unavailable"})
			return
		}

		claims, err := mustClaims(r)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing auth context"})
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/api/v1/messages/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) == 1 && parts[0] != "" {
			if r.Method != http.MethodGet {
				writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
				return
			}

			item, err := deps.Messages.GetByID(r.Context(), claims.UserID, parts[0])
			if err != nil {
				switch {
				case errors.Is(err, repository.ErrMessageNotFound):
					writeJSON(w, http.StatusNotFound, map[string]string{"error": "message not found"})
				default:
					writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load message"})
				}
				return
			}

			writeJSON(w, http.StatusOK, item)
			return
		}

		if len(parts) != 2 || parts[1] != "redeliver" || parts[0] == "" {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "message route not found"})
			return
		}
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}

		item, err := deps.Messages.Redeliver(r.Context(), claims.UserID, parts[0])
		if err != nil {
			switch {
			case errors.Is(err, repository.ErrMessageNotFound):
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "message not found"})
			case errors.Is(err, repository.ErrClientNotFound):
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "client not found"})
			default:
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to redeliver message"})
			}
			return
		}

		writeJSON(w, http.StatusOK, item)
	}))

	mux.HandleFunc("/api/v1/clients/", requireAuth(deps.Tokens, func(w http.ResponseWriter, r *http.Request) {
		if deps.Clients == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "client service unavailable"})
			return
		}

		claims, err := mustClaims(r)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing auth context"})
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/api/v1/clients/")
		if path == "" {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "client id is required"})
			return
		}

		parts := strings.Split(strings.Trim(path, "/"), "/")
		id := parts[0]

		if len(parts) == 2 && parts[1] == "messages" {
			if r.Method != http.MethodGet {
				writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
				return
			}

			if deps.Messages == nil {
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "message service unavailable"})
				return
			}

			items, listErr := deps.Messages.ListByUserIDAndClientID(r.Context(), claims.UserID, id)
			if listErr != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load client messages"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"items": items})
			return
		}

		if len(parts) == 2 && parts[1] == "headers" {
			if r.Method != http.MethodPost {
				writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
				return
			}

			var req struct {
				SignatureHeader    string `json:"signatureHeader"`
				SignatureAlgorithm string `json:"signatureAlgorithm"`
				EventTypeHeader    string `json:"eventTypeHeader"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
				return
			}

			client, updateErr := deps.Clients.UpdateCustomHeaders(r.Context(), claims.UserID, id, req.SignatureHeader, req.SignatureAlgorithm, req.EventTypeHeader)
			if updateErr != nil {
				switch {
				case errors.Is(updateErr, repository.ErrClientNotFound):
					writeJSON(w, http.StatusNotFound, map[string]string{"error": "client not found"})
				default:
					writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update headers"})
				}
				return
			}
			writeJSON(w, http.StatusOK, client)
			return
		}

		if len(parts) == 2 && parts[1] == "security" {
			if r.Method != http.MethodPost {
				writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
				return
			}

			var req struct {
				VerifySignature bool `json:"verifySignature"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
				return
			}

			client, updateErr := deps.Clients.UpdateSecuritySettings(r.Context(), claims.UserID, id, req.VerifySignature)
			if updateErr != nil {
				switch {
				case errors.Is(updateErr, repository.ErrClientNotFound):
					writeJSON(w, http.StatusNotFound, map[string]string{"error": "client not found"})
				default:
					writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update client security"})
				}
				return
			}
			writeJSON(w, http.StatusOK, client)
			return
		}

		switch r.Method {
		case http.MethodGet:
			client, getErr := deps.Clients.GetByID(r.Context(), claims.UserID, id)
			if getErr != nil {
				switch {
				case errors.Is(getErr, repository.ErrClientNotFound):
					writeJSON(w, http.StatusNotFound, map[string]string{"error": "client not found"})
				default:
					writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load client"})
				}
				return
			}
			writeJSON(w, http.StatusOK, client)
		case http.MethodDelete:
			if deps.MessageRepo == nil {
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "message repository unavailable"})
				return
			}
			if err := deps.Clients.Delete(r.Context(), claims.UserID, id, deps.MessageRepo); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete client"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"status": "deleted", "clientId": id})
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
	}))

	mux.HandleFunc("/webhook/incoming/", withRateLimit(webhookLimiter, func(w http.ResponseWriter, r *http.Request) {
		if deps.Messages == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "message service unavailable"})
			return
		}

		token := strings.Trim(strings.TrimPrefix(r.URL.Path, "/webhook/incoming/"), "/")
		if token == "" {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "webhook token is required"})
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
			return
		}

		message, ingestErr := deps.Messages.IngestWebhook(r.Context(), token, r, body)
		if ingestErr != nil {
			switch {
			case errors.Is(ingestErr, repository.ErrClientNotFound):
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "webhook target not found"})
			default:
				log.Printf("[webhook] ingestion failed for token=%s: %v", token, ingestErr)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to store webhook"})
			}
			return
		}

		writeJSON(w, http.StatusAccepted, map[string]any{
			"status":       "accepted",
			"messageId":    message.ID,
			"webhookToken": token,
			"method":       r.Method,
			"receivedAt":   message.ReceivedAt.Format(time.RFC3339),
		})
	}))

	mux.HandleFunc("/ws/connect", func(w http.ResponseWriter, r *http.Request) {
		if deps.Realtime == nil || deps.Clients == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "realtime service unavailable"})
			return
		}

		conn, err := deps.Realtime.Upgrade(w, r)
		if err != nil {
			return
		}

		conn.SetReadDeadline(time.Now().Add(15 * time.Second))
		var authReq struct {
			Type         string `json:"type"`
			ClientID     string `json:"client_id"`
			ClientSecret string `json:"client_secret"`
		}
		if err := conn.ReadJSON(&authReq); err != nil {
			_ = conn.WriteJSON(map[string]string{"type": "auth_error", "error": "invalid auth payload"})
			_ = conn.Close()
			return
		}
		if authReq.Type != "auth" {
			_ = conn.WriteJSON(map[string]string{"type": "auth_error", "error": "auth message required"})
			_ = conn.Close()
			return
		}

		client, err := deps.Clients.AuthenticateRealtimeClient(r.Context(), authReq.ClientID, authReq.ClientSecret)
		if err != nil {
			_ = conn.WriteJSON(map[string]string{"type": "auth_error", "error": "invalid client credentials"})
			_ = conn.Close()
			return
		}
		_ = deps.Clients.MarkConnected(r.Context(), client.ID, time.Now().UTC())

		deps.Realtime.Serve(client, conn, func(ctx context.Context) {
			if deps.Messages == nil {
				return
			}
			recoverCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			_ = deps.Messages.RecoverPendingByClientID(recoverCtx, client.ID)
		})
	})

	return &http.Server{
		Addr:              cfg.Addr,
		Handler:           withMaxBody(withCORS(mux, cfg.Env, cfg.AllowedOrigins), 1<<20),
		ReadHeaderTimeout: 5 * time.Second,
	}
}

func withMaxBody(next http.Handler, maxBytes int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func withCORS(next http.Handler, env string, allowedOrigins string) http.Handler {
	allowed := strings.Split(allowedOrigins, ",")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if corsOriginAllowed(env, origin, allowed) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}

		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func corsOriginAllowed(env string, origin string, allowed []string) bool {
	if strings.TrimSpace(origin) == "" {
		return false
	}

	if env == "development" {
		parsed, err := url.Parse(origin)
		if err == nil && (parsed.Hostname() == "localhost" || parsed.Hostname() == "127.0.0.1") {
			return true
		}
	}

	for _, item := range allowed {
		normalized := strings.TrimSpace(item)
		if normalized == "*" || normalized == origin {
			return true
		}
	}

	return false
}
