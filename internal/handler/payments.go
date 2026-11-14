package handler

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"

	"qmart/internal/httpx"
	"qmart/internal/model"
	"qmart/internal/store"
)

func newReference() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "mock_" + hex.EncodeToString(b)
}

// CreatePayment initiates a (mock) payment for an order's total.
//
// @Summary      Create payment for an order
// @Tags         payments
// @Produce      json
// @Param        id   path      int64  true  "order id"
// @Success      201  {object}  model.Payment
// @Failure      404  {object}  httpx.ErrorResponse
// @Security     BearerAuth
// @Router       /orders/{id}/payments [post]
func (h *Handler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	orderID, ok := h.pathID(r, w)
	if !ok {
		return
	}
	order, err := h.Store.GetOrder(r.Context(), orderID)
	if errors.Is(err, store.ErrNotFound) {
		httpx.NotFound(w, "order not found")
		return
	}
	if err != nil {
		httpx.Internal(w, "failed to get order")
		return
	}
	if order.Status == model.OrderCancelled {
		httpx.Conflict(w, "cannot pay a cancelled order")
		return
	}

	payment, err := h.Store.CreatePayment(r.Context(), orderID, order.TotalCents, "mock", newReference())
	if err != nil {
		httpx.Internal(w, "failed to create payment")
		return
	}
	httpx.Data(w, http.StatusCreated, payment)
}

type webhookRequest struct {
	OrderID int64  `json:"order_id"`
	Status  string `json:"status"`
}

// PaymentWebhook simulates a provider callback. A real integration would
// verify the provider's signature before mutating state; here we settle the
// payment as "paid" when told to.
//
// @Summary      Payment webhook (simulated provider callback)
// @Tags         payments
// @Accept       json
// @Produce      json
// @Param        body  body      webhookRequest  true  "order + status"
// @Success      200   {object}  model.Payment
// @Failure      400   {object}  httpx.ErrorResponse
// @Router       /payments/webhook [post]
func (h *Handler) PaymentWebhook(w http.ResponseWriter, r *http.Request) {
	var req webhookRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.BadRequest(w, err.Error())
		return
	}
	if req.OrderID <= 0 {
		httpx.ValidationError(w, "order_id is required")
		return
	}

	switch req.Status {
	case model.PaymentPaid:
		if err := h.Store.SettlePayment(r.Context(), req.OrderID); err != nil {
			httpx.Internal(w, "failed to settle payment")
			return
		}
	default:
		httpx.ValidationError(w, "unsupported payment status")
		return
	}

	payment, err := h.Store.GetPaymentByOrder(r.Context(), req.OrderID)
	if err != nil {
		httpx.Internal(w, "failed to get payment")
		return
	}
	httpx.Data(w, http.StatusOK, payment)
}
