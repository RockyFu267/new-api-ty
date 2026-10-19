package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDepartmentModelsDefaultDenyExactNamesTransferAndRevoke(t *testing.T) {
	truncateTables(t)
	t.Cleanup(func() {
		DB.Where("1 = 1").Delete(&DepartmentModelPolicy{})
		DB.Where("1 = 1").Delete(&UserDepartment{})
		DB.Where("1 = 1").Delete(&Department{})
	})
	insertUsersForPaginationTest(t, 2)
	access, err := GetUserDepartmentModelAccess(1)
	require.NoError(t, err)
	assert.False(t, access.Allows("qwen"))
	d, err := SaveDepartment(0, "Engineering")
	require.NoError(t, err)
	require.NoError(t, AssignUserDepartment(1, d.Id))
	access, err = GetUserDepartmentModelAccess(1)
	require.NoError(t, err)
	assert.False(t, access.Allows("qwen"))
	names, err := SaveDepartmentModels(d.Id, []string{"qwen", " qwen ", "deepseek"})
	require.NoError(t, err)
	assert.Equal(t, []string{"deepseek", "qwen"}, names)
	access, err = GetUserDepartmentModelAccess(1)
	require.NoError(t, err)
	assert.True(t, access.Allows("qwen"))
	assert.False(t, access.Allows("qwen-thinking"))
	assert.False(t, access.Allows("QWEN"))
	filtered, err := FilterDepartmentModelNames(1, []string{"qwen", "other", "deepseek"})
	require.NoError(t, err)
	assert.Equal(t, []string{"qwen", "deepseek"}, filtered)
	other, err := SaveDepartment(0, "Research")
	require.NoError(t, err)
	require.NoError(t, AssignUserDepartment(1, other.Id))
	access, err = GetUserDepartmentModelAccess(1)
	require.NoError(t, err)
	assert.False(t, access.Allows("qwen"))
	require.NoError(t, AssignUserDepartment(1, d.Id))
	_, err = SaveDepartmentModels(d.Id, []string{})
	require.NoError(t, err)
	access, err = GetUserDepartmentModelAccess(1)
	require.NoError(t, err)
	assert.Empty(t, access.Models)
	_, err = SaveDepartmentModels(d.Id, []string{"qwen"})
	require.NoError(t, err)
	require.NoError(t, DeleteDepartment(d.Id))
	access, err = GetUserDepartmentModelAccess(1)
	require.NoError(t, err)
	assert.Equal(t, 0, access.DepartmentID)
	_, err = SaveDepartmentModels(999, []string{"qwen"})
	assert.Error(t, err)
}
