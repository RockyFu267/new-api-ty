package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDepartmentsAssignmentFilterAndDeletion(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&Department{}, &UserDepartment{}))
	t.Cleanup(func() { DB.Where("1 = 1").Delete(&UserDepartment{}); DB.Where("1 = 1").Delete(&Department{}) })
	insertUsersForPaginationTest(t, 3)
	department, err := SaveDepartment(0, "Engineering")
	require.NoError(t, err)
	other, err := SaveDepartment(0, "Research")
	require.NoError(t, err)
	require.NoError(t, AssignUserDepartment(1, department.Id))
	require.NoError(t, AssignUserDepartment(2, department.Id))
	users, total, err := SearchUsersWithDepartment("", "", nil, nil, &department.Id, 0, 1, NewUserSortOptions("id", "asc"))
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Equal(t, []int{1}, collectUserIDs(users))
	require.NoError(t, EnrichUsersWithDepartment(users))
	require.Equal(t, "Engineering", users[0].DepartmentName)
	_, err = SaveDepartment(department.Id, "Development")
	require.NoError(t, err)
	require.NoError(t, EnrichUsersWithDepartment(users))
	require.Equal(t, "Development", users[0].DepartmentName)
	require.NoError(t, AssignUserDepartment(1, other.Id))
	zero := 0
	users, total, err = SearchUsersWithDepartment("", "", nil, nil, &zero, 0, 20)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, []int{3}, collectUserIDs(users))
	require.NoError(t, DeleteDepartment(department.Id))
	users, total, err = SearchUsersWithDepartment("", "", nil, nil, &zero, 0, 20, NewUserSortOptions("id", "asc"))
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Equal(t, []int{2, 3}, collectUserIDs(users))
	users, total, err = SearchUsersWithDepartment("", "", nil, nil, nil, 0, 20)
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	require.Len(t, users, 3)
	require.NoError(t, AssignUserDepartment(1, 0))
	departments, err := ListDepartments()
	require.NoError(t, err)
	require.Len(t, departments, 1)
	require.Zero(t, departments[0].UserCount)
}

func TestDepartmentsRejectInvalidNamesAndMissingTargets(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&Department{}, &UserDepartment{}))
	t.Cleanup(func() { DB.Where("1 = 1").Delete(&UserDepartment{}); DB.Where("1 = 1").Delete(&Department{}) })
	insertUsersForPaginationTest(t, 1)
	_, err := SaveDepartment(0, "  ")
	require.Error(t, err)
	department, err := SaveDepartment(0, "Research")
	require.NoError(t, err)
	_, err = SaveDepartment(0, " Research ")
	require.Error(t, err)
	require.Error(t, AssignUserDepartment(1, department.Id+100))
	require.Error(t, AssignUserDepartment(999, department.Id))
	require.NoError(t, AssignUserDepartment(1, department.Id))
	// Updating credentials/profile (including AD sync) cannot overwrite membership.
	require.NoError(t, DB.Model(&User{}).Where("id = ?", 1).Update("display_name", "AD name").Error)
	users := []*User{{Id: 1}}
	require.NoError(t, EnrichUsersWithDepartment(users))
	require.Equal(t, department.Id, users[0].DepartmentId)
}
