package model

import (
	"github.com/QuantumNous/new-api/common"
)

type UserChannelModel struct {
	ChannelId   int    `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	ChannelType int    `json:"channel_type"`
	Group       string `json:"group"`
	Model       string `json:"model"`
}

func GetUserChannelModels(groups []string) ([]UserChannelModel, error) {
	if len(groups) == 0 {
		return []UserChannelModel{}, nil
	}

	var items []UserChannelModel
	err := DB.Table("abilities").
		Select("abilities.channel_id, channels.name AS channel_name, channels.type AS channel_type, abilities."+commonGroupCol+" AS "+commonGroupCol+", abilities.model").
		Joins("JOIN channels ON channels.id = abilities.channel_id").
		Where("abilities.enabled = ? AND channels.status = ?", true, common.ChannelStatusEnabled).
		Where("abilities."+commonGroupCol+" IN ?", groups).
		Order("channels.name ASC, abilities.model ASC, abilities." + commonGroupCol + " ASC").
		Scan(&items).Error
	return items, err
}
