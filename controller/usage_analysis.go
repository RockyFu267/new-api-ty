package controller

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func usageAnalysisFilter(c *gin.Context) (model.UsageAnalysisFilter, error) {
	now := time.Now().In(model.UsageAnalysisLocation)
	f := model.UsageAnalysisFilter{Start: c.DefaultQuery("start", now.Format("2006-01-01")), End: c.DefaultQuery("end", now.Format("2006-01-02")), Dimension: c.DefaultQuery("dimension", "department"), Model: c.Query("model"), Local: c.Query("local"), Page: 1}
	f.Source = c.Query("source")
	f.DepartmentBasis = c.Query("department_basis")
	if f.DepartmentBasis != "" && f.DepartmentBasis != "current" && f.DepartmentBasis != "historical" {
		return f, fmt.Errorf("invalid department basis")
	}
	if f.Source != "" && f.Source != "measured" && f.Source != "estimated" && f.Source != "unknown" {
		return f, fmt.Errorf("invalid token data source")
	}
	if value := c.Query("department_id"); value != "" {
		id, err := strconv.Atoi(value)
		if err != nil || id < -1 {
			return f, fmt.Errorf("invalid department")
		}
		f.DepartmentID = &id
	}
	if value := c.Query("user_id"); value != "" {
		id, err := strconv.Atoi(value)
		if err != nil || id <= 0 {
			return f, fmt.Errorf("invalid user")
		}
		f.UserID = id
	}
	if value := c.Query("page"); value != "" {
		page, err := strconv.Atoi(value)
		if err != nil || page < 1 || page > 100000 {
			return f, fmt.Errorf("invalid page")
		}
		f.Page = page
	}
	if f.Local != "" && f.Local != "local" && f.Local != "commercial" {
		return f, fmt.Errorf("invalid model source")
	}
	return f, nil
}

func UsageAnalysisOptions(c *gin.Context) {
	f, err := usageAnalysisFilter(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	options, err := model.GetUsageAnalysisOptions(f)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, options)
}

func UsageAnalysis(c *gin.Context) {
	f, err := usageAnalysisFilter(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	report, err := model.GetUsageAnalysisReport(f, time.Now())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": report})
}

func CreateUsageAnalysisPrice(c *gin.Context) {
	var price model.UsageAnalysisPrice
	if err := common.DecodeJson(c.Request.Body, &price); err != nil {
		common.ApiError(c, err)
		return
	}
	price.CreatedBy = c.GetInt("id")
	if err := model.SaveUsageAnalysisPrice(&price); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": price})
}

func ExportUsageAnalysis(c *gin.Context) {
	f, err := usageAnalysisFilter(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	f.Page = 1
	report, err := model.GetUsageAnalysisReport(f, time.Now())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	basis := f.DepartmentBasis
	if basis == "" {
		basis = "historical"
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=usage-analysis-%s.csv", basis))
	_, _ = c.Writer.Write([]byte{0xef, 0xbb, 0xbf})
	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"record_type", "name", "requests", "input_tokens", "output_tokens", "cache_read_tokens", "cache_write_tokens", "cache_reporting_requests", "measured_requests", "estimated_requests", "unknown_requests", "estimated_avoided_usd", "priced_requests"})
	writeRow := func(kind string, row model.UsageAnalysisBucket) {
		name := row.Name
		trimmed := strings.TrimLeft(name, " \t\r\n")
		if len(trimmed) > 0 && strings.ContainsRune("=+-@", rune(trimmed[0])) {
			name = "'" + name
		}
		cost, priced := "", ""
		if kind == "range_total" || kind == "daily_trend" || kind == "model" {
			cost = strconv.FormatFloat(row.AvoidedUSD, 'f', 6, 64)
			priced = strconv.FormatInt(row.PricedRequests, 10)
		}
		_ = w.Write([]string{kind, name, strconv.FormatInt(row.Requests, 10), strconv.FormatInt(row.Input, 10), strconv.FormatInt(row.Output, 10), strconv.FormatInt(row.CacheRead, 10), strconv.FormatInt(row.CacheWrite, 10), strconv.FormatInt(row.CacheReported, 10), strconv.FormatInt(row.Measured, 10), strconv.FormatInt(row.Estimated, 10), strconv.FormatInt(row.Unknown, 10), cost, priced})
	}
	report.Totals.Name = f.Start + ".." + f.End
	writeRow("range_total", report.Totals)
	for _, key := range []string{"today", "week", "month", "monthly_average", "weekly_average", "daily_average"} {
		row := report.Periods[key]
		row.Name = key
		writeRow("period", row)
	}
	for _, row := range report.Trend {
		writeRow("daily_trend", row)
	}
	for {
		for _, row := range report.Rows {
			writeRow(f.Dimension, row)
		}
		w.Flush()
		if w.Error() != nil {
			return
		}
		if int64(f.Page*20) >= report.RowCount {
			return
		}
		f.Page++
		report, err = model.GetUsageAnalysisReport(f, time.Now())
		if err != nil {
			c.Error(err)
			return
		}
	}
}
