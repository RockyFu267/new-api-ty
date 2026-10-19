package model

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DepartmentModelPolicy struct {
	DepartmentId int    `json:"department_id" gorm:"primaryKey;autoIncrement:false"`
	Models       string `json:"-" gorm:"type:text;not null"`
}

type DepartmentModelAccess struct {
	DepartmentID int      `json:"department_id"`
	Models       []string `json:"models"`
}

func (a DepartmentModelAccess) Allows(name string) bool {
	if a.DepartmentID <= 0 || name == "" {
		return false
	}
	for _, allowed := range a.Models {
		if name == allowed {
			return true
		}
	}
	return false
}

func GetUserDepartmentModelAccess(userID int) (DepartmentModelAccess, error) {
	access := DepartmentModelAccess{Models: []string{}}
	if userID <= 0 {
		return access, nil
	}
	var row struct {
		DepartmentID int
		Models       string
	}
	err := DB.Table("user_departments").Select("user_departments.department_id, department_model_policies.models").Joins("JOIN departments ON departments.id = user_departments.department_id").Joins("LEFT JOIN department_model_policies ON department_model_policies.department_id = user_departments.department_id").Where("user_departments.user_id = ?", userID).Scan(&row).Error
	if err != nil {
		return access, err
	}
	access.DepartmentID = row.DepartmentID
	if row.Models != "" {
		if err := common.UnmarshalJsonStr(row.Models, &access.Models); err != nil {
			return DepartmentModelAccess{}, err
		}
	}
	return access, nil
}

func FilterDepartmentModelNames(userID int, names []string) ([]string, error) {
	access, err := GetUserDepartmentModelAccess(userID)
	if err != nil {
		return nil, err
	}
	filtered := make([]string, 0, len(names))
	for _, name := range names {
		if access.Allows(name) {
			filtered = append(filtered, name)
		}
	}
	return filtered, nil
}

func SaveDepartmentModels(departmentID int, names []string) ([]string, error) {
	if departmentID <= 0 || len(names) > 5000 {
		return nil, fmt.Errorf("invalid department or model count")
	}
	models := make([]string, 0, len(names))
	seen := map[string]bool{}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || utf8.RuneCountInString(name) > 255 || strings.ContainsAny(name, "\x00\r\n") {
			return nil, fmt.Errorf("invalid model name")
		}
		if !seen[name] {
			models = append(models, name)
			seen[name] = true
		}
	}
	sort.Strings(models)
	encoded, err := common.Marshal(models)
	if err != nil {
		return nil, err
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		var department Department
		if err := lockForUpdate(tx).First(&department, departmentID).Error; err != nil {
			return err
		}
		policy := DepartmentModelPolicy{DepartmentId: departmentID, Models: string(encoded)}
		return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "department_id"}}, DoUpdates: clause.AssignmentColumns([]string{"models"})}).Create(&policy).Error
	})
	return models, err
}

func ListDepartmentModelPolicies() ([]DepartmentModelAccess, error) {
	var rows []DepartmentModelPolicy
	if err := DB.Order("department_id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	policies := make([]DepartmentModelAccess, 0, len(rows))
	for _, row := range rows {
		policy := DepartmentModelAccess{DepartmentID: row.DepartmentId, Models: []string{}}
		if err := common.UnmarshalJsonStr(row.Models, &policy.Models); err != nil {
			return nil, err
		}
		policies = append(policies, policy)
	}
	return policies, nil
}
