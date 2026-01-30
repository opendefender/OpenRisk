package migrations

import (
    "log"
    "os"

    "github.com/golang-migrate/migrate/v"
    _ "github.com/golang-migrate/migrate/v/database/postgres"
    _ "github.com/golang-migrate/migrate/v/source/file"
)

// RunMigrations runs SQL migrations located in the repository /migrations directory.
// It expects environment variable DATABASE_URL to be set (format: postgres://user:pass@host:port/dbname?sslmode=disable)
func RunMigrations() {
    dbURL := os.Getenv("DATABASE_URL")
    if dbURL == "" {
        log.Println("DATABASE_URL not set — skipping migrations")
        return
    }

    // Use file:// relative to repo root 'migrations' directory
    m, err := migrate.New("file://migrations", dbURL)
    if err != nil {
        log.Printf("migrations: failed to initialize: %v", err)
        return
    }

    if err := m.Up(); err != nil {
        if err == migrate.ErrNoChange {
            log.Println("migrations: no change")
            return
        }
        log.Printf("migrations: up failed: %v", err)
    } else {
        log.Println("migrations: applied successfully")
    }
}
