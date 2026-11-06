package model

import (
	"crypto/sha256"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var UsageAnalysisLocation = time.FixedZone("Asia/Shanghai", 8*3600)
var usageAnalysisSync sync.Mutex

// These tables live in the primary database and survive consume-log cleanup.
type UsageAnalysisTotals struct {
	Requests      int64 `json:"requests"`
	Input         int64 `json:"input"`
	Output        int64 `json:"output"`
	CacheRead     int64 `json:"cache_read"`
	CacheWrite    int64 `json:"cache_write"`
	CacheReported int64 `json:"cache_reported"`
	Measured      int64 `json:"measured"`
	Estimated     int64 `json:"estimated"`
	Unknown       int64 `json:"unknown"`
}

type UsageAnalysisDaily struct {
	Key                 string `json:"-" gorm:"primaryKey;size:64"`
	Day                 string `json:"day" gorm:"size:10;index"`
	UserID              int    `json:"user_id" gorm:"index"`
	Username            string `json:"username" gorm:"size:64"`
	DepartmentID        int    `json:"department_id" gorm:"index"`
	Department          string `json:"department" gorm:"size:64"`
	Model               string `json:"model" gorm:"size:255;index"`
	Source              string `json:"source" gorm:"size:16;index"`
	UsageAnalysisTotals `gorm:"embedded"`
}

type UsageAnalysisEvent struct {
	ID      int64  `gorm:"primaryKey"`
	Payload string `gorm:"type:text"`
}

type UsageAnalysisState struct {
	ID            int `gorm:"primaryKey;autoIncrement:false"`
	ImportThrough int64
	Imported      int64
	StartedAt     int64
	LastSync      int64
}

// Price versions are append-only. USD per million tokens; no cache discount is assumed.
type UsageAnalysisPrice struct {
	ID             int     `json:"id" gorm:"primaryKey"`
	LocalModel     string  `json:"local_model" gorm:"size:255;uniqueIndex:idx_usage_price,priority:1"`
	ReferenceModel string  `json:"reference_model" gorm:"size:255"`
	EffectiveDay   string  `json:"effective_day" gorm:"size:10;uniqueIndex:idx_usage_price,priority:2"`
	InputPrice     float64 `json:"input_price"`
	OutputPrice    float64 `json:"output_price"`
	CreatedBy      int     `json:"created_by"`
	CreatedAt      int64   `json:"created_at"`
}

func InitUsageAnalysis() error {
	if err := DB.AutoMigrate(&UsageAnalysisDaily{}, &UsageAnalysisEvent{}, &UsageAnalysisState{}, &UsageAnalysisPrice{}); err != nil {
		return err
	}
	var through int64
	if err := LOG_DB.Model(&Log{}).Select("COALESCE(MAX(id), 0)").Scan(&through).Error; err != nil {
		return err
	}
	return DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&UsageAnalysisState{ID: 1, ImportThrough: through, StartedAt: time.Now().Unix()}).Error
}

// Capture before calling upstream, so transfers between departments don't rewrite history.
func CaptureUsageDepartment(c *gin.Context, userID int) {
	var row struct {
		DepartmentID int
		Name         string
	}
	err := DB.Table("user_departments").Select("user_departments.department_id, departments.name").Joins("LEFT JOIN departments ON departments.id = user_departments.department_id").Where("user_departments.user_id = ?", userID).Scan(&row).Error
	if err != nil {
		common.SysError("usage analysis department snapshot: " + err.Error())
		return
	}
	c.Set("usage_department_id", row.DepartmentID)
	c.Set("usage_department", row.Name)
}

func analysisNumber(other map[string]interface{}, key string) (int64, bool) {
	v, ok := other[key]
	if !ok || v == nil {
		return 0, false
	}
	var n float64
	switch v := v.(type) {
	case float64:
		n = v
	case int:
		n = float64(v)
	case int64:
		n = float64(v)
	default:
		return 0, false
	}
	// Reject corrupted upstream values, including NaN, instead of overflowing aggregates.
	if math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n > 1e12 {
		return 0, false
	}
	return int64(n), true
}

func usageAnalysisRow(userID int, username, model string, created int64, prompt, completion int, other map[string]interface{}) UsageAnalysisDaily {
	row := UsageAnalysisDaily{Day: time.Unix(created, 0).In(UsageAnalysisLocation).Format("2006-01-02"), UserID: userID, Username: username, Model: model, DepartmentID: -1}
	row.Requests = 1
	row.Input, _ = analysisNumber(map[string]interface{}{"v": prompt}, "v")
	row.Output, _ = analysisNumber(map[string]interface{}{"v": completion}, "v")
	row.CacheRead, _ = analysisNumber(other, "cache_tokens")
	row.CacheWrite, _ = analysisNumber(other, "cache_write_tokens")
	if _, ok := other["cache_write_tokens"]; !ok {
		row.CacheWrite, _ = analysisNumber(other, "cache_creation_tokens")
		five, _ := analysisNumber(other, "cache_creation_tokens_5m")
		hour, _ := analysisNumber(other, "cache_creation_tokens_1h")
		if five+hour > 0 {
			row.CacheWrite = five + hour
		}
	}
	if id, ok := analysisNumber(other, "analysis_department_id"); ok {
		row.DepartmentID = int(id)
		row.Department, _ = other["analysis_department"].(string)
	}
	path := ""
	if admin, ok := other["admin_info"].(map[string]interface{}); ok {
		path, _ = admin["usage_billing_path"].(string)
	}
	if strings.Contains(path, "anthropic") || other["claude"] == true {
		row.Input += row.CacheRead + row.CacheWrite
	}
	if total, ok := analysisNumber(other, "input_tokens_total"); ok {
		row.Input = total
	}
	if total, ok := analysisNumber(other, "analysis_input_total"); ok {
		row.Input = total
	}
	if strings.Contains(path, "estimated") || path == "local" || other["analysis_estimated"] == true {
		row.Estimated = 1
	} else if path != "" && row.Input+row.Output > 0 {
		row.Measured = 1
	} else {
		row.Unknown = 1
	}
	row.Source = "unknown"
	if row.Measured > 0 {
		row.Source = "measured"
	}
	if row.Estimated > 0 {
		row.Source = "estimated"
	}
	// A zero in a synthetic log field does not prove upstream cache reporting.
	if other["analysis_cache_reported"] == true || row.CacheRead+row.CacheWrite > 0 {
		row.CacheReported = 1
	}
	return row
}

