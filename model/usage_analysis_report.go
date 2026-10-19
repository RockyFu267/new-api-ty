package model

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UsageAnalysisFilter struct {
	Start           string
	End             string
	DepartmentID    *int
	DepartmentBasis string
	UserID          int
	Model           string
	Source          string
	Local           string
	Dimension       string
	Page            int
}

type UsageAnalysisBucket struct {
	Name string `json:"name"`
	UsageAnalysisTotals
	AvoidedUSD     float64 `json:"avoided_usd"`
	PricedRequests int64   `json:"priced_requests"`
}

type UsageAnalysisReport struct {
	Totals        UsageAnalysisBucket            `json:"totals"`
	Periods       map[string]UsageAnalysisBucket `json:"periods"`
	Trend         []UsageAnalysisBucket          `json:"trend"`
	Rows          []UsageAnalysisBucket          `json:"rows"`
	RowCount      int64                          `json:"row_count"`
	Prices        []UsageAnalysisPrice           `json:"prices"`
	CoverageStart string                         `json:"coverage_start"`
	Imported      bool                           `json:"imported"`
	LastSync      int64                          `json:"last_sync"`
	Pending       int64                          `json:"pending"`
	MonthCount    int                            `json:"month_count"`
	DayCount      int                            `json:"day_count"`
	Users         int64                          `json:"users"`
	Models        int64                          `json:"models"`
}

func (a *UsageAnalysisTotals) Add(b UsageAnalysisTotals) {
	a.Requests += b.Requests
	a.Input += b.Input
	a.Output += b.Output
	a.CacheRead += b.CacheRead
	a.CacheWrite += b.CacheWrite
	a.CacheReported += b.CacheReported
	a.Measured += b.Measured
	a.Estimated += b.Estimated
	a.Unknown += b.Unknown
}

func (a UsageAnalysisTotals) Average(divisor float64) UsageAnalysisTotals {
	if divisor <= 0 {
		return UsageAnalysisTotals{}
	}
	return UsageAnalysisTotals{Requests: int64(float64(a.Requests) / divisor), Input: int64(float64(a.Input) / divisor), Output: int64(float64(a.Output) / divisor), CacheRead: int64(float64(a.CacheRead) / divisor), CacheWrite: int64(float64(a.CacheWrite) / divisor), CacheReported: int64(float64(a.CacheReported) / divisor), Measured: int64(float64(a.Measured) / divisor), Estimated: int64(float64(a.Estimated) / divisor), Unknown: int64(float64(a.Unknown) / divisor)}
}

func analysisSumSelect() string {
	result := ""
	for _, name := range []string{"requests", "input", "output", "cache_read", "cache_write", "cache_reported", "measured", "estimated", "unknown"} {
		quoted := DB.Statement.Quote(clause.Column{Name: name})
		if result != "" {
			result += ", "
		}
		result += "COALESCE(SUM(" + quoted + "), 0) AS " + quoted
	}
	return result
}

func analysisBaseQuery(f UsageAnalysisFilter) *gorm.DB {
	tx := DB.Model(&UsageAnalysisDaily{})
	if f.DepartmentBasis == "current" {
		// A separate reporting view; never rewrite the department saved with the request.
		columns := []string{}
		for _, column := range []string{"day", "user_id", "username", "model", "source", "requests", "input", "output", "cache_read", "cache_write", "cache_reported", "measured", "estimated", "unknown"} {
			columns = append(columns, DB.Statement.Quote(clause.Column{Table: "usage_rows", Name: column}))
		}
		columns = append(columns, "COALESCE(usage_members.department_id, 0) AS department_id", "COALESCE(usage_departments.name, '') AS department")
		view := DB.Table("usage_analysis_dailies AS usage_rows").Select(strings.Join(columns, ", ")).Joins("LEFT JOIN user_departments AS usage_members ON usage_members.user_id = usage_rows.user_id").Joins("LEFT JOIN departments AS usage_departments ON usage_departments.id = usage_members.department_id")
		tx = DB.Table("(?) AS usage_analysis_dailies", view)
	}
	if f.DepartmentID != nil {
		tx = tx.Where("department_id = ?", *f.DepartmentID)
	}
	if f.UserID > 0 {
		tx = tx.Where("user_id = ?", f.UserID)
	}
	if f.Model != "" {
		tx = tx.Where("model = ?", f.Model)
	}
	if f.Source != "" {
		tx = tx.Where("source = ?", f.Source)
	}
	localModels := DB.Model(&UsageAnalysisPrice{}).Select("local_model")
	if f.Local == "local" {
		tx = tx.Where("model IN (?)", localModels)
	}
	if f.Local == "commercial" {
		tx = tx.Where("model NOT IN (?)", localModels)
	}
	return tx
}

