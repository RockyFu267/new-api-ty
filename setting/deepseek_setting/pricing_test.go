package deepseek_setting

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRequestSnapshotPinsVersionPeriodAndFX(t *testing.T) {
	rule := DefaultRule()
	rule.Enabled = true
	rule.Version = 1
	rule.FX = 7.3
	rule.EffectiveAt = 1
	store := Store{Revision: 1, Rules: []Rule{rule}}
	at := time.Date(2026, 11, 10, 11, 59, 59, 0, Location)
	snap := store.Select(Model, at)
	require.True(t, snap.Peak)
	charge, err := snap.Calculate(1000000, 500000, 1000000)
	require.NoError(t, err)
	require.InDelta(t, 9.02, charge.CNY, 1e-9)
	require.Equal(t, 617808, charge.Quota)
	next := rule
	next.Version = 2
	next.FX = 1
	next.Peak.Input = 100
	next.EffectiveAt = at.Unix() + 1
	store.Rules = append(store.Rules, next)
	again, err := snap.Calculate(1000000, 500000, 1000000)
	require.NoError(t, err)
	require.Equal(t, charge, again)
	require.Equal(t, 2, store.Select(Model, at.Add(time.Second)).Rule.Version)
	require.False(t, store.Select(Model, at.Add(time.Second)).Peak)
	require.Nil(t, store.Select("qwen", at))
	next.Enabled = false
	next.Version = 3
	store.Rules = append(store.Rules, next)
	require.Nil(t, store.Select(Model, at.Add(time.Second)))
}

func TestSmallRealUsageRoundsLikeExistingTextBilling(t *testing.T) {
	rule := DefaultRule()
	rule.Enabled = true
	rule.Version = 1
	rule.EffectiveAt = 1
	snap := (Store{Rules: []Rule{rule}}).Select(Model, time.Date(2026, 11, 10, 10, 0, 0, 0, Location))
	charge, err := snap.Calculate(1464, 0, 11)
	require.NoError(t, err)
	require.InDelta(t, .003016, charge.CNY, 1e-12)
	require.Equal(t, 207, charge.Quota)
	require.Equal(t, "nearest_quota_unit", charge.QuotaRounding)
}

func TestPeakBoundariesHolidaysAndInvalidUsage(t *testing.T) {
	r := DefaultRule()
	r.Enabled = true
	r.EffectiveAt = 1
	s := Store{Rules: []Rule{r}}
	for _, test := range []struct {
		at   string
		peak bool
	}{{"2026-11-10 08:59:59", false}, {"2026-11-10 09:00:00", true}, {"2026-11-10 12:00:00", false}, {"2026-11-10 14:00:00", true}, {"2026-11-10 18:00:00", false}, {"2026-11-08 10:00:00", false}, {"2026-10-01 10:00:00", false}} {
		at, err := time.ParseInLocation("2006-01-02 15:04:05", test.at, Location)
		require.NoError(t, err)
		require.Equal(t, test.peak, s.Select(Model, at).Peak, test.at)
	}
	snap := s.Select(Model, time.Date(2026, 11, 10, 9, 0, 0, 0, Location))
	_, err := snap.Calculate(1, 2, 0)
	require.Error(t, err)
	r.FX = 0
	require.Error(t, r.Validate())
}
