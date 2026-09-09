package controller

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestInjectPlaygroundSkill(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest("POST", "/pg/chat/completions", strings.NewReader(`{"model":"demo","skill_id":"showcase-designer","messages":[{"role":"user","content":"换成管理驾驶舱"}]}`))
	context.Request.Header.Set("Content-Type", "application/json")
	require.NoError(t, injectPlaygroundSkill(context))
	data, err := io.ReadAll(context.Request.Body)
	require.NoError(t, err)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &body))
	require.NotContains(t, body, "skill_id")
	messages := body["messages"].([]interface{})
	require.Len(t, messages, 2)
	require.Contains(t, messages[0].(map[string]interface{})["content"], "executive-dark")
}

func TestInjectPlaygroundSkillRejectsUnknownSkill(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest("POST", "/pg/chat/completions", strings.NewReader(`{"skill_id":"shell-runner","messages":[]}`))
	context.Request.Header.Set("Content-Type", "application/json")
	require.Error(t, injectPlaygroundSkill(context))
}
