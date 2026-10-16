package controller

import (
	"fmt"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

func DepartmentQuotaConfig(c *gin.Context) {
	if c.Request.Method == "PUT" {
		var request struct {
			Enabled *bool `json:"enabled"`
		}
		if err := common.DecodeJson(c.Request.Body, &request); err != nil || request.Enabled == nil {
			common.ApiError(c, fmt.Errorf("invalid configuration"))
			return
		}
		if err := model.UpdateOption("quota_setting.enterprise_quota_enabled", strconv.FormatBool(*request.Enabled)); err != nil {
			common.ApiError(c, err)
			return
		}
		recordManageAudit(c, "department_quota.mode", map[string]interface{}{"enabled": *request.Enabled})
	}
	common.ApiSuccess(c, gin.H{"enabled": operation_setting.GetQuotaSetting().EnterpriseQuotaEnabled})
}

func DepartmentQuotaRules(c *gin.Context) {
	if c.Request.Method == "PUT" {
		var rule model.DepartmentQuotaRule
		if err := common.DecodeJson(c.Request.Body, &rule); err != nil {
			common.ApiError(c, err)
			return
		}
		if err := model.SaveDepartmentQuotaRule(&rule); err != nil {
			common.ApiError(c, err)
			return
		}
		recordManageAudit(c, "department_quota.rule", map[string]interface{}{"department_id": rule.DepartmentId, "daily": rule.Daily, "weekly": rule.Weekly, "monthly": rule.Monthly, "enabled": rule.Enabled})
		common.ApiSuccess(c, rule)
		return
	}
	rules := make([]model.DepartmentQuotaRule, 0)
	if err := model.DB.Find(&rules).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rules)
}

func SelfDepartmentQuota(c *gin.Context) {
	usage, err := model.GetDepartmentQuotaUsage(c.GetInt("id"), time.Now())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	usage.WalletMonthlyUsed, err = model.WalletMonthConsumption(usage.UserId, usage.Monthly.Start, usage.Monthly.ResetAt)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"enabled": operation_setting.GetQuotaSetting().EnterpriseQuotaEnabled, "usage": usage})
}

func AdminDepartmentQuota(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiError(c, fmt.Errorf("invalid user"))
		return
	}
	usage, err := model.GetDepartmentQuotaUsage(id, time.Now())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	usage.WalletMonthlyUsed, err = model.WalletMonthConsumption(id, usage.Monthly.Start, usage.Monthly.ResetAt)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"enabled": operation_setting.GetQuotaSetting().EnterpriseQuotaEnabled, "usage": usage})
}

func DepartmentQuotaUsers(c *gin.Context) {
	page := common.GetPageQuery(c)
	var departmentId *int
	if value := c.Query("department_id"); value != "" {
		id, err := strconv.Atoi(value)
		if err != nil || id < 0 {
			common.ApiError(c, fmt.Errorf("invalid department"))
			return
		}
		departmentId = &id
	}
	users, total, err := model.GetCompanyQuotaUsers(c.Query("keyword"), departmentId, page.GetStartIdx(), page.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]*model.DepartmentQuotaUsage, 0, len(users))
	for _, user := range users {
		if user.DeletedAt.Valid {
			continue
		}
		usage, err := model.GetDepartmentQuotaUsage(user.Id, time.Now())
		if err != nil {
			common.ApiError(c, err)
			return
		}
		usage.WalletMonthlyUsed, err = model.WalletMonthConsumption(usage.UserId, usage.Monthly.Start, usage.Monthly.ResetAt)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		items = append(items, usage)
	}
	page.SetTotal(int(total))
	page.SetItems(items)
	common.ApiSuccess(c, page)
}

func CompanyBillingLogs(c *gin.Context) {
	page := common.GetPageQuery(c)
	source := c.Query("source")
	if source != "wallet" && source != "department_quota" {
		common.ApiError(c, fmt.Errorf("invalid billing source"))
		return
	}
	userId := c.GetInt("id")
	admin := c.GetInt("role") >= common.RoleAdminUser && c.GetBool("company_admin_logs")
	if admin {
		userId = 0
		if value := c.Query("user_id"); value != "" {
			id, err := strconv.Atoi(value)
			if err != nil || id <= 0 {
				common.ApiError(c, fmt.Errorf("invalid user"))
				return
			}
			userId = id
		}
	}
	logs, total, err := model.GetCompanyBillingLogs(source, userId, c.Query("model"), page.GetStartIdx(), page.GetPageSize(), !admin)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	page.SetTotal(int(total))
	page.SetItems(logs)
	common.ApiSuccess(c, page)
}

func WalletAdjustmentLogs(c *gin.Context) {
	page := common.GetPageQuery(c)
	logs, total, err := model.GetWalletAdjustmentLogs(c.Query("keyword"), c.Query("action"), c.Query("start"), c.Query("end"), page.GetStartIdx(), page.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	page.SetTotal(int(total))
	page.SetItems(logs)
	common.ApiSuccess(c, page)
}
