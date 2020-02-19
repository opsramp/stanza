package config

import (
	"github.com/bluemedora/log-agent/plugin"
	"github.com/mitchellh/mapstructure"
)

type Config struct {
	Plugins []plugin.PluginConfig
}

func UnmarshalHook(c *mapstructure.DecoderConfig) {
	c.DecodeHook = plugin.PluginConfigToStructHookFunc()
}