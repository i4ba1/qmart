// Package model defines the domain types shared across the store, service and
// handler layers. Money is always represented as integer minor units (cents)
// to avoid floating-point rounding errors.
package model

import (
	"fmt"
	"time"
)

// Timestamp is a time.Time that knows how to scan from the TEXT timestamps
// stored by the SQLite driver, which does not convert strings to time.Time
// natively.
type Timestamp struct {
	time.Time
}

func (t *Timestamp) Scan(v any) error {
	switch s := v.(type) {
	case string:
		if s == "" {
			t.Time = time.Time{}
			return nil
		}
		parsed, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			return err
		}
		t.Time = parsed.UTC()
		return nil
	case time.Time:
		t.Time = s.UTC()
		return nil
	default:
		return fmt.Errorf("unsupported timestamp type %T", v)
	}
}

// Order status lifecycle.
const (
	OrderPending       = "pending"
	OrderConfirmed     = "confirmed"
	OrderPacked        = "packed"
	OrderOutForDeliver = "out_for_delivery"
	OrderDelivered     = "delivered"
	OrderCancelled     = "cancelled"
)

// Payment statuses.
const (
	PaymentPending  = "pending"
	PaymentPaid     = "paid"
	PaymentFailed   = "failed"
	PaymentRefunded = "refunded"
)

type Product struct {
	ID          int64     `json:"id"`
	SKU         string    `json:"sku"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	PriceCents  int64     `json:"price_cents"`
	Currency    string    `json:"currency"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   Timestamp `json:"created_at"`
	UpdatedAt   Timestamp `json:"updated_at"`
}

// InventoryLevel is the stock of a product in a single dark store.
type InventoryLevel struct {
	ProductID int64     `json:"product_id"`
	StoreID   string    `json:"store_id"`
	Quantity  int64     `json:"quantity"`
	Reserved  int64     `json:"reserved"`
	Available int64     `json:"available"`
	UpdatedAt Timestamp `json:"updated_at"`
}

type CartItem struct {
	ID             int64  `json:"id"`
	ProductID      int64  `json:"product_id"`
	Name           string `json:"name"`
	SKU            string `json:"sku"`
	Quantity       int64  `json:"quantity"`
	UnitPriceCents int64  `json:"unit_price_cents"`
	LineTotalCents int64  `json:"line_total_cents"`
}

type Cart struct {
	ID         int64      `json:"id"`
	UserID     int64      `json:"user_id"`
	Items      []CartItem `json:"items"`
	TotalCents int64      `json:"total_cents"`
}

type Order struct {
	ID               int64       `json:"id"`
	UserID           int64       `json:"user_id"`
	StoreID          string      `json:"store_id"`
	Status           string      `json:"status"`
	SubtotalCents    int64       `json:"subtotal_cents"`
	DeliveryFeeCents int64       `json:"delivery_fee_cents"`
	TotalCents       int64       `json:"total_cents"`
	Currency         string      `json:"currency"`
	Items            []OrderItem `json:"items,omitempty"`
	CreatedAt        Timestamp   `json:"created_at"`
	UpdatedAt        Timestamp   `json:"updated_at"`
}

type OrderItem struct {
	ID             int64  `json:"id"`
	ProductID      int64  `json:"product_id"`
	Name           string `json:"name"`
	SKU            string `json:"sku"`
	Quantity       int64  `json:"quantity"`
	UnitPriceCents int64  `json:"unit_price_cents"`
	TotalCents     int64  `json:"total_cents"`
}

type Payment struct {
	ID          int64     `json:"id"`
	OrderID     int64     `json:"order_id"`
	AmountCents int64     `json:"amount_cents"`
	Status      string    `json:"status"`
	Provider    string    `json:"provider"`
	Reference   string    `json:"reference"`
	CreatedAt   Timestamp `json:"created_at"`
	UpdatedAt   Timestamp `json:"updated_at"`
}

type User struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

// Roles.
const (
	RoleAdmin    = "admin"
	RoleCustomer = "customer"
)
