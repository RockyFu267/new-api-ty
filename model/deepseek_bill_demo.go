package model

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"math"
	"time"

	"github.com/QuantumNous/new-api/common"
	ds "github.com/QuantumNous/new-api/setting/deepseek_setting"
)

const DeepSeekDemoModel = "deepseek-v4.1-flash"

// Costs use integer 1e-8 CNY units: the off-peak prices per million tokens
// are 0.02 CNY (cache hit), 1 CNY (miss), and 4 CNY (output).
type DeepSeekDemoTotals struct {
	Requests     int64 `json:"requests"`
	Input        int64 `json:"input"`
	Output       int64 `json:"output"`
	CacheRead    int64 `json:"cache_read"`
	PeakUnits    int64 `json:"peak_units"`
	OffPeakUnits int64 `json:"off_peak_units"`
	Estimated    int64 `json:"estimated"`
	Unpriced     int64 `json:"unpriced"`
	Actual       int64 `json:"actual"`
	ActualQuota  int64 `json:"actual_quota"`
}
type DeepSeekDemoUser struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	DeepSeekDemoTotals
}
type DeepSeekDemoDetail struct {
	ReportKey  string     `json:"report_key,omitempty"`
	ID         int        `json:"id"`
	RequestID  string     `json:"request_id"`
	UserID     int        `json:"user_id"`
	Username   string     `json:"username"`
	Time       int64      `json:"time"`
	TimeSource string     `json:"time_source"`
	Peak       bool       `json:"peak"`
	Input      int64      `json:"input"`
	Output     int64      `json:"output"`
	CacheRead  int64      `json:"cache_read"`
	CostUnits  int64      `json:"cost_units"`
	Estimated  bool       `json:"estimated"`
	Unpriced   bool       `json:"unpriced"`
	Actual     *ds.Charge `json:"actual,omitempty"`
}
type DeepSeekDemoReport struct {
	Model       string               `json:"model"`
	Snapshot    string               `json:"snapshot"`
	Totals      DeepSeekDemoTotals   `json:"totals"`
	Users       []DeepSeekDemoUser   `json:"users"`
	Details     []DeepSeekDemoDetail `json:"details"`
	DetailTotal int                  `json:"detail_total"`
	Page        int                  `json:"page"`
	NextCursor  string               `json:"next_cursor,omitempty"`
	Status      DeepSeekReportStatus `json:"status"`
}

func deepSeekDemoPeak(at time.Time) bool {
	at = at.In(UsageAnalysisLocation)
	if at.Weekday() == time.Saturday || at.Weekday() == time.Sunday {
		return false
	}
	// Verified 2026 holiday schedule. Demo queries are restricted to the period
	// after the official peak/off-peak policy began, and to this calendar year.
	day := at.Format("2006-01-02")
	if day >= "2026-09-25" && day <= "2026-09-27" || day >= "2026-10-01" && day <= "2026-10-07" {
		return false
	}
	minutes := at.Hour()*60 + at.Minute()
	return minutes >= 9*60 && minutes < 12*60 || minutes >= 14*60 && minutes < 18*60
}

