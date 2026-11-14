package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"qmart/internal/model"
)

// ErrInvalidTransition is returned when an order status move is disallowed.
var ErrInvalidTransition = errors.New("invalid order status transition")

// ErrInsufficientStock is returned when checkout cannot reserve enough stock.
type ErrInsufficientStock struct {
	SKU       string
	Requested int64
	Available int64
}

func (e *ErrInsufficientStock) Error() string {
	return fmt.Sprintf("insufficient stock for %s: requested %d, available %d",
		e.SKU, e.Requested, e.Available)
}

// PlaceOrder atomically converts a user's cart into an order:
//  1. snapshots cart lines with current prices,
//  2. reserves inventory per item at the target store,
//  3. inserts the order and its items,
//  4. clears the cart.
//
// Everything runs in a single transaction so a failure anywhere rolls back the
// whole checkout.
func (s *Store) PlaceOrder(ctx context.Context, userID int64, storeID string, deliveryFeeCents int64) (*model.Order, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	lines, err := s.cartLines(ctx, tx, userID)
	if err != nil {
		return nil, fmt.Errorf("load cart lines: %w", err)
	}
	if len(lines) == 0 {
		return nil, ErrNotFound // no cart items
	}

	var subtotal int64
	for i := range lines {
		subtotal += lines[i].UnitPriceCents * lines[i].Quantity
	}

	// Reserve inventory. The WHERE guard makes overselling impossible without
	// an explicit lock.
	for _, l := range lines {
		res, err := tx.ExecContext(ctx, `
			UPDATE inventory
			SET reserved = reserved + $1, updated_at = $2
			WHERE product_id = $3 AND store_id = $4 AND quantity - reserved >= $1`,
			l.Quantity, now(), l.ProductID, storeID)
		if err != nil {
			return nil, fmt.Errorf("reserve inventory: %w", err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return nil, err
		}
		if n == 0 {
			avail, _ := s.GetInventoryAtStore(ctx, l.ProductID, storeID)
			return nil, &ErrInsufficientStock{SKU: l.SKU, Requested: l.Quantity, Available: avail}
		}
	}

	total := subtotal + deliveryFeeCents

	orderID, err := s.insertOrder(ctx, tx, userID, storeID, model.OrderPending, subtotal, deliveryFeeCents, total)
	if err != nil {
		return nil, err
	}
	for _, l := range lines {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO order_items (order_id, product_id, name, sku, quantity, unit_price_cents, total_cents)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			orderID, l.ProductID, l.Name, l.SKU, l.Quantity, l.UnitPriceCents, l.UnitPriceCents*l.Quantity); err != nil {
			return nil, fmt.Errorf("insert order item: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM cart_items WHERE cart_id = (SELECT id FROM carts WHERE user_id = $1)`, userID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return s.GetOrder(ctx, orderID)
}

func (s *Store) insertOrder(ctx context.Context, tx *sql.Tx, userID int64, storeID, status string, subtotal, delivery, total int64) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `
		INSERT INTO orders (user_id, store_id, status, subtotal_cents, delivery_fee_cents, total_cents, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
		RETURNING id`,
		userID, storeID, status, subtotal, delivery, total, now()).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert order: %w", err)
	}
	return id, nil
}

