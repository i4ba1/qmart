// Package server wires the HTTP routes and middleware stack.
package server

import (
	"log/slog"
	"net/http"

	httpSwagger "github.com/swaggo/http-swagger"

	"qmart/internal/auth"
	"qmart/internal/config"
	"qmart/internal/handler"
	"qmart/internal/middleware"
	"qmart/internal/model"
	"qmart/internal/store"
)

func New(cfg config.Config, st *store.Store, log *slog.Logger) http.Handler {
	admins := make(map[string]bool, len(cfg.AdminEmails))
	for _, e := range cfg.AdminEmails {
		admins[e] = true
	}

	h := &handler.Handler{
		Store:       st,
		TokenSecret: cfg.TokenSecret,
		TokenTTL:    cfg.TokenTTL,
		AdminEmails: admins,
	}

	admin := auth.RequireRole(model.RoleAdmin)

	mux := http.NewServeMux()

	// Public endpoints.
	mux.HandleFunc("GET /healthz", h.Health)
	mux.HandleFunc("POST /api/v1/auth/token", h.Token)
	mux.HandleFunc("GET /api/v1/products", h.ListProducts)
	mux.HandleFunc("GET /api/v1/products/{id}", h.GetProduct)
	mux.HandleFunc("POST /api/v1/payments/webhook", h.PaymentWebhook)

	// Authenticated endpoints.
	authed := http.NewServeMux()
	authed.HandleFunc("GET /api/v1/inventory/{id}", h.GetInventory)
	authed.HandleFunc("GET /api/v1/cart", h.GetCart)
	authed.HandleFunc("POST /api/v1/cart/items", h.AddCartItem)
	authed.HandleFunc("PUT /api/v1/cart/items/{id}", h.UpdateCartItem)
	authed.HandleFunc("DELETE /api/v1/cart/items/{id}", h.RemoveCartItem)
	authed.HandleFunc("POST /api/v1/orders", h.CreateOrder)
	authed.HandleFunc("GET /api/v1/orders", h.ListOrders)
	authed.HandleFunc("GET /api/v1/orders/{id}", h.GetOrder)
	authed.HandleFunc("POST /api/v1/orders/{id}/payments", h.CreatePayment)

	// Admin-only endpoints.
	authed.Handle("POST /api/v1/products", admin(http.HandlerFunc(h.CreateProduct)))
	authed.Handle("PUT /api/v1/products/{id}", admin(http.HandlerFunc(h.UpdateProduct)))
	authed.Handle("DELETE /api/v1/products/{id}", admin(http.HandlerFunc(h.DeleteProduct)))
	authed.Handle("POST /api/v1/inventory", admin(http.HandlerFunc(h.AdjustInventory)))
	authed.Handle("POST /api/v1/orders/{id}/status", admin(http.HandlerFunc(h.UpdateOrderStatus)))

	mux.Handle("/api/v1/", auth.Middleware(cfg.TokenSecret)(authed))

	// Swagger UI.
	mux.Handle("/swagger/", httpSwagger.WrapHandler)

	return middleware.RequestID(
		middleware.Logger(log)(
			middleware.Recover(log)(mux),
		),
	)
}
