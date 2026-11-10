package service

import (
	"net/http/httptest"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	ds "github.com/QuantumNous/new-api/setting/deepseek_setting"
	hosttypes "github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDedicatedDeepSeekReplacesOrdinaryPriceWithoutGroupMarkup(t *testing.T) {
	r := ds.DefaultRule()
	r.Enabled = true
	r.EffectiveAt = 1
	r.Version = 1
	at := time.Date(2026, 11, 10, 10, 0, 0, 0, ds.Location)
	snap := (ds.Store{Rules: []ds.Rule{r}}).Select(ds.Model, at)
	info := &relaycommon.RelayInfo{OriginModelName: ds.Model, StartTime: at, DeepSeekBillingSnapshot: snap, PriceData: hosttypes.PriceData{ModelRatio: 99, CompletionRatio: 99, GroupRatioInfo: hosttypes.GroupRatioInfo{GroupRatio: 10}}}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	usage := &dto.Usage{PromptTokens: 1000000, CompletionTokens: 1000000, PromptTokensDetails: dto.InputTokenDetails{CachedTokens: 500000}}
	summary := calculateTextQuotaSummary(c, info, usage)
	require.Equal(t, 617808, summary.Quota)
	require.NotNil(t, info.DeepSeekCharge)
	require.InDelta(t, 9.02, info.DeepSeekCharge.CNY, 1e-9)
	info.DeepSeekBillingSnapshot = nil
	ordinary := calculateTextQuotaSummary(c, info, usage)
	require.NotEqual(t, summary.Quota, ordinary.Quota)
}