func GetUsageAnalysisReport(f UsageAnalysisFilter, now time.Time) (*UsageAnalysisReport, error) {
	now = now.In(UsageAnalysisLocation)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, UsageAnalysisLocation)
	month := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, UsageAnalysisLocation)
	week := today.AddDate(0, 0, -int(today.Weekday()))
	yearStart := month.AddDate(-1, 0, 0)
	start, err := time.ParseInLocation("2006-01-02", f.Start, UsageAnalysisLocation)
	if err != nil {
		return nil, err
	}
	end, err := time.ParseInLocation("2006-01-02", f.End, UsageAnalysisLocation)
	if err != nil {
		return nil, err
	}
	if start.After(end) || end.After(today) || end.Sub(start) > 400*24*time.Hour {
		return nil, fmt.Errorf("invalid date range (maximum 400 days, no future dates)")
	}
	var state UsageAnalysisState
	if err := DB.First(&state, 1).Error; err != nil {
		return nil, err
	}
	report := &UsageAnalysisReport{Periods: map[string]UsageAnalysisBucket{}, Trend: []UsageAnalysisBucket{}, Rows: []UsageAnalysisBucket{}, Prices: []UsageAnalysisPrice{}, Imported: state.Imported >= state.ImportThrough, LastSync: state.LastSync}
	for _, key := range []string{"today", "week", "month"} {
		report.Periods[key] = UsageAnalysisBucket{}
	}
	if err := DB.Order("effective_day DESC, id DESC").Find(&report.Prices).Error; err != nil {
		return nil, err
	}
	if err := DB.Model(&UsageAnalysisEvent{}).Count(&report.Pending).Error; err != nil {
		return nil, err
	}
	var earliest struct{ Day string }
	if err := DB.Model(&UsageAnalysisDaily{}).Select("COALESCE(MIN(day), '') AS day").Scan(&earliest).Error; err != nil {
		return nil, err
	}
	report.CoverageStart = time.Unix(state.StartedAt, 0).In(UsageAnalysisLocation).Format("2006-01-02")
	if earliest.Day != "" && earliest.Day < report.CoverageStart {
		report.CoverageStart = earliest.Day
	}
	queryStart := yearStart.Format("2006-01-02")
	if f.Start < queryStart {
		queryStart = f.Start
	}
	var daily []struct {
		Day   string
		Model string
		UsageAnalysisTotals
	}
	if err := analysisBaseQuery(f).Where("day >= ? AND day <= ?", queryStart, today.Format("2006-01-02")).Select("day, model, " + analysisSumSelect()).Group("day, model").Scan(&daily).Error; err != nil {
		return nil, err
	}
	trend := map[string]*UsageAnalysisBucket{}
	modelSavings := map[string]*UsageAnalysisBucket{}
	monthlyAverage := UsageAnalysisTotals{}
	recent := UsageAnalysisTotals{}
	for _, row := range daily {
		for key, begin := range map[string]string{"today": today.Format("2006-01-02"), "week": week.Format("2006-01-02"), "month": month.Format("2006-01-02")} {
			if row.Day >= begin {
				b := report.Periods[key]
				b.Add(row.UsageAnalysisTotals)
				report.Periods[key] = b
			}
		}
		if row.Day >= yearStart.Format("2006-01-02") && row.Day < month.Format("2006-01-02") {
			monthlyAverage.Add(row.UsageAnalysisTotals)
		}
		if row.Day >= today.AddDate(0, 0, -30).Format("2006-01-02") && row.Day < today.Format("2006-01-02") {
			recent.Add(row.UsageAnalysisTotals)
		}
		if row.Day < f.Start || row.Day > f.End {
			continue
		}
		b := trend[row.Day]
		if b == nil {
			b = &UsageAnalysisBucket{Name: row.Day}
			trend[row.Day] = b
		}
		b.Add(row.UsageAnalysisTotals)
		// Prices are sorted newest first; price changes never rewrite older days.
		for _, p := range report.Prices {
			if p.LocalModel == row.Model && p.EffectiveDay <= row.Day {
				amount := (float64(row.Input)*p.InputPrice + float64(row.Output)*p.OutputPrice) / 1e6
				b.AvoidedUSD += amount
				b.PricedRequests += row.Requests
				if modelSavings[row.Model] == nil {
					modelSavings[row.Model] = &UsageAnalysisBucket{}
				}
				modelSavings[row.Model].AvoidedUSD += amount
				modelSavings[row.Model].PricedRequests += row.Requests
				break
			}
		}
	}
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		b := trend[key]
		if b == nil {
			b = &UsageAnalysisBucket{Name: key}
		}
		report.Trend = append(report.Trend, *b)
		report.Totals.Add(b.UsageAnalysisTotals)
		report.Totals.AvoidedUSD += b.AvoidedUSD
		report.Totals.PricedRequests += b.PricedRequests
	}
	coverage, _ := time.ParseInLocation("2006-01-02", report.CoverageStart, UsageAnalysisLocation)
	for d := yearStart; d.Before(month); d = d.AddDate(0, 1, 0) {
		if !d.Before(coverage) {
			report.MonthCount++
		}
	}
	averageStart := today.AddDate(0, 0, -30)
	if coverage.After(averageStart) {
		averageStart = coverage
	}
	report.DayCount = int(today.Sub(averageStart).Hours() / 24)
	if report.DayCount < 0 {
		report.DayCount = 0
	}
	// A partial first month is excluded from both numerator and denominator.
	if coverage.Day() != 1 && coverage.Before(month) && !coverage.Before(yearStart) {
		monthlyAverage = UsageAnalysisTotals{}
		for _, row := range daily {
			if row.Day >= time.Date(coverage.Year(), coverage.Month()+1, 1, 0, 0, 0, 0, UsageAnalysisLocation).Format("2006-01-02") && row.Day < month.Format("2006-01-02") {
				monthlyAverage.Add(row.UsageAnalysisTotals)
			}
		}
	}
	report.Periods["monthly_average"] = UsageAnalysisBucket{UsageAnalysisTotals: monthlyAverage.Average(float64(report.MonthCount))}
	report.Periods["daily_average"] = UsageAnalysisBucket{UsageAnalysisTotals: recent.Average(float64(report.DayCount))}
	report.Periods["weekly_average"] = UsageAnalysisBucket{UsageAnalysisTotals: recent.Average(float64(report.DayCount) / 7)}
	var selectGroup, group string
	switch f.Dimension {
	case "user":
		selectGroup = "username AS name"
		group = "user_id, username"
	case "model":
		selectGroup = "model AS name"
		group = "model"
	case "department":
		selectGroup = "department AS name"
		group = "department_id, department"
	default:
		return nil, fmt.Errorf("invalid dimension")
	}
	base := analysisBaseQuery(f).Where("day >= ? AND day <= ?", f.Start, f.End)
	if err := base.Session(&gorm.Session{}).Distinct("user_id").Count(&report.Users).Error; err != nil {
		return nil, err
	}
	if err := base.Session(&gorm.Session{}).Distinct("model").Count(&report.Models).Error; err != nil {
		return nil, err
	}
	if err := DB.Table("(?) AS usage_groups", base.Session(&gorm.Session{}).Select(group).Group(group)).Count(&report.RowCount).Error; err != nil {
		return nil, err
	}
	if f.Page < 1 {
		f.Page = 1
	}
	// Include IDs for unambiguous department labels and drilldown.
	type detail struct {
		Name         string
		UserID       int
		DepartmentID int
		UsageAnalysisTotals
	}
	var details []detail
	if f.Dimension == "user" {
		selectGroup += ", user_id"
	}
	if f.Dimension == "department" {
		selectGroup += ", department_id"
	}
	if err := base.Select(selectGroup + ", " + analysisSumSelect()).Group(group).Order("SUM(requests) DESC, name ASC").Offset((f.Page - 1) * 20).Limit(20).Scan(&details).Error; err != nil {
		return nil, err
	}
	for _, d := range details {
		name := d.Name
		if f.Dimension == "department" {
			if d.DepartmentID == -1 {
				name = "historical_unknown"
			} else if d.DepartmentID == 0 {
				name = "unassigned"
			}
			name = fmt.Sprintf("%d|%s", d.DepartmentID, name)
		}
		if f.Dimension == "user" {
			name = fmt.Sprintf("%d|%s", d.UserID, name)
		}
		bucket := UsageAnalysisBucket{Name: name, UsageAnalysisTotals: d.UsageAnalysisTotals}
		if f.Dimension == "model" && modelSavings[d.Name] != nil {
			bucket.AvoidedUSD = modelSavings[d.Name].AvoidedUSD
			bucket.PricedRequests = modelSavings[d.Name].PricedRequests
		}
		report.Rows = append(report.Rows, bucket)
	}
	sort.Slice(report.Trend, func(i, j int) bool { return report.Trend[i].Name < report.Trend[j].Name })
	return report, nil
}
