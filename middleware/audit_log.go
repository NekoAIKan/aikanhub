package middleware

import (
	"io"
	"math/rand"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/service/auditlog"
	"github.com/QuantumNous/new-api/setting/audit_setting"

	"github.com/gin-gonic/gin"
)

// auditCaptureWriter is a streaming-safe response tee. CRITICAL invariant:
// every Write/WriteString passes straight through to the embedded
// gin.ResponseWriter FIRST, with no buffering delay — SSE/chat streaming
// must not be slowed or broken by audit. We only additionally copy a
// bounded prefix into `buf` (capped at `limit`); once the cap is hit we
// stop copying so a long stream can't grow memory. Hijack/Flush/Pusher are
// inherited from the embedded writer, so websockets and SSE flushing keep
// working untouched.
type auditCaptureWriter struct {
	gin.ResponseWriter
	buf       []byte
	limit     int
	streaming bool
	truncated bool
	detected  bool
}

func (w *auditCaptureWriter) detectStreaming() {
	if w.detected {
		return
	}
	w.detected = true
	ct := strings.ToLower(w.Header().Get("Content-Type"))
	if strings.Contains(ct, "text/event-stream") ||
		strings.Contains(ct, "application/x-ndjson") ||
		strings.Contains(ct, "stream") {
		w.streaming = true
	}
}

func (w *auditCaptureWriter) tee(b []byte) {
	w.detectStreaming()
	if len(w.buf) >= w.limit {
		if len(b) > 0 {
			w.truncated = true
		}
		return
	}
	room := w.limit - len(w.buf)
	if room >= len(b) {
		w.buf = append(w.buf, b...)
		return
	}
	w.buf = append(w.buf, b[:room]...)
	w.truncated = true
}

func (w *auditCaptureWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b) // passthrough first — never delay the stream
	if n > 0 {
		w.tee(b[:n])
	}
	return n, err
}

func (w *auditCaptureWriter) WriteString(s string) (int, error) {
	n, err := w.ResponseWriter.WriteString(s)
	if n > 0 {
		w.tee([]byte(s[:n]))
	}
	return n, err
}

// AuditLog is the global capture middleware. Registered right after
// RequestId() so it sees every route. It is a hard no-op when disabled
// (returns before wrapping anything), and never blocks the request: body
// capture is size-bounded and the actual persistence is handed to the
// async auditlog package.
func AuditLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		s := audit_setting.GetAuditSetting()
		if s == nil || !s.Enabled {
			c.Next()
			return
		}
		path := c.Request.URL.Path
		for _, p := range s.PathSkipPrefixes {
			if p != "" && strings.HasPrefix(path, p) {
				c.Next()
				return
			}
		}
		if s.SampleRate < 1.0 && rand.Float64() > s.SampleRate {
			c.Next()
			return
		}

		trace := common.NewRequestTrace()
		common.SetRequestTrace(c, trace)
		trace.Mark("received")
		start := time.Now()

		// --- request body (size-bounded, reuse-safe) ---
		// The gateway already buffers the request body for downstream reuse
		// (BodyStorage), so slicing a bounded prefix here is free — no extra
		// copy of huge uploads. We always clip to MaxBodyBytes and flag
		// truncation; auditlog.capBody applies the same ceiling again.
		var reqBody []byte
		reqTruncated := false
		if s.CaptureRequestBody && hasBody(c.Request.Method) {
			if storage, err := common.GetBodyStorage(c); err == nil {
				if data, bErr := storage.Bytes(); bErr == nil {
					reqBody = clip(data, s.MaxBodyBytes, &reqTruncated)
				}
				// Restore body for any handler that reads c.Request.Body
				// directly; UnmarshalBodyReusable also re-seeks the same
				// cached storage, so downstream parsing is unaffected.
				if _, sErr := storage.Seek(0, io.SeekStart); sErr == nil {
					c.Request.Body = io.NopCloser(storage)
				}
			}
		}

		// --- wrap response ---
		limit := int(s.MaxBodyBytes)
		if limit <= 0 {
			limit = 64 << 10
		}
		cw := &auditCaptureWriter{ResponseWriter: c.Writer, limit: limit}
		c.Writer = cw

		c.Next()

		trace.Mark("responded")

		// If this turned out to be a stream, shrink what we keep to the
		// configured first-chunk window (we already passed everything
		// through to the client; this only bounds what we persist).
		respBody := cw.buf
		if cw.streaming && s.StreamingFirstChunkBytes > 0 && int64(len(respBody)) > s.StreamingFirstChunkBytes {
			respBody = respBody[:s.StreamingFirstChunkBytes]
			cw.truncated = true
		}
		if !s.CaptureResponseBody {
			respBody = nil
		}

		rec := &auditlog.Record{
			CreatedAtMs:       start.UnixMilli(),
			RequestId:         c.GetString(common.RequestIdKey),
			UserId:            common.GetContextKeyInt(c, constant.ContextKeyUserId),
			TokenId:           common.GetContextKeyInt(c, constant.ContextKeyTokenId),
			ChannelId:         common.GetContextKeyInt(c, constant.ContextKeyChannelId),
			Method:            c.Request.Method,
			Path:              path,
			Query:             c.Request.URL.RawQuery,
			StatusCode:        c.Writer.Status(),
			ClientIp:          c.ClientIP(),
			ModelName:         common.GetContextKeyString(c, constant.ContextKeyOriginalModel),
			RequestCT:         c.Request.Header.Get("Content-Type"),
			ResponseCT:        c.Writer.Header().Get("Content-Type"),
			RequestSize:       c.Request.ContentLength,
			RespSize:          int64(c.Writer.Size()),
			LatencyMs:         time.Since(start).Milliseconds(),
			UpstreamLatencyMs: upstreamLatencyFromTrace(trace),
			RequestBody:       reqBody,
			ResponseBody:      respBody,
			Lifecycle:         trace.Spans(),
			Streaming:         cw.streaming,
			Truncated:         reqTruncated || cw.truncated,
		}
		auditlog.Submit(rec)
	}
}

func hasBody(method string) bool {
	switch method {
	case "POST", "PUT", "PATCH", "DELETE":
		return true
	default:
		return false
	}
}

func clip(b []byte, max int64, truncated *bool) []byte {
	if max <= 0 || int64(len(b)) <= max {
		out := make([]byte, len(b))
		copy(out, b)
		return out
	}
	*truncated = true
	out := make([]byte, max)
	copy(out, b[:max])
	return out
}

// upstreamLatencyFromTrace derives the upstream call duration from the
// lifecycle marks the relay path emits (upstream_sent → upstream_done /
// upstream_first_byte). Returns 0 for non-relay routes that never marked.
func upstreamLatencyFromTrace(t *common.RequestTrace) int64 {
	var sent, done int64
	for _, sp := range t.Spans() {
		switch sp.Stage {
		case "upstream_sent":
			sent = sp.AtUnixMs
		case "upstream_done", "upstream_first_byte":
			if done == 0 {
				done = sp.AtUnixMs
			}
		}
	}
	if sent > 0 && done >= sent {
		return done - sent
	}
	return 0
}
