package model

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUsageAnalysisSeparateLogDatabaseAndAtomicRetry(t *testing.T) {
	setupUsageAnalysisTest(t)
	separate, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := separate.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	previous := LOG_DB
	LOG_DB = separate
	t.Cleanup(func() { LOG_DB = previous; sqlDB.Close() })
	require.NoError(t, LOG_DB.AutoMigrate(&Log{}))
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, UserId: 9, Username: "bob", ModelName: "local", PromptTokens: 42, CreatedAt: time.Now().Unix()}).Error)
	require.NoError(t, InitUsageAnalysis())
	good, _ := common.Marshal(UsageAnalysisDaily{Day: "2026-10-19", Model: "queued", UsageAnalysisTotals: UsageAnalysisTotals{Requests: 1, Input: 100}})
	require.NoError(t, DB.Create(&UsageAnalysisEvent{Payload: string(good)}).Error)
	bad := UsageAnalysisEvent{Payload: "invalid-json"}
	require.NoError(t, DB.Create(&bad).Error)
	require.Error(t, SyncUsageAnalysis())
	var state UsageAnalysisState
	require.NoError(t, DB.First(&state, 1).Error)
	assert.Zero(t, state.Imported)
	var count int64
	require.NoError(t, DB.Model(&UsageAnalysisDaily{}).Count(&count).Error)
	assert.Zero(t, count)
	require.NoError(t, DB.Delete(&bad).Error)
	require.NoError(t, SyncUsageAnalysis())
	require.NoError(t, SyncUsageAnalysis())
	var rows []UsageAnalysisDaily
	require.NoError(t, DB.Find(&rows).Error)
	require.Len(t, rows, 2)
	total := UsageAnalysisTotals{}
	for _, row := range rows {
		total.Add(row.UsageAnalysisTotals)
	}
	assert.Equal(t, int64(2), total.Requests)
	assert.Equal(t, int64(142), total.Input)
}

func TestUsageAnalysisTokenNormalization(t *testing.T) {
	for _, tc := range []struct {
		name               string
		prompt             int
		other              map[string]interface{}
		input, read, write int64
	}{
		{"openai-inclusive", 100, map[string]interface{}{"cache_tokens": 40}, 100, 40, 0},
		{"anthropic-exclusive", 100, map[string]interface{}{"claude": true, "cache_tokens": 40, "cache_creation_tokens": 10}, 150, 40, 10},
		{"split-cache-not-double-counted", 100, map[string]interface{}{"claude": true, "cache_tokens": 40, "cache_creation_tokens": 90, "cache_creation_tokens_5m": 10, "cache_creation_tokens_1h": 20}, 170, 40, 30},
		{"normalized-total-wins", 100, map[string]interface{}{"claude": true, "cache_tokens": 40, "input_tokens_total": 120}, 120, 40, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := usageAnalysisRow(1, "alice", "model", time.Now().Unix(), tc.prompt, 20, tc.other)
			assert.Equal(t, tc.input, row.Input)
			assert.Equal(t, tc.read, row.CacheRead)
			assert.Equal(t, tc.write, row.CacheWrite)
		})
	}
}

func TestUsageAnalysisBatchBoundaryPreservesCommittedTotalsOnRetry(t *testing.T) {
	setupUsageAnalysisTest(t)
	require.NoError(t, InitUsageAnalysis())
	row := UsageAnalysisDaily{Day: "2026-11-12", UserID: 1, Username: "agent", Model: "deepseek", DepartmentID: 1, Source: "measured", UsageAnalysisTotals: UsageAnalysisTotals{Requests: 1, Input: 100, Output: 10, CacheRead: 90, Measured: 1}}
	payload, err := common.Marshal(row)
	require.NoError(t, err)
	// 501 records deliberately cross the worker's 500-event transaction boundary.
	events := make([]UsageAnalysisEvent, 501)
	for i := range events {
		events[i].Payload = string(payload)
	}
	require.NoError(t, DB.CreateInBatches(events, 100).Error)
	bad := UsageAnalysisEvent{Payload: "invalid-json"}
	require.NoError(t, DB.Create(&bad).Error)
	require.NoError(t, SyncUsageAnalysis())
	require.Error(t, SyncUsageAnalysis())
	var stored UsageAnalysisDaily
	require.NoError(t, DB.First(&stored).Error)
	assert.Equal(t, int64(500), stored.Requests)
	assert.Equal(t, int64(50000), stored.Input)
	var pending int64
	require.NoError(t, DB.Model(&UsageAnalysisEvent{}).Count(&pending).Error)
	assert.Equal(t, int64(2), pending)
	require.NoError(t, DB.Delete(&bad).Error)
	require.NoError(t, SyncUsageAnalysis())
	require.NoError(t, SyncUsageAnalysis())
	require.NoError(t, DB.First(&stored).Error)
	assert.Equal(t, UsageAnalysisTotals{Requests: 501, Input: 50100, Output: 5010, CacheRead: 45090, Measured: 501}, stored.UsageAnalysisTotals)
	require.NoError(t, DB.Model(&UsageAnalysisEvent{}).Count(&pending).Error)
	assert.Zero(t, pending)
}

