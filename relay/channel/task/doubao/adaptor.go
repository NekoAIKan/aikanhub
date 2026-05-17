package doubao

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/imageaudit"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/samber/lo"
)

// ============================
// Request / Response structures
// ============================

type ContentItem struct {
	Type     string    `json:"type,omitempty"`
	Text     string    `json:"text,omitempty"`
	ImageURL *MediaURL `json:"image_url,omitempty"`
	VideoURL *MediaURL `json:"video_url,omitempty"`
	AudioURL *MediaURL `json:"audio_url,omitempty"`
	Role     string    `json:"role,omitempty"`
}

type MediaURL struct {
	URL string `json:"url,omitempty"`
}

type requestPayload struct {
	Model                 string         `json:"model"`
	Content               []ContentItem  `json:"content,omitempty"`
	CallbackURL           string         `json:"callback_url,omitempty"`
	ReturnLastFrame       *dto.BoolValue `json:"return_last_frame,omitempty"`
	ServiceTier           string         `json:"service_tier,omitempty"`
	ExecutionExpiresAfter *dto.IntValue  `json:"execution_expires_after,omitempty"`
	GenerateAudio         *dto.BoolValue `json:"generate_audio,omitempty"`
	Draft                 *dto.BoolValue `json:"draft,omitempty"`
	Tools                 []struct {
		Type string `json:"type,omitempty"`
	} `json:"tools,omitempty"`
	Resolution  string         `json:"resolution,omitempty"`
	Ratio       string         `json:"ratio,omitempty"`
	Duration    *dto.IntValue  `json:"duration,omitempty"`
	Frames      *dto.IntValue  `json:"frames,omitempty"`
	Seed        *dto.IntValue  `json:"seed,omitempty"`
	CameraFixed *dto.BoolValue `json:"camera_fixed,omitempty"`
	Watermark   *dto.BoolValue `json:"watermark,omitempty"`
}

type responsePayload struct {
	ID string `json:"id"` // task_id
}

type responseTask struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Status  string `json:"status"`
	Content struct {
		VideoURL string `json:"video_url"`
	} `json:"content"`
	Seed            int    `json:"seed"`
	Resolution      string `json:"resolution"`
	Duration        int    `json:"duration"`
	Ratio           string `json:"ratio"`
	FramesPerSecond int    `json:"framespersecond"`
	ServiceTier     string `json:"service_tier"`
	Tools           []struct {
		Type string `json:"type"`
	} `json:"tools"`
	Usage struct {
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
		ToolUsage        struct {
			WebSearch int `json:"web_search"`
		} `json:"tool_usage"`
	} `json:"usage"`
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
}

// ============================
// Adaptor implementation
// ============================

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = info.ChannelBaseUrl
	a.apiKey = info.ApiKey
}

// ValidateRequestAndSetAction parses body, validates fields and sets the
// action label that will surface in the task-log UI.
//
// Volcano's content[] doesn't have a single "kind" field, so we infer:
//
//   - first_frame + last_frame  → firstTailGenerate (首尾生视频)
//   - any video_url             → referenceGenerate (参照生视频, covers
//     multi-modal / edit / extend)
//   - any image_url             → generate (图生视频)
//   - text only                 → textGenerate (文生视频)
//
// Falling back to plain "generate" mislabels every text-only and
// multi-modal task as "图生视频" in the dashboard.
func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) (taskErr *dto.TaskError) {
	if taskErr := relaycommon.ValidateBasicTaskRequest(c, info, inferAction(c)); taskErr != nil {
		return taskErr
	}
	// After the common validator parses the body (model/prompt checks
	// intact, req stored in context), capture Volc Ark native top-level
	// fields that TaskSubmitReq doesn't declare. We fold them into
	// req.Metadata so convertToRequestPayload's UnmarshalMetadata picks
	// them up via the JSON tags on requestPayload. Without this, an
	// OpenAI-shape `/v1/videos` client that flat-lists `seed`,
	// `watermark`, `camera_fixed`, etc. gets them silently dropped
	// during JSON parse — see
	// https://github.com/NekoAIKan/aikanhub/issues/63. Volc-native callers
	// hitting `/api/v3/contents/generations/tasks` are already covered by
	// the volcArkSubmitConvert middleware (which folds the entire raw
	// body into metadata), so this only matters for OpenAI-shape entry.
	if err := foldVolcRootFieldsIntoMetadata(c); err != nil {
		return service.TaskErrorWrapper(err, "fold_root_fields_failed", http.StatusBadRequest)
	}
	return nil
}

