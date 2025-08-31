package config

import (
	"github.com/spf13/viper"
)

type TelemetryConfiguration struct {
	ServiceName    string `mapstructure:"serviceName"`
	ServiceVersion string `mapstructure:"serviceVersion"`
	Enabled        bool   `mapstructure:"telemetryEnabled"`
}

func loadTelemetryConfiguration() error {
	viper.SetConfigFile("./config/telemetryConfig.yml")

	err := viper.ReadInConfig()
	if err != nil {
		return err
	}

	return nil
}

func NewTelemetryConfiguration() (*TelemetryConfiguration, error) {
	err := loadTelemetryConfiguration()
	if err != nil {
		return nil, err
	}

	var config TelemetryConfiguration

	err = viper.Unmarshal(&config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
