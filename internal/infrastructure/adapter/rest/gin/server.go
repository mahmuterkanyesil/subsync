package gin

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"subsync/internal/core/application/port"
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

func (s *HTTPServer) Start(ctx context.Context) error {
	r := gin.Default()

	api := r.Group("/api")
	{
		api.GET("/health", s.health)
		api.GET("/stats", s.getStats)
		api.GET("/records", s.listRecords)
		api.GET("/records/:engPath", s.findByPath)
		api.POST("/records/:engPath/retry", s.reTranslate)
		api.POST("/records/:engPath/re-embed", s.reEmbed)
		api.POST("/keys", s.addApiKey)
		api.POST("/keys/:id/disable", s.disableApiKey)
		api.POST("/keys/:id/reset-quota", s.resetQuota)
		api.POST("/keys/:id/model", s.updateApiKeyModel)
		api.GET("/logs", s.apiLogs)
		api.POST("/internal/logs", s.receiveLog)
		api.GET("/prompts", s.getPrompt)
		api.POST("/prompts", s.setPrompt)
	}

	r.GET("/health", s.health)

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
	r.POST("/keys/:id/model", s.webUpdateKeyModel)
	r.GET("/settings", s.webSettings)
	r.POST("/settings/dirs", s.webAddWatchDir)
	r.POST("/settings/dirs/:id/toggle", s.webToggleWatchDir)
	r.POST("/settings/dirs/:id/delete", s.webDeleteWatchDir)

	srv := &http.Server{
		Addr:    ":" + s.port,
		Handler: r,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

// ─── shared helpers ───────────────────────────────────────────────────────────

func parseID(s string) int {
	var id int
	fmt.Sscanf(s, "%d", &id)
	return id
}

func encodeMsg(s string) string {
	return filepath.Base(s)
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
