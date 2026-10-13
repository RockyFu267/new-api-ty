package controller

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

var safePrivateKnowledgeUsername = regexp.MustCompile(`^[A-Za-z0-9._@-]+$`)

func privateKnowledgeRoot() (string, error) {
	configured := strings.TrimSpace(os.Getenv("PRIVATE_KNOWLEDGE_BASE_PATH"))
	if configured == "" {
		return "", fmt.Errorf("PRIVATE_KNOWLEDGE_BASE_PATH is not configured")
	}
	return filepath.Abs(configured)
}

func currentPrivateKnowledgeOwner(c *gin.Context) (int, string, error) {
	userId := c.GetInt("id")
	user, err := model.GetUserById(userId, false)
	if err != nil {
		return 0, "", err
	}
	if !safePrivateKnowledgeUsername.MatchString(user.Username) {
		return 0, "", fmt.Errorf("用户名不能用于私有知识库目录")
	}
	return userId, user.Username, nil
}

func privateKnowledgePath(username, relative string, createParents bool) (string, string, error) {
	root, err := privateKnowledgeRoot()
	if err != nil {
		return "", "", err
	}
	userRoot := filepath.Join(root, "users", username)
	relative = filepath.ToSlash(strings.TrimSpace(relative))
	extension := strings.ToLower(filepath.Ext(relative))
	if !filepath.IsLocal(filepath.FromSlash(relative)) || extension != ".md" && extension != ".txt" {
		return "", "", fmt.Errorf("只支持私有目录内的 Markdown 或 TXT 文件")
	}
	full := filepath.Join(userRoot, filepath.FromSlash(relative))
	if createParents {
		if err = os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return "", "", err
		}
	}
	absoluteRoot, _ := filepath.Abs(userRoot)
	absoluteFull, _ := filepath.Abs(full)
	if local, relErr := filepath.Rel(absoluteRoot, absoluteFull); relErr != nil || !filepath.IsLocal(local) {
		return "", "", fmt.Errorf("路径超出用户私有目录")
	}
	if info, statErr := os.Lstat(full); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", "", fmt.Errorf("私有知识文件不能是符号链接")
	}
	return full, relative, nil
}

func privateKnowledgeDirectoryPath(username, relative string, create bool) (string, string, error) {
	root, err := privateKnowledgeRoot()
	if err != nil {
		return "", "", err
	}
	relative = filepath.ToSlash(strings.Trim(strings.TrimSpace(relative), "/"))
	if relative == "" || !filepath.IsLocal(filepath.FromSlash(relative)) {
		return "", "", fmt.Errorf("无效的私有知识库目录")
	}
	userRoot := filepath.Join(root, "users", username)
	full := filepath.Join(userRoot, filepath.FromSlash(relative))
	absoluteRoot, _ := filepath.Abs(userRoot)
	absoluteFull, _ := filepath.Abs(full)
	if local, relErr := filepath.Rel(absoluteRoot, absoluteFull); relErr != nil || !filepath.IsLocal(local) {
		return "", "", fmt.Errorf("目录超出用户私有目录")
	}
	if create {
		err = os.MkdirAll(full, 0o755)
	}
	return full, relative, err
}

func listPrivateKnowledgeDirectories(username string) ([]string, error) {
	root, err := privateKnowledgeRoot()
	if err != nil {
		return nil, err
	}
	userRoot := filepath.Join(root, "users", username)
	if err = os.MkdirAll(userRoot, 0o755); err != nil {
		return nil, err
	}
	directories := make([]string, 0)
	err = filepath.WalkDir(userRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.IsDir() || path == userRoot {
			return nil
		}
		relative, relErr := filepath.Rel(userRoot, path)
		if relErr != nil {
			return relErr
		}
		directories = append(directories, filepath.ToSlash(relative))
		return nil
	})
	return directories, err
}

func ListPrivateKnowledgeDocuments(c *gin.Context) {
	userId, username, err := currentPrivateKnowledgeOwner(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	documents, err := model.ListPrivateKnowledgeDocuments(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	directories, err := listPrivateKnowledgeDirectories(username)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"documents": documents, "directories": directories})
}

