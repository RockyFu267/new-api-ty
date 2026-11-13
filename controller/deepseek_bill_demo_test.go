package controller

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestDeepSeekDemoOrdinaryUserCannotOverrideAccountScope(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(1)
	previous := model.LOG_DB
	model.LOG_DB = db
	t.Cleanup(func() { model.LOG_DB = previous; require.NoError(t, pool.Close()) })
	require.NoError(t, db.AutoMigrate(&model.Log{}))
	at := time.Date(2026, 11, 8, 10, 0, 0, 0, model.UsageAnalysisLocation)
	for _, id := range []int{1, 2} {
		require.NoError(t, db.Create(&model.Log{Type: model.LogTypeConsume, UserId: id, ModelName: model.DeepSeekDemoModel, CreatedAt: at.Unix(), PromptTokens: 100}).Error)
	}
	reports, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	reportPool, err := reports.DB()
	require.NoError(t, err)
	reportPool.SetMaxOpenConns(1)
	oldReport := model.DeepSeekReportDB
	model.DeepSeekReportDB = reports
	t.Cleanup(func() { model.DeepSeekReportDB = oldReport; require.NoError(t, reportPool.Close()) })
	require.NoError(t, model.InitDeepSeekReportStore(reports))
	require.NoError(t, model.ProcessDeepSeekReportBatch(context.Background(), db, reports, 200, time.Now()))

	for _, requested := range []string{"0", "2", "-1", "invalid"} {
		t.Run(requested, func(t *testing.T) {
			response := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(response)
			c.Request = httptest.NewRequest("GET", "/?start=2026-11-08&end=2026-11-08&user_id="+requested, nil)
			c.Set("id", 1)
			c.Set("role", common.RoleCommonUser)
			DeepSeekBillDemo(c)
			require.Equal(t, 200, response.Code)
			var body struct {
				Success bool                     `json:"success"`
				Data    model.DeepSeekDemoReport `json:"data"`
			}
			require.NoError(t, common.Unmarshal(response.Body.Bytes(), &body))
			require.True(t, body.Success)
			require.EqualValues(t, 1, body.Data.Totals.Requests)
			require.Len(t, body.Data.Users, 1)
			require.Equal(t, 1, body.Data.Users[0].UserID)
			require.Len(t, body.Data.Details, 1)
			require.Equal(t, 1, body.Data.Details[0].UserID)
		})
	}
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = httptest.NewRequest("GET", "/", nil)
	DeepSeekBillDemo(c)
	require.Equal(t, 401, response.Code)
}

func TestDeepSeekUnavailableReportStoreReturnsIsolatedUnavailableResponse(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, pool.Close())
	oldReport, oldLog := model.DeepSeekReportDB, model.LOG_DB
	model.DeepSeekReportDB, model.LOG_DB = db, nil
	t.Cleanup(func() { model.DeepSeekReportDB, model.LOG_DB = oldReport, oldLog })
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = httptest.NewRequest("GET", "/?start=2026-11-08&end=2026-11-08", nil)
	c.Set("id", 1)
	c.Set("role", common.RoleCommonUser)
	DeepSeekBillDemo(c)
	require.Equal(t, 503, response.Code)
	require.Contains(t, response.Body.String(), "Billing reports are temporarily unavailable")
	require.NotContains(t, response.Body.String(), "database is closed")
}