func (s *Store) GetOrder(ctx context.Context, id int64) (*model.Order, error) {
	o := &model.Order{}
	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, store_id, status, subtotal_cents, delivery_fee_cents, total_cents, currency, created_at, updated_at
		FROM orders WHERE id = $1`, id).
		Scan(&o.ID, &o.UserID, &o.StoreID, &o.Status, &o.SubtotalCents, &o.DeliveryFeeCents,
			&o.TotalCents, &o.Currency, &o.CreatedAt, &o.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get order: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, product_id, name, sku, quantity, unit_price_cents, total_cents
		FROM order_items WHERE order_id = $1 ORDER BY id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	o.Items = make([]model.OrderItem, 0)
	for rows.Next() {
		var it model.OrderItem
		if err := rows.Scan(&it.ID, &it.ProductID, &it.Name, &it.SKU, &it.Quantity, &it.UnitPriceCents, &it.TotalCents); err != nil {
			return nil, err
		}
		o.Items = append(o.Items, it)
	}
	return o, rows.Err()
}

func (s *Store) ListOrders(ctx context.Context, userID int64, limit int, cursor int64) ([]model.Order, int64, bool, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	where := []string{"user_id = $1"}
	args := []any{userID}
	if cursor > 0 {
		args = append(args, cursor)
		where = append(where, "id < $"+strconv.Itoa(len(args)))
	}
	q := `SELECT id, user_id, store_id, status, subtotal_cents, delivery_fee_cents, total_cents, currency, created_at, updated_at
		FROM orders WHERE ` + strings.Join(where, " AND ") +
		` ORDER BY id DESC LIMIT $` + strconv.Itoa(len(args)+1)
	args = append(args, limit+1)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, false, fmt.Errorf("list orders: %w", err)
	}
	defer rows.Close()

	orders := make([]model.Order, 0, limit)
	for rows.Next() {
		var o model.Order
		if err := rows.Scan(&o.ID, &o.UserID, &o.StoreID, &o.Status, &o.SubtotalCents,
			&o.DeliveryFeeCents, &o.TotalCents, &o.Currency, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, 0, false, err
		}
		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, false, err
	}

	hasMore := len(orders) > limit
	if hasMore {
		orders = orders[:limit]
	}
	var next int64
	if n := len(orders); n > 0 {
		next = orders[n-1].ID
	}
	return orders, next, hasMore, nil
}

// allowedTransitions enforces the order status lifecycle. Terminal states
// (delivered, cancelled) have no entry.
var allowedTransitions = map[string][]string{
	model.OrderPending:       {model.OrderConfirmed, model.OrderCancelled},
	model.OrderConfirmed:     {model.OrderPacked, model.OrderCancelled},
	model.OrderPacked:        {model.OrderOutForDeliver, model.OrderCancelled},
	model.OrderOutForDeliver: {model.OrderDelivered},
}

// UpdateOrderStatus transitions an order to the target status, validating the
// move against allowedTransitions. Cancelling releases any reserved inventory.
func (s *Store) UpdateOrderStatus(ctx context.Context, id int64, status string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var current, storeID string
	err = tx.QueryRowContext(ctx, `SELECT status, store_id FROM orders WHERE id = $1`, id).Scan(&current, &storeID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	allowed, ok := allowedTransitions[current]
	if !ok {
		return ErrInvalidTransition
	}
	valid := false
	for _, a := range allowed {
		if a == status {
			valid = true
			break
		}
	}
	if !valid {
		return ErrInvalidTransition
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE orders SET status = $1, updated_at = $2 WHERE id = $3`, status, now(), id); err != nil {
		return err
	}

	if status == model.OrderCancelled {
		if err := s.releaseInventory(ctx, tx, id, storeID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// releaseInventory returns reserved stock to available for the order's items.
func (s *Store) releaseInventory(ctx context.Context, tx *sql.Tx, orderID int64, storeID string) error {
	rows, err := tx.QueryContext(ctx,
		`SELECT product_id, quantity FROM order_items WHERE order_id = $1`, orderID)
	if err != nil {
		return err
	}
	type rel struct{ productID, qty int64 }
	var rels []rel
	for rows.Next() {
		var r rel
		if err := rows.Scan(&r.productID, &r.qty); err != nil {
			rows.Close()
			return err
		}
		rels = append(rels, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, r := range rels {
		if _, err := tx.ExecContext(ctx, `
			UPDATE inventory SET reserved = GREATEST(0, reserved - $1), updated_at = $2
			WHERE product_id = $3 AND store_id = $4`,
			r.qty, now(), r.productID, storeID); err != nil {
			return err
		}
	}
	return nil
}