// volcNativeRootFields enumerates Volc Ark video-task root fields that
// `requestPayload` accepts but `relaycommon.TaskSubmitReq` doesn't declare.
// Kept as an explicit whitelist (not "everything not in TaskSubmitReq") so
// future Volc additions don't get implicitly forwarded — adding a new field
// is a deliberate one-line change here, ratcheted by the corresponding
// requestPayload struct tag.
//
// Excluded on purpose:
//   - safety_identifier: OpenAI privacy concept, stripped at the channel-
//     settings layer (`relay/common.applyHeaderOverrides`); not a Volc
//     field.
//   - resolution / ratio / duration / model / prompt / images / content:
//     already declared on TaskSubmitReq, parsed natively.
var volcNativeRootFields = []string{
	"callback_url",
	"return_last_frame",
	"service_tier",
	"execution_expires_after",
	"generate_audio",
	"draft",
	"tools",
	"frames",
	"seed",
	"camera_fixed",
	"watermark",
}

// foldVolcRootFieldsIntoMetadata copies the Volc Ark root fields listed
// in volcNativeRootFields from the raw request body into req.Metadata,
// preserving caller-set metadata values (metadata wins over root). Stores
// the mutated req back into the gin context so the downstream
// convertToRequestPayload path observes the merge.
func foldVolcRootFieldsIntoMetadata(c *gin.Context) error {
	var raw map[string]interface{}
	if err := common.UnmarshalBodyReusable(c, &raw); err != nil {
		return fmt.Errorf("re-parse body for root-field fold: %w", err)
	}
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return fmt.Errorf("retrieve stored task request: %w", err)
	}
	if req.Metadata == nil {
		req.Metadata = map[string]interface{}{}
	}
	mutated := false
	for _, key := range volcNativeRootFields {
		v, present := raw[key]
		if !present {
			continue
		}
		if _, already := req.Metadata[key]; already {
			// Metadata-set value wins so explicit `metadata.seed=2`
			// overrides root `seed=1`.
			continue
		}
		req.Metadata[key] = v
		mutated = true
	}
	if mutated {
		c.Set("task_request", req)
	}
	return nil
}

func inferAction(c *gin.Context) string {
	var raw map[string]interface{}
	if err := common.UnmarshalBodyReusable(c, &raw); err != nil {
		return constant.TaskActionGenerate
	}

	// Top-level images[] or singular image (legacy field) imply image input.
	hasImage := false
	if imgs, ok := raw["images"].([]interface{}); ok && len(imgs) > 0 {
		hasImage = true
	}
	if img, ok := raw["image"].(string); ok && strings.TrimSpace(img) != "" {
		hasImage = true
	}

	hasVideo := false
	hasFirstFrame := false
	hasLastFrame := false

	walk := func(items []interface{}) {
		for _, item := range items {
			m, _ := item.(map[string]interface{})
			if m == nil {
				continue
			}
			t, _ := m["type"].(string)
			role, _ := m["role"].(string)
			switch t {
			case "image_url":
				hasImage = true
				switch role {
				case "first_frame":
					hasFirstFrame = true
				case "last_frame":
					hasLastFrame = true
				}
			case "video_url":
				hasVideo = true
			}
		}
	}

	if content, ok := raw["content"].([]interface{}); ok {
		walk(content)
	}
	if meta, ok := raw["metadata"].(map[string]interface{}); ok {
		if content, ok := meta["content"].([]interface{}); ok {
			walk(content)
		}
	}

	switch {
	case hasFirstFrame && hasLastFrame:
		return constant.TaskActionFirstTailGenerate
	case hasVideo:
		return constant.TaskActionReferenceGenerate
	case hasImage:
		return constant.TaskActionGenerate
	default:
		return constant.TaskActionTextGenerate
	}
}

// BuildRequestURL constructs the upstream URL.
func (a *TaskAdaptor) BuildRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	return fmt.Sprintf("%s/api/v3/contents/generations/tasks", a.baseURL), nil
}

// BuildRequestHeader sets required headers.
func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, _ *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	return nil
}

// BuildRequestBody converts request into Doubao specific format.
func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}

	body, err := a.convertToRequestPayload(&req)
	if err != nil {
		return nil, errors.Wrap(err, "convert request payload failed")
	}

	// audit_image=true → run every image_url through ARK Assets audit before
	// submitting upstream. Each URL becomes asset://<id>. Unset / false keeps
	// the original passthrough behaviour.
	if shouldAuditImages(req.Metadata) {
		if err := a.auditImageContent(c, info, body); err != nil {
			return nil, err
		}
	}

	if info.IsModelMapped {
		body.Model = info.UpstreamModelName
	} else {
		info.UpstreamModelName = body.Model
	}
	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

