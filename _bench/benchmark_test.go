package bench_test

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	_ "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"os"
	"testing"
)

func BenchmarkStd(b *testing.B) {
	db, err := sql.Open("sqlite3", "bench.sqlite")
	assert.NoError(b, err)

	defer func() {
		assert.NoError(b, db.Close())
		assert.NoError(b, os.Remove("bench.sqlite"))
	}()

}

func BenchmarkSqluct(b *testing.B) {

}

type User struct {
	gorm.Model
	Name        string
	CreditCards []CreditCard
}

type CreditCard struct {
	gorm.Model
	Number string
	UserID uint
}

func BenchmarkGorm(b *testing.B) {
	db, err := gorm.Open(sqlite.Open("bench.sqlite"))
	require.NoError(b, err)

	defer func() {
		//assert.NoError(b, os.Remove("bench.sqlite"))
	}()

	// Create the table from our struct.
	require.NoError(b, db.AutoMigrate(&User{}))
	require.NoError(b, db.AutoMigrate(&CreditCard{}))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		u := &User{
			Name: "Craig",
			CreditCards: []CreditCard{
				{Number: "abc123"},
				{Number: "def456"},
			},
		}

		db.Create(u)

		var (
			users  []User
			ccards []CreditCard
		)
		err := db.Model(&User{}).Preload("CreditCards").Find(&users).Error

		if err != nil {
			b.Fatal(err)
		}

		db.Unscoped().Delete(u)
		db.Unscoped().Delete(&ccards, []uint{u.CreditCards[0].ID, u.CreditCards[1].ID})
	}

}
