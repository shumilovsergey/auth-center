package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// initDataMaxAge bounds how long a signed initData string stays acceptable. A
// mini app tab can legitimately sit open for a while, so this is generous; the
// signature — not the clock — is what proves identity.
const initDataMaxAge = 24 * time.Hour

// TGUser is the `user` object Telegram packs into initData. The ID is the same
// permanent numeric Telegram ID auth-center already uses as a primary key.
type TGUser struct {
	ID              int64  `json:"id"`
	FirstName       string `json:"first_name,omitempty"`
	LastName        string `json:"last_name,omitempty"`
	Username        string `json:"username,omitempty"`
	LanguageCode    string `json:"language_code,omitempty"`
	IsPremium       bool   `json:"is_premium,omitempty"`
	PhotoURL        string `json:"photo_url,omitempty"`
	AllowsWriteToPM bool   `json:"allows_write_to_pm,omitempty"`
}

// validateInitData verifies the HMAC Telegram puts in `hash`, per
// https://core.telegram.org/bots/webapps#validating-data-received-via-the-mini-app
//
//	secret_key = HMAC_SHA256(key="WebAppData", data=bot_token)
//	hash       = HMAC_SHA256(key=secret_key, data=data_check_string)
//
// data_check_string is every field except `hash`, sorted by key, joined by "\n"
// as "key=value" — using the raw, already-decoded values.
func validateInitData(initData, botToken string) (url.Values, error) {
	values, err := url.ParseQuery(initData)
	if err != nil {
		return nil, fmt.Errorf("init_data is not a query string: %w", err)
	}

	got := values.Get("hash")
	if got == "" {
		return nil, errors.New("init_data has no hash")
	}

	keys := make([]string, 0, len(values))
	for k := range values {
		if k == "hash" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(values.Get(k))
	}

	secret := hmacSHA256([]byte("WebAppData"), []byte(botToken))
	want := hex.EncodeToString(hmacSHA256(secret, []byte(b.String())))

	// Constant-time compare — this is the line that decides whether we believe
	// the Telegram ID at all.
	if !hmac.Equal([]byte(want), []byte(got)) {
		return nil, errors.New("init_data signature mismatch")
	}

	if raw := values.Get("auth_date"); raw != "" {
		sec, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("auth_date is not a unix timestamp: %w", err)
		}
		if age := time.Since(time.Unix(sec, 0)); age > initDataMaxAge {
			return nil, fmt.Errorf("init_data expired (%s old)", age.Round(time.Second))
		}
	}

	return values, nil
}

// parseUser pulls the `user` field out of initData without checking anything.
// Only call it after validateInitData — or knowingly, when BOT_TOKEN is unset.
func parseUser(initData string) (*TGUser, error) {
	values, err := url.ParseQuery(initData)
	if err != nil {
		return nil, fmt.Errorf("init_data is not a query string: %w", err)
	}
	raw := values.Get("user")
	if raw == "" {
		return nil, errors.New("init_data has no user field")
	}
	var u TGUser
	if err := json.Unmarshal([]byte(raw), &u); err != nil {
		return nil, fmt.Errorf("user field is not json: %w", err)
	}
	if u.ID == 0 {
		return nil, errors.New("user has no id")
	}
	return &u, nil
}

// initDataField reads one raw field out of initData without verifying anything.
// Same caveat as parseUser: only trust the result after validateInitData.
func initDataField(initData, key string) string {
	values, err := url.ParseQuery(initData)
	if err != nil {
		return ""
	}
	return values.Get(key)
}

func hmacSHA256(key, data []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(data)
	return m.Sum(nil)
}
