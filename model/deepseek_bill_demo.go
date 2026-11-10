package model

import (
	"fmt"
	"math"
	"sort"
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

func GetDeepSeekDemoReport(startDay, endDay string, userID, page int) (*DeepSeekDemoReport, error) {
	return getDeepSeekDemoReport(startDay, endDay, userID, page, false)
}

func GetDeepSeekSelfDemoReport(startDay, endDay string, userID, page int) (*DeepSeekDemoReport, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user")
	}
	return getDeepSeekDemoReport(startDay, endDay, userID, page, true)
}

func getDeepSeekDemoReport(startDay, endDay string, userID, page int, selfOnly bool) (*DeepSeekDemoReport, error) {
	start, err := time.ParseInLocation("2006-01-02", startDay, UsageAnalysisLocation)
	if err != nil {
		return nil, fmt.Errorf("invalid start date")
	}
	end, err := time.ParseInLocation("2006-01-02", endDay, UsageAnalysisLocation)
	if err != nil || end.Before(start) || end.Sub(start) > 89*24*time.Hour || startDay < "2026-08-17" || endDay > time.Now().In(UsageAnalysisLocation).Format("2006-01-02") || userID < 0 || page < 1 || page > 100000 {
		return nil, fmt.Errorf("select up to 90 days since 2026-08-17, not future dates")
	}
	end = end.AddDate(0, 0, 1)
	var logs []Log
	// Logs carry completion time. Include one extra day, then filter by the
	// request timestamp, so a conversation crossing midnight isn't misclassified.
	query := LOG_DB.Model(&Log{}).Select("id, user_id, username, created_at, prompt_tokens, completion_tokens, other, request_id, model_name").Where("type = ? AND model_name = ? AND created_at >= ? AND created_at < ?", LogTypeConsume, DeepSeekDemoModel, start.Unix(), end.AddDate(0, 0, 1).Unix())
	if selfOnly {
		query = query.Where("user_id = ?", userID)
	}
	err = query.Order("id DESC").Limit(200001).Find(&logs).Error
	if err != nil {
		return nil, err
	}
	if len(logs) > 200000 {
		return nil, fmt.Errorf("too many records; select a shorter date range")
	}
	report := &DeepSeekDemoReport{Model: DeepSeekDemoModel, Snapshot: "2026-11-09", Users: []DeepSeekDemoUser{}, Details: []DeepSeekDemoDetail{}, Page: page}
	users := map[int]*DeepSeekDemoUser{}
	details := []DeepSeekDemoDetail{}
	for _, log := range logs {
		detail := deepSeekDemoDetail(log)
		if detail.Time < start.Unix() || detail.Time >= end.Unix() {
			continue
		}
		user := users[log.UserId]
		if user == nil {
			user = &DeepSeekDemoUser{UserID: log.UserId, Username: log.Username}
			users[log.UserId] = user
		}
		for _, total := range []*DeepSeekDemoTotals{&user.DeepSeekDemoTotals, &report.Totals} {
			if detail.Actual != nil {
				total.Actual++
				total.ActualQuota += int64(detail.Actual.Quota)
			}
			total.Requests++
			total.Input += detail.Input
			total.Output += detail.Output
			total.CacheRead += detail.CacheRead
			if detail.Peak {
				total.PeakUnits += detail.CostUnits
			} else {
				total.OffPeakUnits += detail.CostUnits
			}
			if detail.Estimated {
				total.Estimated++
			}
			if detail.Unpriced {
				total.Unpriced++
			}
		}
		if userID == 0 || userID == log.UserId {
			details = append(details, detail)
		}
	}
	for _, user := range users {
		report.Users = append(report.Users, *user)
	}
	sort.Slice(report.Users, func(i, j int) bool {
		a, b := report.Users[i], report.Users[j]
		if a.PeakUnits+a.OffPeakUnits == b.PeakUnits+b.OffPeakUnits {
			return a.UserID < b.UserID
		}
		return a.PeakUnits+a.OffPeakUnits > b.PeakUnits+b.OffPeakUnits
	})
	sort.Slice(details, func(i, j int) bool {
		if details[i].Time == details[j].Time {
			return details[i].ID > details[j].ID
		}
		return details[i].Time > details[j].Time
	})
	report.DetailTotal = len(details)
	offset := (page - 1) * 50
	if offset < len(details) {
		report.Details = details[offset:min(offset+50, len(details))]
	}
	return report, nil
}
