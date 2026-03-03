package util

import "github.com/spf13/viper"

func YAMLReading(ConfigPath string) (interface{}, error) {
	var config interface{}
	viper.SetConfigFile(ConfigPath)
	err := viper.ReadInConfig()
	if err != nil {
		return config, err
	}
	err = viper.Unmarshal(&config)
	return config, err
}
