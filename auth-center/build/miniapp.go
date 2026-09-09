package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

// ── shared secret ─────────────────────────────────────────────────────────────

// auth-miniapp verifies Telegram initData itself, so it already holds BOT_TOKEN.
// Rather than introduce a second credential, both sides derive the same value
// from that token and compare it. The token itself never travels over the wire.
const miniappSecretPurpose = "auth-center/miniapp/v1"

var miniappSecret string

func miniappSecretFrom(token string) string {
	mac := hmac.New(sha256.New, []byte(token))
	mac.Write([]byte(miniappSecretPurpose)) //nolint:errcheck
	return hex.EncodeToString(mac.Sum(nil))
}

func initMiniapp() {
	if botToken == "" {
		log.Print("miniapp: BOT_TOKEN not set — POST /miniapp/auth disabled")
		return
	}
	miniappSecret = miniappSecretFrom(botToken)
}

// ── handler ───────────────────────────────────────────────────────────────────

// POST /miniapp/auth — server-to-server only, never called from the browser.
//
// This is how a Telegram identity reaches auth-center: auth-miniapp checks the
// initData signature and reports the identity here, naming the session by the
// token it carried in start_param. From this point the flow is the ordinary one
// — the browser tab picks the result up on /poll. The bot round-trip this
// replaced is written up in tools/old_bot_auth.md.
func handleMiniappAuth(w http.ResponseWriter, r *http.Request) {
	cleanSessions()

	var body struct {
		SessionToken string `json:"session_token"`
		Secret       string `json:"secret"`
		// FromQR says the user scanned the QR, which means the browser waiting
		// for this login is on another machine. No code is issued in that case:
		// the mini app has nowhere useful to send the phone in the user's hand,
		// and a one-time code nobody spends is a credential kept alive for no
		// reason.
		FromQR bool `json:"from_qr"`
		User   struct {
			ID        int64  `json:"id"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			Username  string `json:"username"`
		} `json:"user"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonErr(w, "no data", http.StatusBadRequest)
		return
	}

	if miniappSecret == "" {
		log.Printf("miniapp error=disabled from=%s", r.RemoteAddr)
		jsonErr(w, "miniapp auth not configured", http.StatusServiceUnavailable)
		return
	}
	if !hmac.Equal([]byte(body.Secret), []byte(miniappSecret)) {
		log.Printf("miniapp error=unauthorized from=%s", r.RemoteAddr)
		jsonErr(w, "unauthorized", http.StatusForbidden)
		return
	}
	if body.SessionToken == "" || body.User.ID == 0 {
		jsonErr(w, "missing fields", http.StatusBadRequest)
		return
	}

	from := body.User
	sessionsMu.Lock()
	sess, ok := sessions[body.SessionToken]
	if !ok {
		sessionsMu.Unlock()
		log.Printf("miniapp error=unknown_session uid=%d", from.ID)
		jsonErr(w, "expired", http.StatusNotFound)
		return
	}
	if time.Since(sess.CreatedAt) > sessionTTL {
		delete(sessions, body.SessionToken)
		sessionsMu.Unlock()
		log.Printf("miniapp error=expired_session uid=%d", from.ID)
		jsonErr(w, "expired", http.StatusNotFound)
		return
	}
	if sess.Status != "pending" {
		sessionsMu.Unlock()
		log.Printf("miniapp error=session_used uid=%d", from.ID)
		jsonErr(w, "session already used", http.StatusConflict)
		return
	}

	user := map[string]any{
		"id":         from.ID,
		"first_name": from.FirstName,
		"last_name":  from.LastName,
		"username":   from.Username,
	}
	sess.Status = "authenticated"
	sess.User = user
	if sess.Redirect != "" {
		sess.Code = newCode(user, "telegram")
	}
	redirect := sess.Redirect
	sessionsMu.Unlock()

	log.Printf("miniapp uid=%d name=%q", from.ID, strings.TrimSpace(from.FirstName+" "+from.LastName))

	// Hand back the app's address plus a code of the mini app's own, so it can
	// offer a way home that lands signed in. A scanned QR gets the address but
	// no code — see FromQR above.
	//
	// It has to be a second code, not the one the session holds: the browser tab
	// that started the login is polling for that one and will spend it, and a
	// one-time code spent twice fails for whoever is second. Two codes for one
	// verified identity give nothing away — each is single-use, each expires in
	// codeTTL, and redeeming either still needs an app token.
	resp := map[string]any{"ok": true, "redirect": redirect}
	if redirect != "" && !body.FromQR {
		resp["code"] = newCode(user, "telegram")
	}
	jsonOK(w, resp)
}
