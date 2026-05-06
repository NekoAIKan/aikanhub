package profit_setting

import "github.com/QuantumNous/new-api/setting/config"

type ProfitSetting struct {
	DefaultMarkupPercent         float64 `json:"default_markup_percent"`
	UpstreamCostPerMillionTokens float64 `json:"upstream_cost_per_million_tokens"`
	ApplyToDefaultVideoProfiles  bool    `json:"apply_to_default_video_profiles"`
}

var profitSetting = defaultProfitSetting()

func init() {
	config.GlobalConfig.Register("profit_setting", &profitSetting)
}

func defaultProfitSetting() ProfitSetting {
	return ProfitSetting{
		ApplyToDefaultVideoProfiles: true,
	}
}

func GetProfitSetting() ProfitSetting {
	return profitSetting
}
