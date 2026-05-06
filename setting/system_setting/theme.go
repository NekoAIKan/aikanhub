package system_setting

import (
	"github.com/QuantumNous/new-api/setting/config"
)

type ThemeSettings struct {
	Frontend string `json:"frontend"`
}

var themeSettings = ThemeSettings{
	Frontend: "default",
}

func init() {
	config.GlobalConfig.Register("theme", &themeSettings)
}

func normalizeThemeSettings() {
	if themeSettings.Frontend != "default" {
		themeSettings.Frontend = "default"
	}
}

func GetThemeSettings() *ThemeSettings {
	normalizeThemeSettings()
	return &themeSettings
}

func UpdateAndSyncTheme() {
	normalizeThemeSettings()
}
