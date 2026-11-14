package handler

import (
	"errors"
	"net/http"
	"strings"

	"qmart/internal/auth"
	"qmart/internal/httpx"
	"qmart/internal/store"
)

type createOrderRequest struct {
	StoreID          string `json:"store_id"`
	DeliveryFeeCents int64  `json:"delivery_fee_cents"`
}

// CreateOrder checks out the caller's cart, reserving inventory atomically.
//
// @Summary      Checkout cart into an order
// @Tags         orders
// @Accept       json
// @Produce      json
// @Param        body  body      createOrderRequest  true  "store + delivery fee"
// @Success      201   {object}  model.Order
// @Failure      409   {object}  httpx.ErrorResponse
// @Security     BearerAuth
// @Router       /orders [post]
func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var req createOrderRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	if strings.TrimSpace(req.StoreID) == "" {
		httpx.ValidationError(w, "store_id is required")
		return
	}
	if req.DeliveryFeeCents < 0 {
		httpx.ValidationError(w, "delivery_fee_cents cannot be negative")
		return
	}

	userID, _ := auth.UserID(r.Context())
	order, err := h.Store.PlaceOrder(r.Context(), userID, req.StoreID, req.DeliveryFeeCents)
	if errors.Is(err, store.ErrNotFound) {
		httpx.Conflict(w, "cart is empty")
		return
	}
	var stockErr *store.ErrInsufficientStock
	if errors.As(err, &stockErr) {
		httpx.Conflict(w, stockErr.Error())
		return
	}
	if err != nil {
		httpx.Internal(w, "failed to place order")
		return
	}
	httpx.Data(w, http.StatusCreated, order)
}

// ListOrders returns the caller's orders, newest first.
//
// @Summary      List orders
// @Tags         orders
// @Produce      json
// @Param        limit   query     int  false  "page size"
// @Param        cursor  query     int  false  "keyset cursor"
// @Success      200     {array}   model.Order
// @Security     BearerAuth
// @Router       /orders [get]
func (h *Handler) ListOrders(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	orders, next, hasMore, err := h.Store.ListOrders(r.Context(), userID, h.queryInt(r, "limit", 20), int64(h.queryInt(r, "cursor", 0)))
	if err != nil {
		httpx.Internal(w, "failed to list orders")
		return
	}
	httpx.DataMeta(w, http.StatusOK, orders, map[string]any{
		"next_cursor": next,
		"has_more":    hasMore,
	})
}

// GetOrder returns an order and its items.
//
// @Summary      Get order
// @Tags         orders
// @Produce      json
// @Param        id   path      int64  true  "order id"
// @Success      200  {object}  model.Order
// @Failure      404  {object}  httpx.ErrorResponse
// @Security     BearerAuth
// @Router       /orders/{id} [get]
func (h *Handler) GetOrder(w http.ResponseWriter, r *http.Request) {
	id, ok := h.pathID(r, w)
	if !ok {
		return
	}
	order, err := h.Store.GetOrder(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		httpx.NotFound(w, "order not found")
		return
	}
	if err != nil {
		httpx.Internal(w, "failed to get order")
		return
	}
	httpx.Data(w, http.StatusOK, order)
}

type updateOrderStatusRequest struct {
	Status string `json:"status"`
}

// UpdateOrderStatus advances the order lifecycle (admin only).
//
// @Summary      Update order status (admin)
// @Tags         orders
// @Accept       json
// @Produce      json
// @Param        id    path      int64                   true  "order id"
// @Param        body  body      updateOrderStatusRequest  true  "target status"
// @Success      200   {object}  model.Order
// @Failure      409   {object}  httpx.ErrorResponse
// @Security     BearerAuth
// @Router       /orders/{id}/status [post]
func (h *Handler) UpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := h.pathID(r, w)
	if !ok {
		return
	}
	var req updateOrderStatusRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	switch err := h.Store.UpdateOrderStatus(r.Context(), id, req.Status); {
	case errors.Is(err, store.ErrNotFound):
		httpx.NotFound(w, "order not found")
		return
	case errors.Is(err, store.ErrInvalidTransition):
		httpx.Conflict(w, err.Error())
		return
	case err != nil:
		httpx.Internal(w, "failed to update order status")
		return
	}
	order, err := h.Store.GetOrder(r.Context(), id)
	if err != nil {
		httpx.Internal(w, "failed to get order")
		return
	}
	httpx.Data(w, http.StatusOK, order)
}
