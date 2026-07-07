package main

import (
	"crypto/subtle"
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

// POST /alert — validate the Grafana caller, then relay its payload to Telegram.
// For now the raw request body is forwarded as-is so we can see Grafana's real
// shape before building a formatter.
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

	text := strings.TrimSpace(string(body))
	if text == "" {
		text = "(empty grafana payload)"
	}
	text = truncateRunes(text, 4000) // Telegram hard-caps messages at 4096 chars

	if err := sendTelegramMessage(text); err != nil {
		log.Printf("alert error=telegram: %v", err)
		http.Error(w, "telegram send failed", http.StatusBadGateway)
		return
	}
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
