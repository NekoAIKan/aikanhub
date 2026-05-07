package pixverse

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

// ============================
// Request / Response structures
// ============================

// requestPayload is shared across text/image/transition/fusion endpoints.
// Field presence is endpoint-specific; encoding/json with omitempty drops the
// fields that don't apply.
type requestPayload struct {
	Model               string `json:"model"`
	Prompt              string `json:"prompt"`
	NegativePrompt      string `json:"negative_prompt,omitempty"`
	Duration            int    `json:"duration"`
	Quality             string `json:"quality"`
	AspectRatio         string `json:"aspect_ratio,omitempty"`
	MotionMode          string `json:"motion_mode,omitempty"`
	Seed                *int64 `json:"seed,omitempty"`
	WaterMark           *bool  `json:"water_mark,omitempty"`
	GenerateAudioSwitch *bool  `json:"generate_audio_switch,omitempty"`
	Style               string `json:"style,omitempty"`
	TemplateID          int64  `json:"template_id,omitempty"`

	// Image-to-video.
	ImgID int64 `json:"img_id,omitempty"`

	// Transition (first/last frame).
	FirstFrameImg int64 `json:"first_frame_img,omitempty"`
	LastFrameImg  int64 `json:"last_frame_img,omitempty"`

	// Fusion (reference-to-video). Each entry is {type, img_id, ref_name}.
	// Pixverse uses ref_name with "@" prefix in the prompt to bind a noun to
	// a specific reference image (e.g. "@dog plays at @room").
	ImageReferences []ImageReference `json:"image_references,omitempty"`
}

// ImageReference is the fusion-mode reference shape.
type ImageReference struct {
	Type    string `json:"type"` // "subject" | "background"
	ImgID   int64  `json:"img_id"`
	RefName string `json:"ref_name"`
}

type submitResponse struct {
	ErrCode int    `json:"ErrCode"`
	ErrMsg  string `json:"ErrMsg"`
	Resp    struct {
		VideoID int64 `json:"video_id"`
	} `json:"Resp"`
}

type uploadResponse struct {
	ErrCode int    `json:"ErrCode"`
	ErrMsg  string `json:"ErrMsg"`
	Resp    struct {
		ImgID  int64  `json:"img_id"`
		ImgURL string `json:"img_url"`
	} `json:"Resp"`
}

type resultResponse struct {
	ErrCode int    `json:"ErrCode"`
	ErrMsg  string `json:"ErrMsg"`
	Resp    struct {
		ID              int64  `json:"id"`
		Status          int    `json:"status"`
		URL             string `json:"url"`
		Prompt          string `json:"prompt"`
		NegativePrompt  string `json:"negative_prompt"`
		Style           string `json:"style"`
		Seed            int64  `json:"seed"`
		Size            int64  `json:"size"`
		ResolutionRatio int    `json:"resolution_ratio"`
		OutputWidth     int    `json:"outputWidth"`
		OutputHeight    int    `json:"outputHeight"`
		CreateTime      string `json:"create_time"`
		ModifyToken     string `json:"modify_time"`
	} `json:"Resp"`
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

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	if err := relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate); err != nil {
		return err
	}
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return service.TaskErrorWrapper(err, "get_task_request_failed", http.StatusBadRequest)
	}

	// Determine action: explicit metadata.action wins; otherwise infer from images.
	action := constant.TaskActionTextGenerate
	if metaAction, ok := req.Metadata["action"]; ok {
		if s, _ := metaAction.(string); s != "" {
			action = s
		}
	} else if req.HasImage() {
		switch len(req.Images) {
		case 1:
			action = constant.TaskActionGenerate
		case 2:
			action = constant.TaskActionFirstTailGenerate
		default:
			action = constant.TaskActionReferenceGenerate
		}
	}
	info.Action = action
	return nil
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	var path string
	switch info.Action {
	case constant.TaskActionGenerate:
		path = endpointImageToVideo
	case constant.TaskActionFirstTailGenerate:
		path = endpointTransition
	case constant.TaskActionReferenceGenerate:
		path = endpointFusion
	default:
		path = endpointTextToVideo
	}
	return a.baseURL + path, nil
}

