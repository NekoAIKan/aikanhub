package common

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// KeyRequestTrace is the gin-context key under which a *RequestTrace lives
// for the lifetime of one request. The audit middleware seeds it; any code
// on the request path may append spans via TraceMark without coupling to
// the audit subsystem (TraceMark is a no-op when no trace is attached, so
// callers never need a nil check or a feature-flag guard).
const KeyRequestTrace = "key_request_trace"

// TraceSpan is one named lifecycle checkpoint. AtUnixMs is wall-clock ms so
// it serialises compactly and survives a JSON round-trip into the audit row.
type TraceSpan struct {
	Stage    string `json:"stage"`
	AtUnixMs int64  `json:"at_unix_ms"`
}

// RequestTrace accumulates ordered lifecycle spans for a single request.
// It is mutated from at most a few goroutines (the request goroutine plus
// the relay copy goroutine), so appends are mutex-guarded.
type RequestTrace struct {
	mu    sync.Mutex
	start time.Time
	spans []TraceSpan
}

// NewRequestTrace starts a trace whose elapsed offsets are measured from
// now. The audit middleware calls this once, before c.Next().
func NewRequestTrace() *RequestTrace {
	return &RequestTrace{start: time.Now()}
}

// Mark appends a span. Safe for concurrent use. Duplicate stage names are
// allowed (e.g. retries re-marking "upstream_sent") — the timeline keeps
// every occurrence in order.
func (t *RequestTrace) Mark(stage string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.spans = append(t.spans, TraceSpan{Stage: stage, AtUnixMs: time.Now().UnixMilli()})
	t.mu.Unlock()
}

// Spans returns a copy of the accumulated timeline.
func (t *RequestTrace) Spans() []TraceSpan {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]TraceSpan, len(t.spans))
	copy(out, t.spans)
	return out
}

// SetRequestTrace attaches a trace to the gin context.
func SetRequestTrace(c *gin.Context, t *RequestTrace) {
	if c == nil || t == nil {
		return
	}
	c.Set(KeyRequestTrace, t)
}

// GetRequestTrace returns the attached trace or nil.
func GetRequestTrace(c *gin.Context) *RequestTrace {
	if c == nil {
		return nil
	}
	if v, ok := c.Get(KeyRequestTrace); ok && v != nil {
		if t, ok := v.(*RequestTrace); ok {
			return t
		}
	}
	return nil
}

// TraceMark is the ergonomic entry point for code anywhere on the request
// path: TraceMark(c, "upstream_sent"). It is a no-op when audit logging is
// disabled or no trace is attached, so call sites stay flag-free.
func TraceMark(c *gin.Context, stage string) {
	GetRequestTrace(c).Mark(stage)
}
