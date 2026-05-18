package auditlog

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/setting/audit_setting"
)

var testSettings = audit_setting.AuditSetting{
	Redact:       true,
	RedactKeys:   []string{"api_key", "authorization", "token", "secret"},
	MaxBodyBytes: 64,
}

func TestCapBodyBoundedPrefix(t *testing.T) {
	var trunc bool
	out := capBody([]byte(strings.Repeat("A", 200)), 16, &trunc)
	if !trunc {
		t.Fatalf("expected truncated flag set")
	}
	if !strings.HasPrefix(out, strings.Repeat("A", 16)) {
		t.Fatalf("expected 16-byte prefix, got %q", out)
	}
	if !strings.Contains(out, "[truncated: 16 of 200 bytes]") {
		t.Fatalf("expected truncation marker, got %q", out)
	}
}

func TestCapBodyUnderCapUnchanged(t *testing.T) {
	var trunc bool
	out := capBody([]byte("small"), 64, &trunc)
	if trunc || out != "small" {
		t.Fatalf("small body must pass through untouched: trunc=%v out=%q", trunc, out)
	}
	if got := capBody(nil, 64, &trunc); got != "" {
		t.Fatalf("nil body → empty string, got %q", got)
	}
}

func TestSubmitBeforeInitIsNoop(t *testing.T) {
	// initStarted is false until Init(); Submit must not panic / block /
	// nil-deref the queue.
	Submit(&Record{RequestId: "x"})
	if DroppedCount() != 0 {
		t.Fatalf("pre-init Submit should be a silent no-op, not a drop")
	}
}

// prepareBodies must redact before the line is rendered and never leave the
// raw []byte bodies attached (they'd double the JSON size and skip the cap).
func TestPrepareBodiesRedactsAndCapsAndClears(t *testing.T) {
	rec := &Record{
		RequestBody:  []byte(`{"api_key":"sk-leak","prompt":"hi"}`),
		ResponseBody: []byte(strings.Repeat("B", 100)),
	}
	prepareBodies(rec, &testSettings)
	if !rec.Redacted {
		t.Fatalf("expected Redacted=true")
	}
	if strings.Contains(rec.RequestBodyStr, "sk-leak") {
		t.Fatalf("secret leaked: %s", rec.RequestBodyStr)
	}
	if !strings.Contains(rec.RequestBodyStr, "hi") {
		t.Fatalf("non-secret param must survive: %s", rec.RequestBodyStr)
	}
	if !rec.Truncated || !strings.Contains(rec.ResponseBodyStr, "[truncated") {
		t.Fatalf("oversized response must be capped: %q", rec.ResponseBodyStr)
	}
	if rec.RequestBody != nil || rec.ResponseBody != nil {
		t.Fatalf("raw byte bodies must be cleared after render")
	}
}
