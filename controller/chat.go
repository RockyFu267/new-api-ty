package controller

import (
	"net/http"
	"sort"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func GetUserChannelModels(c *gin.Context) {
	user, err := model.GetUserCache(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}

	usableGroups := service.GetUserUsableGroups(user.Group)
	groups := make([]string, 0, len(usableGroups))
	for group := range usableGroups {
		if group != "auto" {
			groups = append(groups, group)
		}
	}
	sort.Strings(groups)

	items, err := model.GetUserChannelModels(groups)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    items,
	})
}
