package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// This handle has its own small pool and points to a separate reporting database.
// No report request reads the gateway's logs or uses its accounting pool.
var DeepSeekReportDB *gorm.DB

type DeepSeekBillFact struct {
	ReportKey string `gorm:"primaryKey;size:191;index:idx_ds_bill_time,priority:3;index:idx_ds_bill_user_time,priority:4"`
	LogID     int64  `gorm:"index:idx_ds_bill_time,priority:2;index:idx_ds_bill_user_time,priority:3"`
	UserID    int    `gorm:"index:idx_ds_bill_user_time,priority:1"`
	Time      int64  `gorm:"index:idx_ds_bill_time,priority:1;index:idx_ds_bill_user_time,priority:2"`
	Payload   string `gorm:"type:text"`
}

func deepSeekBillFactKey(log Log) string { return fmt.Sprintf("%d/%s", log.Id, log.RequestId) }

type DeepSeekBillDaily struct {
	Day                string `gorm:"primaryKey;size:10;index:idx_ds_daily_user_day,priority:2"`
	UserID             int    `gorm:"primaryKey;autoIncrement:false;index:idx_ds_daily_user_day,priority:1"`
	Username           string `gorm:"size:64"`
	DeepSeekDemoTotals `gorm:"embedded"`
}

type DeepSeekBillState struct {
	ID              int `gorm:"primaryKey;autoIncrement:false"`
	Cursor          int64
	InitialThrough  int64
	SourceThrough   int64
	ReplayCursor    int64
	ReplayThrough   int64
	LastReplayAt    int64
	LastSync        int64
	HistoryComplete bool
	TailBatches     int
}

type DeepSeekReportStatus struct {
	Configured      bool  `json:"configured"`
	HistoryComplete bool  `json:"history_complete"`
	WorkerRunning   bool  `json:"worker_running"`
	LastSync        int64 `json:"last_sync"`
	ImportedThrough int64 `json:"imported_through"`
	SourceThrough   int64 `json:"source_through"`
}

// OpenDeepSeekReportDatabase never runs main-database migrations or starts jobs.
// The process supplying this environment variable owns the returned connection.
func OpenDeepSeekReportDatabase(envName string, maxConnections int) (*gorm.DB, error) {
	dsn := strings.TrimSpace(os.Getenv(envName))
	if dsn == "" {
		return nil, fmt.Errorf("%s is required", envName)
	}
	var db *gorm.DB
	var err error
	config := newGormConfig(true)
	config.DisableAutomaticPing = true
	if strings.HasPrefix(dsn, "sqlite://") {
		db, err = gorm.Open(sqlite.Open(strings.TrimPrefix(dsn, "sqlite://")), config)
	} else if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		db, err = gorm.Open(postgres.Open(dsn), config)
	} else {
		db, err = gorm.Open(mysql.New(mysql.Config{DSN: dsn, SkipInitializeWithVersion: true}), config)
	}
	if err != nil {
		return nil, err
	}
	pool, err := db.DB()
	if err != nil {
		return nil, err
	}
	pool.SetMaxOpenConns(maxConnections)
	pool.SetMaxIdleConns(maxConnections)
	pool.SetConnMaxLifetime(300 * time.Second)
	return db, nil
}

func InitDeepSeekReportReader() error {
	if strings.TrimSpace(os.Getenv("DEEPSEEK_REPORT_DSN")) == "" {
		return nil
	}
	targetType, targetName, err := deepSeekReportDatabaseIdentity(os.Getenv("DEEPSEEK_REPORT_DSN"))
	if err != nil {
		return err
	}
	for _, key := range []string{"SQL_DSN", "LOG_SQL_DSN"} {
		dsn := os.Getenv(key)
		if dsn == "" || dsn == "local" {
			if key == "LOG_SQL_DSN" {
				continue
			}
			dsn = "sqlite://" + common.SQLitePath
		}
		kind, name, err := deepSeekReportDatabaseIdentity(dsn)
		if err != nil {
			return err
		}
		if kind == targetType && name == targetName {
			return fmt.Errorf("reporting must use a separate database from accounting/logs")
		}
	}
	db, err := OpenDeepSeekReportDatabase("DEEPSEEK_REPORT_DSN", 2)
	if err != nil {
		return err
	}
	DeepSeekReportDB = db
	return nil
}

func deepSeekReportDatabaseIdentity(dsn string) (string, string, error) {
	if strings.HasPrefix(dsn, "sqlite://") {
		name, _, _ := strings.Cut(strings.TrimPrefix(dsn, "sqlite://"), "?")
		name = strings.TrimPrefix(name, "file:")
		path, err := filepath.Abs(name)
		if err != nil {
			return "", "", err
		}
		if resolved, err := filepath.EvalSymlinks(path); err == nil {
			path = resolved
		}
		return "sqlite", path, nil
	}
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		if err != nil {
			return "", "", err
		}
		if u.Path == "" || u.Path == "/" {
			return "", "", fmt.Errorf("report database name must be explicit")
		}
		return "postgres", strings.TrimPrefix(u.Path, "/"), nil
	}
	config, err := mysqlDriver.ParseDSN(dsn)
	if err != nil {
		return "", "", err
	}
	if config.DBName == "" {
		return "", "", fmt.Errorf("report database name must be explicit")
	}
	return "mysql", config.DBName, nil
}

