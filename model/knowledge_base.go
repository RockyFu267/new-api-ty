package model

import (
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

type KnowledgeDocument struct {
	Id            int64     `json:"id" gorm:"primaryKey"`
	RelativePath  string    `json:"relative_path" gorm:"type:varchar(500);not null;uniqueIndex"`
	Name          string    `json:"name" gorm:"type:varchar(255);not null"`
	Content       string    `json:"-" gorm:"type:text;not null"`
	Enabled       bool      `json:"enabled" gorm:"not null;default:false"`
	AllowDownload bool      `json:"allow_download" gorm:"not null;default:false"`
	Available     bool      `json:"available" gorm:"not null;default:true"`
	Size          int64     `json:"size"`
	UpdatedAt     time.Time `json:"updated_at"`
	CreatedAt     time.Time `json:"created_at"`
}

func ListKnowledgeDocuments(admin bool) ([]KnowledgeDocument, error) {
	var documents []KnowledgeDocument
	query := DB.Order("relative_path ASC")
	if !admin {
		query = query.Where("enabled = ? AND available = ?", true, true)
	}
	err := query.Find(&documents).Error
	for i := range documents {
		documents[i].Content = ""
	}
	return documents, err
}

func SyncKnowledgeDocuments(discovered []KnowledgeDocument) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&KnowledgeDocument{}).Where("1 = 1").Update("available", false).Error; err != nil {
			return err
		}
		for _, document := range discovered {
			var existing KnowledgeDocument
			err := tx.Where("relative_path = ?", document.RelativePath).First(&existing).Error
			if err == nil {
				if err = tx.Model(&existing).Updates(map[string]any{"name": document.Name, "content": document.Content, "size": document.Size, "available": true}).Error; err != nil {
					return err
				}
				continue
			}
			if err != gorm.ErrRecordNotFound {
				return err
			}
			document.Available = true
			if err = tx.Create(&document).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func UpdateKnowledgeDocumentAccess(id int64, enabled, allowDownload bool) error {
	return DB.Model(&KnowledgeDocument{}).Where("id = ? AND available = ?", id, true).Updates(map[string]any{"enabled": enabled, "allow_download": allowDownload}).Error
}

func GetKnowledgeDocument(id int64) (*KnowledgeDocument, error) {
	var document KnowledgeDocument
	err := DB.Where("id = ? AND enabled = ? AND available = ?", id, true, true).First(&document).Error
	return &document, err
}

func RetrieveKnowledgeDocuments(ids []int64, query string) ([]KnowledgeDocument, string, error) {
	if len(ids) == 0 {
		return nil, "", nil
	}
	var documents []KnowledgeDocument
	if err := DB.Where("id IN ? AND enabled = ? AND available = ?", ids, true, true).Order("relative_path ASC").Find(&documents).Error; err != nil {
		return nil, "", err
	}
	parts := make([]string, 0, len(documents))
	for _, document := range documents {
		parts = append(parts, retrieveMarkdown(document, query))
	}
	return documents, strings.Join(parts, "\n\n"), nil
}

func retrieveMarkdown(document KnowledgeDocument, query string) string {
	chunks := strings.Split(strings.ReplaceAll(document.Content, "\r\n", "\n"), "\n\n")
	type scored struct {
		text  string
		score int
	}
	terms := knowledgeTerms(query)
	scores := make([]scored, 0, len(chunks))
	for _, chunk := range chunks {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}
		lower, score := strings.ToLower(chunk), 0
		for _, term := range terms {
			if strings.Contains(lower, term) {
				score++
			}
		}
		scores = append(scores, scored{text: chunk, score: score})
	}
	sort.SliceStable(scores, func(i, j int) bool { return scores[i].score > scores[j].score })
	limit := 6
	if len(scores) < limit {
		limit = len(scores)
	}
	selected := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		selected = append(selected, scores[i].text)
	}
	return "[来源：" + document.RelativePath + "]\n" + strings.Join(selected, "\n\n")
}

func knowledgeTerms(query string) []string {
	query = strings.ToLower(strings.TrimSpace(query))
	terms, runes := strings.Fields(query), []rune(query)
	for size := 2; size <= 4; size++ {
		for i := 0; i+size <= len(runes); i++ {
			if part := strings.TrimSpace(string(runes[i : i+size])); part != "" {
				terms = append(terms, part)
			}
		}
	}
	return terms
}

func BuildKnowledgeSystemPrompt(documents []KnowledgeDocument, context string) string {
	if len(documents) == 0 || context == "" {
		return ""
	}
	sources := make([]string, 0, len(documents))
	for _, document := range documents {
		sources = append(sources, document.RelativePath)
	}
	return "你正在使用 TIANYAN 企业内部知识库。请优先依据检索结果回答；资料未说明时明确表示未知，不要编造。回答末尾列出知识来源。\n\n<knowledge>\n" + context + "\n</knowledge>\n\n可用来源：" + strings.Join(sources, "、")
}
