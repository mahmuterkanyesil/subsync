package gin

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"strconv"
	"subsync/internal/core/application/port"
	valueobject "subsync/internal/core/domain/valueobject"
	"subsync/pkg/logger"
	"time"

	"github.com/gin-gonic/gin"
)

//go:embed templates
var templateFS embed.FS

type templates struct {
	dashboard *template.Template
	records   *template.Template
	keys      *template.Template
	settings  *template.Template
	logs      *template.Template
}

var tmplFuncs = template.FuncMap{
	"base": filepath.Base,
}

func newTemplates() (*templates, error) {
	dashboard, err := template.New("").Funcs(tmplFuncs).ParseFS(templateFS, "templates/layout.html", "templates/dashboard.html")
	if err != nil {
		return nil, fmt.Errorf("parse dashboard template: %w", err)
	}
	records, err := template.New("").Funcs(tmplFuncs).ParseFS(templateFS, "templates/layout.html", "templates/records.html")
	if err != nil {
		return nil, fmt.Errorf("parse records template: %w", err)
	}
	keys, err := template.New("").Funcs(tmplFuncs).ParseFS(templateFS, "templates/layout.html", "templates/keys.html")
	if err != nil {
		return nil, fmt.Errorf("parse keys template: %w", err)
	}
	settings, err := template.New("").Funcs(tmplFuncs).ParseFS(templateFS, "templates/layout.html", "templates/settings.html")
	if err != nil {
		return nil, fmt.Errorf("parse settings template: %w", err)
	}
	logs, err := template.New("").Funcs(tmplFuncs).ParseFS(templateFS, "templates/layout.html", "templates/logs.html")
	if err != nil {
		return nil, fmt.Errorf("parse logs template: %w", err)
	}
	return &templates{
		dashboard: dashboard,
		records:   records,
		keys:      keys,
		settings:  settings,
		logs:      logs,
	}, nil
}

type HTTPServer struct {
	statsUseCase port.StatsUseCase
	port         string
	tmpl         *templates
}

func NewHTTPServer(statsUseCase port.StatsUseCase, port string) *HTTPServer {
	tmpl, err := newTemplates()
	if err != nil {
		panic(fmt.Sprintf("failed to parse templates: %v", err))
	}
	return &HTTPServer{
		statsUseCase: statsUseCase,
		port:         port,
		tmpl:         tmpl,
	}
}

func (s *HTTPServer) Start() error {
	r := gin.Default()

	// JSON API routes (existing — kept intact)
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
		api.GET("/logs", s.apiLogs)
		// receive forwarded logs from other services
		api.POST("/internal/logs", s.receiveLog)
	}

	// Web UI routes
	r.GET("/", s.webDashboard)
	r.GET("/logs", s.webLogs)
	r.GET("/records", s.webRecords)
	r.POST("/records/retry", s.webRetry)
	r.POST("/records/re-embed", s.webReEmbed)
	r.GET("/keys", s.webKeys)
	r.POST("/keys", s.webAddKey)
	r.POST("/keys/:id/delete", s.webDeleteKey)
	r.POST("/keys/:id/activate", s.webActivateKey)
	r.POST("/keys/:id/disable", s.webDisableKey)
	r.POST("/keys/:id/reset-quota", s.webResetQuota)

	r.GET("/settings", s.webSettings)
	r.POST("/settings/dirs", s.webAddWatchDir)
	r.POST("/settings/dirs/:id/toggle", s.webToggleWatchDir)
	r.POST("/settings/dirs/:id/delete", s.webDeleteWatchDir)

	return r.Run(":" + s.port)
}

// logs API
func (s *HTTPServer) apiLogs(c *gin.Context) {
	limit := 200
	if l := c.Query("limit"); l != "" {
		if i, err := strconv.Atoi(l); err == nil {
			limit = i
		}
	}
	entries := logger.GetRecent(limit)
	// format entries for JSON
	out := make([]gin.H, 0, len(entries))
	for _, e := range entries {
		out = append(out, gin.H{"time": e.Time.Format("2006-01-02T15:04:05Z"), "level": e.Level, "message": e.Message})
	}
	c.JSON(http.StatusOK, out)
}

// receive forwarded logs from other services
func (s *HTTPServer) receiveLog(c *gin.Context) {
	var body struct {
		Time    string `json:"time"`
		Level   string `json:"level"`
		Message string `json:"message"`
	}
	if err := c.BindJSON(&body); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	// parse time if present
	t := time.Now().UTC()
	if body.Time != "" {
		if parsed, err := time.Parse(time.RFC3339, body.Time); err == nil {
			t = parsed
		}
	}
	logger.ReceiveRemote(t, body.Level, body.Message)
	c.Status(http.StatusNoContent)
}

func (s *HTTPServer) webLogs(c *gin.Context) {
	data := LogsData{CurrentPage: "logs"}
	c.Status(http.StatusOK)
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.logs.ExecuteTemplate(c.Writer, "layout", data); err != nil {
		_ = err
	}
}

// ─── JSON API handlers ───────────────────────────────────────────────────────

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

// ─── Web UI handlers ─────────────────────────────────────────────────────────

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
		_ = err // template already started writing
	}
}

