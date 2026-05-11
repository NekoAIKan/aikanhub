// refresh.go — on-demand pull of the latest ARK state for a record.
//
// Design: no background goroutines. Persistence (Submit) and terminal-
// state discovery (RefreshFromUpstream) are independent. A row sits in
// `processing` until somebody queries it; the query path does the one
// ARK call needed to settle it. Terminal rows are never re-queried —
// moderation verdicts are deterministic per content, so once we have
// `active` or `failed` we treat it as authoritative forever.
//
// Best-effort semantics: any infra failure (missing ARK creds, network
// error, upstream non-2xx) returns the input record unchanged with a
// nil error. We never fail a read because of an upstream hiccup; the
// next GET will try again.
package imageaudit

import (
	"context"
	"errors"

	"github.com/QuantumNous/new-api/model"
)

// RefreshFromUpstream asks ARK once for the asset's current state and
// updates the DB row if a terminal verdict came back. Returns the
// latest record either way.
//
// Callers (GET /v1/image-audits/:id, Wait()) should NOT treat a nil
// error as "definitely terminal" — check rec.Status.IsTerminal()
// instead. The function deliberately swallows ARK errors so that a
// transient upstream blip doesn't 500 a perfectly valid status read.
func RefreshFromUpstream(ctx context.Context, rec *model.ImageAuditRecord) (*model.ImageAuditRecord, error) {
	if rec == nil {
		return nil, errors.New("imageaudit: nil record")
	}
	if rec.Status.IsTerminal() {
		return rec, nil
	}
	if rec.AssetID == "" {
		// Synthetic / pre-persist record with no upstream id — nothing
		// to ask ARK about.
		return rec, nil
	}
	cfg := Load()
	if !cfg.HasARK() {
		return rec, nil
	}
	client := newARKClient(cfg)
	res, err := client.getAsset(ctx, rec.AssetID)
	if err != nil {
		// Network / upstream error — best-effort, return what we have.
		return rec, nil
	}

	switch res.Status {
	case "Active":
		rec.Status = model.ImageAuditStatusActive
		if rec.AssetURI == "" {
			rec.AssetURI = "asset://" + rec.AssetID
		}
		// Don't overwrite PublicURL with the signed ARK-internal URL —
		// it expires; we want to keep the original public URL we
		// submitted for diagnostics.
		if err := model.SaveImageAuditRecord(rec); err != nil {
			return rec, nil
		}
	case "Failed":
		reason := string(res.Error)
		if reason == "" {
			reason = "asset audit returned Failed without reason"
		}
		const maxReason = 2048
		if len(reason) > maxReason {
			reason = reason[:maxReason] + "...(truncated)"
		}
		rec.Status = model.ImageAuditStatusFailed
		rec.Reason = reason
		if err := model.SaveImageAuditRecord(rec); err != nil {
			return rec, nil
		}
	default:
		// Still Processing upstream. Return as-is; the next GET will
		// try again.
		return rec, nil
	}
	return rec, nil
}
