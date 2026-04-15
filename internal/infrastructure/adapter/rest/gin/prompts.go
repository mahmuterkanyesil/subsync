package gin

import (
	"net/http"
	"subsync/internal/infrastructure/adapter/persistence/sqlite"

	"github.com/gin-gonic/gin"
)

// ─── Prompts (system instruction persistence) — JSON API ─────────────────────

func (s *HTTPServer) getPrompt(c *gin.Context) {
	instr, err := sqlite.ReadPrompt()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"system_instruction": instr})
}

func (s *HTTPServer) setPrompt(c *gin.Context) {
	var req struct {
		SystemInstruction string `json:"system_instruction"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := sqlite.WritePrompt(req.SystemInstruction); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
