// Package auditlog is the async sink for the full-request audit subsystem.
//
// Storage model (operator decision — NOT a DB):
//
//	request → middleware captures a *Record → Submit (non-blocking) →
//	single writer goroutine appends one redacted JSON line to the active
//	local file → on size/age threshold the file is sealed (renamed
//	*.sealed) and a new active file opened → an archive goroutine uploads
//	each sealed file to COS as ONE object (never one per request) and, on
//	confirmed upload, deletes the local copy. The active file always stays
//	local. A retention goroutine GCs sealed files per policy.
//
// Nothing here blocks or panics the request path: Submit never blocks
// (queue-full drops with a counter — the file IS the durable store, the
// queue is just handoff), and every file/COS step is failure-isolated.
package auditlog

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service/imageaudit"
	"github.com/QuantumNous/new-api/setting/audit_setting"
)

// Record is the raw capture handed over by the middleware. Bodies are
// already size-bounded by the middleware before submission.
type Record struct {
	CreatedAtMs int64  `json:"created_at_ms"`
	RequestId   string `json:"request_id"`
	UserId      int    `json:"user_id"`
	TokenId     int    `json:"token_id"`
	ChannelId   int    `json:"channel_id"`

	Method     string `json:"method"`
	Path       string `json:"path"`
	Query      string `json:"query,omitempty"`
	StatusCode int    `json:"status_code"`
	ClientIp   string `json:"client_ip,omitempty"`
	ModelName  string `json:"model_name,omitempty"`

	RequestCT   string `json:"request_content_type,omitempty"`
	ResponseCT  string `json:"response_content_type,omitempty"`
	RequestSize int64  `json:"request_size"`
	RespSize    int64  `json:"response_size"`

	LatencyMs         int64 `json:"latency_ms"`
	UpstreamLatencyMs int64 `json:"upstream_latency_ms,omitempty"`

	RequestBody  []byte `json:"-"`
	ResponseBody []byte `json:"-"`
	// Rendered (redacted, capped) string forms actually written.
	RequestBodyStr  string             `json:"request_body,omitempty"`
	ResponseBodyStr string             `json:"response_body,omitempty"`
	Lifecycle       []common.TraceSpan `json:"lifecycle,omitempty"`

	Streaming bool `json:"streaming,omitempty"`
	Truncated bool `json:"truncated,omitempty"`
	Redacted  bool `json:"redacted,omitempty"`
}

const queueCapacity = 8192

var (
	queue       chan *Record
	archiveQ    chan string
	initStarted atomic.Bool
	droppedCnt  atomic.Int64
	// owned exclusively by the writer goroutine — no lock needed.
	cur         *os.File
	curPath     string
	curBytes    int64
	curOpenedAt time.Time
)

const (
	activePrefix = "audit-"
	activeExt    = ".jsonl"
	sealedSuffix = ".sealed"
)

// Init starts the writer, archive and retention goroutines. Idempotent.
func Init() {
	if !initStarted.CompareAndSwap(false, true) {
		return
	}
	queue = make(chan *Record, queueCapacity)
	archiveQ = make(chan string, 256)

	if err := os.MkdirAll(auditDir(), 0o755); err != nil {
		common.SysError("auditlog: cannot create audit dir: " + err.Error())
		// Still start goroutines; writeLine will keep retrying MkdirAll.
	}
	// Seal anything left over from a previous run (a crashed process
	// leaves an un-sealed active file). Treat every pre-existing audit
	// file as sealed so it gets archived rather than appended to.
	for _, p := range listExisting() {
		sealed := p
		if !strings.HasSuffix(p, sealedSuffix) {
			sealed = p + sealedSuffix
			if err := os.Rename(p, sealed); err != nil {
				continue
			}
		}
		enqueueArchive(sealed)
	}

	go writerLoop()
	go archiveLoop()
	go retentionLoop()
	common.SysLog("auditlog: file writer started (dir=" + auditDir() + ")")
}

// Submit hands a record to the writer without ever blocking the caller.
func Submit(rec *Record) {
	if rec == nil || !initStarted.Load() {
		return
	}
	select {
	case queue <- rec:
	default:
		droppedCnt.Add(1)
	}
}

// DroppedCount exposes the queue-overflow counter for ops/metrics.
func DroppedCount() int64 { return droppedCnt.Load() }

func auditDir() string {
	base := "./logs"
	if common.LogDir != nil && *common.LogDir != "" {
		base = *common.LogDir
	}
	return filepath.Join(base, "request-audit")
}

func listExisting() []string {
	entries, err := os.ReadDir(auditDir())
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasPrefix(n, activePrefix) && (strings.HasSuffix(n, activeExt) || strings.HasSuffix(n, sealedSuffix)) {
			out = append(out, filepath.Join(auditDir(), n))
		}
	}
	sort.Strings(out)
	return out
}

// --- writer goroutine (sole owner of cur*) ---

func writerLoop() {
	defer func() {
		if r := recover(); r != nil {
			common.SysError(fmt.Sprintf("auditlog: writer panic recovered: %v", r))
			go writerLoop()
		}
	}()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case rec := <-queue:
			writeLine(rec)
		case <-ticker.C:
			if rotateDueByTime() {
				rotate()
			}
		}
	}
}

