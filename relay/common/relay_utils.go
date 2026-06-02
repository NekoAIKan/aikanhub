package common

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

type HasPrompt interface {
	GetPrompt() string
}

type HasImage interface {
	HasImage() bool
}

const (
	TaskRequestContextKey         = "task_request"
	TaskUpstreamRequestContextKey = "task_upstream_request"
	maxTaskSnapshotBytes          = 32768
)

func GetFullRequestURL(baseURL string, requestURL string, channelType int) string {
	fullRequestURL := fmt.Sprintf("%s%s", baseURL, requestURL)

	if strings.HasPrefix(baseURL, "https://gateway.ai.cloudflare.com") {
		switch channelType {
		case constant.ChannelTypeOpenAI:
			fullRequestURL = fmt.Sprintf("%s%s", baseURL, strings.TrimPrefix(requestURL, "/v1"))
		case constant.ChannelTypeAzure:
			fullRequestURL = fmt.Sprintf("%s%s", baseURL, strings.TrimPrefix(requestURL, "/openai/deployments"))
		}
	}
	return fullRequestURL
}

func GetAPIVersion(c *gin.Context) string {
	query := c.Request.URL.Query()
	apiVersion := query.Get("api-version")
	if apiVersion == "" {
		apiVersion = c.GetString("api_version")
	}
	return apiVersion
}

func createTaskError(err error, code string, statusCode int, localError bool) *dto.TaskError {
	return &dto.TaskError{
		Code:       code,
		Message:    err.Error(),
		StatusCode: statusCode,
		LocalError: localError,
		Error:      err,
	}
}

func storeTaskRequest(c *gin.Context, info *RelayInfo, action string, requestObj TaskSubmitReq) {
	if info != nil {
		info.Action = action
	}
	c.Set(TaskRequestContextKey, requestObj)
}
func GetTaskRequest(c *gin.Context) (TaskSubmitReq, error) {
	v, exists := c.Get(TaskRequestContextKey)
	if !exists {
		return TaskSubmitReq{}, fmt.Errorf("request not found in context")
	}
	req, ok := v.(TaskSubmitReq)
	if !ok {
		return TaskSubmitReq{}, fmt.Errorf("invalid task request type")
	}
	return req, nil
}

func SetTaskUpstreamRequest(c *gin.Context, data []byte) {
	if c == nil || len(data) == 0 {
		return
	}
	c.Set(TaskUpstreamRequestContextKey, cloneTaskSnapshot(data))
}

func GetTaskUpstreamRequest(c *gin.Context) json.RawMessage {
	if c == nil {
		return nil
	}
	v, ok := c.Get(TaskUpstreamRequestContextKey)
	if !ok {
		return nil
	}
	switch data := v.(type) {
	case []byte:
		return json.RawMessage(data)
	case json.RawMessage:
		return data
	default:
		return nil
	}
}

func cloneTaskSnapshot(data []byte) []byte {
	if len(data) <= maxTaskSnapshotBytes {
		out := make([]byte, len(data))
		copy(out, data)
		return out
	}
	summary, err := common.Marshal(map[string]any{
		"truncated": true,
		"bytes":     len(data),
	})
	if err != nil {
		return []byte(`{"truncated":true}`)
	}
	return summary
}

func validatePrompt(prompt string) *dto.TaskError {
	if strings.TrimSpace(prompt) == "" {
		return createTaskError(fmt.Errorf("prompt is required"), "invalid_request", http.StatusBadRequest, true)
	}
	return nil
}

// imagesFromMetadataContent extracts image URLs from the
// `metadata.content[]` shape used by OpenAI-compatible video clients.
// Callers expect one URL per item shaped `{type: "image_url",
// image_url: {url: "..."}}`. Items without a URL are skipped silently
// (let the downstream resolver decide whether the missing slot is fatal).
func imagesFromMetadataContent(metadata map[string]any) []string {
	if metadata == nil {
		return nil
	}
	contentRaw, ok := metadata["content"]
	if !ok {
		return nil
	}
	extract := func(items []map[string]any) []string {
		var out []string
		for _, item := range items {
			if item["type"] != nil && item["type"] != "image_url" {
				if _, has := item["image_url"]; !has {
					continue
				}
			}
			urlField, ok := item["image_url"]
			if !ok {
				continue
			}
			switch v := urlField.(type) {
			case string:
				if v != "" {
					out = append(out, v)
				}
			case map[string]any:
				if u, ok := v["url"].(string); ok && u != "" {
					out = append(out, u)
				}
			}
		}
		return out
	}
	switch content := contentRaw.(type) {
	case []any:
		converted := make([]map[string]any, 0, len(content))
		for _, item := range content {
			if itemMap, ok := item.(map[string]any); ok {
				converted = append(converted, itemMap)
			}
		}
		return extract(converted)
	case []map[string]any:
		return extract(content)
	}
	return nil
}

