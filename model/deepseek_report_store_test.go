package model

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func billingReportDatabases(t *testing.T) (*gorm.DB, *gorm.DB) {
	t.Helper()
	open := func() *gorm.DB {
		db, err := gorm.Open(sqlite.Open(":memory:"), newGormConfig(true))
		require.NoError(t, err)
		pool, err := db.DB()
		require.NoError(t, err)
		pool.SetMaxOpenConns(1)
		t.Cleanup(func() { require.NoError(t, pool.Close()) })
		return db
	}
	source, target := open(), open()
	require.NoError(t, source.AutoMigrate(&Log{}))
	require.NoError(t, InitDeepSeekReportStore(target))
	old := DeepSeekReportDB
	DeepSeekReportDB = target
	t.Cleanup(func() { DeepSeekReportDB = old })
	return source, target
}

func TestDeepSeekReportReaderCanStartWhileItsDatabaseIsOffline(t *testing.T) {
	source, _ := billingReportDatabases(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())
	t.Setenv("SQL_DSN", "postgres://user:password@"+address+"/primary?connect_timeout=1")
	t.Setenv("LOG_SQL_DSN", "")
	t.Setenv("DEEPSEEK_REPORT_DSN", "postgres://user:password@"+address+"/reports?connect_timeout=1")
	require.NoError(t, InitDeepSeekReportReader())
	pool, err := DeepSeekReportDB.DB()
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })
	_, err = GetDeepSeekDemoReport("2026-11-08", "2026-11-08", 0, 1)
	require.Error(t, err)
	// An unavailable report connection never takes over the primary handle.
	var count int64
	require.NoError(t, source.Model(&Log{}).Count(&count).Error)
	t.Setenv("DEEPSEEK_REPORT_DSN", "postgres://user:password@"+address+"/primary?connect_timeout=1")
	require.ErrorContains(t, InitDeepSeekReportReader(), "separate database")
}

func TestDeepSeekReportsReadSavedResultsWhenSourceIsUnavailable(t *testing.T) {
	source, target := billingReportDatabases(t)
	now := time.Now()
	log := demoConsume(t, "2026-11-08 10:00:00", 1)
	require.NoError(t, source.Create(&log).Error)
	require.NoError(t, ProcessDeepSeekReportBatch(context.Background(), source, target, 100, now))
	// Reading a page must not query even a single raw log or start computation.
	require.NoError(t, source.Migrator().DropTable(&Log{}))
	for range 2 {
		report, err := GetDeepSeekSelfDemoReport("2026-11-08", "2026-11-08", 1, 1)
		require.NoError(t, err)
		assert.True(t, report.Status.HistoryComplete)
		assert.EqualValues(t, 1, report.Totals.Requests)
		assert.Len(t, report.Details, 1)
	}
	old := DeepSeekReportDB
	DeepSeekReportDB = nil
	t.Cleanup(func() { DeepSeekReportDB = old })
	report, err := GetDeepSeekDemoReport("2026-11-08", "2026-11-08", 0, 1)
	require.NoError(t, err)
	assert.False(t, report.Status.Configured)
	assert.Empty(t, report.Details)
}

func TestDeepSeekReportReplayFindsLateCommitWithoutDoubleCounting(t *testing.T) {
	source, target := billingReportDatabases(t)
	now := time.Now()
	for _, id := range []int{1, 3} {
		log := demoConsume(t, "2026-11-08 10:00:00", 1)
		log.Id = id
		log.RequestId = fmt.Sprint(id)
		require.NoError(t, source.Create(&log).Error)
	}
	require.NoError(t, ProcessDeepSeekReportBatch(context.Background(), source, target, 100, now))
	require.NoError(t, ProcessDeepSeekReportBatch(context.Background(), source, target, 100, now.Add(time.Second)))
	late := demoConsume(t, "2026-11-08 10:00:00", 1)
	late.Id = 2
	late.RequestId = "late-commit"
	require.NoError(t, source.Create(&late).Error)
	// The next scheduled full reconciliation recovers IDs below the cursor.
	for i := 0; i < 4; i++ {
		require.NoError(t, ProcessDeepSeekReportBatch(context.Background(), source, target, 100, now.Add(2*time.Hour+time.Duration(i)*time.Second)))
	}
	report, err := GetDeepSeekDemoReport("2026-11-08", "2026-11-08", 0, 1)
	require.NoError(t, err)
	assert.EqualValues(t, 3, report.Totals.Requests)
	assert.EqualValues(t, 1353000000, report.Totals.OffPeakUnits)
	require.NoError(t, source.Where("1 = 1").Delete(&Log{}).Error)
	require.NoError(t, ProcessDeepSeekReportBatch(context.Background(), source, target, 100, now.Add(3*time.Hour)))
	report, err = GetDeepSeekDemoReport("2026-11-08", "2026-11-08", 0, 1)
	require.NoError(t, err)
	assert.True(t, report.Status.HistoryComplete)
	assert.EqualValues(t, 3, report.Totals.Requests)
}

