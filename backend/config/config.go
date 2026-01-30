package config

// import "github.com/spf/viper"
import (
	"os"
	"strconv"
)


type ServerConfig struct {
	Port int
	JWTSecret string
}

// Structure pour la configuration de la base de donn√es (r√sout "undefined: DatabaseConfig")
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
	TheHive  ExternalService mapstructure:"thehive"
	OpenCTI  ExternalService mapstructure:"opencti"
	OpenRMF  ExternalService mapstructure:"openrmf"
}

type ExternalService struct {
	Enabled bool   mapstructure:"enabled"
	URL     string mapstructure:"url"
	APIKey  string mapstructure:"api_key"
}

// LoadConfig charge les configurations depuis les variables d'environnement
func LoadConfig() Config {
	// Impl√mentation simplifi√e
	port, _ := strconv.Atoi(os.Getenv("PORT"))
	
	// Dans un environnement de dev, le port DB est souvent le  par d√faut
	dbPort :=  

	return &Config{
		Server: ServerConfig{
			Port: port,
			JWTSecret: os.Getenv("JWT_SECRET"),
		},
		Database: DatabaseConfig{
			Host: os.Getenv("DB_HOST"),
			Port: dbPort,
			User: os.Getenv("DB_USER"),
			Password: os.Getenv("DB_PASSWORD"),
			DBName: os.Getenv("DB_NAME"),
		},
	}
}