// shouldAuditImages reports whether the caller asked for the audit pipeline.
// The flag is read from req.Metadata so it works for both request shapes:
//
//   - OpenAI-form  POST /v1/videos with `metadata.audit_image: true`
//   - Volcano-form POST /api/v3/contents/generations/tasks with `audit_image: true`
//     at the top level (the volcArkSubmitConvert middleware copies the entire
//     raw body into metadata, so the flag is reachable here either way).
//
// Truthy values: bool true, "true", "1", "yes", "on" (case-insensitive).
// Anything else (including absent) → false → original passthrough.
func shouldAuditImages(metadata map[string]any) bool {
	if metadata == nil {
		return false
	}
	switch v := metadata["audit_image"].(type) {
	case bool:
		return v
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1", "yes", "on":
			return true
		}
	case float64:
		return v != 0
	case int:
		return v != 0
	}
	return false
}

// auditImageContent walks body.Content, replaces every image_url URL with the
// audited asset:// URI, and short-circuits on the first audit failure. The
// returned error wraps an *imageaudit.PerImageAuditError when a specific image
// was rejected, so the upstream error envelope can identify which image
// (index, role, source) needs replacing.
//
// Dedup happens at the imageaudit.Submit layer via the (user_id, source_hash)
// composite index, so identical images across replicas share one audit.
func (a *TaskAdaptor) auditImageContent(c *gin.Context, info *relaycommon.RelayInfo, body *requestPayload) error {
	ctx := c.Request.Context()
	// Route audit to the right region. BytePlus海外 channels use a separate
	// BytePlus Asset library (project / AK / SK / host); China Doubao
	// channels stay on Volcano Ark. Asset URIs from one region are
	// invalid on the other, so cross-region cache hits must be avoided —
	// handled inside imageaudit.Submit via project-scoped cache lookup.
	region := imageaudit.RegionCN
	if info.ChannelType == constant.ChannelTypeBytePlusVideo {
		region = imageaudit.RegionGlobal
	}
	for i := range body.Content {
		item := &body.Content[i]
		if item.Type != "image_url" || item.ImageURL == nil || item.ImageURL.URL == "" {
			continue
		}
		audited, err := imageaudit.EnsureAudited(ctx, item.ImageURL.URL, info.UserId, info.TokenId, region)
		if err != nil {
			perImg := &imageaudit.PerImageAuditError{
				Index:  i,
				Role:   item.Role,
				Source: item.ImageURL.URL,
				Err:    err,
			}
			if imageaudit.IsAuditFailure(err) {
				return errors.Wrap(perImg, "image_audit_failed")
			}
			return errors.Wrap(perImg, "image_audit_error")
		}
		item.ImageURL.URL = audited
	}
	return nil
}

// DoRequest delegates to common helper.
func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

// DoResponse handles upstream response, returns taskID etc.
func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	_ = resp.Body.Close()

	// Parse Doubao response
	var dResp responsePayload
	if err := common.Unmarshal(responseBody, &dResp); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
		return
	}

	if dResp.ID == "" {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("task_id is empty"), "invalid_response", http.StatusInternalServerError)
		return
	}

	ov := dto.NewOpenAIVideo()
	ov.ID = info.PublicTaskID
	ov.TaskID = info.PublicTaskID
	ov.CreatedAt = time.Now().Unix()
	ov.Model = info.OriginModelName

	c.JSON(http.StatusOK, ov)
	return dResp.ID, responseBody, nil
}

// FetchTask fetch task status
func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}

	uri := fmt.Sprintf("%s/api/v3/contents/generations/tasks/%s", baseUrl, taskID)

	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

// looksLikeAspectRatio reports whether s is shaped like "X:Y" with positive
// integer parts (e.g. "16:9", "9:16", "1:1"). Volc Ark's `ratio` accepts this
// form; "WxH" pixel sizes go to `resolution` instead.
func looksLikeAspectRatio(s string) bool {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return false
	}
	a, errA := strconv.Atoi(strings.TrimSpace(parts[0]))
	b, errB := strconv.Atoi(strings.TrimSpace(parts[1]))
	return errA == nil && errB == nil && a > 0 && b > 0
}

