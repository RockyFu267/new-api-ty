package model

import (
	"context"
	"fmt"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"testing"
	"time"
)

func demoConsume(t *testing.T, at string, user int) Log {
	t.Helper()
	stamp, err := time.ParseInLocation("2006-01-02 15:04:05", at, UsageAnalysisLocation)
	require.NoError(t, err)
	return Log{Type: LogTypeConsume, UserId: user, Username: fmt.Sprintf("user-%d", user), ModelName: DeepSeekDemoModel, CreatedAt: stamp.Unix() + 2, RequestId: stamp.UTC().Format("20060102150405") + "-request", PromptTokens: 1000000, CompletionTokens: 1000000, Other: `{"cache_tokens":500000,"analysis_cache_reported":true,"admin_info":{"usage_billing_path":"openai"}}`}
}

func TestDeepSeekDemoPeakCalendarAndBoundaries(t *testing.T) {
	for _, test := range []struct {
		at   string
		peak bool
	}{
		{"2026-11-09 08:59:59", false}, {"2026-11-09 09:00:00", true}, {"2026-11-09 11:59:59", true}, {"2026-11-09 12:00:00", false}, {"2026-11-09 14:00:00", true}, {"2026-11-09 18:00:00", false},
		{"2026-11-08 10:00:00", false}, {"2026-10-01 10:00:00", false}, {"2026-09-25 10:00:00", false},
	} {
		t.Run(test.at, func(t *testing.T) {
			at, err := time.ParseInLocation("2006-01-02 15:04:05", test.at, UsageAnalysisLocation)
			require.NoError(t, err)
			require.Equal(t, test.peak, deepSeekDemoPeak(at))
		})
	}
}

func TestDeepSeekDemoPricesCacheOnceAndMarksIncompleteUsage(t *testing.T) {
	log := demoConsume(t, "2026-11-09 09:00:00", 1)
	detail := deepSeekDemoDetail(log)
	require.EqualValues(t, 902000000, detail.CostUnits) // 0.5M*0.04 + 0.5M*2 + 1M*8 = CNY 9.02.
	require.False(t, detail.Estimated)
	log = demoConsume(t, "2026-11-08 09:00:00", 1)
	require.EqualValues(t, 451000000, deepSeekDemoDetail(log).CostUnits)
	log.Other = `{"cache_tokens":2000000}`
	require.True(t, deepSeekDemoDetail(log).Unpriced)
	log.Other = `{bad json`
	require.True(t, deepSeekDemoDetail(log).Unpriced)
	log.Other = `{}`
	detail = deepSeekDemoDetail(log)
	require.True(t, detail.Estimated)
	require.EqualValues(t, 500000000, detail.CostUnits) // Missing cache detail is a labelled uncached estimate.
	log.RequestId = "legacy"
	require.Equal(t, "log_time", deepSeekDemoDetail(log).TimeSource)
}

func TestDeepSeekDemoOnlyShowsUsedModelUsersAndIncludesMidnightRequests(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(1)
	previous := LOG_DB
	LOG_DB = db
	t.Cleanup(func() { LOG_DB = previous; require.NoError(t, pool.Close()) })
	require.NoError(t, db.AutoMigrate(&Log{}))
	reportStore, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	reportPool, err := reportStore.DB()
	require.NoError(t, err)
	reportPool.SetMaxOpenConns(1)
	oldReport := DeepSeekReportDB
	DeepSeekReportDB = reportStore
	t.Cleanup(func() { DeepSeekReportDB = oldReport; require.NoError(t, reportPool.Close()) })
	require.NoError(t, InitDeepSeekReportStore(reportStore))
	a := demoConsume(t, "2026-11-08 10:00:00", 1)
	b := demoConsume(t, "2026-11-08 23:59:59", 2)
	b.CreatedAt += 60
	excluded := demoConsume(t, "2026-11-08 10:00:00", 3)
	excluded.ModelName = "qwen"
	nextDay := demoConsume(t, "2026-11-09 00:00:00", 4)
	require.NoError(t, db.Create(&[]Log{a, b, excluded, nextDay}).Error)
	require.NoError(t, ProcessDeepSeekReportBatch(context.Background(), db, reportStore, 200, time.Now()))
	report, err := GetDeepSeekDemoReport("2026-11-08", "2026-11-08", 2, 1)
	require.NoError(t, err)
	require.Len(t, report.Users, 2)
	require.EqualValues(t, 2, report.Totals.Requests)
	require.EqualValues(t, 902000000, report.Totals.OffPeakUnits)
	require.Len(t, report.Details, 1)
	require.Equal(t, 2, report.Details[0].UserID)
	require.Equal(t, "request_id", report.Details[0].TimeSource)
	require.Equal(t, 1, report.DetailTotal)
	_, err = GetDeepSeekDemoReport("2026-11-09", "2026-11-08", 0, 1)
	require.Error(t, err)
	_, err = GetDeepSeekDemoReport("2026-11-08", "2027-01-01", 0, 1)
	require.Error(t, err)
	var count int64
	require.NoError(t, db.Model(&Log{}).Count(&count).Error)
	require.EqualValues(t, 4, count)
}

func TestDeepSeekSelfDemoReportScopesTotalsUsersAndDetails(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(1)
	previous := LOG_DB
	LOG_DB = db
	t.Cleanup(func() { LOG_DB = previous; require.NoError(t, pool.Close()) })
	require.NoError(t, db.AutoMigrate(&Log{}))
	reportStore, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	reportPool, err := reportStore.DB()
	require.NoError(t, err)
	reportPool.SetMaxOpenConns(1)
	oldReport := DeepSeekReportDB
	DeepSeekReportDB = reportStore
	t.Cleanup(func() { DeepSeekReportDB = oldReport; require.NoError(t, reportPool.Close()) })
	require.NoError(t, InitDeepSeekReportStore(reportStore))
	require.NoError(t, db.Create(&[]Log{demoConsume(t, "2026-11-08 10:00:00", 1), demoConsume(t, "2026-11-08 10:00:00", 2)}).Error)
	require.NoError(t, ProcessDeepSeekReportBatch(context.Background(), db, reportStore, 200, time.Now()))
	report, err := GetDeepSeekSelfDemoReport("2026-11-08", "2026-11-08", 1, 1)
	require.NoError(t, err)
	require.EqualValues(t, 1, report.Totals.Requests)
	require.EqualValues(t, 451000000, report.Totals.OffPeakUnits)
	require.Len(t, report.Users, 1)
	require.Equal(t, 1, report.Users[0].UserID)
	require.Len(t, report.Details, 1)
	require.Equal(t, 1, report.Details[0].UserID)
	empty, err := GetDeepSeekSelfDemoReport("2026-11-08", "2026-11-08", 3, 1)
	require.NoError(t, err)
	require.Empty(t, empty.Users)
	require.Empty(t, empty.Details)
	require.Zero(t, empty.Totals.Requests)
	_, err = GetDeepSeekSelfDemoReport("2026-11-08", "2026-11-08", 0, 1)
	require.Error(t, err)
}
