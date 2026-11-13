package model

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Opt-in integration tests. Every run owns a new schema in a loopback *_test
// database. Never point these tests at an application or production database.
func postgresAccountingDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("LOCAL_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("set LOCAL_POSTGRES_TEST_DSN for isolated PostgreSQL integration tests")
	}
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	require.Contains(t, []string{"127.0.0.1", "localhost", "::1"}, u.Hostname())
	require.True(t, strings.HasSuffix(u.Path, "_test"), "test database name must end in _test")
	schema := fmt.Sprintf("accounting_test_%d", time.Now().UnixNano())
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	adminPool, err := admin.DB()
	require.NoError(t, err)
	require.NoError(t, admin.Exec(`CREATE SCHEMA "`+schema+`"`).Error)
	query := u.Query()
	query.Set("search_path", schema)
	query.Set("lock_timeout", "20")
	u.RawQuery = query.Encode()
	db, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(16)
	previous, previousLog := DB, LOG_DB
	mainType, logType := common.MainDatabaseType(), common.LogDatabaseType()
	DB, LOG_DB = db, db
	common.SetDatabaseTypes(common.DatabaseTypePostgreSQL, common.DatabaseTypePostgreSQL)
	initCol()
	t.Cleanup(func() {
		DB, LOG_DB = previous, previousLog
		common.SetDatabaseTypes(mainType, logType)
		initCol()
		require.NoError(t, pool.Close())
		require.NoError(t, admin.Exec(`DROP SCHEMA "`+schema+`" CASCADE`).Error)
		require.NoError(t, adminPool.Close())
	})
	require.NoError(t, db.AutoMigrate(&User{}, &Department{}, &UserDepartment{}, &DepartmentQuotaRule{}, &DepartmentQuotaCharge{}, &DepartmentQuotaCounter{}, &Token{}, &Channel{}, &Log{}))
	return db
}

func TestPostgreSQLConcurrentAgentsCannotReuseMonthlyRemainder(t *testing.T) {
	db := postgresAccountingDatabase(t)
	require.NoError(t, db.Create(&User{Id: 1, Username: "pg-agent"}).Error)
	require.NoError(t, db.Create(&Department{Id: 1, Name: "Research"}).Error)
	require.NoError(t, db.Create(&UserDepartment{UserId: 1, DepartmentId: 1}).Error)
	require.NoError(t, db.Create(&DepartmentQuotaRule{DepartmentId: 1, Enabled: true, Daily: 100, Weekly: 180, Monthly: 200}).Error)
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, UsageAnalysisLocation)
	require.NoError(t, db.Create(&DepartmentQuotaCharge{RequestId: "old-week", UserId: 1, CreatedAt: now.AddDate(0, 0, -7).Unix(), Amount: 150, Status: "settled"}).Error)
	start := make(chan struct{})
	results := make(chan error, 32)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, err := ReserveDepartmentQuota(fmt.Sprintf("agent-%d", i), 1, "deepseek", 40, now)
			results <- err
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else {
			require.ErrorIs(t, err, ErrDepartmentQuotaUnavailable)
		}
	}
	assert.Equal(t, 1, successes)
	var reservation DepartmentQuotaCharge
	require.NoError(t, db.Where("status = ?", "reserved").First(&reservation).Error)
	results = make(chan error, 16)
	for range 16 {
		wg.Add(1)
		go func() { defer wg.Done(); results <- FinishDepartmentQuota(reservation.RequestId, 55, false) }()
	}
	wg.Wait()
	close(results)
	for err := range results {
		require.NoError(t, err)
	}
	usage, err := GetDepartmentQuotaUsage(1, now)
	require.NoError(t, err)
	assert.Equal(t, int64(205), usage.Monthly.Used)
	assert.Zero(t, usage.Monthly.Reserved)
	_, err = ReserveDepartmentQuota("next-request", 1, "deepseek", 1, now)
	require.ErrorIs(t, err, ErrDepartmentQuotaUnavailable)
	var total int64
	require.NoError(t, db.Model(&DepartmentQuotaCharge{}).Where("status = ?", "settled").Select("SUM(amount)").Scan(&total).Error)
	assert.Equal(t, total, usage.Monthly.Used)
}

func TestPostgreSQLTokenLockRetriesWithoutDoubleDebit(t *testing.T) {
	db := postgresAccountingDatabase(t)
	require.NoError(t, db.Create(&Token{Id: 1, UserId: 1, Key: "pg-lock-key", RemainQuota: 1000}).Error)
	blocker := db.Begin()
	require.NoError(t, blocker.Error)
	defer blocker.Rollback()
	require.NoError(t, blocker.Model(&Token{}).Where("id = ?", 1).Update("used_quota", gorm.Expr("used_quota + 0")).Error)
	locked := make(chan struct{}, 1)
	require.NoError(t, db.Callback().Update().After("gorm:update").Register("observe_lock_rejection", func(tx *gorm.DB) {
		if IsTransientDatabaseError(tx.Error) {
			select {
			case locked <- struct{}{}:
			default:
			}
		}
	}))
	t.Cleanup(func() { db.Callback().Update().Remove("observe_lock_rejection") })
	result := make(chan error, 1)
	go func() {
		ok, err := TryReserveTokenQuota(1, "pg-lock-key", 200, false)
		if err == nil && !ok {
			err = fmt.Errorf("unexpected budget rejection")
		}
		result <- err
	}()
	select {
	case <-locked:
	case <-time.After(5 * time.Second):
		t.Fatal("expected a real PostgreSQL lock timeout")
	}
	require.NoError(t, blocker.Commit().Error)
	require.NoError(t, <-result)
	var token Token
	require.NoError(t, db.First(&token, 1).Error)
	assert.Equal(t, 800, token.RemainQuota)
	assert.Equal(t, 200, token.UsedQuota)
	// A persistent lock is a database failure, never an insufficient balance.
	blocker = db.Begin()
	require.NoError(t, blocker.Error)
	defer blocker.Rollback()
	require.NoError(t, blocker.Model(&Token{}).Where("id = ?", 1).Update("used_quota", gorm.Expr("used_quota + 0")).Error)
	_, err := TryReserveTokenQuota(1, "pg-lock-key", 200, false)
	require.Error(t, err)
	assert.True(t, IsTransientDatabaseError(err))
	require.NoError(t, blocker.Rollback().Error)
	require.NoError(t, db.First(&token, 1).Error)
	assert.Equal(t, 800, token.RemainQuota)
	assert.Equal(t, 200, token.UsedQuota)
}
