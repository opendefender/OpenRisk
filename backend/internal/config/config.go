package config

import (
	"os"
	"strconv"
)

type ServerConfig struct {
	Port                int
	JWTSecret           string // DEPRECATED: for legacy HMAC tokens only
	RSAPrivateKeyPath   string // Path to private.pem for RS256 signing
	RSAPublicKeyPath    string // Path to public.pem for RS256 verification
	RSAPrivateKeyInline string // Alternative: inline PEM content
	RSAPublicKeyInline  string // Alternative: inline PEM content
}

// Structure pour la configuration de la base de données
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
}

type Config struct {
	Server       ServerConfig
	Database     DatabaseConfig
	Integrations IntegrationsConfig
}

type IntegrationsConfig struct {
	TheHive ExternalService `mapstructure:"thehive"`
	OpenCTI ExternalService `mapstructure:"opencti"`
	OpenRMF ExternalService `mapstructure:"openrmf"`
}

type ExternalService struct {
	Enabled bool   `mapstructure:"enabled"`
	URL     string `mapstructure:"url"`
	APIKey  string `mapstructure:"api_key"`
}

// LoadConfig loads configuration from environment variables
// Panics on missing critical RSA keys (fail-fast security)
func LoadConfig() *Config {
	port, _ := strconv.Atoi(os.Getenv("PORT"))
	dbPort := 5432

	// Legacy JWT_SECRET (for backward compatibility, but no longer used for new tokens)
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		// Don't panic yet - RS256 is the primary method
		jwtSecret = "deprecated"
	}

	// RSA Keys for RS256 (CRITICAL — must be present)
	rsaPrivateKeyPath := os.Getenv("RSA_PRIVATE_KEY_PATH")
	rsaPublicKeyPath := os.Getenv("RSA_PUBLIC_KEY_PATH")
	rsaPrivateKeyInline := os.Getenv("RSA_PRIVATE_KEY")
	rsaPublicKeyInline := os.Getenv("RSA_PUBLIC_KEY")

	// Validate that at least one pair of RSA keys is available
	if (rsaPrivateKeyPath == "" && rsaPrivateKeyInline == "") ||
		(rsaPublicKeyPath == "" && rsaPublicKeyInline == "") {
		panic("CRITICAL: RSA keys required for RS256 JWT. Set RSA_PRIVATE_KEY_PATH + RSA_PUBLIC_KEY_PATH, or RSA_PRIVATE_KEY + RSA_PUBLIC_KEY")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" && os.Getenv("DB_HOST") == "" {
		panic("CRITICAL: DATABASE_URL or DB_HOST environment variable is required")
	}

	return &Config{
		Server: ServerConfig{
			Port:                port,
			JWTSecret:           jwtSecret,
			RSAPrivateKeyPath:   rsaPrivateKeyPath,
			RSAPublicKeyPath:    rsaPublicKeyPath,
			RSAPrivateKeyInline: rsaPrivateKeyInline,
			RSAPublicKeyInline:  rsaPublicKeyInline,
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
