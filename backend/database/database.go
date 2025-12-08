package database

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Connect() {
	// Build DSN from environment variables with sensible defaults
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "localhost"
	}

	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "5434"
	}

	user := os.Getenv("DB_USER")
	if user == "" {
		user = "openrisk"
	}

	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		password = "openrisk"
	}

	dbname := os.Getenv("DB_NAME")
	if dbname == "" {
		dbname = "openrisk"
	}

	// Try to use DATABASE_URL if provided (takes precedence)
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = fmt.Sprintf(
			"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
			host, user, password, dbname, port,
		)
	}

	var err error
	DB, err = gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})

	if err != nil {
		log.Fatal("❌ Failed to connect to database! \n", err)
	}

	log.Println("✅ Connected to PostgreSQL database successfully")

	sqlDB, err := DB.DB()
	if err != nil {
		log.Fatal("❌ Failed to get database instance")
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)
}
