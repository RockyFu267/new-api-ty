package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	ds "github.com/QuantumNous/new-api/setting/deepseek_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestDeepSeekRulesAppendOnlyAndRejectStaleSave(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(1)
	previous := DB
	DB = db
	common.OptionMapRWMutex.Lock()
	old := common.OptionMap[ds.OptionKey]
	if common.OptionMap == nil {
		common.OptionMap = map[string]string{}
	}
	delete(common.OptionMap, ds.OptionKey)
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		DB = previous
		require.NoError(t, pool.Close())
		common.OptionMapRWMutex.Lock()
		common.OptionMap[ds.OptionKey] = old
		common.OptionMapRWMutex.Unlock()
	})
	require.NoError(t, db.AutoMigrate(&Option{}))
	rule := ds.DefaultRule()
	rule.Enabled = true
	store, err := SaveDeepSeekBillingRule(rule, 0, 99)
	require.NoError(t, err)
	require.Equal(t, 1, store.Revision)
	_, err = SaveDeepSeekBillingRule(rule, 0, 99)
	require.Error(t, err)
	rule.Enabled = false
	store, err = SaveDeepSeekBillingRule(rule, 1, 99)
	require.NoError(t, err)
	require.Len(t, store.Rules, 2)
	require.True(t, store.Rules[0].Enabled)
	require.False(t, store.Rules[1].Enabled)
	require.Equal(t, 99, store.Rules[0].CreatedBy)
	rule.EffectiveAt = time.Now().Add(-time.Hour).Unix()
	_, err = SaveDeepSeekBillingRule(rule, 2, 99)
	require.Error(t, err)
	require.Error(t, UpdateOption(ds.OptionKey, `{}`))
}

func TestDeepSeekRecordedChargeSurvivesLaterPricingChanges(t *testing.T) {
	rule := ds.DefaultRule()
	rule.Enabled = true
	rule.EffectiveAt = 1
	rule.Version = 4
	rule.FX = 7
	at := time.Date(2026, 11, 10, 10, 0, 0, 0, ds.Location)
	snap := (ds.Store{Rules: []ds.Rule{rule}}).Select(ds.Model, at)
	charge, err := snap.Calculate(1000000, 500000, 1000000)
	require.NoError(t, err)
	data, err := common.Marshal(map[string]interface{}{"deepseek_billing": charge})
	require.NoError(t, err)
	log := demoConsume(t, "2026-11-10 19:00:00", 1)
	log.Other = string(data)
	detail := deepSeekDemoDetail(log)
	require.Equal(t, at.Unix(), detail.Time)
	require.True(t, detail.Peak)
	require.EqualValues(t, 902000000, detail.CostUnits)
	require.Equal(t, 4, detail.Actual.Snapshot.Rule.Version)
	require.Equal(t, charge.Quota, detail.Actual.Quota)
}
