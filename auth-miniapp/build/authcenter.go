package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ── shared secret ─────────────────────────────────────────────────────────────

// This app verifies Telegram initData itself, so it already holds BOT_TOKEN —
// the same token auth-center holds for its bot. Both sides derive this value
// from it and compare, which proves the caller is trusted without inventing a
// second credential and without ever putting the token on the wire. The purpose
// string must match miniappSecretPurpose in auth-center's miniapp.go.
const secretPurpose = "auth-center/miniapp/v1"

func deriveSecret(token string) string {
	return hex.EncodeToString(hmacSHA256([]byte(token), []byte(secretPurpose)))
}

// ── client ────────────────────────────────────────────────────────────────────

var authClient = &http.Client{Timeout: 10 * time.Second}

type bindUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
}

// bindResult is what auth-center reports back once the session is bound.
type bindResult struct {
	Redirect string `json:"redirect"`
	Code     string `json:"code"`
}

type bindRequest struct {
	SessionToken string   `json:"session_token"`
	Secret       string   `json:"secret"`
	User         bindUser `json:"user"`
}

// bindSession hands the verified identity to auth-center, naming the session by
// the token that rode in start_param. Once this returns, the browser tab that
// opened the link picks the result up on its next /poll and leaves for the
// client app — this app's part in the login is over.
//
// It returns the address of the app the login started from together with a
// one-time code of our own, so the page can offer a way back that lands signed
// in. The code is not the one the waiting tab will spend — auth-center issues a
// second one for exactly this.
func bindSession(sessionToken string, u *TGUser) (bindResult, error) {
	body, _ := json.Marshal(bindRequest{
		SessionToken: sessionToken,
		Secret:       authSecret,
		User: bindUser{
			ID:        u.ID,
			FirstName: u.FirstName,
			LastName:  u.LastName,
			Username:  u.Username,
		},
	})

	url := strings.TrimRight(authInternal, "/") + "/miniapp/auth"
	resp, err := authClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return bindResult{}, fmt.Errorf("auth-center unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var out bindResult
		json.NewDecoder(resp.Body).Decode(&out) //nolint:errcheck
		return out, nil
	}

	// auth-center answers errors as {"error": "..."} — surface its wording
	// rather than a bare status, since "expired" and "unauthorized" mean very
	// different things to whoever is reading the log.
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
	var e struct {
		Error string `json:"error"`
	}
	json.Unmarshal(raw, &e) //nolint:errcheck
	if e.Error == "" {
		e.Error = strings.TrimSpace(string(raw))
	}
	return bindResult{}, fmt.Errorf("auth-center said %d: %s", resp.StatusCode, e.Error)
}
