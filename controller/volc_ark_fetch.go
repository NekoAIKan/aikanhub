package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// VolcArkFetchTask handles `GET /api/v3/contents/generations/tasks/:task_id`.
// It returns the upstream Volcano Ark response stored in `task.Data` verbatim,
// with the `id` field rewritten to the gateway's public task ID so the
// `volcenginesdkarkruntime` SDK consistently sees the ID it submitted.
func VolcArkFetchTask(c *gin.Context) {
	taskID := c.Param("task_id")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "task_id is required", "code": "missing_task_id"}})
		return
	}
	userID := c.GetInt("id")
	originTask, exist, err := model.GetByTaskId(userID, taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error(), "code": "get_task_failed"}})
		return
	}
	if !exist {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"message": "task_not_exist", "code": "task_not_exist"}})
		return
	}

	out := map[string]interface{}{
		"id":         originTask.TaskID,
		"model":      originTask.Properties.OriginModelName,
		"status":     statusToVolcArk(string(originTask.Status)),
		"created_at": originTask.CreatedAt,
		"updated_at": originTask.UpdatedAt,
	}
	if len(originTask.Data) > 0 {
		var upstream map[string]interface{}
		if err := common.Unmarshal(originTask.Data, &upstream); err == nil {
			for k, v := range upstream {
				out[k] = v
			}
			// always overwrite id with the public one
			out["id"] = originTask.TaskID
		}
	}
	if originTask.FailReason != "" {
		out["error"] = gin.H{"message": originTask.FailReason}
	}
	c.JSON(http.StatusOK, out)
}

// statusToVolcArk maps the gateway's internal task status to the lower-case
// strings the Volcano Ark API exposes (queued/running/succeeded/failed/cancelled).
func statusToVolcArk(s string) string {
	switch s {
	case "QUEUED", "SUBMITTED", "NOT_START":
		return "queued"
	case "IN_PROGRESS":
		return "running"
	case "SUCCESS":
		return "succeeded"
	case "FAILURE":
		return "failed"
	default:
		if s == "" {
			return "queued"
		}
		return s
	}
}
