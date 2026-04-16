package gin

import (
	"net/http"
	"strconv"
	"subsync/pkg/logger"
	"time"

	"github.com/gin-gonic/gin"
)

// ─── Health ───────────────────────────────────────────────────────────────────

func (s *HTTPServer) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "time": time.Now().UTC().Format(time.RFC3339)})
}

// ─── Logs — JSON API ──────────────────────────────────────────────────────────

func (s *HTTPServer) apiLogs(c *gin.Context) {
	limit := 200
	if l := c.Query("limit"); l != "" {
		if i, err := strconv.Atoi(l); err == nil {
			limit = i
		}
	}
	entries := logger.GetRecent(limit)
	out := make([]gin.H, 0, len(entries))
	for _, e := range entries {
		out = append(out, gin.H{
			"time":    e.Time.Format("15:04:05"),
			"level":   e.Level,
			"service": e.Service,
			"message": e.Message,
		})
	}
	c.JSON(http.StatusOK, out)
}

func (s *HTTPServer) receiveLog(c *gin.Context) {
	var body struct {
		Time    string `json:"time"`
		Level   string `json:"level"`
		Service string `json:"service"`
		Message string `json:"message"`
	}
	if err := c.BindJSON(&body); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	t := time.Now().UTC()
	if body.Time != "" {
		if parsed, err := time.Parse(time.RFC3339, body.Time); err == nil {
			t = parsed
		}
	}
	logger.ReceiveRemote(t, body.Service, body.Level, body.Message)
	c.Status(http.StatusNoContent)
}

// ─── Logs — Web UI ────────────────────────────────────────────────────────────

func (s *HTTPServer) webLogs(c *gin.Context) {
	data := LogsData{CurrentPage: "logs"}
	c.Status(http.StatusOK)
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.logs.ExecuteTemplate(c.Writer, "layout", data); err != nil {
		_ = err
	}
}
