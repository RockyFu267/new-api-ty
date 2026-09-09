package controller

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestInjectPlaygroundKnowledgeUsesOnlySelectedEnabledDocuments(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	enabled := model.KnowledgeDocument{RelativePath: "k8s/k8s-clusters.md", Name: "k8s-clusters.md", Content: "# 集群\n公司共有三套 K8s 集群。", Enabled: true, Available: true}
	disabled := model.KnowledgeDocument{RelativePath: "private/secret.md", Name: "secret.md", Content: "不应出现的秘密", Enabled: false, Available: true}
	require.NoError(t, db.Create(&enabled).Error)
	require.NoError(t, db.Create(&disabled).Error)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	body := `{"model":"qwen-demo","knowledge_document_ids":[` + strconv.FormatInt(enabled.Id, 10) + `,` + strconv.FormatInt(disabled.Id, 10) + `],"messages":[{"role":"user","content":"有几套集群？"}]}`
	context.Request = httptest.NewRequest("POST", "/pg/chat/completions", strings.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	require.NoError(t, injectPlaygroundKnowledge(context))

	var enriched map[string]interface{}
	require.NoError(t, common.UnmarshalBodyReusable(context, &enriched))
	messages := enriched["messages"].([]interface{})
	system := messages[0].(map[string]interface{})["content"].(string)
	require.Contains(t, system, "公司共有三套 K8s 集群")
	require.Contains(t, system, "k8s/k8s-clusters.md")
	require.NotContains(t, system, "不应出现的秘密")
	require.NotContains(t, enriched, "knowledge_document_ids")
}

func TestInjectPlaygroundKnowledgeDoesNothingWithoutSelection(t *testing.T) {
	setupModelListControllerTestDB(t)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest("POST", "/pg/chat/completions", strings.NewReader(`{"model":"qwen-demo","messages":[{"role":"user","content":"你好"}]}`))
	context.Request.Header.Set("Content-Type", "application/json")
	require.NoError(t, injectPlaygroundKnowledge(context))
	var original struct {
		Messages []map[string]interface{} `json:"messages"`
	}
	require.NoError(t, common.UnmarshalBodyReusable(context, &original))
	require.Len(t, original.Messages, 1)
}

func TestKnowledgeDocumentCreateEditDelete(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}))
	root := t.TempDir()
	t.Setenv("KNOWLEDGE_BASE_PATH", root)

	createRecorder := httptest.NewRecorder()
	createContext, _ := gin.CreateTestContext(createRecorder)
	createContext.Request = httptest.NewRequest(http.MethodPost, "/api/knowledge-bases/documents", strings.NewReader(`{"relative_path":"k8s/demo.md","content":"# 初始内容"}`))
	createContext.Set("id", 1)
	CreateKnowledgeDocumentAdmin(createContext)
	require.Equal(t, http.StatusOK, createRecorder.Code)

	var document model.KnowledgeDocument
	require.NoError(t, db.Where("relative_path = ?", "k8s/demo.md").First(&document).Error)
	content, err := os.ReadFile(filepath.Join(root, "k8s", "demo.md"))
	require.NoError(t, err)
	require.Equal(t, "# 初始内容", string(content))

	editRecorder := httptest.NewRecorder()
	editContext, _ := gin.CreateTestContext(editRecorder)
	editContext.Params = gin.Params{{Key: "id", Value: strconv.FormatInt(document.Id, 10)}}
	editContext.Request = httptest.NewRequest(http.MethodPut, "/api/knowledge-bases/documents/id/content", strings.NewReader(`{"content":"# 更新内容"}`))
	editContext.Set("id", 1)
	EditKnowledgeDocumentAdmin(editContext)
	require.Equal(t, http.StatusOK, editRecorder.Code)
	content, err = os.ReadFile(filepath.Join(root, "k8s", "demo.md"))
	require.NoError(t, err)
	require.Equal(t, "# 更新内容", string(content))

	deleteRecorder := httptest.NewRecorder()
	deleteContext, _ := gin.CreateTestContext(deleteRecorder)
	deleteContext.Params = gin.Params{{Key: "id", Value: strconv.FormatInt(document.Id, 10)}}
	deleteContext.Request = httptest.NewRequest(http.MethodDelete, "/api/knowledge-bases/documents/id", nil)
	deleteContext.Set("id", 1)
	DeleteKnowledgeDocumentAdmin(deleteContext)
	require.Equal(t, http.StatusOK, deleteRecorder.Code)
	_, err = os.Stat(filepath.Join(root, "k8s", "demo.md"))
	require.True(t, os.IsNotExist(err))
}

func TestResolveKnowledgeMarkdownPathRejectsTraversal(t *testing.T) {
	t.Setenv("KNOWLEDGE_BASE_PATH", t.TempDir())
	_, _, err := resolveKnowledgeMarkdownPath("../outside.md", false)
	require.Error(t, err)
	_, _, err = resolveKnowledgeMarkdownPath("secret.txt", false)
	require.Error(t, err)
}
