package logger

import (
	"bytes"
	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
)

func TestTransientChatDebugDoesNotLogAttachments(t *testing.T) {
	previousDebug := common.DebugEnabled
	previousWriter := gin.DefaultErrorWriter
	t.Cleanup(func() { common.DebugEnabled = previousDebug; gin.DefaultErrorWriter = previousWriter })
	common.DebugEnabled = true
	var output bytes.Buffer
	gin.DefaultErrorWriter = &output
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/pg/chat/completions", nil)
	LogDebug(c, "request body: %s", "private attachment")
	require.Empty(t, output.String())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	LogDebug(c, "ordinary API debug")
	require.Contains(t, output.String(), "ordinary API debug")
}
