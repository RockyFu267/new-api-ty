package model

import (
	"context"
	"errors"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func departmentFileDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	dsn, err := sqliteConnectionDSN(filepath.Join(t.TempDir(), "quota.db"))
	require.NoError(t, err)
	db, err := gorm.Open(sqlite.Open(dsn), newGormConfig(true))
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(4)
	previous := DB
	DB = db
	t.Cleanup(func() { DB = previous; require.NoError(t, pool.Close()) })
	require.NoError(t, db.AutoMigrate(&User{}, &Department{}, &UserDepartment{}, &DepartmentQuotaRule{}, &DepartmentQuotaCharge{}))
	require.NoError(t, db.Create(&User{Id: 1, Username: "parallel", Quota: 0}).Error)
	require.NoError(t, db.Create(&Department{Id: 1, Name: "Research"}).Error)
	require.NoError(t, db.Create(&UserDepartment{UserId: 1, DepartmentId: 1}).Error)
	require.NoError(t, db.Create(&DepartmentQuotaRule{DepartmentId: 1, Enabled: true, Daily: 100, Weekly: 180, Monthly: 200}).Error)
	return db
}

func TestSQLiteParallelQuotaRespectsMonthlyBudgetAcrossDailyReset(t *testing.T) {
	db := departmentFileDatabase(t)
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*3600))
	// A prior week consumed 150; the fresh day's and week's limits are higher
	// than the remaining monthly amount. Only one 40-unit request can enter.
	require.NoError(t, db.Create(&DepartmentQuotaCharge{RequestId: "prior-week", UserId: 1, DepartmentId: 1, ModelName: "qwen", CreatedAt: now.AddDate(0, 0, -7).Unix(), Amount: 150, Status: "settled"}).Error)
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, id := range []string{"agent-a", "agent-b"} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			<-start
			_, err := ReserveDepartmentQuota(id, 1, "qwen", 40, now)
			results <- err
		}(id)
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
	require.Equal(t, 1, successes)
	usage, err := GetDepartmentQuotaUsage(1, now)
	require.NoError(t, err)
	require.Equal(t, int64(150), usage.Monthly.Used)
	require.Equal(t, int64(40), usage.Monthly.Reserved)
	var charges []DepartmentQuotaCharge
	require.NoError(t, db.Where("status = ?", "reserved").Find(&charges).Error)
	require.Len(t, charges, 1)
	// Multiple finish attempts on separate connections must release the same
	// reservation once and preserve the actual charge rather than refunding it.
	results = make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); results <- FinishDepartmentQuota(charges[0].RequestId, 25, false) }()
	}
	wg.Wait()
	close(results)
	for err := range results {
		require.NoError(t, err)
	}
	usage, err = GetDepartmentQuotaUsage(1, now)
	require.NoError(t, err)
	require.Zero(t, usage.Monthly.Reserved)
	require.Equal(t, int64(175), usage.Monthly.Used)
	_, err = ReserveDepartmentQuota("refund", 1, "qwen", 20, now)
	require.NoError(t, err)
	results = make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); results <- FinishDepartmentQuota("refund", 0, true) }()
	}
	wg.Wait()
	close(results)
	for err := range results {
		require.NoError(t, err)
	}
	usage, err = GetDepartmentQuotaUsage(1, now)
	require.NoError(t, err)
	require.Zero(t, usage.Monthly.Reserved)
	require.Equal(t, int64(175), usage.Monthly.Used)
	var user User
	require.NoError(t, db.First(&user, 1).Error)
	require.Zero(t, user.Quota)
}

func TestSQLiteTransactionRetriesRollbackWithoutDuplicatingCharge(t *testing.T) {
	db := departmentFileDatabase(t)
	dsn := db.Dialector.(*sqlite.Dialector).DSN
	name, raw, _ := strings.Cut(dsn, "?")
	query, err := url.ParseQuery(raw)
	require.NoError(t, err)
	query.Del("_pragma")
	query.Add("_pragma", "busy_timeout(1)")
	contender, err := gorm.Open(sqlite.Open(name+"?"+query.Encode()), newGormConfig(true))
	require.NoError(t, err)
	pool, err := contender.DB()
	require.NoError(t, err)
	defer pool.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	mainPool, err := db.DB()
	require.NoError(t, err)
	blocker, err := mainPool.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer blocker.Rollback()
	_, err = blocker.ExecContext(ctx, "UPDATE department_quota_rules SET daily = 101 WHERE department_id = 1")
	require.NoError(t, err)
	// The WAL reader sees the committed value while a different connection has
	// an uncommitted write; loading permissions/budgets need not wait for it.
	var committedLimit int64
	require.NoError(t, mainPool.QueryRowContext(ctx, "SELECT daily FROM department_quota_rules WHERE department_id = 1").Scan(&committedLimit))
	require.EqualValues(t, 100, committedLimit)
	busyErr := contender.Exec("UPDATE department_quota_rules SET daily = daily WHERE department_id = 1").Error
	require.Error(t, busyErr)
	require.NoError(t, blocker.Rollback())
	// Inject the real driver's lock error after a write to verify that retries
	// restart the rolled-back transaction and cannot double-count that write.
	calls := 0
	require.NoError(t, quotaWriteTransaction(db, func(tx *gorm.DB) error {
		calls++
		if err := tx.Create(&DepartmentQuotaCharge{RequestId: "retry", UserId: 1, Amount: 25, Status: "reserved"}).Error; err != nil {
			return err
		}
		if calls == 1 {
			return busyErr
		}
		return nil
	}))
	require.Equal(t, 2, calls)
	var total int64
	require.NoError(t, db.Model(&DepartmentQuotaCharge{}).Where("request_id = ?", "retry").Count(&total).Error)
	require.EqualValues(t, 1, total)
	businessErr := errors.New("quota policy failure")
	calls = 0
	require.ErrorIs(t, quotaWriteTransaction(db, func(tx *gorm.DB) error { calls++; return businessErr }), businessErr)
	require.Equal(t, 1, calls)
}
