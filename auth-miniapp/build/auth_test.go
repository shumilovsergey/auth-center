package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// A stale session is the refusal handleAuth does not pass on. Telegram resumes
// a mini app that was left open, so the same login link arrives a second time —
// against a session already spent (409) or long expired (404) — while the
// person holding it is verified in Telegram right then. These tests pin the
// fork: both of auth-center's own stale-session refusals go through the front
// door, a 404 from anything else does not.

// authCenterStub answers /miniapp/auth with the given status and body, and
// records whether the front door was tried at all.
func authCenterStub(t *testing.T, authStatus int, authBody string) (*httptest.Server, *bool) {
	t.Helper()
	homeCalled := false

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/miniapp/auth":
			w.WriteHeader(authStatus)
			io.WriteString(w, authBody) //nolint:errcheck
		case "/miniapp/home":
			homeCalled = true
			io.WriteString(w, `{"redirect":"https://home.example/callback","code":"CODE-HOME"}`) //nolint:errcheck
		default:
			t.Errorf("unexpected call to %s", r.URL.Path)
		}
	}))

	prevToken, prevInternal, prevSecret := botToken, authInternal, authSecret
	botToken, authInternal, authSecret = testToken, srv.URL, deriveSecret(testToken)
	t.Cleanup(func() {
		srv.Close()
		botToken, authInternal, authSecret = prevToken, prevInternal, prevSecret
	})

	return srv, &homeCalled
}

// postAuth drives handleAuth the way the page does: one signed initData whose
// start_param names a session behind the button flag.
func postAuth(t *testing.T) *httptest.ResponseRecorder {
	t.Helper()
	f := freshFields()
	f["start_param"] = fromButton + "sess_used"

	body, _ := json.Marshal(authRequest{InitData: signInitData(f)})
	req := httptest.NewRequest(http.MethodPost, "/api/auth", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handleAuth(rec, req)
	return rec
}

func TestAuthFallsThroughToHomeWhenSessionIsStale(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"already used", http.StatusConflict, `{"error":"session already used"}`},
		{"expired", http.StatusNotFound, `{"error":"expired"}`},
	} {
		t.Run(tc.name, func(t *testing.T) { assertFrontDoor(t, tc.status, tc.body) })
	}
}

func assertFrontDoor(t *testing.T, status int, body string) {
	t.Helper()
	_, homeCalled := authCenterStub(t, status, body)

	rec := postAuth(t)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — body %s", rec.Code, rec.Body.String())
	}
	if !*homeCalled {
		t.Fatal("front door was never tried")
	}

	var got authResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if !got.OK || got.Code != "CODE-HOME" || got.Redirect != "https://home.example/callback" {
		t.Fatalf("answer = %+v, want the front door's code and redirect", got)
	}
	// The page must not be able to tell this apart from an ordinary login: it
	// greets whoever it is told about, and this is the same person.
	if got.UserID != 42424242 {
		t.Fatalf("user_id = %d, want the verified Telegram id", got.UserID)
	}
}

// A 404 that is not auth-center's own "expired" is a 404 from somewhere else —
// a mistyped AUTH_INTERNAL, a proxy, a path that moved. Treating that as a
// stale session would send every login to the home app instead of the app the
// user asked for, and nothing on screen would say so.
func TestAuthReportsForeign404InsteadOfFallingThrough(t *testing.T) {
	_, homeCalled := authCenterStub(t, http.StatusNotFound, "404 page not found\n")

	rec := postAuth(t)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502 — body %s", rec.Code, rec.Body.String())
	}
	if *homeCalled {
		t.Fatal("a 404 from an unknown path must not become a front-door login")
	}
}

// When the front door is not configured, auth-center answers the fallback with
// 503. The user is still owed the reason their own login failed, not ours.
func TestAuthReportsOriginalRefusalWhenFrontDoorUnavailable(t *testing.T) {
	homeCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/miniapp/auth":
			w.WriteHeader(http.StatusConflict)
			io.WriteString(w, `{"error":"session already used"}`) //nolint:errcheck
		case "/miniapp/home":
			homeCalled = true
			w.WriteHeader(http.StatusServiceUnavailable)
			io.WriteString(w, `{"error":"home redirect not configured"}`) //nolint:errcheck
		}
	}))
	defer srv.Close()

	prevToken, prevInternal, prevSecret := botToken, authInternal, authSecret
	botToken, authInternal, authSecret = testToken, srv.URL, deriveSecret(testToken)
	defer func() { botToken, authInternal, authSecret = prevToken, prevInternal, prevSecret }()

	rec := postAuth(t)
	if !homeCalled {
		t.Fatal("front door was never tried")
	}
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
	if body := rec.Body.String(); !bytes.Contains([]byte(body), []byte("session already used")) {
		t.Fatalf("body = %s, want the original refusal", body)
	}
}