func DeepSeekReportingConfigured() bool {
	return DeepSeekReportDB != nil || strings.TrimSpace(os.Getenv("DEEPSEEK_REPORT_DSN")) != ""
}

func ValidateDeepSeekReportIsolation(source, target *gorm.DB) error {
	sourcePool, err := source.DB()
	if err != nil {
		return err
	}
	targetPool, err := target.DB()
	if err != nil {
		return err
	}
	if sourcePool == targetPool {
		return fmt.Errorf("reporting must use a separate database and pool")
	}
	if source.Dialector.Name() != target.Dialector.Name() {
		return nil
	}
	if source.Dialector.Name() == "sqlite" {
		var a, b []struct {
			Name string
			File string
		}
		if err := source.Raw("PRAGMA database_list").Scan(&a).Error; err != nil {
			return err
		}
		if err := target.Raw("PRAGMA database_list").Scan(&b).Error; err != nil {
			return err
		}
		for _, left := range a {
			for _, right := range b {
				if left.File != "" && left.File == right.File {
					return fmt.Errorf("reporting must not share the accounting SQLite file")
				}
			}
		}
		return nil
	}
	if source.Migrator().CurrentDatabase() == target.Migrator().CurrentDatabase() {
		return fmt.Errorf("reporting must use a different database name from accounting/logs")
	}
	return nil
}

func InitDeepSeekReportStore(target *gorm.DB) error {
	return target.AutoMigrate(&DeepSeekBillFact{}, &DeepSeekBillDaily{}, &DeepSeekBillState{})
}

func billDailyFromDetail(detail DeepSeekDemoDetail) DeepSeekBillDaily {
	row := DeepSeekBillDaily{Day: time.Unix(detail.Time, 0).In(UsageAnalysisLocation).Format("2006-01-02"), UserID: detail.UserID, Username: detail.Username}
	row.Requests = 1
	row.Input = detail.Input
	row.Output = detail.Output
	row.CacheRead = detail.CacheRead
	if detail.Peak {
		row.PeakUnits = detail.CostUnits
	} else {
		row.OffPeakUnits = detail.CostUnits
	}
	if detail.Estimated {
		row.Estimated = 1
	}
	if detail.Unpriced {
		row.Unpriced = 1
	}
	if detail.Actual != nil {
		row.Actual = 1
		row.ActualQuota = int64(detail.Actual.Quota)
	}
	return row
}

func mergeDeepSeekBillDaily(tx *gorm.DB, row DeepSeekBillDaily) error {
	values := map[string]int64{"requests": row.Requests, "input": row.Input, "output": row.Output, "cache_read": row.CacheRead, "peak_units": row.PeakUnits, "off_peak_units": row.OffPeakUnits, "estimated": row.Estimated, "unpriced": row.Unpriced, "actual": row.Actual, "actual_quota": row.ActualQuota}
	updates := map[string]interface{}{"username": row.Username}
	for column, value := range values {
		updates[column] = gorm.Expr("? + ?", clause.Column{Table: clause.CurrentTable, Name: column}, value)
	}
	return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "day"}, {Name: "user_id"}}, DoUpdates: clause.Assignments(updates)}).Create(&row).Error
}

