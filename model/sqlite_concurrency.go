package model

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"gorm.io/gorm"
)

// These options are understood by glebarez/go-sqlite. The legacy
// _busy_timeout option alone does not configure this driver's connections.
func sqliteConnectionDSN(path string) (string, error) {
	name, rawQuery, _ := strings.Cut(path, "?")
	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		return "", fmt.Errorf("invalid SQLite connection options: %w", err)
	}
	query.Set("_txlock", "immediate")
	hasBusy, hasJournal := false, false
	for _, pragma := range query["_pragma"] {
		value := strings.ToLower(strings.TrimSpace(pragma))
		hasBusy = hasBusy || strings.HasPrefix(value, "busy_timeout")
		hasJournal = hasJournal || strings.HasPrefix(value, "journal_mode")
	}
	if !hasBusy {
		query.Add("_pragma", "busy_timeout(1000)")
	}
	if !hasJournal {
		query.Add("_pragma", "journal_mode(WAL)")
	}
	return name + "?" + query.Encode(), nil
}

// Retry the whole rolled-back transaction, never an individual write based on
// an obsolete quota snapshot. Callbacks must only perform transactional DB work.
// Other databases retain their normal row-locking transaction behavior.
func quotaWriteTransaction(db *gorm.DB, fn func(*gorm.DB) error) error {
	attempts := 1
	if db.Dialector.Name() == "sqlite" {
		attempts = 6
	}
	for attempt := 0; attempt < attempts; attempt++ {
		err := db.Transaction(fn)
		if err == nil {
			return nil
		}
		var sqliteErr interface{ Code() int }
		if !errors.As(err, &sqliteErr) || (sqliteErr.Code()&255 != 5 && sqliteErr.Code()&255 != 6) || attempt+1 == attempts {
			return err
		}
		timer := time.NewTimer(time.Duration(10*(1<<attempt)) * time.Millisecond)
		select {
		case <-db.Statement.Context.Done():
			timer.Stop()
			return db.Statement.Context.Err()
		case <-timer.C:
		}
	}
	return nil
}
