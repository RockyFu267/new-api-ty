package service

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"time"
)

func TestEnterpriseBillingUsesQuotaBeforeWalletAndFallsBackWithoutMixing(t *testing.T) {
	truncate(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Department{}, &model.UserDepartment{}, &model.DepartmentQuotaRule{}, &model.DepartmentQuotaCharge{}))
	t.Cleanup(func() {
		model.DB.Where("1 = 1").Delete(&model.DepartmentQuotaCharge{})
		model.DB.Where("1 = 1").Delete(&model.DepartmentQuotaRule{})
		model.DB.Where("1 = 1").Delete(&model.UserDepartment{})
		model.DB.Where("1 = 1").Delete(&model.Department{})
	})
	old := operation_setting.GetQuotaSetting().EnterpriseQuotaEnabled
	operation_setting.GetQuotaSetting().EnterpriseQuotaEnabled = true
	t.Cleanup(func() { operation_setting.GetQuotaSetting().EnterpriseQuotaEnabled = old })
	seedUser(t, 1, 0)
	department, err := model.SaveDepartment(0, "Development")
	require.NoError(t, err)
	require.NoError(t, model.AssignUserDepartment(1, department.Id))
	require.NoError(t, model.SaveDepartmentQuotaRule(&model.DepartmentQuotaRule{DepartmentId: department.Id, Enabled: true, Daily: 100}))
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{UserId: 1, IsPlayground: true, RequestId: "quota-request", OriginModelName: "qwen"}
	session, apiErr := NewBillingSession(context, info, 70)
	require.Nil(t, apiErr)
	require.Equal(t, BillingSourceDepartment, info.BillingSource)
	require.NoError(t, session.Settle(70)) // delta=0 must finalize reservation.
	var charge model.DepartmentQuotaCharge
	require.NoError(t, model.DB.First(&charge, "request_id = ?", info.RequestId).Error)
	require.Equal(t, "settled", charge.Status)
	quota, err := model.GetUserQuota(1, true)
	require.NoError(t, err)
	require.Zero(t, quota)
	_, apiErr = NewBillingSession(context, &relaycommon.RelayInfo{UserId: 1, IsPlayground: true, RequestId: "no-wallet"}, 40)
	require.NotNil(t, apiErr)
	require.NoError(t, model.IncreaseUserQuota(1, 100, true))
	walletInfo := &relaycommon.RelayInfo{UserId: 1, IsPlayground: true, RequestId: "wallet-fallback"}
	wallet, apiErr := NewBillingSession(context, walletInfo, 40)
	require.Nil(t, apiErr)
	require.Equal(t, BillingSourceWallet, walletInfo.BillingSource)
	require.NoError(t, wallet.Settle(35))
	quota, err = model.GetUserQuota(1, true)
	require.NoError(t, err)
	require.Equal(t, 65, quota)
	usage, err := model.GetDepartmentQuotaUsage(1, time.Unix(charge.CreatedAt, 0))
	require.NoError(t, err)
	require.Equal(t, int64(70), usage.Daily.Used)
}

func TestEnterpriseAPIRequestSettlesTokenBudgetOnceAcrossWalletFallback(t *testing.T) {
	truncate(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Department{}, &model.UserDepartment{}, &model.DepartmentQuotaRule{}, &model.DepartmentQuotaCharge{}))
	t.Cleanup(func() {
		model.DB.Where("1 = 1").Delete(&model.DepartmentQuotaCharge{})
		model.DB.Where("1 = 1").Delete(&model.DepartmentQuotaRule{})
		model.DB.Where("1 = 1").Delete(&model.UserDepartment{})
		model.DB.Where("1 = 1").Delete(&model.Department{})
	})
	old := operation_setting.GetQuotaSetting().EnterpriseQuotaEnabled
	operation_setting.GetQuotaSetting().EnterpriseQuotaEnabled = true
	t.Cleanup(func() { operation_setting.GetQuotaSetting().EnterpriseQuotaEnabled = old })
	seedUser(t, 1, 200)
	seedToken(t, 1, 1, "enterprise-token", 1000)
	department, err := model.SaveDepartment(0, "Engineering")
	require.NoError(t, err)
	require.NoError(t, model.AssignUserDepartment(1, department.Id))
	require.NoError(t, model.SaveDepartmentQuotaRule(&model.DepartmentQuotaRule{DepartmentId: department.Id, Enabled: true, Daily: 100}))
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Set("token_quota", 1000)
	info := &relaycommon.RelayInfo{UserId: 1, TokenId: 1, TokenKey: "enterprise-token", RequestId: "api-company"}
	session, apiErr := NewBillingSession(context, info, 70)
	require.Nil(t, apiErr)
	require.NoError(t, session.Settle(50))
	require.NoError(t, session.Settle(50))
	info = &relaycommon.RelayInfo{UserId: 1, TokenId: 1, TokenKey: "enterprise-token", RequestId: "api-wallet"}
	session, apiErr = NewBillingSession(context, info, 70)
	require.Nil(t, apiErr)
	require.Equal(t, BillingSourceWallet, info.BillingSource)
	require.NoError(t, session.Settle(65))
	var token model.Token
	require.NoError(t, model.DB.First(&token, 1).Error)
	require.Equal(t, 885, token.RemainQuota)
	require.Equal(t, 115, token.UsedQuota)
	wallet, err := model.GetUserQuota(1, true)
	require.NoError(t, err)
	require.Equal(t, 135, wallet)
	usage, err := model.GetDepartmentQuotaUsage(1, time.Now())
	require.NoError(t, err)
	require.Equal(t, int64(50), usage.Daily.Used)
}
