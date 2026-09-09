package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

const (
	DefaultChatPromptTitle       = "开始与该模型对话"
	DefaultChatPromptDescription = "请在发送消息前确认当前渠道、模型及相关使用说明，并根据实际需要合理使用。"
)

type ChatPrompt struct {
	Id          int64     `json:"id" gorm:"primaryKey"`
	ChannelId   int       `json:"channel_id" gorm:"not null;uniqueIndex:idx_chat_prompt_channel_model,priority:1"`
	Model       string    `json:"model" gorm:"type:varchar(191);not null;uniqueIndex:idx_chat_prompt_channel_model,priority:2"`
	Title       string    `json:"title" gorm:"type:varchar(200);not null"`
	Description string    `json:"description" gorm:"type:text;not null"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (ChatPrompt) TableName() string { return "chat_prompts" }

type ChatPromptAdminItem struct {
	ChannelId   int    `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	ChannelType int    `json:"channel_type"`
	Model       string `json:"model"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Configured  bool   `json:"configured"`
}

func ListChatPromptAdminItems() ([]ChatPromptAdminItem, error) {
	var items []ChatPromptAdminItem
	err := DB.Table("abilities").
		Select("abilities.channel_id, channels.name AS channel_name, channels.type AS channel_type, abilities.model, COALESCE(chat_prompts.title, '') AS title, COALESCE(chat_prompts.description, '') AS description, CASE WHEN chat_prompts.id IS NULL THEN ? ELSE ? END AS configured", false, true).
		Joins("JOIN channels ON channels.id = abilities.channel_id").
		Joins("LEFT JOIN chat_prompts ON chat_prompts.channel_id = abilities.channel_id AND chat_prompts.model = abilities.model").
		Group("abilities.channel_id, channels.name, channels.type, abilities.model, chat_prompts.id, chat_prompts.title, chat_prompts.description").
		Order("channels.name ASC, abilities.model ASC").Scan(&items).Error
	return items, err
}

func UpsertChatPrompt(prompt *ChatPrompt) error {
	var existing ChatPrompt
	result := DB.Where("channel_id = ? AND model = ?", prompt.ChannelId, prompt.Model).First(&existing)
	if result.Error == nil {
		return DB.Model(&existing).Updates(map[string]any{"title": prompt.Title, "description": prompt.Description}).Error
	}
	if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return result.Error
	}
	return DB.Create(prompt).Error
}

func DeleteChatPrompt(channelId int, modelName string) error {
	return DB.Where("channel_id = ? AND model = ?", channelId, modelName).Delete(&ChatPrompt{}).Error
}

func ChatPromptChannelModelExists(channelId int, modelName string) bool {
	var count int64
	return DB.Model(&Ability{}).Where("channel_id = ? AND model = ?", channelId, modelName).Count(&count).Error == nil && count > 0
}
