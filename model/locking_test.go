package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/utils/tests"
)

type lockTestDialect struct {
	tests.DummyDialector
	dialect string
}

func (d lockTestDialect) Name() string { return d.dialect }

// lockForUpdate must emit FOR UPDATE on databases that support it and skip
// it on SQLite, where the syntax does not exist.
//
// The dummy dialector is used because SQLite drivers strip locking clauses
// from the generated SQL, which would mask what the helper itself does.
func TestLockForUpdateEmitsRowLock(t *testing.T) {
	old := common.MainDatabaseType()
	t.Cleanup(func() { common.SetMainDatabaseType(old) })
	for _, dialect := range []string{"mysql", "postgres", "sqlite"} {
		t.Run(dialect, func(t *testing.T) {
			// A report DB may differ from the primary engine in the same process.
			common.SetMainDatabaseType(common.DatabaseTypePostgreSQL)
			if dialect != "sqlite" {
				common.SetMainDatabaseType(common.DatabaseTypeSQLite)
			}
			db, err := gorm.Open(lockTestDialect{dialect: dialect}, &gorm.Config{DryRun: true})
			require.NoError(t, err)
			var rows []Redemption
			sql := lockForUpdate(db).Where("id = ?", 1).Find(&rows).Statement.SQL.String()
			if dialect == "sqlite" {
				assert.NotContains(t, sql, "FOR UPDATE")
			} else {
				assert.Contains(t, sql, "FOR UPDATE")
			}
		})
	}
}
