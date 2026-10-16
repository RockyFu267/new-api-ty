package model

import (
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/stretchr/testify/require"
)

func setupDepartmentQuotaTest(t *testing.T) int {
	t.Helper()
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&Department{}, &UserDepartment{}, &DepartmentQuotaRule{}, &DepartmentQuotaCharge{}))
	t.Cleanup(func() {
		DB.Where("1 = 1").Delete(&DepartmentQuotaCharge{})
		DB.Where("1 = 1").Delete(&DepartmentQuotaRule{})
		DB.Where("1 = 1").Delete(&UserDepartment{})
		DB.Where("1 = 1").Delete(&Department{})
	})
	insertUsersForPaginationTest(t, 2)
	department, err := SaveDepartment(0, "Research")
	require.NoError(t, err)
	require.NoError(t, AssignUserDepartment(1, department.Id))
	require.NoError(t, AssignUserDepartment(2, department.Id))
	require.NoError(t, SaveDepartmentQuotaRule(&DepartmentQuotaRule{DepartmentId: department.Id, Enabled: true, Daily: 100, Weekly: 150, Monthly: 180}))
	return department.Id
}

func TestEnterpriseQuotaModeOptionUpdatesRuntimeConfiguration(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Option{}))
	common.OptionMapRWMutex.Lock()
	oldMap := common.OptionMap
	common.OptionMap = make(map[string]string)
	common.OptionMapRWMutex.Unlock()
	old := operation_setting.GetQuotaSetting().EnterpriseQuotaEnabled
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = oldMap
		common.OptionMapRWMutex.Unlock()
		operation_setting.GetQuotaSetting().EnterpriseQuotaEnabled = old
		DB.Where(commonKeyCol+" = ?", "quota_setting.enterprise_quota_enabled").Delete(&Option{})
	})
	require.NoError(t, UpdateOption("quota_setting.enterprise_quota_enabled", "true"))
	require.True(t, operation_setting.GetQuotaSetting().EnterpriseQuotaEnabled)
	require.NoError(t, UpdateOption("quota_setting.enterprise_quota_enabled", "false"))
	require.False(t, operation_setting.GetQuotaSetting().EnterpriseQuotaEnabled)
}

func TestDepartmentQuotaConcurrentReservationsCannotShareRemainingBudget(t *testing.T) {
	setupDepartmentQuotaTest(t)
	var wait sync.WaitGroup
	results := make(chan error, 2)
	for _, requestId := range []string{"concurrent-a", "concurrent-b"} {
		wait.Add(1)
		go func(id string) {
			defer wait.Done()
			_, err := ReserveDepartmentQuota(id, 1, "qwen", 70, time.Now())
			results <- err
		}(requestId)
	}
	wait.Wait()
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
	usage, err := GetDepartmentQuotaUsage(1, time.Now())
	require.NoError(t, err)
	require.Equal(t, int64(70), usage.Daily.Reserved)
}

func TestDepartmentQuotaCalendarWindowsCrossMonth(t *testing.T) {
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.FixedZone("UTC+8", 8*3600))
	periods := DepartmentQuotaPeriods(now)
	require.Equal(t, "2026-09-27 00:00", time.Unix(periods[1].Start, 0).In(now.Location()).Format("2006-01-02 15:04"))
	require.Equal(t, "2026-10-04 00:00", time.Unix(periods[1].ResetAt, 0).In(now.Location()).Format("2006-01-02 15:04"))
	require.Equal(t, now.Unix(), periods[2].Start)
	sunday := DepartmentQuotaPeriods(time.Date(2026, 10, 4, 0, 0, 0, 0, now.Location()))
	require.Equal(t, periods[1].ResetAt, sunday[1].Start)
}

func TestDepartmentQuotaReservesPerUserAndRefundsExactlyOnce(t *testing.T) {
	setupDepartmentQuotaTest(t)
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	_, err := ReserveDepartmentQuota("first", 1, "qwen", 70, now)
	require.NoError(t, err)
	_, err = ReserveDepartmentQuota("blocked", 1, "qwen", 40, now)
	require.ErrorIs(t, err, ErrDepartmentQuotaUnavailable)
	_, err = ReserveDepartmentQuota("second-user", 2, "qwen", 70, now)
	require.NoError(t, err)
	require.NoError(t, FinishDepartmentQuota("first", 50, false))
	require.NoError(t, FinishDepartmentQuota("first", 80, false))
	usage, err := GetDepartmentQuotaUsage(1, now)
	require.NoError(t, err)
	require.Equal(t, int64(50), usage.Daily.Used)
	require.Zero(t, usage.Daily.Reserved)
	_, err = ReserveDepartmentQuota("refund", 1, "qwen", 30, now)
	require.NoError(t, err)
	require.NoError(t, FinishDepartmentQuota("refund", 0, true))
	require.NoError(t, FinishDepartmentQuota("refund", 0, true))
	usage, err = GetDepartmentQuotaUsage(1, now)
	require.NoError(t, err)
	require.Equal(t, int64(50), usage.Daily.Used)
	require.Zero(t, usage.Daily.Reserved)
	var user User
	require.NoError(t, DB.First(&user, 1).Error)
	require.Zero(t, user.Quota)
}