func validateMultipartTaskRequest(c *gin.Context, info *RelayInfo, action string) (TaskSubmitReq, error) {
	var req TaskSubmitReq
	if _, err := c.MultipartForm(); err != nil {
		return req, err
	}

	formData := c.Request.PostForm
	req = TaskSubmitReq{
		Prompt:      formData.Get("prompt"),
		Model:       formData.Get("model"),
		Mode:        formData.Get("mode"),
		Image:       formData.Get("image"),
		Size:        formData.Get("size"),
		Resolution:  formData.Get("resolution"),
		Ratio:       formData.Get("ratio"),
		AspectRatio: formData.Get("aspect_ratio"),
		Metadata:    make(map[string]interface{}),
	}

	if durationStr := formData.Get("seconds"); durationStr != "" {
		if duration, err := strconv.Atoi(durationStr); err == nil {
			req.Duration = duration
		}
	}

	if images := formData["images"]; len(images) > 0 {
		req.Images = images
	}

	for key, values := range formData {
		if len(values) > 0 && !isKnownTaskField(key) {
			if intVal, err := strconv.Atoi(values[0]); err == nil {
				req.Metadata[key] = intVal
			} else if floatVal, err := strconv.ParseFloat(values[0], 64); err == nil {
				req.Metadata[key] = floatVal
			} else {
				req.Metadata[key] = values[0]
			}
		}
	}
	return req, nil
}

func ValidateMultipartDirect(c *gin.Context, info *RelayInfo) *dto.TaskError {
	var prompt string
	var model string
	var seconds int
	var size string
	var hasInputReference bool

	var req TaskSubmitReq
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return createTaskError(err, "invalid_json", http.StatusBadRequest, true)
	}

	prompt = req.Prompt
	model = req.Model
	size = req.Size
	seconds, _ = strconv.Atoi(req.Seconds)
	if seconds == 0 {
		seconds = req.Duration
	}
	if req.InputReference != "" {
		req.Images = []string{req.InputReference}
	}
	// Hydrate `req.Images` from `metadata.content[].image_url` so adaptor
	// code can keep reading a single canonical field. Callers using the
	// OpenAI Videos shape pass image references inside metadata.content;
	// without this fold the downstream image-resolution path (e.g.
	// pixverse.resolveImageID looking at req.Images[i]) returns
	// "missing image at index 0" even though the request did include one.
	if extras := imagesFromMetadataContent(req.Metadata); len(extras) > 0 {
		req.Images = append(req.Images, extras...)
	}

	if strings.TrimSpace(req.Model) == "" {
		return createTaskError(fmt.Errorf("model field is required"), "missing_model", http.StatusBadRequest, true)
	}

	if req.HasImage() {
		hasInputReference = true
	}

	if taskErr := validatePrompt(prompt); taskErr != nil {
		return taskErr
	}

	action := constant.TaskActionTextGenerate
	if hasInputReference {
		action = constant.TaskActionGenerate
	}
	if strings.HasPrefix(model, "sora-2") {

		if size == "" {
			size = "720x1280"
		}

		if seconds <= 0 {
			seconds = 4
		}

		if model == "sora-2" && !lo.Contains([]string{"720x1280", "1280x720"}, size) {
			return createTaskError(fmt.Errorf("sora-2 size is invalid"), "invalid_size", http.StatusBadRequest, true)
		}
		if model == "sora-2-pro" && !lo.Contains([]string{"720x1280", "1280x720", "1792x1024", "1024x1792"}, size) {
			return createTaskError(fmt.Errorf("sora-2 size is invalid"), "invalid_size", http.StatusBadRequest, true)
		}
		// OtherRatios 已移到 Sora adaptor 的 EstimateBilling 中设置
	}

	storeTaskRequest(c, info, action, req)

	return nil
}

func isKnownTaskField(field string) bool {
	knownFields := map[string]bool{
		"prompt":          true,
		"model":           true,
		"mode":            true,
		"image":           true,
		"images":          true,
		"size":            true,
		"resolution":      true,
		"ratio":           true,
		"aspect_ratio":    true,
		"duration":        true,
		"input_reference": true, // Sora 特有字段
	}
	return knownFields[field]
}

func ValidateBasicTaskRequest(c *gin.Context, info *RelayInfo, action string) *dto.TaskError {
	var err error
	contentType := c.GetHeader("Content-Type")
	var req TaskSubmitReq
	if strings.HasPrefix(contentType, "multipart/form-data") {
		req, err = validateMultipartTaskRequest(c, info, action)
		if err != nil {
			return createTaskError(err, "invalid_multipart_form", http.StatusBadRequest, true)
		}
	}
	// 为了metadata字段的兼容性，统一UnmarshalBodyReusable
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return createTaskError(err, "invalid_request", http.StatusBadRequest, true)
	}

	if taskErr := validatePrompt(req.Prompt); taskErr != nil {
		return taskErr
	}

	if len(req.Images) == 0 && strings.TrimSpace(req.Image) != "" {
		// 兼容单图上传
		req.Images = []string{req.Image}
	}
	// Hydrate from metadata.content[].image_url for OpenAI-style callers
	// who pass image references inside metadata. Without this, downstream
	// adaptors (pixverse i2v, doubao referenceGenerate) read req.Images
	// as empty and reject the request as missing image input even when
	// the metadata clearly carries one.
	if extras := imagesFromMetadataContent(req.Metadata); len(extras) > 0 {
		req.Images = append(req.Images, extras...)
	}

	storeTaskRequest(c, info, action, req)
	return nil
}
