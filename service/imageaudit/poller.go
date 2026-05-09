// poller.go — background goroutine that polls ARK GetAsset for an
// in-flight audit and persists the terminal state to the DB.
//
// One poller per Submit() call. Bounded process-wide by maxConcurrentPollers
// to keep ARK request volume sane during bursty traffic. If the cap is
// exceeded a poller falls back to inline (synchronous) polling — slower
// but never drops audits.
//
// Restart resilience: BootstrapResumePollers() at process start scans
// the DB for processing rows, marks anything past the timeout as
// failed (orphan_poller), and re-spawns pollers for the rest. Without
// this, a deploy mid-audit would leave rows stuck in processing forever.
package imageaudit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// maxConcurrentPollers caps in-flight goroutines. Each poller does one
// HTTP call per ARKPollInterval (default 3s) so 50 concurrent → 17 RPS
// to ARK CreateAsset's GetAsset endpoint, well below the documented
// limits. Tune via env if needed.
const defaultMaxConcurrentPollers = 50

var (
	pollerSem chan struct{}
	pollerMu  sync.Mutex
)

func ensurePollerSem() chan struct{} {
	pollerMu.Lock()
	defer pollerMu.Unlock()
	if pollerSem == nil {
		max := common.GetEnvOrDefault("IMAGE_AUDIT_MAX_POLLERS", defaultMaxConcurrentPollers)
		if max <= 0 {
			max = defaultMaxConcurrentPollers
		}
		pollerSem = make(chan struct{}, max)
	}
	return pollerSem
}

// spawnPoller launches the background poller for a record. Always
// non-blocking — if the concurrency cap is hit the work is done
// inline by the caller's caller so the audit still completes.
func spawnPoller(recordID, assetID string, interval, timeout time.Duration) {
	sem := ensurePollerSem()
	select {
	case sem <- struct{}{}:
		go func() {
			defer func() { <-sem }()
			runPoller(recordID, assetID, interval, timeout)
		}()
	default:
		// Cap exceeded — fall back to inline. Wait blocks the calling
		// HTTP handler but avoids losing the row.
		common.SysLog(fmt.Sprintf("imageaudit: poller cap reached, polling inline (record=%s)", recordID))
		runPoller(recordID, assetID, interval, timeout)
	}
}

// runPoller drives one record from processing → terminal. Always
// updates the DB, even on infra errors (status=failed, reason=infra
// message) — leaving a row stuck in processing is the worst outcome.
func runPoller(recordID, assetID string, interval, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout+time.Second)
	defer cancel()

	cfg := Load()
	if !cfg.HasARK() {
		// We got here from Submit() which checked HasARK already, so
		// this only fires if the env was reset between submit and
		// poll. Mark failed so the row doesn't dangle.
		markFailed(recordID, assetID, "ARK credentials missing at poll time")
		return
	}
	client := newARKClient(cfg)

	if interval <= 0 {
		interval = 3 * time.Second
	}
	deadline := time.Now().Add(timeout)

	for {
		res, err := client.getAsset(ctx, assetID)
		if err != nil {
			// Infra error (network, 5xx). Record but keep polling
			// until deadline — these are usually transient.
			common.SysLog(fmt.Sprintf("imageaudit: GetAsset error (record=%s asset=%s): %v",
				recordID, assetID, err))
			if time.Now().After(deadline) {
				markFailed(recordID, assetID, fmt.Sprintf("GetAsset failed: %v", err))
				return
			}
			if !sleepCtx(ctx, interval) {
				return
			}
			continue
		}

		switch res.Status {
		case "Active":
			markActive(recordID, assetID, res.URL)
			return
		case "Failed":
			reason := string(res.Error)
			if reason == "" {
				reason = "asset audit returned Failed without reason"
			}
			markFailed(recordID, assetID, reason)
			return
		}

		if time.Now().After(deadline) {
			markFailed(recordID, assetID,
				fmt.Sprintf("audit poll timeout after %s (last status=%s)", timeout, res.Status))
			return
		}
		// Touch the row so the startup sweeper can tell live pollers
		// from orphans by updated_at freshness.
		touchProcessing(recordID)
		if !sleepCtx(ctx, interval) {
			return
		}
	}
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

// markActive transitions to active and stores the (signed, transient)
// upstream URL ARK returned. Public callers see AssetURI, not this.
func markActive(recordID, assetID, upstreamURL string) {
	r, err := model.GetImageAuditRecordByRecordID(0, recordID)
	if err != nil {
		common.SysLog(fmt.Sprintf("imageaudit: markActive lookup failed (record=%s): %v", recordID, err))
		return
	}
	r.Status = model.ImageAuditStatusActive
	r.AssetID = assetID
	r.AssetURI = "asset://" + assetID
	// Don't overwrite PublicURL with the signed ARK-internal URL —
	// keep the original public URL we submitted, since the signed one
	// expires.
	if err := model.SaveImageAuditRecord(r); err != nil {
		common.SysLog(fmt.Sprintf("imageaudit: markActive save failed (record=%s): %v", recordID, err))
	}
}

func markFailed(recordID, assetID, reason string) {
	r, err := model.GetImageAuditRecordByRecordID(0, recordID)
	if err != nil {
		common.SysLog(fmt.Sprintf("imageaudit: markFailed lookup failed (record=%s): %v", recordID, err))
		return
	}
	r.Status = model.ImageAuditStatusFailed
	if assetID != "" && r.AssetID == "" {
		r.AssetID = assetID
	}
	if reason != "" {
		// Cap reason length so a verbose ARK error doesn't blow up the
		// row. Most real reasons are short.
		const maxReason = 2048
		if len(reason) > maxReason {
			reason = reason[:maxReason] + "...(truncated)"
		}
		r.Reason = reason
	}
	if err := model.SaveImageAuditRecord(r); err != nil {
		common.SysLog(fmt.Sprintf("imageaudit: markFailed save failed (record=%s): %v", recordID, err))
	}
}

// touchProcessing bumps UpdatedAt without changing status. The
// sweeper uses staleness as the orphan signal, so a live poller must
// keep the heartbeat fresh.
func touchProcessing(recordID string) {
	r, err := model.GetImageAuditRecordByRecordID(0, recordID)
	if err != nil {
		return
	}
	if r.Status != model.ImageAuditStatusProcessing {
		return
	}
	_ = model.SaveImageAuditRecord(r)
}

// BootstrapResumePollers is called from main() after DB init. Walks
// processing rows: anything past the audit timeout is marked failed
// (orphan); everything else gets a fresh poller. Idempotent — safe to
// call multiple times during tests.
func BootstrapResumePollers() {
	cfg := Load()
	if !cfg.HasARK() {
		return
	}
	cutoff := time.Now().Add(-cfg.ARKPollTimeout)
	rows, err := model.FindStaleProcessingImageAudits(time.Now())
	if err != nil {
		common.SysLog(fmt.Sprintf("imageaudit: bootstrap list failed: %v", err))
		return
	}
	resumed := 0
	orphaned := 0
	for _, r := range rows {
		if r.UpdatedAt < cutoff.Unix() {
			markFailed(r.RecordID, r.AssetID, "poller orphaned by service restart past audit timeout")
			orphaned++
			continue
		}
		spawnPoller(r.RecordID, r.AssetID, cfg.ARKPollInterval, cfg.ARKPollTimeout)
		resumed++
	}
	if resumed+orphaned > 0 {
		common.SysLog(fmt.Sprintf("imageaudit: bootstrap resumed %d / orphaned %d processing audits",
			resumed, orphaned))
	}
}
