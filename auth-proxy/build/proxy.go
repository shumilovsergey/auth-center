package main

import (
	"io"
	"log"
	"net/http"
)

// POST /webhook — forward Telegram update to auth-center as-is.
func handleWebhook(w http.ResponseWriter, r *http.Request) {
	req, err := http.NewRequest("POST", authCenterURL+"/webhook", r.Body)
	if err != nil {
		log.Printf("webhook error=build_request: %v", err)
		http.Error(w, "proxy error", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", r.Header.Get("Content-Type"))
	if secret := r.Header.Get("X-Telegram-Bot-Api-Secret-Token"); secret != "" {
		req.Header.Set("X-Telegram-Bot-Api-Secret-Token", secret)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("webhook error=upstream: %v", err)
		http.Error(w, "upstream unreachable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body) //nolint:errcheck
}

// POST /tg-api/{path...} — forward Telegram API calls from auth-center to Telegram.
// Path is passed through as-is: /tg-api/botTOKEN/sendMessage → api.telegram.org/botTOKEN/sendMessage
func handleTelegramAPI(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")
	target := "https://api.telegram.org/" + path

	req, err := http.NewRequest("POST", target, r.Body)
	if err != nil {
		log.Printf("tg-api error=build_request path=%s: %v", path, err)
		http.Error(w, "proxy error", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", r.Header.Get("Content-Type"))

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("tg-api error=upstream path=%s: %v", path, err)
		http.Error(w, "upstream unreachable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body) //nolint:errcheck
}
