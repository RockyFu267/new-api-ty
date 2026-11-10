package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	ds "github.com/QuantumNous/new-api/setting/deepseek_setting"
	"github.com/gin-gonic/gin"
)

func DeepSeekBillDemo(c *gin.Context) {
	now := time.Now().In(model.UsageAnalysisLocation)
	user := c.GetInt("id")
	if user <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "unauthorized"})
		return
	}
	admin := c.GetInt("role") >= common.RoleAdminUser
	if admin {
		var err error
		user, err = strconv.Atoi(c.DefaultQuery("user_id", "0"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid user"})
			return
		}
	}
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid page"})
		return
	}
	getReport := model.GetDeepSeekSelfDemoReport
	if admin {
		getReport = model.GetDeepSeekDemoReport
	}
	report, err := getReport(c.DefaultQuery("start", now.AddDate(0, 0, -29).Format("2006-01-02")), c.DefaultQuery("end", now.Format("2006-01-02")), user, page)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, report)
}

func DeepSeekBillingRules(c *gin.Context) {
	if c.Request.Method == http.MethodPut {
		var input struct {
			Rule     ds.Rule `json:"rule"`
			Revision int     `json:"revision"`
		}
		if err := c.ShouldBindJSON(&input); err != nil {
			common.ApiError(c, err)
			return
		}
		store, err := model.SaveDeepSeekBillingRule(input.Rule, input.Revision, c.GetInt("id"))
		if err != nil {
			common.ApiError(c, err)
			return
		}
		common.ApiSuccess(c, store)
		return
	}
	store, err := model.GetDeepSeekBillingRules()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"store": store, "defaults": ds.DefaultRule(), "active": store.Select(ds.Model, time.Now()), "model": ds.Model})
}
