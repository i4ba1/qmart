package handler

import (
	"net/http"
	"time"

	"qmart/internal/httpx"
)

// Health reports service liveness.
//
// @Summary      Health check
// @Tags         health
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /healthz [get]
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	httpx.Data(w, http.StatusOK, map[string]any{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}