// ProcessDeepSeekReportBatch is called only by the standalone low-priority
// worker. Each source read is a primary-key range capped at 200 rows. A cyclic
// reconciliation pass catches out-of-order commits without changing billing.
func ProcessDeepSeekReportBatch(ctx context.Context, source, target *gorm.DB, batchSize int, now time.Time) error {
	if batchSize < 1 || batchSize > 200 {
		return fmt.Errorf("report batch must be 1..200")
	}
	var maxID int64
	if err := source.WithContext(ctx).Model(&Log{}).Select("COALESCE(MAX(id),0)").Scan(&maxID).Error; err != nil {
		return err
	}
	if err := target.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&DeepSeekBillState{ID: 1, InitialThrough: maxID, SourceThrough: maxID}).Error; err != nil {
		return err
	}
	var before DeepSeekBillState
	if err := target.WithContext(ctx).First(&before, 1).Error; err != nil {
		return err
	}
	// Favor fresh logs under load, but reserve one batch in ten for late-commit
	// reconciliation. When caught up, use idle capacity for reconciliation.
	replay := before.HistoryComplete && before.ReplayCursor < before.ReplayThrough && (before.Cursor >= maxID || before.TailBatches >= 9)
	cursor := before.Cursor
	through := maxID
	// SQLite can reuse row IDs after log cleanup. Composite identities keep
	// retained reports distinct, and a rewound source starts a new tail pass.
	if maxID < cursor {
		cursor = 0
	}
	if replay {
		cursor = before.ReplayCursor
		through = before.ReplayThrough
	}
	var logs []Log
	if err := source.WithContext(ctx).Select("id,type,user_id,username,created_at,prompt_tokens,completion_tokens,other,request_id,model_name").Where("id > ? AND id <= ?", cursor, through).Order("id ASC").Limit(batchSize).Find(&logs).Error; err != nil {
		return err
	}
	next := through
	if len(logs) == batchSize {
		next = int64(logs[len(logs)-1].Id)
	}
	if before.HistoryComplete && !replay && before.Cursor > 0 {
		var head []Log
		if err := source.WithContext(ctx).Select("id,type,user_id,username,created_at,prompt_tokens,completion_tokens,other,request_id,model_name").Where("id <= ?", before.Cursor).Order("id DESC").Limit(200).Find(&head).Error; err != nil {
			return err
		}
		logs = append(logs, head...)
	}
	return target.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var state DeepSeekBillState
		if err := lockForUpdate(tx).First(&state, 1).Error; err != nil {
			return err
		}
		if state.Cursor != before.Cursor || state.ReplayCursor != before.ReplayCursor || state.LastSync != before.LastSync || state.TailBatches != before.TailBatches {
			return nil
		}
		ids := make([]string, 0, len(logs))
		for _, log := range logs {
			if log.Type == LogTypeConsume && log.ModelName == DeepSeekDemoModel {
				ids = append(ids, deepSeekBillFactKey(log))
			}
		}
		var existing []string
		if len(ids) > 0 {
			if err := tx.Model(&DeepSeekBillFact{}).Where("report_key IN ?", ids).Pluck("report_key", &existing).Error; err != nil {
				return err
			}
		}
		seen := make(map[string]bool, len(existing))
		for _, id := range existing {
			seen[id] = true
		}
		facts := make([]DeepSeekBillFact, 0, len(ids))
		dailies := make(map[string]DeepSeekBillDaily)
		for _, log := range logs {
			if log.Type != LogTypeConsume || log.ModelName != DeepSeekDemoModel || seen[deepSeekBillFactKey(log)] {
				continue
			}
			detail := deepSeekDemoDetail(log)
			detail.ReportKey = deepSeekBillFactKey(log)
			payload, err := common.Marshal(detail)
			if err != nil {
				return err
			}
			facts = append(facts, DeepSeekBillFact{ReportKey: detail.ReportKey, LogID: int64(log.Id), UserID: log.UserId, Time: detail.Time, Payload: string(payload)})
			seen[detail.ReportKey] = true
			row := billDailyFromDetail(detail)
			key := fmt.Sprintf("%s/%d", row.Day, row.UserID)
			if prior, ok := dailies[key]; ok {
				prior.DeepSeekDemoTotals.add(row.DeepSeekDemoTotals)
				prior.Username = row.Username
				dailies[key] = prior
			} else {
				dailies[key] = row
			}
		}
		if len(facts) > 0 {
			if err := tx.CreateInBatches(facts, 100).Error; err != nil {
				return err
			}
		}
		for _, row := range dailies {
			if err := mergeDeepSeekBillDaily(tx, row); err != nil {
				return err
			}
		}
		if replay {
			state.TailBatches = 0
			state.ReplayCursor = next
			if next >= state.ReplayThrough {
				state.LastReplayAt = now.Unix()
			}
		} else {
			state.Cursor = next
			state.TailBatches++
		}
		state.SourceThrough = max(maxID, state.Cursor)
		state.HistoryComplete = state.HistoryComplete || state.Cursor >= state.InitialThrough
		if state.HistoryComplete && state.ReplayCursor >= state.ReplayThrough && now.Unix()-state.LastReplayAt >= 3600 {
			state.ReplayCursor = 0
			state.ReplayThrough = maxID
			state.LastReplayAt = now.Unix()
		}
		state.LastSync = now.Unix()
		return tx.Save(&state).Error
	})
}

func (a *DeepSeekDemoTotals) add(b DeepSeekDemoTotals) {
	a.Requests += b.Requests
	a.Input += b.Input
	a.Output += b.Output
	a.CacheRead += b.CacheRead
	a.PeakUnits += b.PeakUnits
	a.OffPeakUnits += b.OffPeakUnits
	a.Estimated += b.Estimated
	a.Unpriced += b.Unpriced
	a.Actual += b.Actual
	a.ActualQuota += b.ActualQuota
}

func reportReadTransaction(ctx context.Context, fn func(*gorm.DB) error) error {
	options := &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead}
	if DeepSeekReportDB.Dialector.Name() == "sqlite" {
		options = &sql.TxOptions{}
	}
	return DeepSeekReportDB.WithContext(ctx).Transaction(fn, options)
}

func readDeepSeekReportStatus(tx *gorm.DB, now time.Time) (DeepSeekReportStatus, error) {
	var state DeepSeekBillState
	if err := tx.First(&state, 1).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return DeepSeekReportStatus{Configured: true}, nil
	} else if err != nil {
		return DeepSeekReportStatus{}, err
	}
	return DeepSeekReportStatus{Configured: true, HistoryComplete: state.HistoryComplete, LastSync: state.LastSync, WorkerRunning: state.LastSync > 0 && now.Unix()-state.LastSync < 90, ImportedThrough: state.Cursor, SourceThrough: state.SourceThrough}, nil
}
