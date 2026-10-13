package model

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

type PrivateKnowledgeDocument struct {
	Id           int64     `json:"id" gorm:"primaryKey"`
	UserId       int       `json:"-" gorm:"not null;uniqueIndex:idx_private_knowledge_owner_path"`
	Username     string    `json:"-" gorm:"type:varchar(255);not null"`
	RelativePath string    `json:"relative_path" gorm:"type:varchar(500);not null;uniqueIndex:idx_private_knowledge_owner_path"`
	Name         string    `json:"name" gorm:"type:varchar(255);not null"`
	Content      string    `json:"-" gorm:"type:text;not null"`
	Size         int64     `json:"size"`
	UpdatedAt    time.Time `json:"updated_at"`
	CreatedAt    time.Time `json:"created_at"`
}

func ListPrivateKnowledgeDocuments(userId int) ([]PrivateKnowledgeDocument, error) {
	var documents []PrivateKnowledgeDocument
	err := DB.Where("user_id = ?", userId).Order("relative_path ASC").Find(&documents).Error
	for i := range documents {
		documents[i].Content = ""
	}
	return documents, err
}

func GetPrivateKnowledgeDocument(id int64, userId int) (*PrivateKnowledgeDocument, error) {
	var document PrivateKnowledgeDocument
	err := DB.Where("id = ? AND user_id = ?", id, userId).First(&document).Error
	return &document, err
}

func SavePrivateKnowledgeDocument(document *PrivateKnowledgeDocument) error {
	var existing PrivateKnowledgeDocument
	err := DB.Where("user_id = ? AND relative_path = ?", document.UserId, document.RelativePath).First(&existing).Error
	if err == nil {
		return gorm.ErrDuplicatedKey
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	return DB.Create(document).Error
}

func UpdatePrivateKnowledgeDocumentContent(id int64, userId int, content string, size int64) error {
	result := DB.Model(&PrivateKnowledgeDocument{}).Where("id = ? AND user_id = ?", id, userId).Updates(map[string]any{"content": content, "size": size})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func DeletePrivateKnowledgeDocument(id int64, userId int) error {
	result := DB.Where("id = ? AND user_id = ?", id, userId).Delete(&PrivateKnowledgeDocument{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func RetrievePrivateKnowledgeDocuments(ids []int64, userId int, query string) ([]PrivateKnowledgeDocument, string, error) {
	var documents []PrivateKnowledgeDocument
	if len(ids) == 0 {
		return documents, "", nil
	}
	if err := DB.Where("id IN ? AND user_id = ?", ids, userId).Order("relative_path ASC").Find(&documents).Error; err != nil {
		return nil, "", err
	}
	parts := make([]string, 0, len(documents))
	for _, document := range documents {
		publicShape := KnowledgeDocument{RelativePath: "我的知识库/" + document.RelativePath, Content: document.Content}
		parts = append(parts, retrieveMarkdown(publicShape, query))
	}
	return documents, strings.Join(parts, "\n\n"), nil
}
