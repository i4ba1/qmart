package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"qmart/internal/model"
)

// AdjustInventory applies a delta to on-hand quantity and reserved count for a
// product at a store. Rows are created on first write via upsert.
func (s *Store) AdjustInventory(ctx context.Context, productID int64, storeID string, qtyDelta, reservedDelta int64) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO inventory (product_id, store_id, quantity, reserved, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (product_id, store_id) DO UPDATE SET
			quantity = GREATEST(0, inventory.quantity + EXCLUDED.quantity),
			reserved = GREATEST(0, inventory.reserved + EXCLUDED.reserved),
			updated_at = EXCLUDED.updated_at`,
		productID, storeID, qtyDelta, reservedDelta, now())
	if err != nil {
		return fmt.Errorf("adjust inventory: %w", err)
	}
	return nil
}

func (s *Store) GetInventory(ctx context.Context, productID int64) ([]model.InventoryLevel, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT product_id, store_id, quantity, reserved, updated_at
		FROM inventory WHERE product_id = $1 ORDER BY store_id`, productID)
	if err != nil {
		return nil, fmt.Errorf("get inventory: %w", err)
	}
	defer rows.Close()

	levels := make([]model.InventoryLevel, 0)
	for rows.Next() {
		var l model.InventoryLevel
		if err := rows.Scan(&l.ProductID, &l.StoreID, &l.Quantity, &l.Reserved, &l.UpdatedAt); err != nil {
			return nil, err
		}
		l.Available = l.Quantity - l.Reserved
		levels = append(levels, l)
	}
	return levels, rows.Err()
}

// GetInventoryAtStore returns available stock for a product at a store.
func (s *Store) GetInventoryAtStore(ctx context.Context, productID int64, storeID string) (available int64, err error) {
	var quantity, reserved int64
	err = s.db.QueryRowContext(ctx, `
		SELECT quantity, reserved FROM inventory WHERE product_id = $1 AND store_id = $2`,
		productID, storeID).Scan(&quantity, &reserved)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return quantity - reserved, nil
}
