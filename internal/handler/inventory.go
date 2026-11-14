package handler

import (
	"net/http"
	"strings"

	"qmart/internal/httpx"
)

// GetInventory returns per-store stock for a product.
//
// @Summary      Get inventory for a product
// @Tags         inventory
// @Produce      json
// @Param        id   path      int64  true  "product id"
// @Success      200  {array}   model.InventoryLevel
// @Security     BearerAuth
// @Router       /inventory/{id} [get]
func (h *Handler) GetInventory(w http.ResponseWriter, r *http.Request) {
	id, ok := h.pathID(r, w)
	if !ok {
		return
	}
	levels, err := h.Store.GetInventory(r.Context(), id)
	if err != nil {
		httpx.Internal(w, "failed to get inventory")
		return
	}
	httpx.Data(w, http.StatusOK, levels)
}

type adjustInventoryRequest struct {
	ProductID     int64  `json:"product_id"`
	StoreID       string `json:"store_id"`
	QuantityDelta int64  `json:"quantity_delta"`
	ReservedDelta int64  `json:"reserved_delta"`
}

// AdjustInventory applies a delta to on-hand and reserved stock.
//
// @Summary      Adjust inventory (admin)
// @Tags         inventory
// @Accept       json
// @Produce      json
// @Param        body  body      adjustInventoryRequest  true  "stock deltas"
// @Success      200   {array}   model.InventoryLevel
// @Failure      400   {object}  httpx.ErrorResponse
// @Security     BearerAuth
// @Router       /inventory [post]
func (h *Handler) AdjustInventory(w http.ResponseWriter, r *http.Request) {
	var req adjustInventoryRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	if req.ProductID <= 0 || strings.TrimSpace(req.StoreID) == "" {
		httpx.ValidationError(w, "product_id and store_id are required")
		return
	}
	if err := h.Store.AdjustInventory(r.Context(), req.ProductID, req.StoreID, req.QuantityDelta, req.ReservedDelta); err != nil {
		httpx.Internal(w, "failed to adjust inventory")
		return
	}
	levels, err := h.Store.GetInventory(r.Context(), req.ProductID)
	if err != nil {
		httpx.Internal(w, "failed to get inventory")
		return
	}
	httpx.Data(w, http.StatusOK, levels)
}
