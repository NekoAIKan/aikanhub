package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

type studioVideoModel struct {
	ID        string                 `json:"id"`
	Label     string                 `json:"label"`
	Provider  string                 `json:"provider"`
	Versions  []string               `json:"versions"`
	TaskTypes []string               `json:"taskTypes"`
	Inputs    []string               `json:"inputs"`
	Settings  map[string]interface{} `json:"settings"`
}

type studioVideoGeneration struct {
	TaskID        string  `json:"taskId"`
	ID            string  `json:"id"`
	Status        string  `json:"status"`
	Model         string  `json:"model,omitempty"`
	ContentURL    string  `json:"contentUrl,omitempty"`
	VideoURL      string  `json:"videoUrl,omitempty"`
	ArtifactState string  `json:"artifactStatus"`
	UsageSource   string  `json:"usageSource"`
	UsageKeyLabel string  `json:"usageKeyLabel"`
	UsageKeyID    int     `json:"usageKeyId,omitempty"`
	Progress      float64 `json:"progress"`
	CreatedAt     int64   `json:"createdAt"`
	Error         *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func StudioSession(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":       c.GetInt("id"),
			"username": c.GetString("username"),
			"group":    c.GetString("group"),
			"role":     c.GetInt("role"),
		},
	})
}

func StudioVideoModels(c *gin.Context) {
	user, err := model.GetUserCache(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	groups := service.GetUserUsableGroups(user.Group)
	seen := map[string]bool{}
	models := make([]studioVideoModel, 0)
	for group := range groups {
		for _, modelName := range model.GetGroupEnabledModels(group) {
			if seen[modelName] || !isStudioVideoModel(modelName) {
				continue
			}
			seen[modelName] = true
			models = append(models, describeStudioVideoModel(modelName))
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    models,
	})
}

func StudioVideoGenerationFetch(c *gin.Context) {
	taskID := c.Param("task_id")
	task, exists, err := model.GetByTaskId(c.GetInt("id"), taskID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !exists || task == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Task not found",
		})
		return
	}
	c.JSON(http.StatusOK, normalizeStudioVideoTask(task))
}

func SetStudioVideoSubmitRelayMode(c *gin.Context) {
	c.Set("relay_mode", relayconstant.RelayModeVideoSubmit)
	c.Next()
}

func SetStudioVideoFetchRelayMode(c *gin.Context) {
	c.Set("relay_mode", relayconstant.RelayModeVideoFetchByID)
	c.Set("task_id", c.Param("task_id"))
	c.Next()
}

func normalizeStudioVideoTask(task *model.Task) studioVideoGeneration {
	contentURL := ""
	artifactStatus := "unavailable"
	if task.Status == model.TaskStatusSuccess {
		if strings.TrimSpace(task.GetResultURL()) != "" {
			contentURL = "/v1/videos/" + task.TaskID + "/content"
			artifactStatus = "proxy_ready"
		}
	} else if task.Status == model.TaskStatusFailure {
		artifactStatus = "fetch_failed"
	}

	resp := studioVideoGeneration{
		TaskID:        task.TaskID,
		ID:            task.TaskID,
		Status:        task.Status.ToVideoStatus(),
		Model:         task.Properties.OriginModelName,
		ContentURL:    contentURL,
		VideoURL:      contentURL,
		ArtifactState: artifactStatus,
		UsageSource:   "studio",
		UsageKeyLabel: "Studio",
		UsageKeyID:    task.PrivateData.TokenId,
		Progress:      parseTaskProgress(task.Progress),
		CreatedAt:     task.CreatedAt,
	}
	if resp.Model == "" {
		resp.Model = task.Properties.UpstreamModelName
	}
	if task.Status == model.TaskStatusFailure {
		resp.Error = &struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}{
			Code:    "provider_error",
			Message: task.FailReason,
		}
	}
	return resp
}

func parseTaskProgress(progress string) float64 {
	progress = strings.TrimSpace(strings.TrimSuffix(progress, "%"))
	if progress == "" {
		return 0
	}
	value, err := strconv.ParseFloat(progress, 64)
	if err != nil {
		return 0
	}
	if value > 1 {
		return value
	}
	return value * 100
}

func isStudioVideoModel(modelName string) bool {
	lower := strings.ToLower(modelName)
	return strings.Contains(lower, "seedance") ||
		strings.Contains(lower, "pixverse") ||
		strings.Contains(lower, "kling") ||
		strings.Contains(lower, "sora") ||
		strings.Contains(lower, "vidu") ||
		strings.Contains(lower, "hailuo") ||
		strings.Contains(lower, "wan")
}

func describeStudioVideoModel(modelName string) studioVideoModel {
	provider := "video"
	lower := strings.ToLower(modelName)
	switch {
	case strings.Contains(lower, "pixverse"):
		provider = "pixverse"
	case strings.Contains(lower, "seedance") || strings.Contains(lower, "doubao"):
		provider = "doubao"
	case strings.Contains(lower, "kling"):
		provider = "kling"
	case strings.Contains(lower, "sora"):
		provider = "sora"
	case strings.Contains(lower, "vidu"):
		provider = "vidu"
	case strings.Contains(lower, "hailuo"):
		provider = "hailuo"
	}

	return studioVideoModel{
		ID:        modelName,
		Label:     modelName,
		Provider:  provider,
		Versions:  []string{"default"},
		TaskTypes: []string{"text_to_video", "image_to_video", "video_to_video"},
		Inputs:    []string{"prompt", "first_frame", "last_frame", "reference_image", "reference_video", "reference_audio"},
		Settings: map[string]interface{}{
			"durations":   []int{5, 8, 10},
			"ratios":      []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9"},
			"resolutions": []string{"480p", "720p", "1080p"},
		},
	}
}
