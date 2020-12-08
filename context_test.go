package sqluct_test

import (
	"context"
	"testing"

	"github.com/bool64/sqluct"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
)

func TestTxFromContext(t *testing.T) {
	tx := sqlx.Tx{}
	ctx := context.Background()
	assert.Nil(t, sqluct.TxFromContext(ctx))
	ctx = sqluct.TxToContext(ctx, &tx)
	assert.Equal(t, &tx, sqluct.TxFromContext(ctx))
}
