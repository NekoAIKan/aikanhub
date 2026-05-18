package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/setting/audit_setting"

	"github.com/gin-gonic/gin"
)

// The capture writer's contract: every byte is passed through to the
// underlying writer immediately (streaming must not be delayed), and the
// audit copy is bounded by `limit` (a long stream can't grow memory).
func TestAuditCaptureWriterPassthroughAndBoundedTee(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	cw := &auditCaptureWriter{ResponseWriter: c.Writer, limit: 8}

	for i := 0; i < 5; i++ {
		if _, err := cw.Write([]byte("ABCD")); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}
	// Client got everything (20 bytes), audit kept only the 8-byte cap.
	if got := rec.Body.Len(); got != 20 {
		t.Fatalf("passthrough lost bytes: client saw %d, want 20", got)
	}
	if len(cw.buf) != 8 {
		t.Fatalf("tee not bounded: buffered %d, want 8", len(cw.buf))
	}
	if !cw.truncated {
		t.Fatalf("expected truncated flag once cap exceeded")
	}
}

func TestAuditCaptureWriterDetectsSSE(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	cw := &auditCaptureWriter{ResponseWriter: c.Writer, limit: 1 << 20}
	_, _ = cw.Write([]byte("data: hi\n\n"))
	if !cw.streaming {
		t.Fatalf("expected streaming detected for text/event-stream")
	}
}

func TestAuditLogDisabledIsNoop(t *testing.T) {
	gin.SetMode(gin.TestMode)
	prev := audit_setting.GetAuditSetting().Enabled
	audit_setting.GetAuditSetting().Enabled = false
	defer func() { audit_setting.GetAuditSetting().Enabled = prev }()

	r := gin.New()
	r.Use(AuditLog())
	r.POST("/x", func(c *gin.Context) { c.String(200, "ok") })

	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"a":1}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 || w.Body.String() != "ok" {
		t.Fatalf("disabled audit must not alter response: code=%d body=%q", w.Code, w.Body.String())
	}
}

func TestAuditLogSkipPrefix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := audit_setting.GetAuditSetting()
	prevEnabled, prevSkips := s.Enabled, s.PathSkipPrefixes
	s.Enabled = true
	s.PathSkipPrefixes = []string{"/api/status"}
	defer func() { s.Enabled, s.PathSkipPrefixes = prevEnabled, prevSkips }()

	r := gin.New()
	r.Use(AuditLog())
	r.GET("/api/status", func(c *gin.Context) { c.String(200, "skipped-ok") })

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 || w.Body.String() != "skipped-ok" {
		t.Fatalf("skip-prefix path must pass through untouched: %d %q", w.Code, w.Body.String())
	}
}
