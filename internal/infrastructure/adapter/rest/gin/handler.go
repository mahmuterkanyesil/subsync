package gin

import (
	"fmt"
	"net/http"
	"subsync/internal/core/application/port"

	"github.com/gin-gonic/gin"
)

type HTTPServer struct {
	statsUseCase port.StatsUseCase
	port         string
}

func NewHTTPServer(statsUseCase port.StatsUseCase, port string) *HTTPServer {
	return &HTTPServer{
		statsUseCase: statsUseCase,
		port:         port,
	}
}

func (s *HTTPServer) Start() error {
	r := gin.Default()

	api := r.Group("/api")
	{
		api.GET("/stats", s.getStats)
		api.GET("/records", s.listRecords)
		api.GET("/records/:engPath", s.findByPath)
		api.POST("/records/:engPath/retry", s.reTranslate)
		api.POST("/records/:engPath/re-embed", s.reEmbed)
		api.POST("/keys", s.addApiKey)
		api.POST("/keys/:id/disable", s.disableApiKey)
		api.POST("/keys/:id/reset-quota", s.resetQuota)
	}

	return r.Run(":" + s.port)
}

func (s *HTTPServer) getStats(c *gin.Context) {
	stats, err := s.statsUseCase.GetStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (s *HTTPServer) listRecords(c *gin.Context) {
	records, err := s.statsUseCase.ListRecords(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, records)
}

func (s *HTTPServer) findByPath(c *gin.Context) {
	engPath := c.Param("engPath")
	record, err := s.statsUseCase.FindByPath(c.Request.Context(), engPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, record)
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

func (s *HTTPServer) addApiKey(c *gin.Context) {
	var req struct {
		Service  string `json:"service"`
		KeyValue string `json:"key_value"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := s.statsUseCase.AddApiKey(c.Request.Context(), req.Service, req.KeyValue); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"status": "ok"})
}

func (s *HTTPServer) disableApiKey(c *gin.Context) {
	var id int
	if _, err := fmt.Sscanf(c.Param("id"), "%d", &id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := s.statsUseCase.DisableApiKey(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *HTTPServer) resetQuota(c *gin.Context) {
	var id int
	if _, err := fmt.Sscanf(c.Param("id"), "%d", &id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := s.statsUseCase.ResetQuotaApiKey(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
