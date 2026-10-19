package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Routing/pricing fixtures explicitly grant their test models; default-deny tests do not.
func authorizeControllerDepartmentModels(t *testing.T, c *gin.Context) {
	t.Helper()
	id := c.GetInt("id")
	if id == 0 {
		id = 99001
		c.Set("id", id)
	}
	var count int64
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", id).Count(&count).Error)
	if count == 0 {
		require.NoError(t, model.DB.Create(&model.User{Id: id, Username: fmt.Sprintf("model-%d", id), AffCode: fmt.Sprintf("aff-%d", id)}).Error)
	}
	department, err := model.SaveDepartment(0, fmt.Sprintf("models-%d", id))
	require.NoError(t, err)
	require.NoError(t, model.AssignUserDepartment(id, department.Id))
	var names []string
	require.NoError(t, model.DB.Model(&model.Ability{}).Distinct("model").Pluck("model", &names).Error)
	_, err = model.SaveDepartmentModels(department.Id, names)
	require.NoError(t, err)
}

func TestDepartmentModelListsDenyUnassignedAdminAndIntersectAllLists(t *testing.T) {
	withSelfUseModeEnabled(t)
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{Id: 101, Username: "root-model", AffCode: "root-model", Role: 100, Group: "default"}).Error)
	require.NoError(t, db.Create(&model.Channel{Id: 1, Name: "Models", Type: 1, Status: common.ChannelStatusEnabled, Models: "allowed,denied", Group: "default"}).Error)
	require.NoError(t, db.Create(&[]model.Ability{{Group: "default", Model: "allowed", ChannelId: 1, Enabled: true}, {Group: "default", Model: "denied", ChannelId: 1, Enabled: true}}).Error)
	for _, handler := range []func(*gin.Context){GetUserModels, GetUserChannelModels} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("id", 101)
		c.Set("role", 100)
		c.Request = httptest.NewRequest("GET", "/api/user/models", nil)
		handler(c)
		var result struct {
			Success bool
			Data    []interface{}
		}
		require.NoError(t, common.Unmarshal(w.Body.Bytes(), &result))
		assert.True(t, result.Success)
		assert.Empty(t, result.Data)
	}
	d, err := model.SaveDepartment(0, "Engineering")
	require.NoError(t, err)
	require.NoError(t, model.AssignUserDepartment(101, d.Id))
	_, err = model.SaveDepartmentModels(d.Id, []string{"allowed"})
	require.NoError(t, err)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("id", 101)
	c.Request = httptest.NewRequest("GET", "/api/user/models", nil)
	GetUserModels(c)
	assert.Equal(t, []string{"allowed"}, decodeUserModelsResponse(t, w))
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Set("id", 101)
	c.Request = httptest.NewRequest("GET", "/api/user/chat/channel-models", nil)
	GetUserChannelModels(c)
	var chat userChannelModelsResponse
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &chat))
	require.Len(t, chat.Data, 1)
	assert.Equal(t, "allowed", chat.Data[0].Model)
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Set("id", 101)
	c.Request = httptest.NewRequest("GET", "/v1/models", nil)
	ListModels(c, constant.ChannelTypeOpenAI)
	assert.Equal(t, map[string]struct{}{"allowed": {}}, decodeListModelsResponse(t, w))
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Set("id", 101)
	c.Request = httptest.NewRequest("GET", "/v1/models", nil)
	common.SetContextKey(c, constant.ContextKeyTokenModelLimitEnabled, true)
	common.SetContextKey(c, constant.ContextKeyTokenModelLimit, map[string]bool{"denied": true})
	ListModels(c, constant.ChannelTypeOpenAI)
	assert.Empty(t, decodeListModelsResponse(t, w))
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Set("id", 101)
	c.Params = gin.Params{{Key: "model", Value: "denied"}}
	c.Request = httptest.NewRequest("GET", "/v1/models/denied", nil)
	RetrieveModel(c, constant.ChannelTypeOpenAI)
	assert.Equal(t, http.StatusForbidden, w.Code)
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/api/user/company-quota/models", strings.NewReader(fmt.Sprintf(`{"department_id":%d,"models":[]}`, d.Id)))
	DepartmentModels(c)
	var response struct{ Success bool }
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &response))
	assert.True(t, response.Success)
}
