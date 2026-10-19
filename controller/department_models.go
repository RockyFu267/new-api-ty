package controller

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func DepartmentModels(c *gin.Context) {
	if c.Request.Method == "PUT" {
		var request struct {
			DepartmentID int       `json:"department_id"`
			Models       *[]string `json:"models"`
		}
		if err := common.DecodeJson(c.Request.Body, &request); err != nil || request.Models == nil {
			common.ApiError(c, fmt.Errorf("department_id and models are required"))
			return
		}
		models, err := model.SaveDepartmentModels(request.DepartmentID, *request.Models)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		recordManageAudit(c, "department.models", map[string]interface{}{"department_id": request.DepartmentID, "models": models})
		common.ApiSuccess(c, model.DepartmentModelAccess{DepartmentID: request.DepartmentID, Models: models})
		return
	}
	policies, err := model.ListDepartmentModelPolicies()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	catalog := make([]string, 0)
	if err := model.DB.Model(&model.Ability{}).Distinct("model").Order("model ASC").Pluck("model", &catalog).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"policies": policies, "catalog": catalog})
}