func TestDeepSeekReportKeysetPaginationScopesUsersAndBreaksTimeTies(t *testing.T) {
	source, target := billingReportDatabases(t)
	logs := make([]Log, 53)
	for i := range logs {
		logs[i] = demoConsume(t, "2026-11-08 10:00:00", 1)
		logs[i].RequestId = fmt.Sprint(i)
	}
	other := demoConsume(t, "2026-11-08 10:00:00", 2)
	other.RequestId = "other-user"
	logs = append(logs, other)
	require.NoError(t, source.Create(&logs).Error)
	require.NoError(t, ProcessDeepSeekReportBatch(context.Background(), source, target, 100, time.Now()))
	// A retained log and a new log can share both time and a reused SQLite ID.
	// Put the pair on the page boundary to exercise the final key component.
	require.NoError(t, source.Delete(&Log{}, 4).Error)
	reused := demoConsume(t, "2026-11-08 10:00:00", 1)
	reused.Id, reused.RequestId = 4, "reused-4"
	require.NoError(t, source.Create(&reused).Error)
	require.NoError(t, ProcessDeepSeekReportBatch(context.Background(), source, target, 100, time.Now().Add(time.Second)))
	first, err := ReadDeepSeekReport(context.Background(), "2026-11-08", "2026-11-08", 1, 1, true, "")
	require.NoError(t, err)
	require.Len(t, first.Details, 50)
	require.NotEmpty(t, first.NextCursor)
	second, err := ReadDeepSeekReport(context.Background(), "2026-11-08", "2026-11-08", 1, 2, true, first.NextCursor)
	require.NoError(t, err)
	assert.Len(t, second.Details, 4)
	assert.Empty(t, second.NextCursor)
	assert.Equal(t, first.Details[49].ID, second.Details[0].ID)
	assert.NotEqual(t, first.Details[49].ReportKey, second.Details[0].ReportKey)
	seen := map[string]bool{}
	for _, row := range append(first.Details, second.Details...) {
		assert.False(t, seen[row.ReportKey])
		seen[row.ReportKey] = true
		assert.Equal(t, 1, row.UserID)
	}
	assert.Equal(t, 54, first.DetailTotal)
	assert.EqualValues(t, 54, second.Totals.Requests)
	_, err = ReadDeepSeekReport(context.Background(), "2026-11-08", "2026-11-08", 1, 2, true, "")
	require.Error(t, err)
}

func TestDeepSeekReportFailedBatchRollsBackFactsDailyTotalsAndCursor(t *testing.T) {
	source, target := billingReportDatabases(t)
	log := demoConsume(t, "2026-11-08 10:00:00", 1)
	require.NoError(t, source.Create(&log).Error)
	require.NoError(t, target.Exec(`CREATE TRIGGER reject_daily BEFORE INSERT ON deep_seek_bill_dailies BEGIN SELECT RAISE(ABORT,'report failure'); END`).Error)
	require.ErrorContains(t, ProcessDeepSeekReportBatch(context.Background(), source, target, 100, time.Now()), "report failure")
	var count int64
	require.NoError(t, target.Model(&DeepSeekBillFact{}).Count(&count).Error)
	assert.Zero(t, count)
	var state DeepSeekBillState
	require.NoError(t, target.First(&state, 1).Error)
	assert.Zero(t, state.Cursor)
	require.NoError(t, target.Exec("DROP TRIGGER reject_daily").Error)
	require.NoError(t, ProcessDeepSeekReportBatch(context.Background(), source, target, 100, time.Now()))
	require.NoError(t, ProcessDeepSeekReportBatch(context.Background(), source, target, 100, time.Now()))
	report, err := GetDeepSeekDemoReport("2026-11-08", "2026-11-08", 0, 1)
	require.NoError(t, err)
	assert.EqualValues(t, 1, report.Totals.Requests)
}

func TestLogCleanupWaitsUntilDeepSeekReportPersistsTheBatch(t *testing.T) {
	source, target := billingReportDatabases(t)
	oldDB, oldLog := DB, LOG_DB
	DB, LOG_DB = source, source
	t.Cleanup(func() { DB, LOG_DB = oldDB, oldLog })
	log := demoConsume(t, "2026-11-08 10:00:00", 1)
	require.NoError(t, source.Create(&log).Error)
	_, err := DeleteOldLogBatch(context.Background(), time.Now().Unix(), 1)
	require.ErrorContains(t, err, "billing report import")
	require.NoError(t, ProcessDeepSeekReportBatch(context.Background(), source, target, 100, time.Now()))
	deleted, err := DeleteOldLogBatch(context.Background(), time.Now().Unix(), 1)
	require.NoError(t, err)
	assert.EqualValues(t, 1, deleted)
	report, err := GetDeepSeekDemoReport("2026-11-08", "2026-11-08", 0, 1)
	require.NoError(t, err)
	assert.EqualValues(t, 1, report.Totals.Requests)
	require.Error(t, ValidateDeepSeekReportIsolation(source, source))
	require.NoError(t, ValidateDeepSeekReportIsolation(source, target))
	// A source that reuses SQLite row IDs must not silently drop new invoices.
	reused := demoConsume(t, "2026-11-08 11:00:00", 1)
	reused.Id = log.Id
	reused.RequestId = "new-after-cleanup"
	require.NoError(t, source.Create(&reused).Error)
	// First observe the empty/rewound source, then tail/reconcile the reused ID.
	for i := 0; i < 3; i++ {
		require.NoError(t, ProcessDeepSeekReportBatch(context.Background(), source, target, 100, time.Now().Add(2*time.Hour+time.Duration(i)*time.Second)))
	}
	report, err = GetDeepSeekDemoReport("2026-11-08", "2026-11-08", 0, 1)
	require.NoError(t, err)
	assert.EqualValues(t, 2, report.Totals.Requests)
}