func (s *HTTPServer) webRecords(c *gin.Context) {
	filter := c.Query("status")
	flash := c.Query("flash")
	_ = flash

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

func (s *HTTPServer) webKeys(c *gin.Context) {
	flash := c.Query("flash")
	msg := c.Query("msg")

	keys, err := s.statsUseCase.ListAPIKeys(c.Request.Context())
	if err != nil {
		c.String(http.StatusInternalServerError, "error: %v", err)
		return
	}

	flashMsg, flashOK := resolveFlash(flash, msg)

	data := KeysData{
		CurrentPage: "keys",
		Keys:        toAPIKeyResponses(keys),
		Flash:       flashMsg,
		FlashOK:     flashOK,
	}
	c.Status(http.StatusOK)
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.keys.ExecuteTemplate(c.Writer, "layout", data); err != nil {
		_ = err
	}
}

func (s *HTTPServer) webAddKey(c *gin.Context) {
	service := c.PostForm("service")
	keyValue := c.PostForm("key_value")
	if err := s.statsUseCase.AddApiKey(c.Request.Context(), service, keyValue); err != nil {
		c.Redirect(http.StatusSeeOther, "/keys?flash=error&msg="+encodeMsg(err.Error()))
		return
	}
	c.Redirect(http.StatusSeeOther, "/keys?flash=success")
}

func (s *HTTPServer) webDeleteKey(c *gin.Context) {
	id := parseID(c.Param("id"))
	if err := s.statsUseCase.DeleteAPIKey(c.Request.Context(), id); err != nil {
		c.Redirect(http.StatusSeeOther, "/keys?flash=error&msg="+encodeMsg(err.Error()))
		return
	}
	c.Redirect(http.StatusSeeOther, "/keys?flash=success")
}

func (s *HTTPServer) webActivateKey(c *gin.Context) {
	id := parseID(c.Param("id"))
	if err := s.statsUseCase.ActivateAPIKey(c.Request.Context(), id); err != nil {
		c.Redirect(http.StatusSeeOther, "/keys?flash=error&msg="+encodeMsg(err.Error()))
		return
	}
	c.Redirect(http.StatusSeeOther, "/keys?flash=success")
}

func (s *HTTPServer) webDisableKey(c *gin.Context) {
	id := parseID(c.Param("id"))
	if err := s.statsUseCase.DisableApiKey(c.Request.Context(), id); err != nil {
		c.Redirect(http.StatusSeeOther, "/keys?flash=error&msg="+encodeMsg(err.Error()))
		return
	}
	c.Redirect(http.StatusSeeOther, "/keys?flash=success")
}

func (s *HTTPServer) webResetQuota(c *gin.Context) {
	id := parseID(c.Param("id"))
	if err := s.statsUseCase.ResetQuotaApiKey(c.Request.Context(), id); err != nil {
		c.Redirect(http.StatusSeeOther, "/keys?flash=error&msg="+encodeMsg(err.Error()))
		return
	}
	c.Redirect(http.StatusSeeOther, "/keys?flash=success")
}

// ─── Settings handlers ───────────────────────────────────────────────────────

func (s *HTTPServer) webSettings(c *gin.Context) {
	flash := c.Query("flash")
	msg := c.Query("msg")

	dirs, err := s.statsUseCase.ListWatchDirs(c.Request.Context())
	if err != nil {
		c.String(http.StatusInternalServerError, "error: %v", err)
		return
	}

	flashMsg, flashOK := resolveFlash(flash, msg)
	data := SettingsData{
		CurrentPage: "settings",
		WatchDirs:   toWatchDirResponses(dirs),
		Flash:       flashMsg,
		FlashOK:     flashOK,
	}
	c.Status(http.StatusOK)
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = s.tmpl.settings.ExecuteTemplate(c.Writer, "layout", data)
}

func (s *HTTPServer) webAddWatchDir(c *gin.Context) {
	path := c.PostForm("path")
	if err := s.statsUseCase.AddWatchDir(c.Request.Context(), path); err != nil {
		c.Redirect(http.StatusSeeOther, "/settings?flash=error&msg="+encodeMsg(err.Error()))
		return
	}
	c.Redirect(http.StatusSeeOther, "/settings?flash=success")
}

func (s *HTTPServer) webToggleWatchDir(c *gin.Context) {
	id := parseID(c.Param("id"))
	if err := s.statsUseCase.ToggleWatchDir(c.Request.Context(), id); err != nil {
		c.Redirect(http.StatusSeeOther, "/settings?flash=error&msg="+encodeMsg(err.Error()))
		return
	}
	c.Redirect(http.StatusSeeOther, "/settings?flash=success")
}

func (s *HTTPServer) webDeleteWatchDir(c *gin.Context) {
	id := parseID(c.Param("id"))
	if err := s.statsUseCase.DeleteWatchDir(c.Request.Context(), id); err != nil {
		c.Redirect(http.StatusSeeOther, "/settings?flash=error&msg="+encodeMsg(err.Error()))
		return
	}
	c.Redirect(http.StatusSeeOther, "/settings?flash=success")
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func parseID(s string) int {
	var id int
	fmt.Sscanf(s, "%d", &id)
	return id
}

func encodeMsg(s string) string {
	return filepath.Base(s) // simple sanitize — strip path separators
}

func resolveFlash(flash, msg string) (string, bool) {
	switch flash {
	case "success":
		return "Operation completed successfully.", true
	case "error":
		if msg == "" {
			msg = "An error occurred."
		}
		return msg, false
	}
	return "", false
}
