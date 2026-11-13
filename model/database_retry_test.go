package model

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatabaseRetryOnlyReplaysKnownRejectedWrites(t *testing.T) {
	for _, tc := range []struct {
		name  string
		err   error
		retry bool
	}{
		{"pg-deadlock", &pgconn.PgError{Code: "40P01"}, true},
		{"pg-serialization", &pgconn.PgError{Code: "40001"}, true},
		{"pg-lock-timeout", &pgconn.PgError{Code: "55P03"}, true},
		{"pg-connection-limit", &pgconn.PgError{Code: "53300"}, true},
		{"mysql-deadlock", &mysqlDriver.MySQLError{Number: 1213}, true},
		{"mysql-lock-timeout", &mysqlDriver.MySQLError{Number: 1205}, true},
		{"pg-duplicate", &pgconn.PgError{Code: "23505"}, false},
		{"ambiguous-connection-loss", &pgconn.PgError{Code: "08006"}, false},
		{"ambiguous-eof", io.ErrUnexpectedEOF, false},
		{"plain-text-is-not-proof", errors.New("database is locked"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := fmt.Errorf("write failed: %w", tc.err)
			assert.Equal(t, tc.retry, IsTransientDatabaseError(err))
			attempts := 0
			result := retryDatabaseWrite(DB, func() error {
				attempts++
				if attempts == 1 {
					return err
				}
				return nil
			})
			if tc.retry {
				require.NoError(t, result)
				assert.Equal(t, 2, attempts)
			} else {
				require.ErrorIs(t, result, tc.err)
				assert.Equal(t, 1, attempts)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	attempted := false
	require.ErrorIs(t, retryDatabaseWrite(DB.WithContext(ctx), func() error { attempted = true; return nil }), context.Canceled)
	assert.False(t, attempted)
}
