package telemetry

import "github.com/spf13/viper"

type TelemetryConfiguration struct {
	ServiceName    string `mapstructure:"serviceName"`
	ServiceVersion string `mapstructure:"serviceVersion"`
	Enabled        bool   `mapstructure:"telemetryEnabled"`
}

func loadTelemetryConfiguration(relativePath string) error {
	viper.SetConfigFile(relativePath)

	err := viper.ReadInConfig()
	if err != nil {
		return err
	}

	return nil
}

func NewTelemetryConfiguration(relativePath string) (*TelemetryConfiguration, error) {
	err := loadTelemetryConfiguration(relativePath)
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
