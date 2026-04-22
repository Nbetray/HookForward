package http

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"hookforward/backend/internal/auth"
)

type contextKey string

const authClaimsKey contextKey = "authClaims"

func requireAuth(tokens *auth.TokenIssuer, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if tokens == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "auth unavailable"})
			return
		}

		header := strings.TrimSpace(r.Header.Get("Authorization"))
		if header == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing bearer token"})
			return
		}

		token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		if token == "" || token == header {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid bearer token"})
			return
		}

		claims, err := tokens.Parse(token)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid bearer token"})
			return
		}

		ctx := context.WithValue(r.Context(), authClaimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

func mustClaims(r *http.Request) (auth.UserClaims, error) {
	claims, ok := r.Context().Value(authClaimsKey).(auth.UserClaims)
	if !ok {
		return auth.UserClaims{}, errors.New("missing auth claims")
	}
	return claims, nil
}

func requireAdmin(tokens *auth.TokenIssuer, next http.HandlerFunc) http.HandlerFunc {
	return requireAuth(tokens, func(w http.ResponseWriter, r *http.Request) {
		claims, err := mustClaims(r)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing auth context"})
			return
		}
		if claims.Role != "admin" {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin only"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
