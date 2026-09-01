package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSelectPlaygroundChannelLocksAuthorizedChannel(t *testing.T) {
	originalUsableGroups := setting.UserUsableGroups2JSONString()
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default"}`))
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsableGroups))
	})

	db, err := gorm.Open(sqlite.Open("file:select-playground-channel?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.MemoryCacheEnabled = false
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))
	require.NoError(t, db.Create(&model.Channel{
		Id: 401, Name: "Local Qwen", Type: 33, Status: common.ChannelStatusEnabled, Key: "secret", Group: "default", Models: "qwen-local",
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group: "default", Model: "qwen-local", ChannelId: 401, Enabled: true,
	}).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/pg/chat/completions", func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
		c.Next()
	}, SelectPlaygroundChannel(), func(c *gin.Context) {
		channelId, exists := common.GetContextKey(c, constant.ContextKeyTokenSpecificChannelId)
		require.True(t, exists)
		require.Equal(t, "401", channelId)
		require.Equal(t, "default", common.GetContextKeyString(c, constant.ContextKeyUsingGroup))
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodPost, "/pg/chat/completions", strings.NewReader(`{"channel_id":401,"group":"default","model":"qwen-local"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.Equal(t, "401", recorder.Header().Get("X-TIANYAN-Channel-ID"))
	require.Equal(t, "33", recorder.Header().Get("X-TIANYAN-Channel-Type"))
	require.Equal(t, "qwen-local", recorder.Header().Get("X-TIANYAN-Model"))
}
