package helper

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	ds "github.com/QuantumNous/new-api/setting/deepseek_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDeepSeekPreconsumePinsConfigAndBypassesOrdinaryPrices(t *testing.T) {
	r := ds.DefaultRule()
	r.Enabled = true
	r.Version = 1
	r.EffectiveAt = 1
	data, err := common.Marshal(ds.Store{Revision: 1, Rules: []ds.Rule{r}})
	require.NoError(t, err)
	common.OptionMapRWMutex.Lock()
	old := common.OptionMap[ds.OptionKey]
	if common.OptionMap == nil {
		common.OptionMap = map[string]string{}
	}
	common.OptionMap[ds.OptionKey] = string(data)
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap[ds.OptionKey] = old
		common.OptionMapRWMutex.Unlock()
	})
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{OriginModelName: ds.Model, StartTime: time.Date(2026, 11, 10, 10, 0, 0, 0, ds.Location), UsingGroup: "default", UserGroup: "default"}
	price, err := ModelPriceHelper(c, info, 1000000, &types.TokenCountMeta{MaxTokens: 1000000})
	require.NoError(t, err)
	expected, err := info.DeepSeekBillingSnapshot.Calculate(1000000, 0, 1000000)
	require.NoError(t, err)
	require.Equal(t, expected.Quota, price.QuotaToPreConsume)
	require.Equal(t, 1.0, price.GroupRatioInfo.GroupRatio)
	common.OptionMapRWMutex.Lock()
	common.OptionMap[ds.OptionKey] = `{"revision":0,"rules":[]}`
	common.OptionMapRWMutex.Unlock()
	again, err := ModelPriceHelper(c, info, 1000000, &types.TokenCountMeta{MaxTokens: 1000000})
	require.NoError(t, err)
	require.Equal(t, price.QuotaToPreConsume, again.QuotaToPreConsume)
}
