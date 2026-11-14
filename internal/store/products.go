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

// ProductFilter drives the catalog list endpoint. Pagination uses keyset
// (cursor) pagination on the sort column rather than OFFSET, which stays fast
// at depth.
type ProductFilter struct {
	Category string
	Query    string
	Limit    int
	Cursor   int64 // last ID from the previous page
}

const defaultProductLimit = 20

type ProductPage struct {
	Items      []model.Product
	NextCursor int64
	HasMore    bool
}

func (s *Store) CreateProduct(ctx context.Context, p *model.Product) (int64, error) {
	ts := now()
	var id int64
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO products (sku, name, description, category, price_cents, currency, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
		RETURNING id`,
		p.SKU, p.Name, p.Description, p.Category, p.PriceCents, p.Currency, p.IsActive, ts).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert product: %w", err)
	}
	return id, nil
}

func (s *Store) GetProduct(ctx context.Context, id int64) (*model.Product, error) {
	p := &model.Product{}
	err := s.db.QueryRowContext(ctx, `
		SELECT id, sku, name, description, category, price_cents, currency, is_active, created_at, updated_at
		FROM products WHERE id = $1`, id).
		Scan(&p.ID, &p.SKU, &p.Name, &p.Description, &p.Category, &p.PriceCents,
			&p.Currency, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get product: %w", err)
	}
	return p, nil
}

func (s *Store) UpdateProduct(ctx context.Context, p *model.Product) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE products
		SET name = $1, description = $2, category = $3, price_cents = $4, currency = $5, is_active = $6, updated_at = $7
		WHERE id = $8`,
		p.Name, p.Description, p.Category, p.PriceCents, p.Currency, p.IsActive, now(), p.ID)
	if err != nil {
		return fmt.Errorf("update product: %w", err)
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

func (s *Store) DeleteProduct(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM products WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete product: %w", err)
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

func (s *Store) ListProducts(ctx context.Context, f ProductFilter) (*ProductPage, error) {
	if f.Limit <= 0 {
		f.Limit = defaultProductLimit
	}
	if f.Limit > 100 {
		f.Limit = 100
	}

	where := []string{"is_active = TRUE"}
	args := []any{}
	add := func(cond string, val any) {
		args = append(args, val)
		where = append(where, cond+" $"+strconv.Itoa(len(args)))
	}
	if f.Category != "" {
		add("category =", f.Category)
	}
	if f.Query != "" {
		add("name ILIKE", "%"+f.Query+"%")
	}
	if f.Cursor > 0 {
		add("id >", f.Cursor)
	}

	q := `SELECT id, sku, name, description, category, price_cents, currency, is_active, created_at, updated_at
		FROM products WHERE ` + strings.Join(where, " AND ") +
		` ORDER BY id ASC LIMIT $` + strconv.Itoa(len(args)+1)
	args = append(args, f.Limit+1) // fetch one extra to detect HasMore

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}
	defer rows.Close()

	page := &ProductPage{Items: make([]model.Product, 0, f.Limit)}
	for rows.Next() {
		var p model.Product
		if err := rows.Scan(&p.ID, &p.SKU, &p.Name, &p.Description, &p.Category,
			&p.PriceCents, &p.Currency, &p.IsActive, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan product: %w", err)
		}
		page.Items = append(page.Items, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(page.Items) > f.Limit {
		page.Items = page.Items[:f.Limit]
		page.HasMore = true
	}
	if n := len(page.Items); n > 0 {
		page.NextCursor = page.Items[n-1].ID
	}
	return page, nil
}
