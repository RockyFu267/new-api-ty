package controller

import (
	"fmt"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func ListDepartments(c *gin.Context) {
	departments, err := model.ListDepartments()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, departments)
}

func SaveDepartment(c *gin.Context) {
	var request struct {
		Id   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil || request.Id < 0 {
		common.ApiError(c, fmt.Errorf("invalid department"))
		return
	}
	department, err := model.SaveDepartment(request.Id, request.Name)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAuditFor(c, 0, "department.save", map[string]interface{}{"department_id": department.Id, "name": department.Name})
	common.ApiSuccess(c, department)
}

func DeleteDepartment(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiError(c, fmt.Errorf("invalid department"))
		return
	}
	if err := model.DeleteDepartment(id); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAuditFor(c, 0, "department.delete", map[string]interface{}{"department_id": id})
	common.ApiSuccess(c, nil)
}

func AssignUserDepartment(c *gin.Context) {
	var request struct {
		DepartmentId *int `json:"department_id"`
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiError(c, fmt.Errorf("invalid user"))
		return
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil || request.DepartmentId == nil || *request.DepartmentId < 0 {
		common.ApiError(c, fmt.Errorf("invalid department"))
		return
	}
	user, err := model.GetUserById(id, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !canManageTargetRole(c.GetInt("role"), user.Role) {
		common.ApiError(c, fmt.Errorf("cannot manage this user"))
		return
	}
	if err := model.AssignUserDepartment(id, *request.DepartmentId); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAuditFor(c, id, "user.department", map[string]interface{}{"department_id": *request.DepartmentId})
	common.ApiSuccess(c, nil)
}
