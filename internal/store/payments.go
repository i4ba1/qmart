package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"qmart/internal/model"
)

// CreatePayment records a payment attempt for an order. The mock provider
// reference is a short random-ish string supplied by the caller.
func (s *Store) CreatePayment(ctx context.Context, orderID, amountCents int64, provider, reference string) (*model.Payment, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO payments (order_id, amount_cents, status, provider, reference, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
		RETURNING id`,
		orderID, amountCents, model.PaymentPending, provider, reference, now()).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("create payment: %w", err)
	}
	return s.GetPayment(ctx, id)
}

func (s *Store) GetPayment(ctx context.Context, id int64) (*model.Payment, error) {
	p := &model.Payment{}
	err := s.db.QueryRowContext(ctx, `
		SELECT id, order_id, amount_cents, status, provider, reference, created_at, updated_at
		FROM payments WHERE id = $1`, id).
		Scan(&p.ID, &p.OrderID, &p.AmountCents, &p.Status, &p.Provider, &p.Reference, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Store) GetPaymentByOrder(ctx context.Context, orderID int64) (*model.Payment, error) {
	p := &model.Payment{}
	err := s.db.QueryRowContext(ctx, `
		SELECT id, order_id, amount_cents, status, provider, reference, created_at, updated_at
		FROM payments WHERE order_id = $1`, orderID).
		Scan(&p.ID, &p.OrderID, &p.AmountCents, &p.Status, &p.Provider, &p.Reference, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

// SettlePayment marks a pending payment as paid and converts the order's
// reserved inventory into sold (decrements on-hand and reserved).
func (s *Store) SettlePayment(ctx context.Context, orderID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var storeID string
	if err := tx.QueryRowContext(ctx, `SELECT store_id FROM orders WHERE id = $1`, orderID).Scan(&storeID); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE payments SET status = $1, updated_at = $2 WHERE order_id = $3`,
		model.PaymentPaid, now(), orderID); err != nil {
		return err
	}

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
			UPDATE inventory SET quantity = GREATEST(0, quantity - $1), reserved = GREATEST(0, reserved - $1), updated_at = $2
			WHERE product_id = $3 AND store_id = $4`,
			r.qty, now(), r.productID, storeID); err != nil {
			return err
		}
	}
	return tx.Commit()
}
