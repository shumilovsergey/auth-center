package main

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
)

// Telegram creds live here on the proxy; Grafana never sees them.
// GRAFANA_ALERT_SECRET is the shared gate secret Grafana sends as a Bearer token.
var (
	grafanaBotToken    string
	grafanaChatID      string
	grafanaAlertSecret string
)

// grafanaAlert is the subset of Grafana's webhook payload we care about.
// Grafana renders the notification's Title/Message templates into these two
// fields, so we relay them straight through instead of the raw JSON.
type grafanaAlert struct {
	Title   string `json:"title"`
	Message string `json:"message"`
}

// POST /sh-grafana — validate the Grafana caller, format its payload, relay to Telegram.
func handleGrafanaAlert(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if subtle.ConstantTimeCompare([]byte(token), []byte(grafanaAlertSecret)) != 1 {
		log.Printf("alert error=unauthorized")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		log.Printf("alert error=read_body: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	text := formatGrafanaAlert(body)
	text = truncateRunes(text, 4000) // Telegram hard-caps messages at 4096 chars

	if err := sendTelegramMessage(text); err != nil {
		log.Printf("alert error=telegram: %v", err)
		http.Error(w, "telegram send failed", http.StatusBadGateway)
		return
	}
}

// formatGrafanaAlert extracts the rendered title + message from Grafana's payload.
// Falls back to the raw body if the JSON can't be parsed or carries no text.
func formatGrafanaAlert(body []byte) string {
	var a grafanaAlert
	if err := json.Unmarshal(body, &a); err == nil {
		parts := make([]string, 0, 2)
		if t := strings.TrimSpace(a.Title); t != "" {
			parts = append(parts, t)
		}
		if m := strings.TrimSpace(a.Message); m != "" {
			parts = append(parts, m)
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n\n")
		}
	}

	if raw := strings.TrimSpace(string(body)); raw != "" {
		return raw
	}
	return "(empty grafana payload)"
}

// sendTelegramMessage posts a plain-text message to GRAFANA_CHAT_ID.
// Called directly against api.telegram.org — the proxy host has outbound access.
func sendTelegramMessage(text string) error {
	api := "https://api.telegram.org/bot" + grafanaBotToken + "/sendMessage"
	form := url.Values{}
	form.Set("chat_id", grafanaChatID)
	form.Set("text", text)

	resp, err := httpClient.PostForm(api, form)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

// truncateRunes clips s to at most n runes (never splitting a multibyte char).
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…(truncated)"
}
