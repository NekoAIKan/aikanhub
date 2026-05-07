package pixverse

const (
	ChannelName = "pixverse"

	// API endpoints (relative to channel base URL).
	// Docs: https://docs.platform.pixverse.ai/
	endpointTextToVideo  = "/openapi/v2/video/text/generate"
	endpointImageToVideo = "/openapi/v2/video/img/generate"
	endpointTransition   = "/openapi/v2/video/transition/generate"
	endpointFusion       = "/openapi/v2/video/fusion/generate"
	endpointImageUpload  = "/openapi/v2/image/upload"
	endpointVideoResult  = "/openapi/v2/video/result/" // append video_id

	// Default request values.
	defaultModel       = "v4.5"
	defaultDuration    = 5
	defaultQuality     = "540p"
	defaultAspectRatio = "16:9"
	defaultMotionMode  = "normal"
)

// Pixverse video.status values.
//
//	1: Generation successful
//	5: Generating
//	6: Deleted
//	7: Contents moderation failed
//	8: Generation failed
const (
	statusSuccess           = 1
	statusGenerating        = 5
	statusDeleted           = 6
	statusModerationFailed  = 7
	statusGenerationFailure = 8
)

// ModelList is what we expose to the frontend channel "fetch models" UI.
//
// Pixverse prices vary per (model × quality × duration); since the gateway's
// `ModelPrice` map keys per model name, each priceable SKU is its own entry.
// The grammar parsed by parseModelSpec:
//
//	pixverse-{ver}                  → defaults (540p, 5s)
//	pixverse-{ver}-{quality}        → e.g. "pixverse-v4.5-720p"  (5s)
//	pixverse-{ver}-{duration}s      → e.g. "pixverse-v4.5-8s"    (540p, 8s)
//	pixverse-{ver}-{quality}-{n}s   → e.g. "pixverse-v4.5-1080p-8s"
//
// When the suffix pins quality/duration, the adapter overrides whatever the
// client passes so billing always matches what's submitted upstream.
var ModelList = []string{
	// v3.5 / v4 / v4.5 / v5 — share the same pricing tier.
	"pixverse-v3.5", "pixverse-v3.5-720p", "pixverse-v3.5-1080p",
	"pixverse-v3.5-8s", "pixverse-v3.5-720p-8s", "pixverse-v3.5-1080p-8s",
	"pixverse-v4", "pixverse-v4-720p", "pixverse-v4-1080p",
	"pixverse-v4-8s", "pixverse-v4-720p-8s", "pixverse-v4-1080p-8s",
	"pixverse-v4.5", "pixverse-v4.5-720p", "pixverse-v4.5-1080p",
	"pixverse-v4.5-8s", "pixverse-v4.5-720p-8s", "pixverse-v4.5-1080p-8s",
	"pixverse-v5", "pixverse-v5-720p", "pixverse-v5-1080p",
	"pixverse-v5-8s", "pixverse-v5-720p-8s", "pixverse-v5-1080p-8s",

	// v5.5 / v5.6 — only 5s priced officially.
	"pixverse-v5.5", "pixverse-v5.5-720p", "pixverse-v5.5-1080p",
	"pixverse-v5.6", "pixverse-v5.6-720p", "pixverse-v5.6-1080p",

	// c1 / v6 — per-second billing; one base entry, configure with a
	// custom expression or use self-use mode for now.
	"pixverse-c1",
	"pixverse-v6",
}