func TestUsageAnalysisFiltersCombineAndCurrentDepartmentsDoNotRewriteHistory(t *testing.T) {
	setupUsageAnalysisTest(t)
	require.NoError(t, InitUsageAnalysis())
	a, err := SaveDepartment(0, "Engineering")
	require.NoError(t, err)
	b, err := SaveDepartment(0, "Research")
	require.NoError(t, err)
	require.NoError(t, DB.Create(&User{Id: 1, Username: "alice", DisplayName: "Alice", AffCode: "usage-alice"}).Error)
	require.NoError(t, DB.Create(&User{Id: 2, Username: "bob", DisplayName: "Bob", AffCode: "usage-bob"}).Error)
	require.NoError(t, AssignUserDepartment(1, a.Id))
	require.NoError(t, AssignUserDepartment(2, b.Id))
	today := time.Now().In(UsageAnalysisLocation).Format("2006-01-02")
	for _, row := range []UsageAnalysisDaily{
		{Day: today, UserID: 1, Username: "alice", Model: "qwen", DepartmentID: -1, Source: "measured", UsageAnalysisTotals: UsageAnalysisTotals{Requests: 1, Input: 100, Output: 10, Measured: 1}},
		{Day: today, UserID: 1, Username: "alice", Model: "deepseek", DepartmentID: -1, Source: "estimated", UsageAnalysisTotals: UsageAnalysisTotals{Requests: 1, Input: 300, Output: 30, Estimated: 1}},
		{Day: today, UserID: 2, Username: "bob", Model: "qwen", DepartmentID: a.Id, Department: a.Name, Source: "measured", UsageAnalysisTotals: UsageAnalysisTotals{Requests: 1, Input: 900, Output: 90, Measured: 1}},
	} {
		require.NoError(t, DB.Transaction(func(tx *gorm.DB) error { return mergeUsageAnalysis(tx, row) }))
	}
	f := UsageAnalysisFilter{Start: today, End: today, DepartmentBasis: "current", DepartmentID: &a.Id, Dimension: "user", Page: 1}
	company := f
	company.DepartmentID = nil
	company.Dimension = "department"
	companyReport, err := GetUsageAnalysisReport(company, time.Now())
	require.NoError(t, err)
	assert.Equal(t, int64(1300), companyReport.Totals.Input)
	assert.Equal(t, int64(2), companyReport.RowCount)
	report, err := GetUsageAnalysisReport(f, time.Now())
	require.NoError(t, err)
	assert.Equal(t, int64(400), report.Totals.Input)
	require.Len(t, report.Rows, 1)
	assert.Equal(t, "1|alice", report.Rows[0].Name)
	assert.Equal(t, int64(400), report.Trend[0].Input)
	assert.Equal(t, int64(400), report.Periods["today"].Input)
	f.UserID = 1
	f.Model = "qwen"
	f.Source = "measured"
	report, err = GetUsageAnalysisReport(f, time.Now())
	require.NoError(t, err)
	assert.Equal(t, int64(100), report.Totals.Input)
	f.UserID = 2
	report, err = GetUsageAnalysisReport(f, time.Now())
	require.NoError(t, err)
	assert.Zero(t, report.Totals.Input)
	f.UserID = 0
	f.Model = ""
	f.Source = ""
	f.DepartmentBasis = "historical"
	report, err = GetUsageAnalysisReport(f, time.Now())
	require.NoError(t, err)
	assert.Equal(t, int64(900), report.Totals.Input)
	f.DepartmentBasis = "current"
	choices, err := GetUsageAnalysisOptions(f)
	require.NoError(t, err)
	require.Len(t, choices.Users, 1)
	assert.Equal(t, "1", choices.Users[0].Value)
	assert.Contains(t, choices.Users[0].Label, "Alice")
	assert.Equal(t, []UsageAnalysisOption{{Value: "deepseek", Label: "deepseek"}, {Value: "qwen", Label: "qwen"}}, choices.Models)
	f.UserID = 2
	choices, err = GetUsageAnalysisOptions(f)
	require.NoError(t, err)
	assert.Empty(t, choices.Models)
	var unchanged int64
	require.NoError(t, DB.Model(&UsageAnalysisDaily{}).Where("department_id = ?", -1).Count(&unchanged).Error)
	assert.Equal(t, int64(2), unchanged)
}

