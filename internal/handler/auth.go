package handler

import (
	"net/http"
	"strings"
	"time"

	"qmart/internal/auth"
	"qmart/internal/httpx"
)

type tokenRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

// Token issues a signed bearer token for a demo user. This is intentionally
// passwordless for the showcase; a real service would exchange credentials or
// an OIDC flow here.
//
// @Summary      Issue an access token
// @Description  Creates (or fetches) the user and returns a signed bearer token. Admin emails (QMART_ADMIN_EMAILS) receive the admin role.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      tokenRequest  true  "email + name"
// @Success      200   {object}  map[string]interface{}
// @Failure      400   {object}  httpx.ErrorResponse
// @Router       /auth/token [post]
func (h *Handler) Token(w http.ResponseWriter, r *http.Request) {
	var req tokenRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || !strings.Contains(req.Email, "@") {
		httpx.ValidationError(w, "a valid email is required")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		req.Name = req.Email
	}

	u, err := h.Store.GetOrCreateUser(r.Context(), req.Email, req.Name, h.roleFor(req.Email))
	if err != nil {
		httpx.Internal(w, "failed to create user")
		return
	}

	token, err := auth.Sign(h.TokenSecret, auth.Claims{
		UserID: u.ID,
		Email:  u.Email,
		Role:   u.Role,
		Exp:    time.Now().Add(h.TokenTTL).Unix(),
	})
	if err != nil {
		httpx.Internal(w, "failed to sign token")
		return
	}

	httpx.Data(w, http.StatusOK, map[string]any{"token": token, "user": u})
}