func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("API-KEY", a.apiKey)
	req.Header.Set("Ai-trace-id", uuid.New().String())
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	v, exists := c.Get("task_request")
	if !exists {
		return nil, fmt.Errorf("request not found in context")
	}
	req, ok := v.(relaycommon.TaskSubmitReq)
	if !ok {
		return nil, fmt.Errorf("invalid task request type")
	}

	body, err := a.convertToRequestPayload(c, &req, info)
	if err != nil {
		return nil, err
	}

	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	_ = resp.Body.Close()

	var pResp submitResponse
	if err := common.Unmarshal(responseBody, &pResp); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_failed", http.StatusInternalServerError)
		return
	}
	if pResp.ErrCode != 0 {
		taskErr = service.TaskErrorWrapperLocal(
			fmt.Errorf("pixverse api error: %s", pResp.ErrMsg),
			strconv.Itoa(pResp.ErrCode),
			http.StatusBadRequest,
		)
		return
	}
	if pResp.Resp.VideoID == 0 {
		taskErr = service.TaskErrorWrapperLocal(fmt.Errorf("pixverse returned empty video_id"), "empty_video_id", http.StatusBadGateway)
		return
	}

	ov := dto.NewOpenAIVideo()
	ov.ID = info.PublicTaskID
	ov.TaskID = info.PublicTaskID
	ov.CreatedAt = time.Now().Unix()
	ov.Model = info.OriginModelName
	c.JSON(http.StatusOK, ov)
	return strconv.FormatInt(pResp.Resp.VideoID, 10), responseBody, nil
}

func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok || taskID == "" {
		return nil, fmt.Errorf("invalid task_id")
	}
	url := baseUrl + endpointVideoResult + taskID

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("API-KEY", key)
	req.Header.Set("Ai-trace-id", uuid.New().String())

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

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var r resultResponse
	if err := common.Unmarshal(respBody, &r); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal pixverse result")
	}

	taskInfo := &relaycommon.TaskInfo{Code: r.ErrCode}
	if r.ErrCode != 0 {
		taskInfo.Status = model.TaskStatusFailure
		taskInfo.Reason = r.ErrMsg
		taskInfo.Progress = taskcommon.ProgressComplete
		return taskInfo, nil
	}

	switch r.Resp.Status {
	case statusGenerating:
		taskInfo.Status = model.TaskStatusInProgress
		taskInfo.Progress = taskcommon.ProgressInProgress
	case statusSuccess:
		taskInfo.Status = model.TaskStatusSuccess
		taskInfo.Progress = taskcommon.ProgressComplete
		taskInfo.Url = r.Resp.URL
	case statusModerationFailed:
		taskInfo.Status = model.TaskStatusFailure
		taskInfo.Progress = taskcommon.ProgressComplete
		taskInfo.Reason = "contents moderation failed"
	case statusGenerationFailure:
		taskInfo.Status = model.TaskStatusFailure
		taskInfo.Progress = taskcommon.ProgressComplete
		taskInfo.Reason = "generation failed"
	case statusDeleted:
		taskInfo.Status = model.TaskStatusFailure
		taskInfo.Progress = taskcommon.ProgressComplete
		taskInfo.Reason = "video deleted"
	default:
		// Unknown status — treat as still running so we keep polling.
		taskInfo.Status = model.TaskStatusInProgress
		taskInfo.Progress = taskcommon.ProgressInProgress
	}
	return taskInfo, nil
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	var r resultResponse
	if err := common.Unmarshal(originTask.Data, &r); err != nil {
		return nil, errors.Wrap(err, "unmarshal pixverse task data failed")
	}

	openAIVideo := dto.NewOpenAIVideo()
	openAIVideo.ID = originTask.TaskID
	openAIVideo.Status = originTask.Status.ToVideoStatus()
	openAIVideo.SetProgressStr(originTask.Progress)
	openAIVideo.CreatedAt = originTask.CreatedAt
	openAIVideo.CompletedAt = originTask.UpdatedAt

	if r.Resp.URL != "" {
		openAIVideo.SetMetadata("url", r.Resp.URL)
	}

	if r.ErrCode != 0 {
		openAIVideo.Error = &dto.OpenAIVideoError{
			Message: r.ErrMsg,
			Code:    strconv.Itoa(r.ErrCode),
		}
	} else {
		switch r.Resp.Status {
		case statusModerationFailed:
			openAIVideo.Error = &dto.OpenAIVideoError{Message: "contents moderation failed", Code: strconv.Itoa(r.Resp.Status)}
		case statusGenerationFailure:
			openAIVideo.Error = &dto.OpenAIVideoError{Message: "generation failed", Code: strconv.Itoa(r.Resp.Status)}
		case statusDeleted:
			openAIVideo.Error = &dto.OpenAIVideoError{Message: "video deleted", Code: strconv.Itoa(r.Resp.Status)}
		}
	}

	return common.Marshal(openAIVideo)
}

// ============================
// helpers
// ============================