func QueueUsageAnalysis(c *gin.Context, userID int, params RecordConsumeLogParams) error {
	other := make(map[string]interface{}, len(params.Other)+2)
	for k, v := range params.Other {
		other[k] = v
	}
	if id, ok := c.Get("usage_department_id"); ok {
		other["analysis_department_id"] = id
		other["analysis_department"] = c.GetString("usage_department")
	}
	row := usageAnalysisRow(userID, c.GetString("username"), params.ModelName, time.Now().Unix(), params.PromptTokens, params.CompletionTokens, other)
	payload, err := common.Marshal(row)
	if err != nil {
		return err
	}
	return DB.Create(&UsageAnalysisEvent{Payload: string(payload)}).Error
}

func mergeUsageAnalysis(tx *gorm.DB, row UsageAnalysisDaily) error {
	identity, err := common.Marshal([]interface{}{row.Day, row.UserID, row.Username, row.DepartmentID, row.Department, row.Model, row.Source})
	if err != nil {
		return err
	}
	row.Key = fmt.Sprintf("%x", sha256.Sum256(identity))
	updates := map[string]interface{}{}
	values := map[string]int64{"requests": row.Requests, "input": row.Input, "output": row.Output, "cache_read": row.CacheRead, "cache_write": row.CacheWrite, "cache_reported": row.CacheReported, "measured": row.Measured, "estimated": row.Estimated, "unknown": row.Unknown}
	for column, value := range values {
		// clause.Column keeps reserved names quoted for all supported databases.
		updates[column] = gorm.Expr("? + ?", clause.Column{Table: clause.CurrentTable, Name: column}, value)
	}
	return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "key"}}, DoUpdates: clause.Assignments(updates)}).Create(&row).Error
}

// Checkpoint, aggregate updates and queue removal commit together. Retrying is safe.
func SyncUsageAnalysis() error {
	usageAnalysisSync.Lock()
	defer usageAnalysisSync.Unlock()
	var before UsageAnalysisState
	if err := DB.First(&before, 1).Error; err != nil {
		return err
	}
	var history []Log
	if before.Imported < before.ImportThrough {
		if err := LOG_DB.Where("id > ? AND id <= ?", before.Imported, before.ImportThrough).Order("id ASC").Limit(500).Find(&history).Error; err != nil {
			return err
		}
	}
	return quotaWriteTransaction(DB, func(tx *gorm.DB) error {
		var state UsageAnalysisState
		if err := lockForUpdate(tx).First(&state, 1).Error; err != nil {
			return err
		}
		// Another instance may have imported this batch while we read it.
		if state.Imported == before.Imported && state.Imported < state.ImportThrough {
			for _, log := range history {
				if log.Type == LogTypeConsume {
					other := map[string]interface{}{}
					_ = common.UnmarshalJsonStr(log.Other, &other)
					row := usageAnalysisRow(log.UserId, log.Username, log.ModelName, log.CreatedAt, log.PromptTokens, log.CompletionTokens, other)
					if err := mergeUsageAnalysis(tx, row); err != nil {
						return err
					}
				}
				state.Imported = int64(log.Id)
			}
			if len(history) < 500 {
				state.Imported = state.ImportThrough
			}
		}
		var events []UsageAnalysisEvent
		if err := tx.Order("id ASC").Limit(500).Find(&events).Error; err != nil {
			return err
		}
		for _, event := range events {
			var row UsageAnalysisDaily
			if err := common.UnmarshalJsonStr(event.Payload, &row); err != nil {
				return err
			}
			if err := mergeUsageAnalysis(tx, row); err != nil {
				return err
			}
			if err := tx.Delete(&event).Error; err != nil {
				return err
			}
		}
		state.LastSync = time.Now().Unix()
		return tx.Save(&state).Error
	})
}

func RunUsageAnalysis() {
	for {
		if err := SyncUsageAnalysis(); err != nil {
			common.SysError("usage analysis aggregation: " + err.Error())
		}
		time.Sleep(5 * time.Second)
	}
}

func SaveUsageAnalysisPrice(price *UsageAnalysisPrice) error {
	price.LocalModel = strings.TrimSpace(price.LocalModel)
	price.ReferenceModel = strings.TrimSpace(price.ReferenceModel)
	day, err := time.ParseInLocation("2006-01-02", price.EffectiveDay, UsageAnalysisLocation)
	today := time.Now().In(UsageAnalysisLocation).Format("2006-01-02")
	if err != nil || day.Format("2006-01-02") < today || price.LocalModel == "" || price.ReferenceModel == "" || len(price.LocalModel) > 255 || len(price.ReferenceModel) > 255 {
		return fmt.Errorf("invalid model or effective date; historical prices are immutable")
	}
	for _, p := range []float64{price.InputPrice, price.OutputPrice} {
		if math.IsNaN(p) || math.IsInf(p, 0) || p < 0 || p > 1e6 {
			return fmt.Errorf("invalid USD price per million tokens")
		}
	}
	price.ID = 0
	price.CreatedAt = time.Now().Unix()
	return DB.Create(price).Error
}
