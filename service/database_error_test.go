package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
)

func TestTokenDatabaseFailuresAreNotReportedAsForbidden(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{
		{"budget-exhausted", fmt.Errorf("budget: %w", ErrInsufficientTokenQuota), http.StatusForbidden},
		{"database-locked", &pgconn.PgError{Code: "55P03"}, http.StatusServiceUnavailable},
		{"deadline", context.DeadlineExceeded, http.StatusServiceUnavailable},
		{"internal-database-failure", errors.New("disk I/O failure"), http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) { assert.Equal(t, tc.status, tokenPreconsumeError(tc.err).StatusCode) })
	}
}
