package service

import (
	"encoding/base64"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTransientChatFileSourceNeverUsesDisk(t *testing.T) {
	previous := common.GetDiskCacheConfig()
	t.Cleanup(func() { common.SetDiskCacheConfig(previous) })
	cacheDir := t.TempDir()
	common.SetDiskCacheConfig(common.DiskCacheConfig{Enabled: true, ThresholdMB: 1, MaxSizeMB: 100, Path: cacheDir})
	payload := strings.Repeat("private image bytes", 100000)
	source := types.NewBase64FileSource(base64.StdEncoding.EncodeToString([]byte(payload)), "image/png")
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/pg/chat/completions", nil)
	data, err := LoadFileSource(c, source)
	require.NoError(t, err)
	require.False(t, data.IsDisk())
	encoded, err := data.GetBase64Data()
	require.NoError(t, err)
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	require.NoError(t, err)
	require.Equal(t, payload, string(decoded))
	CleanupFileSources(c)
}
