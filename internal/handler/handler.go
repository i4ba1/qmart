// Package handler contains the HTTP handlers, one file per resource. Handlers
// stay thin: they parse/validate input, call the store, and write responses.
package handler

import (
	"net/http"
	"strconv"
	"time"

	"qmart/internal/httpx"
	"qmart/internal/model"
	"qmart/internal/store"
)

type Handler struct {
	Store       *store.Store
	TokenSecret string
	TokenTTL    time.Duration
	AdminEmails map[string]bool
}

// roleFor returns the role a user is granted, promoting admin emails to admin.
func (h *Handler) roleFor(email string) string {
	if h.AdminEmails[email] {
		return model.RoleAdmin
	}
	return model.RoleCustomer
}

func (h *Handler) pathID(r *http.Request, w http.ResponseWriter) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		httpx.BadRequest(w, "invalid id")
		return 0, false
	}
	return id, true
}

func (h *Handler) queryInt(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
