package model

import (
	"errors"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

// IsTransientDatabaseError recognizes only errors for which the database has
// rejected/aborted the write. Network failures and unknown commit outcomes are
// deliberately excluded: replaying those could debit a balance twice.
func IsTransientDatabaseError(err error) bool {
	var sqliteErr interface{ Code() int }
	if errors.As(err, &sqliteErr) && (sqliteErr.Code()&255 == 5 || sqliteErr.Code()&255 == 6) {
		return true
	}
	var postgresErr interface{ SQLState() string }
	if errors.As(err, &postgresErr) {
		switch postgresErr.SQLState() {
		case "40001", "40P01", "55P03", "53300":
			return true
		}
	}
	var mysqlErr *mysqlDriver.MySQLError
	return errors.As(err, &mysqlErr) && (mysqlErr.Number == 1205 || mysqlErr.Number == 1213)
}

// retryDatabaseWrite is restricted to atomic statements or complete transactions
// that have rolled back. Never wrap a multi-step nontransactional accounting flow.
func retryDatabaseWrite(db *gorm.DB, write func() error) error {
	for attempt := 0; attempt < 6; attempt++ {
		if err := db.Statement.Context.Err(); err != nil {
			return err
		}
		err := write()
		if err == nil || !IsTransientDatabaseError(err) || attempt == 5 {
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