func setupUsageAnalysisTest(t *testing.T) {
	t.Helper()
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&Department{}, &UserDepartment{}, &UsageAnalysisDaily{}, &UsageAnalysisEvent{}, &UsageAnalysisState{}, &UsageAnalysisPrice{}))
	for _, table := range []interface{}{&UsageAnalysisDaily{}, &UsageAnalysisEvent{}, &UsageAnalysisState{}, &UsageAnalysisPrice{}, &Department{}, &UserDepartment{}} {
		require.NoError(t, DB.Where("1 = 1").Delete(table).Error)
	}
	t.Cleanup(func() {
		for _, table := range []interface{}{&UsageAnalysisDaily{}, &UsageAnalysisEvent{}, &UsageAnalysisState{}, &UsageAnalysisPrice{}, &Department{}, &UserDepartment{}} {
			DB.Where("1 = 1").Delete(table)
		}
	})
}

func TestUsageAnalysisHistoricalImportAndDurableAggregation(t *testing.T) {
	setupUsageAnalysisTest(t)
	old := Log{Type: LogTypeConsume, CreatedAt: time.Now().Unix(), UserId: 7, Username: "alice", ModelName: "local", PromptTokens: 100, CompletionTokens: 20, Other: `{"claude":true,"cache_tokens":30,"cache_creation_tokens":10}`}
	require.NoError(t, LOG_DB.Create(&old).Error)
	require.NoError(t, InitUsageAnalysis())
	_, err := DeleteOldLogBatch(context.Background(), time.Now().Unix()+1, 100)
	require.ErrorContains(t, err, "import")
	require.NoError(t, SyncUsageAnalysis())
	require.NoError(t, SyncUsageAnalysis())
	var rows []UsageAnalysisDaily
	require.NoError(t, DB.Find(&rows).Error)
	require.Len(t, rows, 1)
	assert.Equal(t, int64(140), rows[0].Input)
	assert.Equal(t, -1, rows[0].DepartmentID)
	assert.Equal(t, int64(1), rows[0].Requests)
	// Log cleanup cannot remove the independent summary.
	_, err = DeleteOldLogBatch(context.Background(), time.Now().Unix()+1, 100)
	require.NoError(t, err)
	require.NoError(t, InitUsageAnalysis())
	require.NoError(t, SyncUsageAnalysis())
	var total int64
	require.NoError(t, DB.Model(&UsageAnalysisDaily{}).Select("SUM(requests)").Scan(&total).Error)
	assert.Equal(t, int64(1), total)
}

func TestUsageAnalysisFreeRequestsSnapshotAndRetry(t *testing.T) {
	setupUsageAnalysisTest(t)
	require.NoError(t, InitUsageAnalysis())
	department, err := SaveDepartment(0, "Engineering")
	require.NoError(t, err)
	require.NoError(t, DB.Create(&UserDepartment{UserId: 7, DepartmentId: department.Id}).Error)
	c := &gin.Context{}
	c.Set("username", "alice")
	CaptureUsageDepartment(c, 7)
	require.NoError(t, DB.Model(&UserDepartment{}).Where("user_id = ?", 7).Update("department_id", 999).Error)
	params := RecordConsumeLogParams{ModelName: "free-local", PromptTokens: 100, CompletionTokens: 20, Quota: 0, Other: map[string]interface{}{"cache_tokens": 30, "input_tokens_total": 100, "admin_info": map[string]interface{}{"usage_billing_path": "billing-usage-openai-estimated"}}}
	old := common.LogConsumeEnabled
	common.LogConsumeEnabled = false
	t.Cleanup(func() { common.LogConsumeEnabled = old })
	RecordConsumeLog(c, 7, params)
	RecordConsumeLog(c, 7, params)
	require.NoError(t, SyncUsageAnalysis())
	require.NoError(t, SyncUsageAnalysis())
	var rows []UsageAnalysisDaily
	require.NoError(t, DB.Find(&rows).Error)
	require.Len(t, rows, 1)
	assert.Equal(t, department.Id, rows[0].DepartmentID)
	assert.Equal(t, "Engineering", rows[0].Department)
	assert.Equal(t, int64(200), rows[0].Input)
	assert.Equal(t, int64(40), rows[0].Output)
	assert.Equal(t, int64(60), rows[0].CacheRead)
	assert.Equal(t, int64(2), rows[0].Estimated)
	var pending int64
	require.NoError(t, DB.Model(&UsageAnalysisEvent{}).Count(&pending).Error)
	assert.Zero(t, pending)
}

