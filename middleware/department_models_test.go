package middleware

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestDepartmentModelsBlockAllRolesAndSpecificChannelsBeforeRelay(t *testing.T) {
	require.NoError(t, i18n.Init())
	db, err := gorm.Open(sqlite.Open("file:department-model-middleware?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	old := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = old; sqlDB, _ := db.DB(); sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Department{}, &model.UserDepartment{}, &model.DepartmentModelPolicy{}, &model.Channel{}))
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "root", Role: 100, Quota: 9999999, AffCode: "root"}).Error)
	require.NoError(t, db.Create(&model.Channel{Id: 1, Status: common.ChannelStatusEnabled, Type: 1, Key: "test-key", Models: "qwen"}).Error)
	department, err := model.SaveDepartment(0, "Engineering")
	require.NoError(t, err)
	called := 0
	role := 1
	router := gin.New()
	router.Any("/*path", func(c *gin.Context) {
		c.Set("id", 1)
		c.Set("role", role)
		common.SetContextKey(c, constant.ContextKeyTokenSpecificChannelId, "1")
		c.Next()
	}, Distribute(), func(c *gin.Context) { called++; c.Status(204) })
	invoke := func(path, body string) int {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, r)
		return w.Code
	}
	for _, actor := range []int{1, 10, 100} {
		role = actor
		assert.Equal(t, 403, invoke("/v1/chat/completions", `{"model":"qwen"}`))
	}
	assert.Zero(t, called)
	require.NoError(t, model.AssignUserDepartment(1, department.Id))
	assert.Equal(t, 403, invoke("/v1/chat/completions", `{"model":"qwen"}`))
	_, err = model.SaveDepartmentModels(department.Id, []string{"qwen"})
	require.NoError(t, err)
	for _, path := range []string{"/v1/chat/completions", "/v1/responses", "/v1/messages", "/v1/embeddings", "/pg/chat/completions", "/v1beta/models/qwen:generateContent"} {
		t.Run(path, func(t *testing.T) { assert.Equal(t, 204, invoke(path, `{"model":"qwen"}`)) })
	}
	for _, path := range []string{"/v1/chat/completions", "/v1/responses", "/v1/messages", "/pg/chat/completions", "/v1beta/models/denied:generateContent"} {
		assert.Equal(t, 403, invoke(path, `{"model":"denied"}`))
	}
	assert.Equal(t, 400, invoke("/v1/chat/completions", `{"model":"qwen","model":"denied"}`))
	assert.Equal(t, 400, invoke("/v1/chat/completions", `{"model":"qwen","Model":"denied"}`))
	// A request snapshot survives revocation, but every new request sees the change.
	c := &gin.Context{}
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Set("id", 1)
	require.NoError(t, service.CheckDepartmentModelAccess(c, "qwen"))
	_, err = model.SaveDepartmentModels(department.Id, []string{})
	require.NoError(t, err)
	assert.NoError(t, service.CheckDepartmentModelAccess(c, "qwen"))
	assert.Equal(t, 403, invoke("/v1/chat/completions", `{"model":"qwen"}`))
	assert.Equal(t, 6, called, fmt.Sprintf("only authorized calls may reach relay, got %d", called))
}
