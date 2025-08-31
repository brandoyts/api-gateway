package config

import (
	"time"

	"github.com/spf13/viper"
)

type Route struct {
	Name       string `mapstructure:"name"`
	Prefix     string `mapstructure:"prefix"`
	BackendUrl string `mapstructure:"backendUrl"`
}

type GatewayConfiguration struct {
	ListenAddress  string        `mapstructure:"listenAddress"`
	RequestTimeout time.Duration `mapstructure:"requestTimeout"`
	Routes         []Route       `mapstructure:"routes"`
}

func loadGatewayConfiguration() error {
	viper.SetConfigFile("./config/gatewayConfig.yml")

	err := viper.ReadInConfig()
	if err != nil {
		return err
		// log.Fatalf("error on reading gateway configuration file: %v", err)
	}

	return nil
}

func NewGatewayConfiguration() (*GatewayConfiguration, error) {
	err := loadGatewayConfiguration()
	if err != nil {
		return nil, err
	}

	var config GatewayConfiguration

	err = viper.Unmarshal(&config)
	if err != nil {
		// log.Fatalf("unable to decode gateway configuration into struct: %v", err)
		return nil, err
	}

	return &config, nil
}
