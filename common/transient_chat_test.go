package common

import (
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTransientChatBodyNeverUsesDisk(t *testing.T) {
	previous := GetDiskCacheConfig()
	t.Cleanup(func() { SetDiskCacheConfig(previous) })
	SetDiskCacheConfig(DiskCacheConfig{Enabled: true, ThresholdMB: 1, MaxSizeMB: 100, Path: t.TempDir()})
	payload := strings.Repeat("sensitive attachment", 100000)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/pg/chat/completions", strings.NewReader(payload))
	storage, err := GetBodyStorage(c)
	require.NoError(t, err)
	require.False(t, storage.IsDisk())
	data, err := storage.Bytes()
	require.NoError(t, err)
	require.Equal(t, payload, string(data))
	require.NoError(t, ReplaceRequestBody(c, []byte(payload)))
	replacement, err := GetBodyStorage(c)
	require.NoError(t, err)
	require.False(t, replacement.IsDisk())
	replay, err := c.Request.GetBody()
	require.NoError(t, err)
	bytes, err := io.ReadAll(replay)
	require.NoError(t, err)
	require.Equal(t, payload, string(bytes))
	require.NoError(t, replay.Close())
	CleanupBodyStorage(c)
}
