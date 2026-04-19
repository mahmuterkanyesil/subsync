package gin

import (
	"net/http"
	"sort"
	"subsync/internal/core/domain/valueobject"

	"github.com/gin-gonic/gin"
)

// ─── Watch Dirs / Settings — Web UI ──────────────────────────────────────────

func (s *HTTPServer) webSettings(c *gin.Context) {
	flash := c.Query("flash")
	msg := c.Query("msg")

	dirs, err := s.statsUseCase.ListWatchDirs(c.Request.Context())
	if err != nil {
		c.String(http.StatusInternalServerError, "error: %v", err)
		return
	}

	langs := make([]valueobject.LanguageSpec, 0, len(valueobject.SupportedLanguages))
	for _, spec := range valueobject.SupportedLanguages {
		langs = append(langs, spec)
	}
	// stable order: sort by Code
	sort.Slice(langs, func(i, j int) bool { return langs[i].Code < langs[j].Code })

	flashMsg, flashOK := resolveFlash(flash, msg)
	data := SettingsData{
		CurrentPage:        "settings",
		WatchDirs:          toWatchDirResponses(dirs),
		Flash:              flashMsg,
		FlashOK:            flashOK,
		TargetLanguage:     s.statsUseCase.GetTargetLanguage(c.Request.Context()),
		SupportedLanguages: langs,
	}
	c.Status(http.StatusOK)
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = s.tmpl.settings.ExecuteTemplate(c.Writer, "layout", data)
}

func (s *HTTPServer) webSetLanguage(c *gin.Context) {
	code := c.PostForm("lang_code")
	if err := s.statsUseCase.SetTargetLanguage(c.Request.Context(), code); err != nil {
		c.Redirect(http.StatusSeeOther, "/settings?flash=error&msg="+encodeMsg(err.Error()))
		return
	}
	c.Redirect(http.StatusSeeOther, "/settings?flash=success")
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