func (a *TaskAdaptor) convertToRequestPayload(c *gin.Context, req *relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) (*requestPayload, error) {
	spec := parseModelSpec(info.UpstreamModelName)

	r := &requestPayload{
		Model:       spec.apiModel,
		Prompt:      req.Prompt,
		Duration:    taskcommon.DefaultInt(req.Duration, defaultDuration),
		Quality:     defaultQuality,
		AspectRatio: defaultAspectRatio,
		MotionMode:  defaultMotionMode,
	}
	if q := qualityFromSize(req.Size); q != "" {
		r.Quality = q
	}
	if ar := aspectRatioFromSize(req.Size); ar != "" {
		r.AspectRatio = ar
	}

	// metadata can override anything above (incl. seed, water_mark, style, img_id...).
	if err := taskcommon.UnmarshalMetadata(req.Metadata, r); err != nil {
		return nil, errors.Wrap(err, "unmarshal metadata failed")
	}

	// Pinned dims from the model name win over client/metadata so billing
	// matches what's actually submitted upstream.
	if spec.quality != "" {
		r.Quality = spec.quality
	}
	if spec.duration > 0 {
		r.Duration = spec.duration
	}
	r.Model = spec.apiModel

	// Resolve images to img_ids when an image-using action is selected.
	switch info.Action {
	case constant.TaskActionGenerate:
		if r.ImgID == 0 {
			imgID, err := a.resolveImageID(c, req, 0)
			if err != nil {
				return nil, err
			}
			r.ImgID = imgID
		}
	case constant.TaskActionFirstTailGenerate:
		if r.FirstFrameImg == 0 {
			id, err := a.resolveImageID(c, req, 0)
			if err != nil {
				return nil, err
			}
			r.FirstFrameImg = id
		}
		if r.LastFrameImg == 0 {
			id, err := a.resolveImageID(c, req, 1)
			if err != nil {
				return nil, err
			}
			r.LastFrameImg = id
		}
	case constant.TaskActionReferenceGenerate:
		if len(r.ImageReferences) == 0 {
			refs := make([]ImageReference, 0, len(req.Images))
			for i := range req.Images {
				id, err := a.resolveImageID(c, req, i)
				if err != nil {
					return nil, err
				}
				refs = append(refs, ImageReference{
					Type:    "subject",
					ImgID:   id,
					RefName: fmt.Sprintf("img%d", i+1),
				})
			}
			r.ImageReferences = refs
		}
	}

	return r, nil
}

// resolveImageID converts the i-th request image (URL or base64) into a
// Pixverse-side img_id by uploading to /openapi/v2/image/upload.
func (a *TaskAdaptor) resolveImageID(c *gin.Context, req *relaycommon.TaskSubmitReq, idx int) (int64, error) {
	var src string
	if idx < len(req.Images) {
		src = req.Images[idx]
	} else if idx == 0 && req.Image != "" {
		src = req.Image
	}
	if src == "" {
		return 0, fmt.Errorf("missing image at index %d", idx)
	}

	// Allow user to skip upload by passing a numeric id directly.
	if id, err := strconv.ParseInt(src, 10, 64); err == nil && id > 0 {
		return id, nil
	}

	imgBytes, filename, err := loadImageBytes(c, src)
	if err != nil {
		return 0, err
	}
	return a.uploadImage(c, imgBytes, filename)
}

func (a *TaskAdaptor) uploadImage(c *gin.Context, data []byte, filename string) (int64, error) {
	if filename == "" {
		filename = "image.png"
	}

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	part, err := mw.CreateFormFile("image", filename)
	if err != nil {
		return 0, errors.Wrap(err, "create form file failed")
	}
	if _, err := part.Write(data); err != nil {
		return 0, errors.Wrap(err, "write form file failed")
	}
	if err := mw.Close(); err != nil {
		return 0, errors.Wrap(err, "close multipart writer failed")
	}

	url := a.baseURL + endpointImageUpload
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, url, body)
	if err != nil {
		return 0, errors.Wrap(err, "new upload request failed")
	}
	req.Header.Set("API-KEY", a.apiKey)
	req.Header.Set("Ai-trace-id", uuid.New().String())
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := service.GetHttpClient().Do(req)
	if err != nil {
		return 0, errors.Wrap(err, "upload image request failed")
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, errors.Wrap(err, "read upload response failed")
	}

	var ur uploadResponse
	if err := common.Unmarshal(respBody, &ur); err != nil {
		return 0, errors.Wrapf(err, "unmarshal upload response failed: %s", respBody)
	}
	if ur.ErrCode != 0 {
		return 0, fmt.Errorf("pixverse upload failed (%d): %s", ur.ErrCode, ur.ErrMsg)
	}
	if ur.Resp.ImgID == 0 {
		return 0, fmt.Errorf("pixverse upload returned empty img_id")
	}
	return ur.Resp.ImgID, nil
}

