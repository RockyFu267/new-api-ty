package model

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Department struct {
	Id        int    `json:"id" gorm:"primaryKey"`
	Name      string `json:"name" gorm:"type:varchar(64);not null;uniqueIndex"`
	UserCount int64  `json:"user_count" gorm:"-:all"`
}

// Organization membership is separate from user credentials and billing groups.
type UserDepartment struct {
	UserId       int `gorm:"primaryKey;autoIncrement:false"`
	DepartmentId int `gorm:"not null;index"`
}

func ListDepartments() ([]Department, error) {
	departments := make([]Department, 0)
	if err := DB.Order("name ASC").Find(&departments).Error; err != nil {
		return nil, err
	}
	type count struct {
		DepartmentId int
		Total        int64
	}
	var counts []count
	if err := DB.Model(&UserDepartment{}).Select("department_id, COUNT(*) AS total").Where("user_id IN (?)", DB.Unscoped().Model(&User{}).Select("id")).Group("department_id").Scan(&counts).Error; err != nil {
		return nil, err
	}
	byId := make(map[int]int64)
	for _, item := range counts {
		byId[item.DepartmentId] = item.Total
	}
	for i := range departments {
		departments[i].UserCount = byId[departments[i].Id]
	}
	return departments, nil
}

func SaveDepartment(id int, name string) (*Department, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 64 {
		return nil, fmt.Errorf("department name must contain 1 to 64 characters")
	}
	department := &Department{Id: id, Name: name}
	err := DB.Transaction(func(tx *gorm.DB) error {
		if id == 0 {
			return tx.Create(department).Error
		}
		var existing Department
		if err := lockForUpdate(tx).First(&existing, id).Error; err != nil {
			return err
		}
		return tx.Model(&existing).Update("name", name).Error
	})
	return department, err
}

func DeleteDepartment(id int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var department Department
		if err := lockForUpdate(tx).First(&department, id).Error; err != nil {
			return err
		}
		if err := tx.Where("department_id = ?", id).Delete(&UserDepartment{}).Error; err != nil {
			return err
		}
		return tx.Delete(&department).Error
	})
}

func AssignUserDepartment(userId, departmentId int) error {
	if userId <= 0 || departmentId < 0 {
		return fmt.Errorf("invalid user or department")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if departmentId > 0 {
			var department Department
			if err := lockForUpdate(tx).First(&department, departmentId).Error; err != nil {
				return err
			}
		}
		var user User
		if err := lockForUpdate(tx).First(&user, userId).Error; err != nil {
			return err
		}
		if departmentId == 0 {
			return tx.Where("user_id = ?", userId).Delete(&UserDepartment{}).Error
		}
		membership := UserDepartment{UserId: userId, DepartmentId: departmentId}
		return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}}, DoUpdates: clause.AssignmentColumns([]string{"department_id"})}).Create(&membership).Error
	})
}

func EnrichUsersWithDepartment(users []*User) error {
	ids := make([]int, 0, len(users))
	for _, user := range users {
		if user != nil {
			ids = append(ids, user.Id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	var memberships []UserDepartment
	if err := DB.Where("user_id IN ?", ids).Find(&memberships).Error; err != nil {
		return err
	}
	departmentIds := make([]int, 0, len(memberships))
	byUser := make(map[int]int)
	for _, member := range memberships {
		byUser[member.UserId] = member.DepartmentId
		departmentIds = append(departmentIds, member.DepartmentId)
	}
	var departments []Department
	if len(departmentIds) > 0 {
		if err := DB.Where("id IN ?", departmentIds).Find(&departments).Error; err != nil {
			return err
		}
	}
	names := make(map[int]string)
	for _, department := range departments {
		names[department.Id] = department.Name
	}
	for _, user := range users {
		if user != nil {
			user.DepartmentId = byUser[user.Id]
			user.DepartmentName = names[user.DepartmentId]
		}
	}
	return nil
}