func (a *TaskAdaptor) convertToRequestPayload(req *relaycommon.TaskSubmitReq) (*requestPayload, error) {
	r := requestPayload{
		Model:   req.Model,
		Content: []ContentItem{},
	}

	// Add images if present
	if req.HasImage() {
		for _, imgURL := range req.Images {
			r.Content = append(r.Content, ContentItem{
				Type: "image_url",
				ImageURL: &MediaURL{
					URL: imgURL,
				},
			})
		}
	}

	metadata := req.Metadata
	if err := taskcommon.UnmarshalMetadata(metadata, &r); err != nil {
		return nil, errors.Wrap(err, "unmarshal metadata failed")
	}

	// Honour top-level passthrough fields when metadata didn't already set them.
	// OpenAI Videos clients put `resolution`, `ratio`/`aspect_ratio` and numeric
	// `duration` at the root of the request body; without this fallback the
	// upstream Volcano Ark API receives nothing and silently substitutes its
	// defaults (720p, 5s, ratio=auto). Some clients also overload `size` with
	// either a "WxH" pixel pair (resolution) or an "X:Y" aspect ratio — accept
	// both forms.
	if r.Resolution == "" && req.Resolution != "" {
		r.Resolution = req.Resolution
	}
	if r.Ratio == "" {
		if req.Ratio != "" {
			r.Ratio = req.Ratio
		} else if req.AspectRatio != "" {
			r.Ratio = req.AspectRatio
		}
	}
	if size := strings.TrimSpace(req.Size); size != "" {
		if looksLikeAspectRatio(size) {
			if r.Ratio == "" {
				r.Ratio = size
			}
		} else if r.Resolution == "" {
			r.Resolution = size
		}
	}
	if r.Duration == nil {
		// Forward any non-zero duration as-is, INCLUDING the documented
		// `-1` "智能时长 / let the model decide the total duration"
		// sentinel. The old `> 0` guard silently swallowed `-1`, so the
		// top-level (OpenAI-shape) entry never sent it upstream and Volc
		// fell back to its fixed default duration — adaptive duration
		// appeared to "not be attached". 0 still means "unset" (omit,
		// upstream default). Numeric or string ("-1"/"5") both work
		// because TaskSubmitReq.UnmarshalJSON already normalised them.
		// See https://github.com/NekoAIKan/aikanhub/issues/68.
		if req.Duration != 0 {
			r.Duration = lo.ToPtr(dto.IntValue(req.Duration))
		} else if sec, err := strconv.Atoi(strings.TrimSpace(req.Seconds)); err == nil && sec != 0 {
			r.Duration = lo.ToPtr(dto.IntValue(sec))
		}
	}

	r.Content = lo.Reject(r.Content, func(c ContentItem, _ int) bool { return c.Type == "text" })
	r.Content = append(r.Content, ContentItem{
		Type: "text",
		Text: req.Prompt,
	})

	return &r, nil
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	resTask := responseTask{}
	if err := common.Unmarshal(respBody, &resTask); err != nil {
		return nil, errors.Wrap(err, "unmarshal task result failed")
	}

	taskResult := relaycommon.TaskInfo{
		Code: 0,
	}

	// Map Doubao status to internal status
	switch resTask.Status {
	case "pending", "queued":
		taskResult.Status = model.TaskStatusQueued
		taskResult.Progress = "10%"
	case "processing", "running":
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "50%"
	case "succeeded":
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Progress = "100%"
		taskResult.Url = resTask.Content.VideoURL
		// 解析 usage 信息用于按倍率计费
		taskResult.CompletionTokens = resTask.Usage.CompletionTokens
		taskResult.TotalTokens = resTask.Usage.TotalTokens
	case "failed":
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = "100%"
		taskResult.Reason = resTask.Error.Message
	default:
		// Unknown status, treat as processing
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "30%"
	}

	return &taskResult, nil
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	var dResp responseTask
	if err := common.Unmarshal(originTask.Data, &dResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal doubao task data failed")
	}

	openAIVideo := dto.NewOpenAIVideo()
	openAIVideo.ID = originTask.TaskID
	openAIVideo.TaskID = originTask.TaskID
	openAIVideo.Status = originTask.Status.ToVideoStatus()
	openAIVideo.SetProgressStr(originTask.Progress)
	openAIVideo.SetMetadata("url", dResp.Content.VideoURL)
	openAIVideo.CreatedAt = originTask.CreatedAt
	openAIVideo.CompletedAt = originTask.UpdatedAt
	openAIVideo.Model = originTask.Properties.OriginModelName

	if dResp.Status == "failed" {
		openAIVideo.Error = &dto.OpenAIVideoError{
			Message: dResp.Error.Message,
			Code:    dResp.Error.Code,
		}
	}

	return common.Marshal(openAIVideo)
}
