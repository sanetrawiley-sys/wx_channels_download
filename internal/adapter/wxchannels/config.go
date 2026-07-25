package wxchannels

import (
	"wx_channel/internal/config"
)

// ChannelsPluginConfig implements config.Configurable for wxchannels plugin config.
type ChannelsPluginConfig struct {
	DisableLocationToHome bool
	RefreshInterval       int
}

func (c *ChannelsPluginConfig) ConfigNamespace() string { return "channels" }

func (c *ChannelsPluginConfig) ConfigSchema() []config.ConfigItem {
	return []config.ConfigItem{
		{
			Key:         "disableLocationToHome",
			Type:        config.ConfigTypeBool,
			Default:     false,
			Description: "是否禁止从视频号详情页重定向到首页（视频号默认行为）",
			Title:       "禁止重定向",
			Group:       "Channels",
			HotReload:   true,
		},
		{
			Key:         "refreshInterval",
			Type:        config.ConfigTypeInt,
			Default:     0,
			Description: "视频号页面定时刷新时间间隔（秒），0 为不刷新",
			Title:       "定时刷新间隔",
			Group:       "Channels",
			HotReload:   true,
		},
	}
}

func (c *ChannelsPluginConfig) ApplyConfig(sub *config.SubViper) error {
	c.DisableLocationToHome = sub.GetBool("disableLocationToHome")
	c.RefreshInterval = sub.GetInt("refreshInterval")
	return nil
}

// GetChannelsConfig returns the registered channels plugin config if available.
// Returns nil if the plugin has not been registered.
func GetChannelsConfig() *ChannelsPluginConfig {
	return channelsPluginConfig
}

// channelsPluginConfig is the singleton instance populated during config loading.
var channelsPluginConfig *ChannelsPluginConfig

func init() {
	channelsPluginConfig = &ChannelsPluginConfig{}
	config.RegisterPlugin(channelsPluginConfig)

	// Legacy alias for backward compatibility; registered with its flat key directly
	// to avoid the namespace auto-prefix applied by LoadPluginConfigs.
	config.Register(config.ConfigItem{
		Key:         "channel.disableLocationToHome",
		Type:        config.ConfigTypeBool,
		Default:     false,
		Description: "是否禁止从视频号详情页重定向到首页（视频号默认行为）",
		Title:       "禁止重定向",
		Group:       "Channels",
		HotReload:   true,
	})
}
