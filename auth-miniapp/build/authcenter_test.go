package main

import "testing"

// The secret is derived independently on both sides from the same BOT_TOKEN and
// never travels, so nothing at runtime would report a mismatch — auth-center
// would simply answer 403 and the login would fail with no hint why. This
// fixture is duplicated verbatim in auth-center's miniapp_test.go: if either
// side ever changes the purpose string or the construction, one of the two
// tests fails and names the drift.
func TestDerivedSecretMatchesAuthCenter(t *testing.T) {
	const (
		token = "123456:test-bot-token"
		want  = "d1200c0ddc8b9375df415693b849be3d6a4c7079cc50f73e40ca1bdb0b53fe89"
	)
	if got := deriveSecret(token); got != want {
		t.Fatalf("derived secret drifted:\n got  %s\n want %s", got, want)
	}
}