func deepSeekDemoDetail(log Log) DeepSeekDemoDetail {
	detail := DeepSeekDemoDetail{ID: log.Id, RequestID: log.RequestId, UserID: log.UserId, Username: log.Username, Time: log.CreatedAt, TimeSource: "log_time", Estimated: true}
	if len(log.RequestId) >= 14 {
		if at, err := time.Parse("20060102150405", log.RequestId[:14]); err == nil && at.Unix() <= log.CreatedAt && log.CreatedAt-at.Unix() <= 86400 {
			detail.Time = at.Unix()
			detail.TimeSource = "request_id"
			detail.Estimated = false
		}
	}
	detail.Peak = deepSeekDemoPeak(time.Unix(detail.Time, 0))
	other := map[string]interface{}{}
	if log.Other != "" {
		if err := common.UnmarshalJsonStr(log.Other, &other); err != nil {
			detail.Unpriced = true
			detail.Estimated = true
			return detail
		}
	}
	usage := usageAnalysisRow(log.UserId, log.Username, log.ModelName, log.CreatedAt, log.PromptTokens, log.CompletionTokens, other)
	detail.Input, detail.Output, detail.CacheRead = usage.Input, usage.Output, usage.CacheRead
	detail.Estimated = detail.Estimated || usage.Measured == 0 || usage.CacheReported == 0
	if value, ok := other["deepseek_billing"]; ok {
		data, err := common.Marshal(value)
		var charge ds.Charge
		if err == nil {
			err = common.Unmarshal(data, &charge)
		}
		if err == nil && charge.Quota >= 0 && charge.CNY >= 0 {
			detail.Actual = &charge
			detail.Time = charge.Snapshot.StartedAt
			detail.TimeSource = "billing_snapshot"
			detail.Peak = charge.Snapshot.Peak
			detail.Input, detail.Output, detail.CacheRead = charge.Input, charge.Output, charge.Cache
			detail.CostUnits = int64(math.Round(charge.CNY * 1e8))
			return detail
		}
		detail.Unpriced = true
		detail.Estimated = true
		return detail
	}
	if time.Unix(detail.Time, 0).In(UsageAnalysisLocation).Year() != 2026 {
		detail.Unpriced = true
		detail.Estimated = true
		return detail
	}
	if log.PromptTokens < 0 || log.CompletionTokens < 0 || usage.Input > 1e9 || usage.Output > 1e9 || usage.CacheRead > usage.Input {
		detail.Unpriced = true
		detail.Estimated = true
		return detail
	}
	// Explicit malformed overrides cannot masquerade as trustworthy zero usage.
	for _, key := range []string{"cache_tokens", "input_tokens_total", "analysis_input_total"} {
		if _, exists := other[key]; exists {
			if _, valid := analysisNumber(other, key); !valid {
				detail.Unpriced = true
				detail.Estimated = true
				return detail
			}
		}
	}
	reference := ds.DefaultRule()
	reference.Enabled = true
	reference.EffectiveAt = 1
	snapshot := (ds.Store{Rules: []ds.Rule{reference}}).Select(ds.Model, time.Unix(detail.Time, 0))
	charge, err := snapshot.Calculate(detail.Input, detail.CacheRead, detail.Output)
	if err != nil {
		detail.Unpriced = true
		detail.Estimated = true
		return detail
	}
	detail.CostUnits = int64(math.Round(charge.CNY * 1e8))
	return detail
}

var deepSeekReportSlots = make(chan struct{}, 2)
var ErrDeepSeekReportBusy = errors.New("billing report is busy; retry later")
var ErrInvalidDeepSeekReportQuery = errors.New("invalid billing report query")

type deepSeekReportCursor struct {
	Time int64  `json:"time"`
	ID   int64  `json:"id"`
	Key  string `json:"key"`
}

func GetDeepSeekDemoReport(startDay, endDay string, userID, page int) (*DeepSeekDemoReport, error) {
	return ReadDeepSeekReport(context.Background(), startDay, endDay, userID, page, false, "")
}
func GetDeepSeekSelfDemoReport(startDay, endDay string, userID, page int) (*DeepSeekDemoReport, error) {
	return ReadDeepSeekReport(context.Background(), startDay, endDay, userID, page, true, "")
}

