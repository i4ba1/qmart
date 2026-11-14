package handler

import (
	"errors"
	"net/http"

	"qmart/internal/auth"
	"qmart/internal/httpx"
	"qmart/internal/store"
)

// GetCart returns the caller's cart with computed totals.
//
// @Summary      Get cart
// @Tags         cart
// @Produce      json
// @Success      200  {object}  model.Cart
// @Security     BearerAuth
// @Router       /cart [get]
func (h *Handler) GetCart(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.UserID(r.Context())
	cart, err := h.Store.GetCart(r.Context(), userID)
	if err != nil {
		httpx.Internal(w, "failed to get cart")
		return
	}
	httpx.Data(w, http.StatusOK, cart)
}

type addCartItemRequest struct {
	ProductID int64 `json:"product_id"`
	Quantity  int64 `json:"quantity"`
}

// AddCartItem adds a product to the cart (increments if present).
//
// @Summary      Add item to cart
// @Tags         cart
// @Accept       json
// @Produce      json
// @Param        body  body      addCartItemRequest  true  "product + quantity"
// @Success      200   {object}  model.Cart
// @Failure      400   {object}  httpx.ErrorResponse
// @Security     BearerAuth
// @Router       /cart/items [post]
func (h *Handler) AddCartItem(w http.ResponseWriter, r *http.Request) {
	var req addCartItemRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	if req.ProductID <= 0 || req.Quantity <= 0 {
		httpx.ValidationError(w, "product_id and a positive quantity are required")
		return
	}
	userID, _ := auth.UserID(r.Context())
	if err := h.Store.AddCartItem(r.Context(), userID, req.ProductID, req.Quantity); err != nil {
		httpx.Internal(w, "failed to add cart item")
		return
	}
	h.GetCart(w, r)
}

type updateCartItemRequest struct {
	Quantity int64 `json:"quantity"`
}

// UpdateCartItem sets an item's quantity (0 removes it).
//
// @Summary      Update cart item
// @Tags         cart
// @Accept       json
// @Produce      json
// @Param        id    path      int64                true  "cart item id"
// @Param        body  body      updateCartItemRequest  true  "quantity"
// @Success      200   {object}  model.Cart
// @Failure      404   {object}  httpx.ErrorResponse
// @Security     BearerAuth
// @Router       /cart/items/{id} [put]
func (h *Handler) UpdateCartItem(w http.ResponseWriter, r *http.Request) {
	itemID, ok := h.pathID(r, w)
	if !ok {
		return
	}
	var req updateCartItemRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	userID, _ := auth.UserID(r.Context())
	if err := h.Store.UpdateCartItem(r.Context(), userID, itemID, req.Quantity); errors.Is(err, store.ErrNotFound) {
		httpx.NotFound(w, "cart item not found")
		return
	} else if err != nil {
		httpx.Internal(w, "failed to update cart item")
		return
	}
	h.GetCart(w, r)
}

// RemoveCartItem removes an item from the cart.
//
// @Summary      Remove cart item
// @Tags         cart
// @Produce      json
// @Param        id   path      int64  true  "cart item id"
// @Success      200  {object}  model.Cart
// @Failure      404  {object}  httpx.ErrorResponse
// @Security     BearerAuth
// @Router       /cart/items/{id} [delete]
func (h *Handler) RemoveCartItem(w http.ResponseWriter, r *http.Request) {
	itemID, ok := h.pathID(r, w)
	if !ok {
		return
	}
	userID, _ := auth.UserID(r.Context())
	if err := h.Store.RemoveCartItem(r.Context(), userID, itemID); errors.Is(err, store.ErrNotFound) {
		httpx.NotFound(w, "cart item not found")
		return
	} else if err != nil {
		httpx.Internal(w, "failed to remove cart item")
		return
	}
	h.GetCart(w, r)
}
