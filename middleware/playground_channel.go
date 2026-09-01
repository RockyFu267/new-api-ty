package middleware

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func SelectPlaygroundChannel() gin.HandlerFunc {
	return func(c *gin.Context) {
		request := &dto.PlayGroundRequest{}
		if err := common.UnmarshalBodyReusable(c, request); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error(), "type": "invalid_request_error"}})
			return
		}
		if request.ChannelId == 0 {
			c.Next()
			return
		}
		if request.ChannelId < 0 || request.Model == "" || request.Group == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "channel_id, model and group are required", "type": "invalid_request_error"}})
			return
		}

		userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
		if !service.GroupInUserUsableGroups(userGroup, request.Group) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": gin.H{"message": "group access denied", "type": "access_denied"}})
			return
		}

		channel, err := model.GetChannelById(request.ChannelId, true)
		if err != nil || channel.Status != common.ChannelStatusEnabled {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": gin.H{"message": "channel is unavailable", "type": "access_denied"}})
			return
		}
		if !model.IsChannelEnabledForGroupModel(request.Group, request.Model, request.ChannelId) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": gin.H{"message": "the selected channel cannot serve this model in the requested group", "type": "access_denied"}})
			return
		}

		common.SetContextKey(c, constant.ContextKeyUsingGroup, request.Group)
		common.SetContextKey(c, constant.ContextKeyTokenSpecificChannelId, strconv.Itoa(request.ChannelId))
		c.Header("X-TIANYAN-Channel-ID", strconv.Itoa(request.ChannelId))
		c.Header("X-TIANYAN-Channel-Type", strconv.Itoa(channel.Type))
		c.Header("X-TIANYAN-Model", request.Model)
		c.Next()
	}
}
