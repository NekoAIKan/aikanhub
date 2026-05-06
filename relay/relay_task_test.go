package relay

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/config"
	videobilling "github.com/QuantumNous/new-api/setting/video_billing_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestApplyVideoProfileBillingDoesNotExposeCostMarkupInOtherRatios(t *testing.T) {
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"profit_setting.default_markup_percent":           "25",
		"profit_setting.upstream_cost_per_million_tokens": "0.8",
		"profit_setting.apply_to_default_video_profiles":  "true",
	}))

	c, _ := gin.CreateTestContext(nil)
	c.Set("task_request", relaycommon.TaskSubmitReq{
		Duration: 5,
		Metadata: map[string]any{
			"resolution": "720p",
		},
	})

	info := &relaycommon.RelayInfo{
		OriginModelName: "doubao-seedance-2-0-fast-260128",
		UsingGroup:      "default",
		UserGroup:       "default",
	}
	profile := videobilling.VideoBillingProfile{
		Mode:                    videobilling.ModeFormula,
		UnitPrice:               1,
		FallbackFPS:             24,
		FallbackWidth:           1280,
		FallbackHeight:          720,
		FallbackDurationSeconds: 5,
		UseUpstreamUsage:        true,
		ConservativeMultiplier:  1.25,
		ResolutionAliases: map[string]videobilling.VideoResolution{
			"720p": {Width: 1280, Height: 720},
		},
	}

	require.Nil(t, applyVideoProfileBilling(c, info, profile))

	_, ok := service.VideoBillingResultFromContext(c)
	require.True(t, ok)
	require.Contains(t, info.PriceData.OtherRatios, "video_estimated_quota")
	require.NotContains(t, info.PriceData.OtherRatios, "video_upstream_unit_cost")
	require.NotContains(t, info.PriceData.OtherRatios, "video_markup_percent")
}
