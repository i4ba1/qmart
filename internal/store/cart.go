package store

import (
	"context"
	"database/sql"
	"fmt"

	"qmart/internal/model"
)

// ensureCart returns the cart id for a user, creating it on first access. The
// upsert is concurrency-safe: racing first requests converge on one cart.
func (s *Store) ensureCart(ctx context.Context, userID int64) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO carts (user_id, created_at, updated_at)
		VALUES ($1, $2, $2)
		ON CONFLICT (user_id) DO UPDATE SET user_id = EXCLUDED.user_id
		RETURNING id`, userID, now()).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("ensure cart: %w", err)
	}
	return id, nil
}

func (s *Store) AddCartItem(ctx context.Context, userID, productID, qty int64) error {
	cartID, err := s.ensureCart(ctx, userID)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO cart_items (cart_id, product_id, quantity, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $4)
		ON CONFLICT (cart_id, product_id) DO UPDATE SET
			quantity = cart_items.quantity + EXCLUDED.quantity,
			updated_at = EXCLUDED.updated_at`,
		cartID, productID, qty, now())
	if err != nil {
		return fmt.Errorf("add cart item: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `UPDATE carts SET updated_at = $1 WHERE id = $2`, now(), cartID)
	return err
}

func (s *Store) UpdateCartItem(ctx context.Context, userID, itemID, qty int64) error {
	if qty <= 0 {
		return s.RemoveCartItem(ctx, userID, itemID)
	}
	res, err := s.db.ExecContext(ctx, `
		UPDATE cart_items SET quantity = $1, updated_at = $2
		WHERE id = $3 AND cart_id = (SELECT id FROM carts WHERE user_id = $4)`,
		qty, now(), itemID, userID)
	if err != nil {
		return fmt.Errorf("update cart item: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) RemoveCartItem(ctx context.Context, userID, itemID int64) error {
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM cart_items WHERE id = $1 AND cart_id = (SELECT id FROM carts WHERE user_id = $2)`,
		itemID, userID)
	if err != nil {
		return fmt.Errorf("remove cart item: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetCart(ctx context.Context, userID int64) (*model.Cart, error) {
	cartID, err := s.ensureCart(ctx, userID)
	if err != nil {
		return nil, err
	}
	cart := &model.Cart{ID: cartID, UserID: userID, Items: make([]model.CartItem, 0)}

	rows, err := s.db.QueryContext(ctx, `
		SELECT ci.id, ci.product_id, p.name, p.sku, ci.quantity, p.price_cents
		FROM cart_items ci
		JOIN products p ON p.id = ci.product_id
		WHERE ci.cart_id = $1
		ORDER BY ci.id`, cartID)
	if err != nil {
		return nil, fmt.Errorf("get cart items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var it model.CartItem
		if err := rows.Scan(&it.ID, &it.ProductID, &it.Name, &it.SKU, &it.Quantity, &it.UnitPriceCents); err != nil {
			return nil, err
		}
		it.LineTotalCents = it.UnitPriceCents * it.Quantity
		cart.TotalCents += it.LineTotalCents
		cart.Items = append(cart.Items, it)
	}
	return cart, rows.Err()
}

// cartLine is the locked view of a cart used during checkout.
type cartLine struct {
	ProductID      int64
	Name           string
	SKU            string
	Quantity       int64
	UnitPriceCents int64
}

func (s *Store) cartLines(ctx context.Context, tx *sql.Tx, userID int64) ([]cartLine, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT ci.product_id, p.name, p.sku, ci.quantity, p.price_cents
		FROM cart_items ci
		JOIN products p ON p.id = ci.product_id
		JOIN carts c ON c.id = ci.cart_id
		WHERE c.user_id = $1 AND p.is_active = TRUE
		ORDER BY ci.id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	lines := make([]cartLine, 0)
	for rows.Next() {
		var l cartLine
		if err := rows.Scan(&l.ProductID, &l.Name, &l.SKU, &l.Quantity, &l.UnitPriceCents); err != nil {
			return nil, err
		}
		lines = append(lines, l)
	}
	return lines, rows.Err()
}
