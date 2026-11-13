package model

import (
	"fmt"
	"net/url"
	"strings"

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
	return retryDatabaseWrite(db, func() error { return db.Transaction(fn) })
}
