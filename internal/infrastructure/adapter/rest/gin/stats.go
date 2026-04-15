package gin

import (
	"net/http"
	valueobject "subsync/internal/core/domain/valueobject"

	"github.com/gin-gonic/gin"
)

// ─── Stats / Records — JSON API ───────────────────────────────────────────────

func (s *HTTPServer) getStats(c *gin.Context) {
	stats, err := s.statsUseCase.GetStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, toStatsResponse(stats))
}

func (s *HTTPServer) listRecords(c *gin.Context) {
	records, err := s.statsUseCase.ListRecords(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, toSubtitleResponses(records))
}

func (s *HTTPServer) findByPath(c *gin.Context) {
	engPath := c.Param("engPath")
	record, err := s.statsUseCase.FindByPath(c.Request.Context(), engPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, toSubtitleResponse(record))
}

func (s *HTTPServer) reTranslate(c *gin.Context) {
	engPath := c.Param("engPath")
	if err := s.statsUseCase.ReTranslate(c.Request.Context(), engPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "queued"})
}

func (s *HTTPServer) reEmbed(c *gin.Context) {
	engPath := c.Param("engPath")
	if err := s.statsUseCase.ReEmbed(c.Request.Context(), engPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ─── Stats / Records — Web UI ─────────────────────────────────────────────────

func (s *HTTPServer) webDashboard(c *gin.Context) {
	stats, err := s.statsUseCase.GetStats(c.Request.Context())
	if err != nil {
		c.String(http.StatusInternalServerError, "error: %v", err)
		return
	}
	data := DashboardData{CurrentPage: "dashboard", Stats: toStatsResponse(stats)}
	c.Status(http.StatusOK)
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.dashboard.ExecuteTemplate(c.Writer, "layout", data); err != nil {
		_ = err
	}
}

func (s *HTTPServer) webRecords(c *gin.Context) {
	filter := c.Query("status")

	var records []SubtitleResponse
	if filter == "" {
		all, err := s.statsUseCase.ListRecords(c.Request.Context())
		if err != nil {
			c.String(http.StatusInternalServerError, "error: %v", err)
			return
		}
		records = toSubtitleResponses(all)
	} else {
		filtered, err := s.statsUseCase.ListRecordsByStatus(c.Request.Context(), valueobject.SubtitleStatus(filter))
		if err != nil {
			c.String(http.StatusInternalServerError, "error: %v", err)
			return
		}
		records = toSubtitleResponses(filtered)
	}

	data := RecordsData{
		CurrentPage: "records",
		Records:     records,
		Filter:      filter,
		Total:       len(records),
	}
	c.Status(http.StatusOK)
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.records.ExecuteTemplate(c.Writer, "layout", data); err != nil {
		_ = err
	}
}

func (s *HTTPServer) webRetry(c *gin.Context) {
	engPath := c.PostForm("eng_path")
	if err := s.statsUseCase.ReTranslate(c.Request.Context(), engPath); err != nil {
		c.Redirect(http.StatusSeeOther, "/records?flash=error&msg="+encodeMsg(err.Error()))
		return
	}
	c.Redirect(http.StatusSeeOther, "/records?flash=success")
}

func (s *HTTPServer) webReEmbed(c *gin.Context) {
	engPath := c.PostForm("eng_path")
	if err := s.statsUseCase.ReEmbed(c.Request.Context(), engPath); err != nil {
		c.Redirect(http.StatusSeeOther, "/records?flash=error&msg="+encodeMsg(err.Error()))
		return
	}
	c.Redirect(http.StatusSeeOther, "/records?flash=success")
}
