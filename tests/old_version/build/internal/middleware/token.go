package middleware

import (
	"net/http"
)

type TokenAuth struct {
	validTokens map[string]bool
}

func NewTokenAuth(tokens []string) *TokenAuth {
	tokenMap := make(map[string]bool)
	for _, t := range tokens {
		tokenMap[t] = true
	}
	return &TokenAuth{validTokens: tokenMap}
}

func (ta *TokenAuth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Auth-Token")
		if token == "" {
			http.Error(w, `{"error":"missing X-Auth-Token header"}`, http.StatusUnauthorized)
			return
		}

		if !ta.validTokens[token] {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
