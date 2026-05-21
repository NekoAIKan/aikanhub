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

type studioSession struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Group    string `json:"group,omitempty"`
	Role     int    `json:"role,omitempty"`
}

func StudioSession(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    currentStudioSession(c),
	})
}

func StudioBootstrap(c *gin.Context) {
	data, err := service.BuildStudioBootstrap(
		c.GetInt("id"),
		c.GetString("username"),
		c.GetString("group"),
		c.GetInt("role"),
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}

func StudioVideoModels(c *gin.Context) {
	tokenModelLimitValue, _ := c.Get("token_model_limit")
	tokenModelLimit, _ := tokenModelLimitValue.(map[string]bool)
	models, err := service.ListStudioVideoModels(c.GetInt("id"), tokenModelLimit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    models,
	})
}

func currentStudioSession(c *gin.Context) studioSession {
	return studioSession{
		ID:       c.GetInt("id"),
		Username: c.GetString("username"),
		Group:    c.GetString("group"),
		Role:     c.GetInt("role"),
	}
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