func CreatePrivateKnowledgeDirectory(c *gin.Context) {
	var request struct {
		RelativePath string `json:"relative_path"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false})
		return
	}
	_, username, err := currentPrivateKnowledgeOwner(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	_, relative, err := privateKnowledgeDirectoryPath(username, request.RelativePath, true)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	common.ApiSuccess(c, gin.H{"relative_path": relative})
}

func UploadPrivateKnowledgeDocument(c *gin.Context) {
	userId, username, err := currentPrivateKnowledgeOwner(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil || fileHeader.Size > maxKnowledgeMarkdownSize {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请选择不超过 1 MiB 的 Markdown 或 TXT 文件"})
		return
	}
	name := filepath.Base(fileHeader.Filename)
	directory := strings.Trim(strings.TrimSpace(c.PostForm("directory")), "/")
	relative := filepath.ToSlash(filepath.Join(filepath.FromSlash(directory), name))
	full, relative, err := privateKnowledgePath(username, relative, true)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	source, err := fileHeader.Open()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	defer source.Close()
	content, err := io.ReadAll(source)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	target, err := os.OpenFile(full, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if _, err = target.Write(content); err != nil {
		_ = target.Close()
		_ = os.Remove(full)
		common.ApiError(c, err)
		return
	}
	_ = target.Close()
	document := &model.PrivateKnowledgeDocument{UserId: userId, Username: username, RelativePath: relative, Name: name, Content: string(content), Size: int64(len(content))}
	if err = model.SavePrivateKnowledgeDocument(document); err != nil {
		_ = os.Remove(full)
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, document)
}

func GetPrivateKnowledgeDocument(c *gin.Context) {
	userId, _, err := currentPrivateKnowledgeOwner(c)
	id, parseErr := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || parseErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false})
		return
	}
	document, err := model.GetPrivateKnowledgeDocument(id, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"id": document.Id, "relative_path": document.RelativePath, "content": document.Content})
}

func CreatePrivateKnowledgeDocument(c *gin.Context) {
	var request struct {
		RelativePath string `json:"relative_path"`
		Content      string `json:"content"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil || len([]byte(request.Content)) > maxKnowledgeMarkdownSize {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效参数或文件超过 1 MiB"})
		return
	}
	userId, username, err := currentPrivateKnowledgeOwner(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	full, relative, err := privateKnowledgePath(username, request.RelativePath, true)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	file, err := os.OpenFile(full, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if _, err = file.WriteString(request.Content); err != nil {
		_ = file.Close()
		_ = os.Remove(full)
		common.ApiError(c, err)
		return
	}
	_ = file.Close()
	document := &model.PrivateKnowledgeDocument{UserId: userId, Username: username, RelativePath: relative, Name: filepath.Base(full), Content: request.Content, Size: int64(len([]byte(request.Content)))}
	if err = model.SavePrivateKnowledgeDocument(document); err != nil {
		_ = os.Remove(full)
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, document)
}

func EditPrivateKnowledgeDocument(c *gin.Context) {
	var request struct {
		Content string `json:"content"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil || len([]byte(request.Content)) > maxKnowledgeMarkdownSize {
		c.JSON(http.StatusBadRequest, gin.H{"success": false})
		return
	}
	userId, username, err := currentPrivateKnowledgeOwner(c)
	id, parseErr := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || parseErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false})
		return
	}
	document, err := model.GetPrivateKnowledgeDocument(id, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	full, _, err := privateKnowledgePath(username, document.RelativePath, false)
	if err == nil {
		err = os.WriteFile(full, []byte(request.Content), 0o644)
	}
	if err == nil {
		err = model.UpdatePrivateKnowledgeDocumentContent(id, userId, request.Content, int64(len([]byte(request.Content))))
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func DeletePrivateKnowledgeDocument(c *gin.Context) {
	userId, username, err := currentPrivateKnowledgeOwner(c)
	id, parseErr := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || parseErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false})
		return
	}
	document, err := model.GetPrivateKnowledgeDocument(id, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	full, _, err := privateKnowledgePath(username, document.RelativePath, false)
	if err == nil {
		err = os.Remove(full)
	}
	if err == nil {
		err = model.DeletePrivateKnowledgeDocument(id, userId)
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
