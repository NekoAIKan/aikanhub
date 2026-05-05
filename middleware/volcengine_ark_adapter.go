package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// volcArkSubmitWriter buffers downstream writes so we can rewrite the JSON
// body from OpenAI Video shape into Volcano Ark shape before flushing it.
type volcArkSubmitWriter struct {
	gin.ResponseWriter
	body       bytes.Buffer
	statusCode int
}

func (w *volcArkSubmitWriter) WriteHeader(code int) {
	w.statusCode = code
}

func (w *volcArkSubmitWriter) Write(b []byte) (int, error) {
	return w.body.Write(b)
}

func (w *volcArkSubmitWriter) WriteString(s string) (int, error) {
	return w.body.WriteString(s)
}

// VolcArkSubmitConvert handles `POST /api/v3/contents/generations/tasks`.
// It translates the Volcano Ark request body into the gateway's internal
// {model, prompt, metadata{...}} shape, rewrites the URL to
// /v1/video/generations, then rewrites the JSON response from OpenAI Video
// shape into `{"id": "<task_id>"}` expected by `volcenginesdkarkruntime`.
func VolcArkSubmitConvert() func(c *gin.Context) {
	return func(c *gin.Context) {
		var raw map[string]interface{}
		if err := common.UnmarshalBodyReusable(c, &raw); err != nil {
			abortWithOpenAiMessage(c, http.StatusBadRequest, "invalid_request_body")
			return
		}
		modelName, _ := raw["model"].(string)
		if modelName == "" {
			abortWithOpenAiMessage(c, http.StatusBadRequest, "model is required")
			return
		}

		prompt := ""
		if contentRaw, ok := raw["content"].([]interface{}); ok {
			for _, item := range contentRaw {
				m, _ := item.(map[string]interface{})
				if m == nil {
					continue
				}
				if t, _ := m["type"].(string); t == "text" {
					if txt, ok := m["text"].(string); ok && txt != "" {
						prompt = txt
						break
					}
				}
			}
		}

		seconds := ""
		switch v := raw["duration"].(type) {
		case float64:
			seconds = strconv.Itoa(int(v))
		case int:
			seconds = strconv.Itoa(v)
		case string:
			seconds = v
		}

		unified := map[string]interface{}{
			"model":    modelName,
			"prompt":   prompt,
			"metadata": raw,
		}
		if seconds != "" {
			unified["seconds"] = seconds
		}
		jsonData, err := json.Marshal(unified)
		if err != nil {
			abortWithOpenAiMessage(c, http.StatusInternalServerError, "marshal_failed")
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewBuffer(jsonData))
		c.Request.ContentLength = int64(len(jsonData))
		// Replace any cached body storage so downstream UnmarshalBodyReusable
		// calls see the translated body instead of the original Ark payload.
		c.Set(common.KeyBodyStorage, nil)
		c.Set(common.KeyRequestBody, jsonData)
		c.Request.URL.Path = "/v1/video/generations"

		ww := &volcArkSubmitWriter{ResponseWriter: c.Writer}
		c.Writer = ww
		c.Next()

		status := ww.statusCode
		if status == 0 {
			status = http.StatusOK
		}
		body := ww.body.Bytes()
		var out []byte
		if len(body) > 0 {
			var parsed map[string]interface{}
			if err := json.Unmarshal(body, &parsed); err == nil {
				if status == http.StatusOK {
					id, _ := parsed["task_id"].(string)
					if id == "" {
						id, _ = parsed["id"].(string)
					}
					if id != "" {
						out, _ = json.Marshal(map[string]string{"id": id})
					}
				} else if _, hasErr := parsed["error"]; !hasErr {
					// dto.TaskError shape: {code, message, data} → reshape into
					// the {error: {code, message}} envelope the Ark SDK expects.
					code, _ := parsed["code"].(string)
					msg, _ := parsed["message"].(string)
					if code != "" || msg != "" {
						out, _ = json.Marshal(map[string]interface{}{
							"error": map[string]string{"code": code, "message": msg},
						})
					}
				}
			}
		}
		if out == nil {
			out = body
		}
		ww.ResponseWriter.Header().Set("Content-Type", "application/json")
		ww.ResponseWriter.Header().Set("Content-Length", strconv.Itoa(len(out)))
		ww.ResponseWriter.WriteHeader(status)
		_, _ = ww.ResponseWriter.Write(out)
	}
}
