package controller

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsageAnalysisCSVIncludesAllPagesAndProtectsNames(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}))
	require.NoError(t, model.InitUsageAnalysis())
	today := time.Now().In(model.UsageAnalysisLocation).Format("2006-01-02")
	for i := 0; i < 21; i++ {
		name := fmt.Sprintf("model-%02d", i)
		if i == 0 {
			name = "=SUM(1,2)"
		}
		require.NoError(t, db.Create(&model.UsageAnalysisDaily{Key: fmt.Sprint(i), Day: today, Model: name, UsageAnalysisTotals: model.UsageAnalysisTotals{Requests: 1, Input: 100, Output: 20}}).Error)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/user/usage-analysis/export?dimension=model&start="+today+"&end="+today, nil)
	ExportUsageAnalysis(c)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/csv")
	rows, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(w.Body.String(), "\ufeff"))).ReadAll()
	require.NoError(t, err)
	require.Len(t, rows, 30)
	var models int
	var formulaProtected bool
	for _, row := range rows {
		require.Len(t, row, 13)
		if row[0] == "model" {
			models++
			if row[1] == "'=SUM(1,2)" {
				formulaProtected = true
			}
		}
	}
	assert.Equal(t, 21, models)
	assert.True(t, formulaProtected)
	assert.Equal(t, "range_total", rows[1][0])
	assert.Equal(t, "2100", rows[1][3])
}

func TestUsageAnalysisRejectsInvalidFilters(t *testing.T) {
	for _, query := range []string{"department_id=bad", "user_id=-1", "source=invalid", "page=0", "department_basis=invalid"} {
		t.Run(query, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/api/user/usage-analysis?"+query, nil)
			UsageAnalysis(c)
			var response struct {
				Success bool
				Message string
			}
			require.NoError(t, common.Unmarshal(w.Body.Bytes(), &response))
			assert.False(t, response.Success)
			assert.NotEmpty(t, response.Message)
		})
	}
}
