package gin

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ─── API Keys — JSON API ──────────────────────────────────────────────────────

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

// ─── API Keys — Web UI ────────────────────────────────────────────────────────

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
