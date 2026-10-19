package model

import (
	"fmt"
	"sort"
	"strconv"

	"gorm.io/gorm"
)

type UsageAnalysisOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type UsageAnalysisChoices struct {
	Departments []UsageAnalysisOption `json:"departments"`
	Users       []UsageAnalysisOption `json:"users"`
	Models      []UsageAnalysisOption `json:"models"`
}

func GetUsageAnalysisOptions(f UsageAnalysisFilter) (*UsageAnalysisChoices, error) {
	result := &UsageAnalysisChoices{Departments: []UsageAnalysisOption{}, Users: []UsageAnalysisOption{}, Models: []UsageAnalysisOption{}}
	var departments []Department
	if err := DB.Order("name ASC").Find(&departments).Error; err != nil {
		return nil, err
	}
	names := map[int]string{}
	var historical []struct {
		DepartmentID int
		Department   string
	}
	if f.DepartmentBasis != "current" {
		if err := DB.Model(&UsageAnalysisDaily{}).Select("department_id, department").Where("department_id > 0").Group("department_id, department").Order("department ASC").Scan(&historical).Error; err != nil {
			return nil, err
		}
	}
	for _, department := range historical {
		names[department.DepartmentID] = department.Department
	}
	for _, department := range departments {
		names[department.Id] = department.Name
	}
	for id, name := range names {
		result.Departments = append(result.Departments, UsageAnalysisOption{Value: strconv.Itoa(id), Label: name})
	}
	sort.Slice(result.Departments, func(i, j int) bool { return result.Departments[i].Label < result.Departments[j].Label })
	selectedUser := f.UserID
	f.Model = ""
	f.UserID = 0
	base := analysisBaseQuery(f).Where("day >= ? AND day <= ?", f.Start, f.End)
	var usedUsers []struct {
		UserID   int
		Username string
	}
	if err := base.Session(&gorm.Session{}).Select("user_id, username").Group("user_id, username").Order("username ASC").Scan(&usedUsers).Error; err != nil {
		return nil, err
	}
	userNames := map[int]string{}
	for _, user := range usedUsers {
		userNames[user.UserID] = fmt.Sprintf("%s · #%d", user.Username, user.UserID)
	}
	var users []User
	currentUsers := DB.Model(&User{}).Select("id, username, display_name")
	if f.DepartmentID != nil {
		members := DB.Model(&UserDepartment{}).Select("user_id")
		if *f.DepartmentID > 0 {
			currentUsers = currentUsers.Where("id IN (?)", members.Where("department_id = ?", *f.DepartmentID))
		} else if *f.DepartmentID == 0 {
			currentUsers = currentUsers.Where("id NOT IN (?)", members)
		} else {
			currentUsers = currentUsers.Where("1 = 0")
		}
	}
	// For historical department filtering, use only users actually recorded there.
	if f.DepartmentBasis != "current" && f.DepartmentID != nil {
		ids := make([]int, 0, len(userNames))
		for id := range userNames {
			ids = append(ids, id)
		}
		if len(ids) == 0 {
			currentUsers = currentUsers.Where("1 = 0")
		} else {
			currentUsers = DB.Model(&User{}).Select("id, username, display_name").Where("id IN ?", ids)
		}
	}
	if err := currentUsers.Find(&users).Error; err != nil {
		return nil, err
	}
	for _, user := range users {
		name := user.Username
		if user.DisplayName != "" && user.DisplayName != user.Username {
			name = user.DisplayName + " (" + user.Username + ")"
		}
		userNames[user.Id] = fmt.Sprintf("%s · #%d", name, user.Id)
	}
	for id, name := range userNames {
		result.Users = append(result.Users, UsageAnalysisOption{Value: strconv.Itoa(id), Label: name})
	}
	sort.Slice(result.Users, func(i, j int) bool { return result.Users[i].Label < result.Users[j].Label })
	// Model choices contain the actual recorded identifier, not an assumed alias.
	if selectedUser > 0 {
		base = base.Where("user_id = ?", selectedUser)
	}
	var models []string
	if err := base.Session(&gorm.Session{}).Distinct("model").Order("model ASC").Pluck("model", &models).Error; err != nil {
		return nil, err
	}
	for _, name := range models {
		result.Models = append(result.Models, UsageAnalysisOption{Value: name, Label: name})
	}
	return result, nil
}
