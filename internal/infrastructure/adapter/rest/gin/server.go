package gin

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
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
	"truncate": func(s string, n int) string {
		if len(s) <= n {
			return s
		}
		return s[:n] + "…"
	},
	"currentPage": func(offset, limit int) int {
		return offset/limit + 1
	},
	"buildQuery": func(status, q, sortBy, order string, limit, offset int) template.URL {
		v := url.Values{}
		if status != "" {
			v.Set("status", status)
		}
		if q != "" {
			v.Set("q", q)
		}
		v.Set("sort", sortBy)
		v.Set("order", order)
		v.Set("limit", fmt.Sprintf("%d", limit))
		v.Set("offset", fmt.Sprintf("%d", offset))
		return template.URL(v.Encode())
	},
	"sortHeader": func(col, label, currentSort, currentOrder, status, q string, limit, offset int) template.HTML {
		indicator := ""
		nextOrder := "ASC"
		if col == currentSort {
			if currentOrder == "ASC" {
				indicator = " ↑"
				nextOrder = "DESC"
			} else {
				indicator = " ↓"
				nextOrder = "ASC"
			}
		}
		v := url.Values{}
		if status != "" {
			v.Set("status", status)
		}
		if q != "" {
			v.Set("q", q)
		}
		v.Set("sort", col)
		v.Set("order", nextOrder)
		v.Set("limit", fmt.Sprintf("%d", limit))
		v.Set("offset", "0")
		href := "/records?" + v.Encode()
		return template.HTML(fmt.Sprintf(`<a href="%s" data-sort="%s" style="color:inherit;text-decoration:none">%s%s</a>`, href, col, label, indicator))
	},
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
	statsUseCase      port.StatsUseCase
	port              string
	tmpl              *templates
	dashboardUsername string
	dashboardPassword string
	internalLogToken  string
	db                *sql.DB
	redisURL          string
}

func NewHTTPServer(statsUseCase port.StatsUseCase, port, username, password, logToken string) *HTTPServer {
	tmpl, err := newTemplates()
	if err != nil {
		panic(fmt.Sprintf("failed to parse templates: %v", err))
	}
	return &HTTPServer{
		statsUseCase:      statsUseCase,
		port:              port,
		tmpl:              tmpl,
		dashboardUsername: username,
		dashboardPassword: password,
		internalLogToken:  logToken,
	}
}

func (s *HTTPServer) WithDB(db *sql.DB) *HTTPServer       { s.db = db; return s }
func (s *HTTPServer) WithRedis(u string) *HTTPServer      { s.redisURL = u; return s }

func (s *HTTPServer) deepHealth(c *gin.Context) {
	type check struct {
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}
	type response struct {
		OK       bool             `json:"ok"`
		Database check            `json:"database"`
		Redis    check            `json:"redis"`
		APIKeys  map[string]check `json:"api_keys"`
	}

	resp := response{APIKeys: map[string]check{}}

	// DB ping
	if s.db != nil {
		if err := s.db.PingContext(c.Request.Context()); err != nil {
			resp.Database = check{Error: err.Error()}
		} else {
			resp.Database = check{OK: true}
		}
	} else {
		resp.Database = check{OK: true}
	}

	// Redis TCP check
	if s.redisURL != "" {
		u, err := url.Parse(s.redisURL)
		host := u.Host
		if err != nil || host == "" {
			resp.Redis = check{Error: "invalid redis url"}
		} else {
			conn, err := net.DialTimeout("tcp", host, 3*time.Second)
			if err != nil {
				resp.Redis = check{Error: err.Error()}
			} else {
				conn.Close()
				resp.Redis = check{OK: true}
			}
		}
	} else {
		resp.Redis = check{OK: true}
	}

	// API key status
	keys, err := s.statsUseCase.ListAPIKeys(c.Request.Context())
	if err != nil {
		resp.APIKeys["error"] = check{Error: err.Error()}
	} else {
		activeCount, quotaCount := 0, 0
		for _, k := range keys {
			if k.IsActive() {
				activeCount++
			}
			if k.IsQuotaExceeded() {
				quotaCount++
			}
		}
		resp.APIKeys["total"] = check{OK: true, Error: fmt.Sprintf("%d total", len(keys))}
		resp.APIKeys["active"] = check{OK: activeCount > 0, Error: fmt.Sprintf("%d active", activeCount)}
		resp.APIKeys["quota_exhausted"] = check{OK: quotaCount == 0, Error: fmt.Sprintf("%d quota exhausted", quotaCount)}
	}

	resp.OK = resp.Database.OK && resp.Redis.OK
	status := http.StatusOK
	if !resp.OK {
		status = http.StatusServiceUnavailable
	}
	c.JSON(status, resp)
}

