package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCompanyBillsFilterByFundingSourceWithoutDoubleCounting(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&DepartmentQuotaCharge{}))
	t.Cleanup(func() { DB.Where("1 = 1").Delete(&DepartmentQuotaCharge{}) })
	require.NoError(t, DB.Create(&DepartmentQuotaCharge{RequestId: "company", UserId: 1, ModelName: "qwen", Amount: 10, Status: "settled", CreatedAt: time.Now().Unix()}).Error)
	for _, log := range []Log{
		{RequestId: "company", UserId: 1, Username: "alice", Type: LogTypeConsume, Quota: 10, Other: `{"billing_source":"department_quota","admin_info":{"secret":true}}`, CreatedAt: time.Now().Unix()},
		{UserId: 1, Username: "alice", Type: LogTypeConsume, Quota: 20, Other: `{"billing_source":"wallet"}`, CreatedAt: time.Now().Unix()},
		{UserId: 1, Username: "alice", Type: LogTypeConsume, Quota: 30, Other: `{"billing_source":"subscription"}`, CreatedAt: time.Now().Unix()},
		{UserId: 2, Username: "bob", Type: LogTypeConsume, Quota: 40, Other: "", CreatedAt: time.Now().Unix()},
	} {
		require.NoError(t, LOG_DB.Create(&log).Error)
	}
	logs, total, err := GetCompanyBillingLogs("department_quota", 1, "", 0, 20, true)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, logs, 1)
	require.Equal(t, 10, logs[0].Quota)
	require.NotContains(t, logs[0].Other, "admin_info")
	logs, total, err = GetCompanyBillingLogs("wallet", 1, "", 0, 20, true)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, 20, logs[0].Quota)
	logs, total, err = GetCompanyBillingLogs("wallet", 0, "", 0, 20, false)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, logs, 2)
	stat, err := SumUsedQuota(LogTypeConsume, 0, 0, "", "alice", "", 0, "", "department_quota")
	require.NoError(t, err)
	require.Equal(t, 10, stat.Quota)
	stat, err = SumUsedQuota(LogTypeConsume, 0, 0, "", "alice", "", 0, "", "wallet")
	require.NoError(t, err)
	require.Equal(t, 20, stat.Quota)
	stat, err = SumUsedQuota(LogTypeConsume, 0, 0, "", "alice", "", 0, "")
	require.NoError(t, err)
	require.Equal(t, 60, stat.Quota)
}

func TestWalletAuditFilteringRetainsOperatorTargetAndApprovalNote(t *testing.T) {
	truncateTables(t)
	log := Log{UserId: 99, Type: LogTypeManage, Username: "admin", Other: `{"op":{"action":"user.quota_add","params":{"target_user_id":1,"target_username":"alice","before":0,"after":100,"note":"approved by manager"}},"admin_info":{"admin_username":"admin"}}`}
	require.NoError(t, LOG_DB.Create(&log).Error)
	logs, total, err := GetWalletAdjustmentLogs("alice", "add", "", "", 0, 20)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, logs, 1)
	require.Contains(t, logs[0].Other, "approved by manager")
	require.Contains(t, logs[0].Other, "admin_info")
	_, total, err = GetWalletAdjustmentLogs("alice", "subtract", "", "", 0, 20)
	require.NoError(t, err)
	require.Zero(t, total)
}
