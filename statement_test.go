package sqluct_test

import (
	"github.com/bool64/sqluct"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitStatements(t *testing.T) {
	s := `SELECT "ol'o""lo","",'';
	-- next sta;tement
	SELECT * FROM refers order by ts desc limit 15;
	-- next statement
	SELECT * FROM visitor WHERE hash=8361038239347337526;
SELECT /* 1;2;3 */ 'aaa' 
`

	res := sqluct.SplitStatements(s)
	assert.Equal(t, []string{
		`SELECT "ol'o""lo","",''`,
		"-- next sta;tement\n\tSELECT * FROM refers order by ts desc limit 15",
		"-- next statement\n\tSELECT * FROM visitor WHERE hash=8361038239347337526",
		"SELECT /* 1;2;3 */ 'aaa'",
	}, res)
}

func TestSplitStatements2(t *testing.T) {
	s := `";";';'`
	res := sqluct.SplitStatements(s)

	assert.Equal(t, []string{
		`";"`,
		`';'`,
	}, res)
}
