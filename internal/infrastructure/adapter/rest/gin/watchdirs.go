package gin

import (
	"net/http"

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
