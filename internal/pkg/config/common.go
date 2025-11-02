package config

import "github.com/go-viper/mapstructure/v2"

func TagName(tagName string) func(*mapstructure.DecoderConfig) {
	return func(decoderConfig *mapstructure.DecoderConfig) {
		decoderConfig.TagName = tagName
	}
}
