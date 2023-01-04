//go:build go1.18
// +build go1.18

package sqluct

import "context"

// Get retrieves a single row from database storage.
func Get[V any](ctx context.Context, s *Storage, qb ToSQL) (V, error) {
	var v V

	err := s.Select(ctx, qb, &v)

	return v, err
}

// List retrieves a collection of rows from database storage.
func List[V any](ctx context.Context, s *Storage, qb ToSQL) ([]V, error) {
	var v []V

	err := s.Select(ctx, qb, &v)

	return v, err
}
