package controller

import (
	"net/http/httptest"
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
