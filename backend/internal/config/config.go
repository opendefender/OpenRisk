package config

// import "github.com/spf13/viper"
import (
"os"
"strconv"
)


type ServerConfig struct {
Port int
JWTSecret string
}

// Structure pour la configuration de la base de données (résout "undefined: DatabaseConfig")
type DatabaseConfig struct {
Host string
Port int
User string
Password string
DBName string
}

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

// LoadConfig charge les configurations depuis les variables d'environnement
func LoadConfig() *Config {
	port, _ := strconv.Atoi(os.Getenv("PORT"))
	dbPort := 5432 

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		panic("CRITICAL: JWT_SECRET environment variable is required")
	}
	if len(jwtSecret) < 32 {
		panic("CRITICAL: JWT_SECRET must be at least 32 characters long for security")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" && os.Getenv("DB_HOST") == "" {
		panic("CRITICAL: DATABASE_URL or DB_HOST environment variable is required")
	}

	return &Config{
		Server: ServerConfig{
			Port:      port,
			JWTSecret: jwtSecret,
		},
		Database: DatabaseConfig{
			Host:     os.Getenv("DB_HOST"),
			Port:     dbPort,
			User:     os.Getenv("DB_USER"),
			Password: os.Getenv("DB_PASSWORD"),
			DBName:   os.Getenv("DB_NAME"),
		},
	}
}
