package main

import (
	"crypto/subtle"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// Anthropic creds live here on the proxy; nom-nom never sees the API key.
// NOM_NOM_SECRET is the shared gate secret nom-nom sends as a Bearer token.
var (
	anthropicAPIKey string
	nomNomSecret    string

	// Dedicated client: Opus photo scans legitimately take ~15s and nom-nom
	// bounds each call at 20s, so the proxy must not time out first. (The
	// shared httpClient is only 10s.)
	anthropicClient = &http.Client{Timeout: 30 * time.Second}
)

const anthropicMessagesURL = "https://api.anthropic.com/v1/messages"

// POST /nom-nom-ai — authenticated pass-through to the Anthropic Messages API.
// Validate the shared secret, forward the raw body upstream with the real API
// key, and return Anthropic's status + body verbatim.
func handleNomNomAI(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if subtle.ConstantTimeCompare([]byte(token), []byte(nomNomSecret)) != 1 {
		log.Printf("nom-nom-ai error=unauthorized")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Body is opaque to us and can be ~10 MB (photo scans embed a base64
	// image), so we stream it straight through — no buffering, no reshaping.
	// r.Context() ties the upstream call to the client: if nom-nom aborts, so
	// do we.
	req, err := http.NewRequestWithContext(r.Context(), "POST", anthropicMessagesURL, r.Body)
	if err != nil {
		log.Printf("nom-nom-ai error=build_request: %v", err)
		http.Error(w, "proxy error", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", anthropicAPIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := anthropicClient.Do(req)
	if err != nil {
		log.Printf("nom-nom-ai error=upstream: %v", err)
		http.Error(w, "upstream unreachable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Return Anthropic's response verbatim — status and body unchanged. nom-nom
	// inspects both (e.g. 402 + {"error":{"type":"billing_error"}} = out of
	// credits), so we never rewrite errors.
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body) //nolint:errcheck
}
