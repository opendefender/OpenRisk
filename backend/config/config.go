package config

import "github.com/spf13/viper"

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	// Modules externes
	Integrations IntegrationsConfig
}

type IntegrationsConfig struct {
	TheHive  ExternalService `mapstructure:"thehive"`
	OpenCTI  ExternalService `mapstructure:"opencti"`
	OpenRMF  ExternalService `mapstructure:"openrmf"`
}

type ExternalService struct {
	Enabled bool   `mapstructure:"enabled"`
	URL     string `mapstructure:"url"`
	APIKey  string `mapstructure:"api_key"`
}

// LoadConfig charge depuis .env (ex: THEHIVE_ENABLED=true)
func LoadConfig() (config Config, err error) {
	viper.AddConfigPath(".")
	viper.SetConfigName("app")
	viper.SetConfigType("env")
	
	viper.AutomaticEnv() // Lit les variables d'environnement automatiquement

	err = viper.ReadInConfig()
	if err != nil {
		return
	}
	err = viper.Unmarshal(&config)
	return
}