// ReadDeepSeekReport reads only precomputed data from the reporting pool. It
// never starts a worker, scans raw logs, reprices history, or changes balances.
func ReadDeepSeekReport(ctx context.Context, startDay, endDay string, userID, page int, selfOnly bool, cursor string) (*DeepSeekDemoReport, error) {
	start, err := time.ParseInLocation("2006-01-02", startDay, UsageAnalysisLocation)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid start date", ErrInvalidDeepSeekReportQuery)
	}
	end, err := time.ParseInLocation("2006-01-02", endDay, UsageAnalysisLocation)
	if err != nil || end.Before(start) || end.Sub(start) > 89*24*time.Hour || startDay < "2026-08-17" || endDay > time.Now().In(UsageAnalysisLocation).Format("2006-01-02") || userID < 0 || page < 1 || (selfOnly && userID <= 0) {
		return nil, fmt.Errorf("%w: select up to 90 days since 2026-08-17, not future dates", ErrInvalidDeepSeekReportQuery)
	}
	end = end.AddDate(0, 0, 1)
	var after deepSeekReportCursor
	if cursor != "" {
		if len(cursor) > 1024 {
			return nil, fmt.Errorf("%w: invalid report cursor", ErrInvalidDeepSeekReportQuery)
		}
		data, e := base64.RawURLEncoding.DecodeString(cursor)
		if e != nil || common.Unmarshal(data, &after) != nil || after.ID <= 0 || len(after.Key) == 0 || len(after.Key) > 191 || after.Time < start.Unix() || after.Time >= end.Unix() {
			return nil, fmt.Errorf("%w: invalid report cursor", ErrInvalidDeepSeekReportQuery)
		}
	} else if page > 1 {
		return nil, fmt.Errorf("%w: use the next-page cursor instead of an offset", ErrInvalidDeepSeekReportQuery)
	}
	report := &DeepSeekDemoReport{Model: DeepSeekDemoModel, Snapshot: "2026-11-09", Users: []DeepSeekDemoUser{}, Details: []DeepSeekDemoDetail{}, Page: page}
	if DeepSeekReportDB == nil {
		report.Status.Configured = DeepSeekReportingConfigured()
		return report, nil
	}
	select {
	case deepSeekReportSlots <- struct{}{}:
		defer func() { <-deepSeekReportSlots }()
	default:
		return nil, ErrDeepSeekReportBusy
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	err = reportReadTransaction(ctx, func(tx *gorm.DB) error {
		status, err := readDeepSeekReportStatus(tx, time.Now())
		if err != nil {
			return err
		}
		report.Status = status
		if status.LastSync == 0 || !status.HistoryComplete {
			return nil
		}
		query := tx.Model(&DeepSeekBillDaily{}).Where("day >= ? AND day <= ?", startDay, endDay)
		if selfOnly {
			query = query.Where("user_id = ?", userID)
		}
		fields := "user_id, MAX(username) AS username"
		for _, column := range []string{"requests", "input", "output", "cache_read", "peak_units", "off_peak_units", "estimated", "unpriced", "actual", "actual_quota"} {
			quoted := tx.Statement.Quote(column)
			fields += ", COALESCE(SUM(" + quoted + "),0) AS " + quoted
		}
		if err := query.Select(fields).Group("user_id").Order("SUM(peak_units) + SUM(off_peak_units) DESC").Order("user_id ASC").Scan(&report.Users).Error; err != nil {
			return err
		}
		for _, user := range report.Users {
			report.Totals.add(user.DeepSeekDemoTotals)
			if userID == 0 || user.UserID == userID {
				report.DetailTotal += int(user.Requests)
			}
		}
		facts := tx.Model(&DeepSeekBillFact{}).Where("time >= ? AND time < ?", start.Unix(), end.Unix())
		if selfOnly || userID > 0 {
			facts = facts.Where("user_id = ?", userID)
		}
		if cursor != "" {
			// A row comparison lets PostgreSQL seek directly into the composite
			// index. An OR expansion can scan every newer row on deep pages.
			// The explicit time bound also helps MySQL's range optimizer.
			facts = facts.Where("time <= ?", after.Time).
				Where("(time, log_id, report_key) < (?, ?, ?)", after.Time, after.ID, after.Key)
		}
		var rows []DeepSeekBillFact
		if err := facts.Order("time DESC").Order("log_id DESC").Order("report_key DESC").Limit(51).Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows[:min(len(rows), 50)] {
			var detail DeepSeekDemoDetail
			if err := common.UnmarshalJsonStr(row.Payload, &detail); err != nil {
				return err
			}
			report.Details = append(report.Details, detail)
		}
		if len(rows) > 50 {
			last := rows[49]
			data, err := common.Marshal(deepSeekReportCursor{Time: last.Time, ID: last.LogID, Key: last.ReportKey})
			if err != nil {
				return err
			}
			report.NextCursor = base64.RawURLEncoding.EncodeToString(data)
		}
		return nil
	})
	return report, err
}