// loadImageBytes accepts an HTTP(S) URL, a "data:image/...;base64,..." URI,
// or a raw base64 string, and returns decoded bytes plus a guessed filename.
func loadImageBytes(c *gin.Context, src string) ([]byte, string, error) {
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, src, nil)
		if err != nil {
			return nil, "", errors.Wrap(err, "new image fetch request failed")
		}
		resp, err := service.GetHttpClient().Do(req)
		if err != nil {
			return nil, "", errors.Wrap(err, "fetch image failed")
		}
		defer resp.Body.Close()
		if resp.StatusCode/100 != 2 {
			return nil, "", fmt.Errorf("fetch image failed: status %d", resp.StatusCode)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, "", errors.Wrap(err, "read image body failed")
		}
		return body, filenameFromURL(src), nil
	}

	// data URI: data:image/png;base64,<b64>
	payload := src
	if strings.HasPrefix(payload, "data:") {
		if idx := strings.Index(payload, ","); idx >= 0 {
			payload = payload[idx+1:]
		}
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(payload))
	if err != nil {
		return nil, "", errors.Wrap(err, "decode base64 image failed")
	}
	return decoded, "image.png", nil
}

func filenameFromURL(u string) string {
	if i := strings.LastIndex(u, "/"); i >= 0 && i+1 < len(u) {
		name := u[i+1:]
		if j := strings.IndexAny(name, "?#"); j >= 0 {
			name = name[:j]
		}
		if name != "" {
			return name
		}
	}
	return "image.png"
}

// modelSpec is the parsed shape of a gateway-facing model name like
// "pixverse-v4.5", "pixverse-v4.5-720p", or "pixverse-v4.5-1080p-8s".
//
//	apiModel: what Pixverse expects in the JSON body (e.g. "v4.5", "c1").
//	quality:  pinned quality (e.g. "720p"); "" means caller-controlled.
//	duration: pinned duration in seconds; 0 means caller-controlled.
type modelSpec struct {
	apiModel string
	quality  string
	duration int
}

// parseModelSpec accepts:
//
//	pixverse-{ver}                    → apiModel only
//	pixverse-{ver}-{quality}          → e.g. "pixverse-v4.5-720p"
//	pixverse-{ver}-{duration}s        → e.g. "pixverse-v4.5-8s"
//	pixverse-{ver}-{quality}-{dur}s   → e.g. "pixverse-v4.5-1080p-8s"
//
// Order of the trailing tokens is flexible. Unknown / un-prefixed names are
// passed through as the api model so admins can wire new SKUs via channel
// model mapping without a code change.
func parseModelSpec(name string) modelSpec {
	if name == "" {
		return modelSpec{apiModel: defaultModel}
	}
	lower := strings.ToLower(name)
	rest, ok := strings.CutPrefix(lower, "pixverse-")
	if !ok {
		return modelSpec{apiModel: name}
	}
	parts := strings.Split(rest, "-")
	spec := modelSpec{apiModel: parts[0]}
	for _, p := range parts[1:] {
		switch {
		case strings.HasSuffix(p, "p"):
			// quality: "360p", "540p", "720p", "1080p"
			if isPixverseQuality(p) {
				spec.quality = p
			}
		case strings.HasSuffix(p, "s"):
			// duration: "5s", "8s"
			if n, err := strconv.Atoi(strings.TrimSuffix(p, "s")); err == nil && n > 0 {
				spec.duration = n
			}
		}
	}
	return spec
}

func isPixverseQuality(q string) bool {
	switch q {
	case "360p", "540p", "720p", "1080p":
		return true
	}
	return false
}

// qualityFromSize maps a "WxH" or "Np" hint to Pixverse's quality field.
// Returns "" when no clear mapping is available so the caller keeps its
// default.
func qualityFromSize(size string) string {
	if size == "" {
		return ""
	}
	s := strings.ToLower(size)
	switch {
	case strings.Contains(s, "1080"):
		return "1080p"
	case strings.Contains(s, "720"):
		return "720p"
	case strings.Contains(s, "540"):
		return "540p"
	case strings.Contains(s, "360"):
		return "360p"
	}
	return ""
}

func aspectRatioFromSize(size string) string {
	if size == "" {
		return ""
	}
	parts := strings.Split(strings.ToLower(size), "x")
	if len(parts) != 2 {
		return ""
	}
	w, err1 := strconv.Atoi(parts[0])
	h, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || w <= 0 || h <= 0 {
		return ""
	}
	switch {
	case w == h:
		return "1:1"
	case w*9 == h*16:
		return "16:9"
	case w*16 == h*9:
		return "9:16"
	case w*3 == h*4:
		return "4:3"
	case w*4 == h*3:
		return "3:4"
	case w*2 == h*3:
		return "2:3"
	case w*3 == h*2:
		return "3:2"
	case w*9 == h*21:
		return "21:9"
	}
	return ""
}
