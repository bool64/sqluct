package sqluct

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type ctxKey struct{}

// TxToContext adds transaction to context.
func TxToContext(ctx context.Context, tx *sqlx.Tx) context.Context {
	return context.WithValue(ctx, ctxKey{}, tx)
}

// TxFromContext gets transaction or nil from context.
func TxFromContext(ctx context.Context) *sqlx.Tx {
	tx, ok := ctx.Value(ctxKey{}).(*sqlx.Tx)
	if !ok {
		return nil
	}

	return tx
}
