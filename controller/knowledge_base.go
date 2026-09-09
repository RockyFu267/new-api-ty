package controller

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
)

const maxKnowledgeMarkdownSize = 1024 * 1024

func knowledgeBaseRoot() (string, error) {
	if configured := strings.TrimSpace(os.Getenv("KNOWLEDGE_BASE_PATH")); configured != "" {
		return filepath.Abs(configured)
	}
	for _, candidate := range []string{"knowledge-base", filepath.Join("..", "knowledge-base")} {
		absolute, err := filepath.Abs(candidate)
		if err == nil {
			if info, statErr := os.Stat(absolute); statErr == nil && info.IsDir() {
				return absolute, nil
			}
		}
	}
	return "", fmt.Errorf("knowledge-base directory not found; set KNOWLEDGE_BASE_PATH")
}

func scanKnowledgeMarkdown() ([]model.KnowledgeDocument, error) {
	root, err := knowledgeBaseRoot()
	if err != nil {
		return nil, err
	}
	documents := make([]model.KnowledgeDocument, 0)
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Size() > maxKnowledgeMarkdownSize {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil || !filepath.IsLocal(relative) {
			return fmt.Errorf("invalid knowledge path")
		}
		documents = append(documents, model.KnowledgeDocument{RelativePath: filepath.ToSlash(relative), Name: entry.Name(), Content: string(content), Size: info.Size()})
		return nil
	})
	return documents, err
}

func ListKnowledgeDocumentsAdmin(c *gin.Context) {
	documents, err := model.ListKnowledgeDocuments(true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, documents)
}

func ScanKnowledgeDocumentsAdmin(c *gin.Context) {
	documents, err := scanKnowledgeMarkdown()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err = model.SyncKnowledgeDocuments(documents); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "knowledge_base.scan", map[string]interface{}{"count": len(documents)})
	common.ApiSuccess(c, gin.H{"count": len(documents)})
}

func UpdateKnowledgeDocumentAdmin(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	var request struct {
		Enabled       bool `json:"enabled"`
		AllowDownload bool `json:"allow_download"`
	}
	if err != nil || common.DecodeJson(c.Request.Body, &request) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效参数"})
		return
	}
	if err = model.UpdateKnowledgeDocumentAccess(id, request.Enabled, request.AllowDownload); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "knowledge_base.access", map[string]interface{}{"id": id, "enabled": request.Enabled, "download": request.AllowDownload})
	common.ApiSuccess(c, nil)
}

func resolveKnowledgeMarkdownPath(relativePath string, createParents bool) (string, string, error) {
	root, err := knowledgeBaseRoot()
	if err != nil {
		return "", "", err
	}
	relativePath = filepath.ToSlash(strings.TrimSpace(relativePath))
	if !filepath.IsLocal(filepath.FromSlash(relativePath)) || !strings.EqualFold(filepath.Ext(relativePath), ".md") {
		return "", "", fmt.Errorf("知识文件必须是 knowledge-base 目录内的 .md 相对路径")
	}
	fullPath := filepath.Join(root, filepath.FromSlash(relativePath))
	parent := filepath.Dir(fullPath)
	if createParents {
		if err = os.MkdirAll(parent, 0755); err != nil {
			return "", "", err
		}
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", "", err
	}
	resolvedParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", "", err
	}
	parentRelative, err := filepath.Rel(resolvedRoot, resolvedParent)
	if err != nil || !filepath.IsLocal(parentRelative) && parentRelative != "." {
		return "", "", fmt.Errorf("知识文件路径超出 knowledge-base 目录")
	}
	if info, statErr := os.Lstat(fullPath); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", "", fmt.Errorf("知识文件不能是符号链接")
	}
	return fullPath, relativePath, nil
}

func GetKnowledgeDocumentAdmin(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效参数"})
		return
	}
	document, err := model.GetKnowledgeDocumentAdmin(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"id": document.Id, "relative_path": document.RelativePath, "content": document.Content,
	}})
}

