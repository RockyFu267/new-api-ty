package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type userChannelModelsResponse struct {
	Success bool                     `json:"success"`
	Data    []model.UserChannelModel `json:"data"`
}

func TestGetUserChannelModelsReturnsEnabledAuthorizedChannelModels(t *testing.T) {
	originalUsableGroups := setting.UserUsableGroups2JSONString()
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default"}`))
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(originalUsableGroups))
	})

	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{
		Id:       2201,
		Username: "channel-chat-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&[]model.Channel{
		{Id: 301, Name: "Local Qwen", Type: 33, Status: common.ChannelStatusEnabled, Key: "secret", Group: "default", Models: "qwen-local"},
		{Id: 302, Name: "AWS Bedrock", Type: 24, Status: common.ChannelStatusEnabled, Key: "secret", Group: "default", Models: "claude-bedrock"},
		{Id: 303, Name: "Disabled", Type: 1, Status: common.ChannelStatusManuallyDisabled, Key: "secret", Group: "default", Models: "disabled-model"},
	}).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "default", Model: "qwen-local", ChannelId: 301, Enabled: true},
		{Group: "default", Model: "claude-bedrock", ChannelId: 302, Enabled: true},
		{Group: "default", Model: "disabled-model", ChannelId: 303, Enabled: true},
		{Group: "private", Model: "private-model", ChannelId: 301, Enabled: true},
	}).Error)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/user/chat/channel-models", nil)
	context.Set("id", 2201)

	GetUserChannelModels(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload userChannelModelsResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Equal(t, []model.UserChannelModel{
		{ChannelId: 302, ChannelName: "AWS Bedrock", ChannelType: 24, Group: "default", Model: "claude-bedrock", PromptTitle: model.DefaultChatPromptTitle, PromptDescription: model.DefaultChatPromptDescription},
		{ChannelId: 301, ChannelName: "Local Qwen", ChannelType: 33, Group: "default", Model: "qwen-local", PromptTitle: model.DefaultChatPromptTitle, PromptDescription: model.DefaultChatPromptDescription},
	}, payload.Data)
}

func TestGetUserChannelModelsReturnsPromptForExactChannelModel(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.Create(&[]model.Channel{
		{Id: 401, Name: "Free Qwen", Type: 33, Status: common.ChannelStatusEnabled, Key: "secret", Group: "default", Models: "shared-model"},
		{Id: 402, Name: "Paid Qwen", Type: 24, Status: common.ChannelStatusEnabled, Key: "secret", Group: "default", Models: "shared-model"},
	}).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "default", Model: "shared-model", ChannelId: 401, Enabled: true},
		{Group: "default", Model: "shared-model", ChannelId: 402, Enabled: true},
	}).Error)
	require.NoError(t, db.Create(&model.ChatPrompt{
		ChannelId: 402, Model: "shared-model", Title: "付费模型", Description: "使用会产生费用。",
	}).Error)

	items, err := model.GetUserChannelModels([]string{"default"})
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, model.DefaultChatPromptTitle, items[0].PromptTitle)
	require.Equal(t, model.DefaultChatPromptDescription, items[0].PromptDescription)
	require.Equal(t, "付费模型", items[1].PromptTitle)
	require.Equal(t, "使用会产生费用。", items[1].PromptDescription)
}
