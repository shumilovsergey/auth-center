package main

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"
)

const testToken = "123456:TEST-BOT-TOKEN"

// signInitData builds a correctly signed initData string the way Telegram does,
// so the validator is tested against a real signature rather than a fixture.
func signInitData(fields map[string]string) string {
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var check strings.Builder
	for i, k := range keys {
		if i > 0 {
			check.WriteByte('\n')
		}
		fmt.Fprintf(&check, "%s=%s", k, fields[k])
	}

	secret := hmacSHA256([]byte("WebAppData"), []byte(testToken))
	hash := hex.EncodeToString(hmacSHA256(secret, []byte(check.String())))

	q := url.Values{}
	for k, v := range fields {
		q.Set(k, v)
	}
	q.Set("hash", hash)
	return q.Encode()
}

func freshFields() map[string]string {
	return map[string]string{
		"user":        `{"id":42424242,"first_name":"Poc","username":"poc_user","language_code":"ru"}`,
		"auth_date":   fmt.Sprint(time.Now().Unix()),
		"query_id":    "AAF_test",
		"chat_type":   "private",
		"start_param": "auth",
	}
}

func TestValidateInitDataAcceptsGoodSignature(t *testing.T) {
	if _, err := validateInitData(signInitData(freshFields()), testToken); err != nil {
		t.Fatalf("valid init_data rejected: %v", err)
	}
}

func TestValidateInitDataRejectsTamperedUser(t *testing.T) {
	initData := signInitData(freshFields())
	// Swap the Telegram ID after signing — the exact attack the hash exists for.
	tampered := strings.Replace(initData, "42424242", "99999999", 1)
	if _, err := validateInitData(tampered, testToken); err == nil {
		t.Fatal("tampered init_data accepted")
	}
}

func TestValidateInitDataRejectsWrongToken(t *testing.T) {
	if _, err := validateInitData(signInitData(freshFields()), "999:OTHER"); err == nil {
		t.Fatal("init_data signed by another bot accepted")
	}
}

func TestValidateInitDataRejectsExpired(t *testing.T) {
	f := freshFields()
	f["auth_date"] = fmt.Sprint(time.Now().Add(-initDataMaxAge - time.Hour).Unix())
	if _, err := validateInitData(signInitData(f), testToken); err == nil {
		t.Fatal("expired init_data accepted")
	}
}

func TestValidateInitDataRejectsMissingHash(t *testing.T) {
	if _, err := validateInitData("user=%7B%22id%22%3A1%7D&auth_date=1", testToken); err == nil {
		t.Fatal("init_data without hash accepted")
	}
}

func TestParseUser(t *testing.T) {
	u, err := parseUser(signInitData(freshFields()))
	if err != nil {
		t.Fatalf("parseUser: %v", err)
	}
	if u.ID != 42424242 || u.Username != "poc_user" || u.FirstName != "Poc" {
		t.Fatalf("unexpected user: %+v", u)
	}
}

func TestParseUserWithoutUserField(t *testing.T) {
	if _, err := parseUser("auth_date=1&hash=deadbeef"); err == nil {
		t.Fatal("init_data without a user field accepted")
	}
}

// A direct mini app link (`?startapp=…`) delivers its value inside the signed
// initData, so auth-center could hand a session id through the link and trust
// what comes back.
func TestStartParamIsCoveredByTheSignature(t *testing.T) {
	f := freshFields()
	f["start_param"] = "sess_abc123"
	initData := signInitData(f)

	values, err := validateInitData(initData, testToken)
	if err != nil {
		t.Fatalf("init_data with start_param rejected: %v", err)
	}
	if got := values.Get("start_param"); got != "sess_abc123" {
		t.Fatalf("start_param = %q, want sess_abc123", got)
	}
	if got := initDataField(initData, "start_param"); got != "sess_abc123" {
		t.Fatalf("initDataField start_param = %q", got)
	}

	tampered := strings.Replace(initData, "sess_abc123", "sess_hijack", 1)
	if _, err := validateInitData(tampered, testToken); err == nil {
		t.Fatal("tampered start_param accepted")
	}
}