func TestUsageAnalysisPricesCalendarAndSavings(t *testing.T) {
	setupUsageAnalysisTest(t)
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, UsageAnalysisLocation)
	require.NoError(t, DB.Create(&UsageAnalysisState{ID: 1, StartedAt: time.Date(2025, 10, 1, 0, 0, 0, 0, UsageAnalysisLocation).Unix()}).Error)
	for _, day := range []string{"2025-10-01", "2026-09-20", "2026-10-18", "2026-10-19"} {
		require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
			return mergeUsageAnalysis(tx, UsageAnalysisDaily{Day: day, UserID: 7, Username: "alice", DepartmentID: 1, Department: "Engineering", Model: "local", UsageAnalysisTotals: UsageAnalysisTotals{Requests: 1, Input: 1000000, Output: 500000, Measured: 1}})
		}))
	}
	require.NoError(t, DB.Create(&UsageAnalysisPrice{LocalModel: "local", ReferenceModel: "before", EffectiveDay: "2026-10-01", InputPrice: 2, OutputPrice: 4}).Error)
	require.NoError(t, DB.Create(&UsageAnalysisPrice{LocalModel: "local", ReferenceModel: "after", EffectiveDay: "2026-10-19", InputPrice: 3, OutputPrice: 6}).Error)
	report, err := GetUsageAnalysisReport(UsageAnalysisFilter{Start: "2026-10-01", End: "2026-10-19", Dimension: "department", Page: 1}, now)
	require.NoError(t, err)
	assert.Equal(t, int64(2), report.Totals.Requests)
	assert.Equal(t, 10.0, report.Totals.AvoidedUSD)
	assert.Equal(t, int64(2), report.Periods["week"].Requests) // Sunday boundary.
	assert.Equal(t, 12, report.MonthCount)
	assert.Equal(t, 30, report.DayCount)
	assert.Equal(t, int64(66666), report.Periods["daily_average"].Input)
	assert.Equal(t, int64(166666), report.Periods["monthly_average"].Input)
	assert.Len(t, report.Trend, 19) // Zero-use days are retained.
	assert.Equal(t, int64(1), report.RowCount)
	assert.Equal(t, int64(1), report.Users)
	assert.Equal(t, int64(1), report.Models)
	require.Len(t, report.Rows, 1)
	assert.Equal(t, "1|Engineering", report.Rows[0].Name)
	price := UsageAnalysisPrice{LocalModel: "new-local", ReferenceModel: "test", EffectiveDay: time.Now().In(UsageAnalysisLocation).Format("2006-01-02"), InputPrice: 1, OutputPrice: 2}
	require.NoError(t, SaveUsageAnalysisPrice(&price))
	assert.Error(t, SaveUsageAnalysisPrice(&price)) // Append-only, no overwrite.
	price.EffectiveDay = "2020-01-01"
	assert.Error(t, SaveUsageAnalysisPrice(&price))
	modelReport, err := GetUsageAnalysisReport(UsageAnalysisFilter{Start: "2026-10-01", End: "2026-10-19", Dimension: "model", Page: 1}, now)
	require.NoError(t, err)
	require.Len(t, modelReport.Rows, 1)
	assert.Equal(t, 10.0, modelReport.Rows[0].AvoidedUSD)
	missing := 999
	emptyReport, err := GetUsageAnalysisReport(UsageAnalysisFilter{Start: "2026-10-01", End: "2026-10-19", DepartmentID: &missing, Dimension: "user", Page: 1}, now)
	require.NoError(t, err)
	assert.Zero(t, emptyReport.Periods["today"].Requests)
	assert.Zero(t, emptyReport.Totals.Input)
	assert.Empty(t, emptyReport.Rows)
}
