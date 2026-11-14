package handler

import (
	"errors"
	"net/http"
	"strings"

	"qmart/internal/httpx"
	"qmart/internal/model"
	"qmart/internal/store"
)

// ListProducts returns the active catalog with keyset pagination.
//
// @Summary      List products
// @Tags         products
// @Produce      json
// @Param        category  query     string  false  "filter by category"
// @Param        q         query     string  false  "case-insensitive name search"
// @Param        limit     query     int     false  "page size (max 100)"
// @Param        cursor    query     int     false  "keyset cursor (last id from previous page)"
// @Success      200       {array}   model.Product
// @Failure      500       {object}  httpx.ErrorResponse
// @Router       /products [get]
func (h *Handler) ListProducts(w http.ResponseWriter, r *http.Request) {
	page, err := h.Store.ListProducts(r.Context(), store.ProductFilter{
		Category: strings.TrimSpace(r.URL.Query().Get("category")),
		Query:    strings.TrimSpace(r.URL.Query().Get("q")),
		Limit:    h.queryInt(r, "limit", 20),
		Cursor:   int64(h.queryInt(r, "cursor", 0)),
	})
	if err != nil {
		httpx.Internal(w, "failed to list products")
		return
	}
	httpx.DataMeta(w, http.StatusOK, page.Items, map[string]any{
		"next_cursor": page.NextCursor,
		"has_more":    page.HasMore,
	})
}

// GetProduct returns a single product.
//
// @Summary      Get product
// @Tags         products
// @Produce      json
// @Param        id   path      int64  true  "product id"
// @Success      200  {object}  model.Product
// @Failure      404  {object}  httpx.ErrorResponse
// @Router       /products/{id} [get]
func (h *Handler) GetProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := h.pathID(r, w)
	if !ok {
		return
	}
	p, err := h.Store.GetProduct(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		httpx.NotFound(w, "product not found")
		return
	}
	if err != nil {
		httpx.Internal(w, "failed to get product")
		return
	}
	httpx.Data(w, http.StatusOK, p)
}

type productRequest struct {
	SKU         string `json:"sku"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	PriceCents  int64  `json:"price_cents"`
	Currency    string `json:"currency"`
	IsActive    *bool  `json:"is_active"`
}

// CreateProduct adds a product to the catalog.
//
// @Summary      Create product (admin)
// @Tags         products
// @Accept       json
// @Produce      json
// @Param        body  body      productRequest  true  "product"
// @Success      201   {object}  model.Product
// @Failure      400   {object}  httpx.ErrorResponse
// @Failure      403   {object}  httpx.ErrorResponse
// @Security     BearerAuth
// @Router       /products [post]
func (h *Handler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	var req productRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	if !validProduct(&req) {
		httpx.ValidationError(w, "sku, name and a non-negative price_cents are required")
		return
	}

	p := &model.Product{
		SKU:         req.SKU,
		Name:        req.Name,
		Description: req.Description,
		Category:    req.Category,
		PriceCents:  req.PriceCents,
		Currency:    currency(req.Currency),
		IsActive:    isActive(req.IsActive),
	}
	id, err := h.Store.CreateProduct(r.Context(), p)
	if err != nil {
		httpx.Internal(w, "failed to create product")
		return
	}
	created, err := h.Store.GetProduct(r.Context(), id)
	if err != nil {
		httpx.Internal(w, "failed to load product")
		return
	}
	httpx.Data(w, http.StatusCreated, created)
}

// UpdateProduct replaces a product's fields.
//
// @Summary      Update product (admin)
// @Tags         products
// @Accept       json
// @Produce      json
// @Param        id    path      int64           true  "product id"
// @Param        body  body      productRequest  true  "product"
// @Success      200   {object}  model.Product
// @Failure      400   {object}  httpx.ErrorResponse
// @Failure      404   {object}  httpx.ErrorResponse
// @Security     BearerAuth
// @Router       /products/{id} [put]
func (h *Handler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := h.pathID(r, w)
	if !ok {
		return
	}
	var req productRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	if !validProduct(&req) {
		httpx.ValidationError(w, "sku, name and a non-negative price_cents are required")
		return
	}

	p := &model.Product{
		ID:          id,
		SKU:         req.SKU,
		Name:        req.Name,
		Description: req.Description,
		Category:    req.Category,
		PriceCents:  req.PriceCents,
		Currency:    currency(req.Currency),
		IsActive:    isActive(req.IsActive),
	}
	if err := h.Store.UpdateProduct(r.Context(), p); errors.Is(err, store.ErrNotFound) {
		httpx.NotFound(w, "product not found")
		return
	} else if err != nil {
		httpx.Internal(w, "failed to update product")
		return
	}
	updated, err := h.Store.GetProduct(r.Context(), id)
	if err != nil {
		httpx.Internal(w, "failed to load product")
		return
	}
	httpx.Data(w, http.StatusOK, updated)
}

// DeleteProduct removes a product from the catalog.
//
// @Summary      Delete product (admin)
// @Tags         products
// @Produce      json
// @Param        id   path      int64  true  "product id"
// @Success      200  {object}  map[string]interface{}
// @Failure      404  {object}  httpx.ErrorResponse
// @Security     BearerAuth
// @Router       /products/{id} [delete]
func (h *Handler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := h.pathID(r, w)
	if !ok {
		return
	}
	if err := h.Store.DeleteProduct(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		httpx.NotFound(w, "product not found")
		return
	} else if err != nil {
		httpx.Internal(w, "failed to delete product")
		return
	}
	httpx.Data(w, http.StatusOK, map[string]any{"deleted": id})
}

func validProduct(req *productRequest) bool {
	return strings.TrimSpace(req.SKU) != "" && strings.TrimSpace(req.Name) != "" && req.PriceCents >= 0
}

func currency(c string) string {
	if c == "" {
		return "USD"
	}
	return c
}

func isActive(b *bool) bool {
	if b == nil {
		return true
	}
	return *b
}
