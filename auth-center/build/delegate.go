package main

import (
	"encoding/json"
	"log"
	"net/http"
)

// POST /delegate — server-to-server only, never called from the browser.
//
// Called by a trusted app to issue a one-time cross-app login code for a user
// who is already authenticated in that app. Any other trusted app can redeem
// the code via POST /exchange — no target binding needed since all apps are trusted.
func handleDelegate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		UserID   string `json:"user_id"`
		AppToken string `json:"app_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonErr(w, "no data", http.StatusBadRequest)
		return
	}
	if body.UserID == "" || body.AppToken == "" {
		jsonErr(w, "missing fields", http.StatusBadRequest)
		return
	}
	if len(appTokens) > 0 && !appTokens[body.AppToken] {
		log.Printf("delegate error=unauthorized from=%s", r.RemoteAddr)
		jsonErr(w, "unauthorized", http.StatusForbidden)
		return
	}

	user := map[string]any{"id": body.UserID}
	code := newCode(user, "delegate")
	log.Printf("delegate user_id=%s", body.UserID)
	jsonOK(w, map[string]string{"code": code})
}
