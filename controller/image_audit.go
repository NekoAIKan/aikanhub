// Package controller — image_audit.go exposes the /v1/image-audits
// endpoints described in issue #43:
//
//	POST /v1/image-audits        submit one image, returns audit record
//	GET  /v1/image-audits/:id    fetch an audit record by id
//	GET  /v1/image-audits        list the caller's records (paginated)
//
// Async by default: POST returns immediately with status=processing
// (or status=active when a cached record is reused). Clients poll GET
// until the status is terminal. Add `?wait=true` (or body `wait: true`)
// to block synchronously up to ARK_ASSETS_POLL_TIMEOUT — convenient for
// scripts but not recommended for production callers because audit
// latency is uncontrolled.
package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/imageaudit"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// imageAuditCreateRequest is the POST body. `Image` is the only
// required field. We accept both `image` (singular, OpenAI-style) and
// `images[0]` (array) so callers cribbing from /v1/video/generations
// don't have to remember which name.
type imageAuditCreateRequest struct {
	Image  string   `json:"image"`
	Images []string `json:"images,omitempty"`
	Wait   bool     `json:"wait,omitempty"`
}

func (r *imageAuditCreateRequest) imageSource() string {
	if s := strings.TrimSpace(r.Image); s != "" {
		return s
	}
	for _, s := range r.Images {
		if s = strings.TrimSpace(s); s != "" {
			return s
		}
	}
	return ""
}

// imageAuditError is the OpenAI-flavored error envelope. We use the
// raw map (not types.OpenAIError) because we want the `failed_image`
// metadata to be a structured object, not a JSON-encoded string.
func imageAuditError(c *gin.Context, status int, code, message string, extras map[string]any) {
	body := map[string]any{
		"message": message,
		"type":    "image_audit_error",
		"code":    code,
	}
	for k, v := range extras {
		body[k] = v
	}
	c.AbortWithStatusJSON(status, gin.H{"error": body})
}

// CreateImageAudit handles POST /v1/image-audits.
func CreateImageAudit(c *gin.Context) {
	var req imageAuditCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		imageAuditError(c, http.StatusBadRequest, "invalid_request_body",
			"failed to parse request body: "+err.Error(), nil)
		return
	}
	src := req.imageSource()
	if src == "" {
		imageAuditError(c, http.StatusBadRequest, "missing_image",
			"`image` is required (URL or base64 data URI)", nil)
		return
	}
	// asset://<id> is not auditable — the caller already has an audited
	// reference. Tell them so explicitly instead of returning a synthetic
	// non-queryable record_id like external_xxx.
	if strings.HasPrefix(src, "asset://") {
		imageAuditError(c, http.StatusBadRequest, "asset_uri_not_auditable",
			"asset://<id> is already an audited reference; pass an image URL or base64 data URI to create a new audit",
			nil)
		return
	}

	// `?wait=true` query param overrides body field for convenience.
	wait := req.Wait
	if v := strings.ToLower(c.Query("wait")); v == "1" || v == "true" || v == "yes" {
		wait = true
	}

	userID := common.GetContextKeyInt(c, constant.ContextKeyUserId)
	tokenID := common.GetContextKeyInt(c, constant.ContextKeyTokenId)
	if userID <= 0 {
		// Token auth middleware should have populated this; defensive.
		imageAuditError(c, http.StatusUnauthorized, "unauthorized",
			"user context missing", nil)
		return
	}

	rec, err := imageaudit.Submit(c.Request.Context(), src, userID, tokenID)
	if err != nil {
		imageAuditError(c, http.StatusBadGateway, "image_audit_error", err.Error(), nil)
		return
	}

	if wait && !rec.Status.IsTerminal() {
		// Wait blocks until terminal or ARK_ASSETS_POLL_TIMEOUT.
		updated, werr := imageaudit.Wait(c.Request.Context(), rec.RecordID)
		if werr != nil && !errors.Is(werr, gorm.ErrRecordNotFound) {
			// Wait returned the latest record we have; surface as a
			// 504 if we hit timeout, otherwise 502.
			status := http.StatusBadGateway
			code := "image_audit_error"
			if strings.Contains(werr.Error(), "timed out") {
				status = http.StatusGatewayTimeout
				code = "image_audit_wait_timeout"
			}
			if updated != nil {
				rec = updated
			}
			imageAuditError(c, status, code, werr.Error(),
				map[string]any{"record": rec})
			return
		}
		if updated != nil {
			rec = updated
		}
	}

	c.JSON(http.StatusOK, rec)
}

// GetImageAudit handles GET /v1/image-audits/:id.
//
// Two-step lookup, cheap-path first:
//
//  1. Read the DB row scoped to userID. Terminal rows are
//     authoritative; return immediately.
//  2. For a `processing` row, call ARK once to see if it settled. If
//     it did, persist and return the new terminal state; if not,
//     return the still-processing row so the client polls again.
//
// Upstream is the source of truth for state transitions; we never
// keep a long-lived poller. Best-effort: an ARK hiccup here just
// means the client gets back `processing` and tries again later.
func GetImageAudit(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		imageAuditError(c, http.StatusBadRequest, "missing_id", "id required", nil)
		return
	}
	userID := common.GetContextKeyInt(c, constant.ContextKeyUserId)
	if userID <= 0 {
		imageAuditError(c, http.StatusUnauthorized, "unauthorized", "user context missing", nil)
		return
	}
	rec, err := model.GetImageAuditRecordByRecordID(userID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			imageAuditError(c, http.StatusNotFound, "record_not_found",
				"image audit record not found", nil)
			return
		}
		imageAuditError(c, http.StatusInternalServerError, "db_error", err.Error(), nil)
		return
	}
	rec, _ = imageaudit.RefreshFromUpstream(c.Request.Context(), rec)
	c.JSON(http.StatusOK, rec)
}

// ListImageAudits handles GET /v1/image-audits.
//
// Query params:
//
//	page  — 1-based page number, default 1
//	limit — page size, default 20, max 100
func ListImageAudits(c *gin.Context) {
	userID := common.GetContextKeyInt(c, constant.ContextKeyUserId)
	if userID <= 0 {
		imageAuditError(c, http.StatusUnauthorized, "unauthorized", "user context missing", nil)
		return
	}
	page, _ := strconv.Atoi(c.Query("page"))
	limit, _ := strconv.Atoi(c.Query("limit"))

	rows, total, err := model.ListImageAuditRecords(userID, page, limit)
	if err != nil {
		imageAuditError(c, http.StatusInternalServerError, "db_error", err.Error(), nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":  rows,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}