func (s *HTTPServer) basicAuthMiddleware() gin.HandlerFunc {
	return gin.BasicAuth(gin.Accounts{s.dashboardUsername: s.dashboardPassword})
}

func (s *HTTPServer) internalTokenMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.internalLogToken != "" && c.GetHeader("X-Internal-Token") != s.internalLogToken {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Next()
	}
}

func (s *HTTPServer) Start(ctx context.Context) error {
	r := gin.Default()

	// /health ve /api/internal/logs auth dışında
	r.GET("/health", s.health)
	r.GET("/api/health/deep", s.deepHealth)
	r.POST("/api/internal/logs", s.internalTokenMiddleware(), s.receiveLog)

	var protected gin.IRoutes
	if s.dashboardUsername != "" {
		protected = r.Group("/", s.basicAuthMiddleware())
	} else {
		protected = r.Group("/")
	}

	protected.GET("/api/health", s.health)
	protected.GET("/api/stats", s.getStats)
	protected.GET("/api/records", s.listRecords)
	protected.GET("/api/records/preview", s.getTranslationPreview)
	protected.GET("/api/records/:engPath", s.findByPath)
	protected.POST("/api/records/:engPath/retry", s.reTranslate)
	protected.POST("/api/records/:engPath/re-embed", s.reEmbed)
	protected.POST("/api/keys", s.addApiKey)
	protected.POST("/api/keys/:id/disable", s.disableApiKey)
	protected.POST("/api/keys/:id/reset-quota", s.resetQuota)
	protected.POST("/api/keys/:id/model", s.updateApiKeyModel)
	protected.GET("/api/logs", s.apiLogs)
	protected.GET("/api/prompts", s.getPrompt)
	protected.POST("/api/prompts", s.setPrompt)

	protected.GET("/", s.webDashboard)
	protected.GET("/logs", s.webLogs)
	protected.GET("/records", s.webRecords)
	protected.POST("/records/retry", s.webRetry)
	protected.POST("/records/re-embed", s.webReEmbed)
	protected.POST("/records/delete", s.webDeleteRecord)
	protected.POST("/records/bulk-retry", s.webBulkRetry)
	protected.POST("/records/bulk-delete", s.webBulkDelete)
	protected.GET("/keys", s.webKeys)
	protected.POST("/keys", s.webAddKey)
	protected.POST("/keys/:id/delete", s.webDeleteKey)
	protected.POST("/keys/:id/activate", s.webActivateKey)
	protected.POST("/keys/:id/disable", s.webDisableKey)
	protected.POST("/keys/:id/reset-quota", s.webResetQuota)
	protected.POST("/keys/:id/model", s.webUpdateKeyModel)
	protected.GET("/settings", s.webSettings)
	protected.POST("/settings/language", s.webSetLanguage)
	protected.POST("/settings/model-priority", s.webSetModelPriority)
	protected.POST("/settings/dirs", s.webAddWatchDir)
	protected.POST("/settings/dirs/:id/toggle", s.webToggleWatchDir)
	protected.POST("/settings/dirs/:id/delete", s.webDeleteWatchDir)

	srv := &http.Server{
		Addr:    ":" + s.port,
		Handler: r,
	}

	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = s.statsUseCase.RefreshKeyStatuses(context.Background())
			case <-ctx.Done():
				return
			}
		}
	}()

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
	return strings.NewReplacer(" ", "+", "&", "", "=", "", "?", "").Replace(s)
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