func writeLine(rec *Record) {
	s := audit_setting.GetAuditSetting()
	prepareBodies(rec, s)

	data, err := common.Marshal(rec)
	if err != nil {
		return
	}
	if cur == nil {
		if !openActive() {
			droppedCnt.Add(1)
			return
		}
	}
	if curBytes+int64(len(data))+1 > s.RotateMaxBytes || rotateDueByTime() {
		rotate()
		if cur == nil && !openActive() {
			droppedCnt.Add(1)
			return
		}
	}
	n, werr := cur.Write(append(data, '\n'))
	if werr != nil {
		common.SysError("auditlog: write failed: " + werr.Error())
		_ = cur.Close()
		cur = nil
		droppedCnt.Add(1)
		return
	}
	curBytes += int64(n)
}

// prepareBodies applies redaction + the MaxBodyBytes cap, producing the
// string forms that are actually serialized.
func prepareBodies(rec *Record, s *audit_setting.AuditSetting) {
	rb, resp := rec.RequestBody, rec.ResponseBody
	if s.Redact {
		if len(rb) > 0 {
			rb = redactBody(rb, s.RedactKeys)
		}
		if len(resp) > 0 {
			resp = redactBody(resp, s.RedactKeys)
		}
		rec.Redacted = true
	}
	rec.RequestBodyStr = capBody(rb, s.MaxBodyBytes, &rec.Truncated)
	rec.ResponseBodyStr = capBody(resp, s.MaxBodyBytes, &rec.Truncated)
	rec.RequestBody, rec.ResponseBody = nil, nil
}

func capBody(b []byte, max int64, truncated *bool) string {
	if len(b) == 0 {
		return ""
	}
	if max <= 0 || int64(len(b)) <= max {
		return string(b)
	}
	*truncated = true
	return string(b[:max]) + fmt.Sprintf("\n[truncated: %d of %d bytes]", max, len(b))
}

func rotateDueByTime() bool {
	s := audit_setting.GetAuditSetting()
	if cur == nil || s.RotateIntervalMinutes <= 0 {
		return false
	}
	return time.Since(curOpenedAt) >= time.Duration(s.RotateIntervalMinutes)*time.Minute
}

func openActive() bool {
	if err := os.MkdirAll(auditDir(), 0o755); err != nil {
		common.SysError("auditlog: mkdir failed: " + err.Error())
		return false
	}
	name := activePrefix + time.Now().UTC().Format("20060102-150405.000") + activeExt
	p := filepath.Join(auditDir(), name)
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		common.SysError("auditlog: open active failed: " + err.Error())
		return false
	}
	cur, curPath, curBytes, curOpenedAt = f, p, 0, time.Now()
	return true
}

func rotate() {
	if cur == nil {
		return
	}
	_ = cur.Sync()
	_ = cur.Close()
	cur = nil
	sealed := curPath + sealedSuffix
	if err := os.Rename(curPath, sealed); err != nil {
		common.SysError("auditlog: seal rename failed: " + err.Error())
		return
	}
	enqueueArchive(sealed)
	openActive()
}

// --- archive goroutine ---

func enqueueArchive(path string) {
	select {
	case archiveQ <- path:
	default:
		// Archive queue saturated — the file stays on disk; the next
		// retentionLoop pass re-enqueues unarchived sealed files.
	}
}

func archiveLoop() {
	defer func() {
		if r := recover(); r != nil {
			common.SysError(fmt.Sprintf("auditlog: archive panic recovered: %v", r))
			go archiveLoop()
		}
	}()
	for path := range archiveQ {
		archiveOne(path)
	}
}

func archiveOne(path string) {
	s := audit_setting.GetAuditSetting()
	if !s.ArchiveToCOS || !imageaudit.COSConfigured() {
		return // sealed file stays local; retentionLoop handles GC
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return // gone already, or unreadable; nothing to do
	}
	key := "request-audit/" + time.Now().UTC().Format("2006/01/02") + "/" + filepath.Base(path)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if _, upErr := imageaudit.PutObject(ctx, key, data, "application/x-ndjson"); upErr != nil {
		common.SysError("auditlog: COS archive failed (kept local, will retry): " + upErr.Error())
		return
	}
	// Durable copy is in COS now — drop the local sealed file.
	if rmErr := os.Remove(path); rmErr != nil {
		common.SysError("auditlog: archived to COS but local remove failed: " + rmErr.Error())
	}
}

// --- retention goroutine ---

func retentionLoop() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		s := audit_setting.GetAuditSetting()
		// Re-drive any sealed files that never made it to COS.
		for _, p := range listExisting() {
			if strings.HasSuffix(p, sealedSuffix) {
				enqueueArchive(p)
			}
		}
		if s.LocalRetentionHours <= 0 {
			continue
		}
		cutoff := time.Now().Add(-time.Duration(s.LocalRetentionHours) * time.Hour)
		for _, p := range listExisting() {
			if !strings.HasSuffix(p, sealedSuffix) {
				continue // never GC the active file
			}
			fi, err := os.Stat(p)
			if err != nil || fi.ModTime().After(cutoff) {
				continue
			}
			// When archiving to COS, only GC files that are already gone
			// from disk via archiveOne; a still-present sealed file here
			// means archive hasn't confirmed — keep it (durability >
			// disk). When ArchiveToCOS is off, GC by age is the policy.
			if s.ArchiveToCOS && imageaudit.COSConfigured() {
				continue
			}
			if rmErr := os.Remove(p); rmErr != nil {
				common.SysError("auditlog: retention remove failed: " + rmErr.Error())
			}
		}
	}
}
