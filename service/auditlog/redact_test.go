package auditlog

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
)

var testKeys = []string{"authorization", "api_key", "token", "password", "secret"}

func TestRedactBodyJSONDeepWalk(t *testing.T) {
	in := []byte(`{
		"model":"gpt-4",
		"api_key":"sk-must-not-leak",
		"nested":{"token":"t-secret","keep":"visible"},
		"list":[{"password":"pw"},{"ok":1}]
	}`)
	out := redactBody(in, testKeys)
	s := string(out)

	for _, leaked := range []string{"sk-must-not-leak", "t-secret", "pw"} {
		if strings.Contains(s, leaked) {
			t.Fatalf("secret %q leaked through redaction: %s", leaked, s)
		}
	}
	// Non-secret fields and structure must survive.
	for _, keep := range []string{"gpt-4", "visible", "\"ok\""} {
		if !strings.Contains(s, keep) {
			t.Fatalf("expected %q preserved, got: %s", keep, s)
		}
	}
	if strings.Count(s, redactedMarker) < 3 {
		t.Fatalf("expected >=3 redacted markers, got: %s", s)
	}
}

func TestRedactBodyNonJSONBearerSweep(t *testing.T) {
	in := []byte("GET /v1/x\nAuthorization: Bearer sk-live-DEADBEEF\nX: y")
	out := redactBody(in, testKeys)
	if strings.Contains(string(out), "sk-live-DEADBEEF") {
		t.Fatalf("bearer token leaked from non-JSON body: %s", out)
	}
	if !strings.Contains(string(out), redactedMarker) {
		t.Fatalf("expected redaction marker in non-JSON sweep: %s", out)
	}
}

func TestRedactBodyEmptyAndCaseInsensitive(t *testing.T) {
	if got := redactBody(nil, testKeys); got != nil {
		t.Fatalf("nil body should pass through, got %v", got)
	}
	// Mixed-case key must still be masked (keys lower-cased both sides).
	out := redactBody([]byte(`{"API_KEY":"x"}`), testKeys)
	if strings.Contains(string(out), `"x"`) {
		t.Fatalf("case-insensitive key not redacted: %s", out)
	}
}

// redactBody output must remain valid JSON for JSON input so downstream
// viewers can still parse the stored body.
func TestRedactBodyStaysValidJSON(t *testing.T) {
	out := redactBody([]byte(`{"api_key":"x","a":{"b":[1,2]}}`), testKeys)
	var v any
	if err := common.Unmarshal(out, &v); err != nil {
		t.Fatalf("redacted JSON no longer parses: %v (%s)", err, out)
	}
}
