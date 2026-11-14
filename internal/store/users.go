package store

import (
	"context"
	"fmt"

	"qmart/internal/model"
)

// GetOrCreateUser finds a user by email or creates one with the given role.
// The upsert returns the existing row on conflict, so the role of an already
// known user is never silently changed.
func (s *Store) GetOrCreateUser(ctx context.Context, email, name, role string) (*model.User, error) {
	u := &model.User{Email: email, Name: name, Role: role}
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO users (email, name, role, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
		RETURNING id, email, name, role`,
		email, name, role, now()).Scan(&u.ID, &u.Email, &u.Name, &u.Role)
	if err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}
	return u, nil
}