func TestDepartmentQuotaDailyResetPreservesWeeklyAndDepartmentTransfers(t *testing.T) {
	departmentId := setupDepartmentQuotaTest(t)
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.FixedZone("UTC+8", 8*3600))
	_, err := ReserveDepartmentQuota("first-day", 1, "qwen", 90, now)
	require.NoError(t, err)
	require.NoError(t, FinishDepartmentQuota("first-day", 90, false))
	nextDay := now.AddDate(0, 0, 1)
	usage, err := GetDepartmentQuotaUsage(1, nextDay)
	require.NoError(t, err)
	require.Zero(t, usage.Daily.Used)
	require.Equal(t, int64(90), usage.Weekly.Used)
	_, err = ReserveDepartmentQuota("weekly-blocked", 1, "qwen", 70, nextDay)
	require.ErrorIs(t, err, ErrDepartmentQuotaUnavailable)
	other, err := SaveDepartment(0, "Engineering")
	require.NoError(t, err)
	require.NoError(t, SaveDepartmentQuotaRule(&DepartmentQuotaRule{DepartmentId: other.Id, Enabled: true, Monthly: 100}))
	require.NoError(t, AssignUserDepartment(1, other.Id))
	_, err = ReserveDepartmentQuota("transfer-blocked", 1, "qwen", 20, nextDay)
	require.ErrorIs(t, err, ErrDepartmentQuotaUnavailable)
	usage, err = GetDepartmentQuotaUsage(1, nextDay)
	require.NoError(t, err)
	require.Equal(t, int64(90), usage.Monthly.Used)
	var charge DepartmentQuotaCharge
	require.NoError(t, DB.First(&charge, "request_id = ?", "first-day").Error)
	require.Equal(t, departmentId, charge.DepartmentId)
	require.Equal(t, "Research", charge.DepartmentName)
}

func TestDepartmentQuotaLateSettlementStaysInOriginalMonth(t *testing.T) {
	setupDepartmentQuotaTest(t)
	started := time.Date(2026, 9, 30, 23, 59, 0, 0, time.FixedZone("UTC+8", 8*3600))
	_, err := ReserveDepartmentQuota("late", 1, "qwen", 50, started)
	require.NoError(t, err)
	require.NoError(t, ResizeDepartmentQuota("late", 70))
	require.NoError(t, FinishDepartmentQuota("late", 60, false))
	usage, err := GetDepartmentQuotaUsage(1, started.Add(2*time.Minute))
	require.NoError(t, err)
	require.Zero(t, usage.Daily.Used)
	require.Zero(t, usage.Monthly.Used)
	require.Equal(t, int64(60), usage.Weekly.Used)
}

func TestDepartmentQuotaMonthlyExhaustionStillBlocksAfterThirdWeekReset(t *testing.T) {
	departmentId := setupDepartmentQuotaTest(t)
	unit := int64(common.QuotaPerUnit)
	require.NoError(t, SaveDepartmentQuotaRule(&DepartmentQuotaRule{DepartmentId: departmentId, Enabled: true, Daily: 20 * unit, Weekly: 100 * unit, Monthly: 200 * unit}))
	zone := time.FixedZone("UTC+8", 8*3600)
	for _, day := range []int{4, 5, 6, 7, 8, 11, 12, 13, 14, 15} {
		started := time.Date(2026, 10, day, 12, 0, 0, 0, zone)
		requestId := started.Format("2006-01-02")
		_, err := ReserveDepartmentQuota(requestId, 1, "qwen", 20*unit, started)
		require.NoError(t, err)
		require.NoError(t, FinishDepartmentQuota(requestId, 20*unit, false))
	}
	thirdWeek := time.Date(2026, 10, 18, 0, 0, 0, 0, zone)
	usage, err := GetDepartmentQuotaUsage(1, thirdWeek)
	require.NoError(t, err)
	require.Zero(t, usage.Daily.Used)
	require.Zero(t, usage.Weekly.Used)
	require.Equal(t, 200*unit, usage.Monthly.Used)
	_, err = ReserveDepartmentQuota("third-week", 1, "qwen", unit, thirdWeek)
	require.ErrorIs(t, err, ErrDepartmentQuotaUnavailable)
	_, err = ReserveDepartmentQuota("next-month", 1, "qwen", 20*unit, time.Date(2026, 11, 1, 0, 0, 0, 0, zone))
	require.NoError(t, err)
}

func TestWalletReceiptPreservesArithmeticAndRejectsOverflow(t *testing.T) {
	truncateTables(t)
	insertUsersForPaginationTest(t, 1)
	before, after, err := AdjustWalletWithReceipt(1, "add", 100)
	require.NoError(t, err)
	require.Equal(t, 0, before)
	require.Equal(t, 100, after)
	before, after, err = AdjustWalletWithReceipt(1, "subtract", 30)
	require.NoError(t, err)
	require.Equal(t, 100, before)
	require.Equal(t, 70, after)
	before, after, err = AdjustWalletWithReceipt(1, "override", 10)
	require.NoError(t, err)
	require.Equal(t, 70, before)
	require.Equal(t, 10, after)
	_, _, err = AdjustWalletWithReceipt(1, "add", 1<<53-1)
	require.Error(t, err)
}
