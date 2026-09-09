package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func ListChatPrompts(c *gin.Context) {
	items, err := model.ListChatPromptAdminItems()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, items)
}

type chatPromptRequest struct {
	ChannelId   int    `json:"channel_id"`
	Model       string `json:"model"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

func SaveChatPrompt(c *gin.Context) {
	var request chatPromptRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的参数"})
		return
	}
	request.Model = strings.TrimSpace(request.Model)
	request.Title = strings.TrimSpace(request.Title)
	request.Description = strings.TrimSpace(request.Description)
	if request.ChannelId <= 0 || request.Model == "" || request.Title == "" || len([]rune(request.Title)) > 200 || len([]rune(request.Description)) > 4000 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "渠道、模型和标题不能为空，标题最多 200 字，说明最多 4000 字"})
		return
	}
	if !model.ChatPromptChannelModelExists(request.ChannelId, request.Model) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "渠道模型不存在"})
		return
	}
	prompt := &model.ChatPrompt{ChannelId: request.ChannelId, Model: request.Model, Title: request.Title, Description: request.Description}
	if err := model.UpsertChatPrompt(prompt); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "chat_prompt.update", map[string]interface{}{"channel_id": request.ChannelId, "model": request.Model})
	common.ApiSuccess(c, prompt)
}

func RemoveChatPrompt(c *gin.Context) {
	channelId, err := strconv.Atoi(c.Query("channel_id"))
	modelName := strings.TrimSpace(c.Query("model"))
	if err != nil || channelId <= 0 || modelName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的渠道模型"})
		return
	}
	if err = model.DeleteChatPrompt(channelId, modelName); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "chat_prompt.delete", map[string]interface{}{"channel_id": channelId, "model": modelName})
	common.ApiSuccess(c, nil)
}
