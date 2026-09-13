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
	FromQR       bool     `json:"from_qr"`
	User         bindUser `json:"user"`
}

// homeRequest names no session, because there is none — see homeLogin.
type homeRequest struct {
	Secret string   `json:"secret"`
	User   bindUser `json:"user"`
}

// bindSession hands the verified identity to auth-center, naming the session by
// the token that rode in start_param. Once this returns, the browser tab that
// opened the link picks the result up on its next /poll and leaves for the
// client app — this app's part in the login is over.
//
// It returns the address of the app the login started from together with a
// one-time code of our own, so the page can offer a way back that lands signed
// in. The code is not the one the waiting tab will spend — auth-center issues a
// second one for exactly this, and skips it entirely when fromQR says the
// waiting browser is on another machine.
func bindSession(sessionToken string, fromQR bool, u *TGUser) (bindResult, error) {
	body, _ := json.Marshal(bindRequest{
		SessionToken: sessionToken,
		Secret:       authSecret,
		FromQR:       fromQR,
		User: bindUser{
			ID:        u.ID,
			FirstName: u.FirstName,
			LastName:  u.LastName,
			Username:  u.Username,
		},
	})

	return postAuthCenter("/miniapp/auth", body)
}

// homeLogin is the other way in. Nobody is waiting for this user: they opened
// the mini app on their own, so there is no session to name and no browser tab
// polling for a result. auth-center answers with a code and with the address of
// the app a Telegram visitor belongs in — which is its decision, not ours. This
// app never proposes a destination; holding the secret lets it assert who
// somebody is, and that is deliberately all it lets it do.
func homeLogin(u *TGUser) (bindResult, error) {
	body, _ := json.Marshal(homeRequest{
		Secret: authSecret,
		User: bindUser{
			ID:        u.ID,
			FirstName: u.FirstName,
			LastName:  u.LastName,
			Username:  u.Username,
		},
	})
	return postAuthCenter("/miniapp/home", body)
}

// apiError is auth-center's refusal with its status still attached. The status
// is what lets a caller tell one refusal from another — "already used" is a
// session that did its job, "expired" is one that never will — without
// matching on the wording of a message meant for a human.
type apiError struct {
	Status  int
	Message string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("auth-center said %d: %s", e.Status, e.Message)
}

// postAuthCenter is the shared half of both calls: one POST, and auth-center's
// own wording on the way out.
func postAuthCenter(path string, body []byte) (bindResult, error) {
	url := strings.TrimRight(authInternal, "/") + path
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
	return bindResult{}, &apiError{Status: resp.StatusCode, Message: e.Error}
}