func CreateKnowledgeDocumentAdmin(c *gin.Context) {
	var request struct {
		RelativePath string `json:"relative_path"`
		Content      string `json:"content"`
	}
	if common.DecodeJson(c.Request.Body, &request) != nil || len([]byte(request.Content)) > maxKnowledgeMarkdownSize {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效参数或文件超过 1 MiB"})
		return
	}
	fullPath, relativePath, err := resolveKnowledgeMarkdownPath(request.RelativePath, true)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	file, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "文件已存在或无法创建"})
		return
	}
	if _, err = file.WriteString(request.Content); err != nil {
		_ = file.Close()
		_ = os.Remove(fullPath)
		common.ApiError(c, err)
		return
	}
	if err = file.Close(); err != nil {
		_ = os.Remove(fullPath)
		common.ApiError(c, err)
		return
	}
	document := &model.KnowledgeDocument{
		RelativePath: relativePath, Name: filepath.Base(fullPath), Content: request.Content,
		Available: true, Size: int64(len([]byte(request.Content))),
	}
	if err = model.CreateKnowledgeDocument(document); err != nil {
		_ = os.Remove(fullPath)
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "knowledge_base.create", map[string]interface{}{"path": relativePath})
	common.ApiSuccess(c, document)
}

func EditKnowledgeDocumentAdmin(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	var request struct {
		Content string `json:"content"`
	}
	if err != nil || common.DecodeJson(c.Request.Body, &request) != nil || len([]byte(request.Content)) > maxKnowledgeMarkdownSize {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效参数或文件超过 1 MiB"})
		return
	}
	document, err := model.GetKnowledgeDocumentAdmin(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	fullPath, _, err := resolveKnowledgeMarkdownPath(document.RelativePath, false)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	if err = os.WriteFile(fullPath, []byte(request.Content), 0644); err != nil {
		common.ApiError(c, err)
		return
	}
	if err = model.UpdateKnowledgeDocumentContent(id, request.Content, int64(len([]byte(request.Content)))); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "knowledge_base.edit", map[string]interface{}{"path": document.RelativePath})
	common.ApiSuccess(c, nil)
}

func DeleteKnowledgeDocumentAdmin(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效参数"})
		return
	}
	document, err := model.GetKnowledgeDocumentAdmin(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	fullPath, _, err := resolveKnowledgeMarkdownPath(document.RelativePath, false)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	if err = os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		common.ApiError(c, err)
		return
	}
	if err = model.DeleteKnowledgeDocument(id); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "knowledge_base.delete", map[string]interface{}{"path": document.RelativePath})
	common.ApiSuccess(c, nil)
}

func ListUserKnowledgeDocuments(c *gin.Context) {
	documents, err := model.ListKnowledgeDocuments(false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, documents)
}

func DownloadKnowledgeDocument(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false})
		return
	}
	document, err := model.GetKnowledgeDocument(id)
	if err != nil || !document.AllowDownload {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "该知识文件仅允许在 Chat 中使用"})
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", document.Name))
	c.Data(http.StatusOK, "text/markdown; charset=utf-8", []byte(document.Content))
}

func injectPlaygroundKnowledge(c *gin.Context) error {
	var request struct {
		KnowledgeDocumentIds []int64       `json:"knowledge_document_ids"`
		Messages             []dto.Message `json:"messages"`
	}
	if err := common.UnmarshalBodyReusable(c, &request); err != nil {
		return err
	}
	if len(request.KnowledgeDocumentIds) == 0 {
		return nil
	}
	query := ""
	for i := len(request.Messages) - 1; i >= 0; i-- {
		if request.Messages[i].Role == "user" {
			query = request.Messages[i].StringContent()
			break
		}
	}
	documents, context, err := model.RetrieveKnowledgeDocuments(request.KnowledgeDocumentIds, query)
	if err != nil {
		return err
	}
	prompt := model.BuildKnowledgeSystemPrompt(documents, context)
	if prompt == "" {
		return nil
	}
	var raw map[string]interface{}
	if err = common.UnmarshalBodyReusable(c, &raw); err != nil {
		return err
	}
	messages, _ := raw["messages"].([]interface{})
	raw["messages"] = append([]interface{}{map[string]interface{}{"role": "system", "content": prompt}}, messages...)
	delete(raw, "knowledge_document_ids")
	data, err := common.Marshal(raw)
	if err != nil {
		return err
	}
	c.Header("X-TIANYAN-Knowledge-Count", strconv.Itoa(len(documents)))
	return common.ReplaceRequestBody(c, data)
}
