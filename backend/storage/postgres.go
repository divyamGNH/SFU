package storage

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	ErrRoomNotFound          = errors.New("room not found")
	ErrClientNotFound        = errors.New("client not found")
	ErrInvalidRoomID         = errors.New("invalid room ID")
	ErrInvalidClientID       = errors.New("invalid client ID")
	ErrEmptyConnectionString = errors.New("PostgreSql connection string is required.")
)

type PostgresStorage struct {
	db *gorm.DB
}

// Create a new PostgreSql storage instance.
func NewPostgresStorage(connectionString string) (*PostgresStorage, error) {
	log.Println("[POSTGRES] Initiating new Postgres storage")

	//Check if connection string is empty or not.
	if connectionString == "" {
		log.Println("[POSTGRES] ERROR : Connection string is empty!!")
		return nil, ErrEmptyConnectionString
	}

	log.Printf("[POSTGRES] Connection string received (length=%d, preview=%s)", len(connectionString), maskConnString(connectionString))

	//Actually connect to the DB.
	log.Println("[POSTGRES] Attempting to connect to the database")
	// We are not passing any gorm configurations here.
	db, err := gorm.Open(postgres.Open(connectionString), &gorm.Config{})
	if err != nil {
		log.Println("[POSTGRES] Error connecting to the database : ", err)
		return nil, err
	}

	log.Println("[POSTGRES] Successfully connected to PostgreSQL database")

	//Auto migrate current tables to the database.
	//Migrate the tables seperately according to logic.

	//TODO : Make and migrate signalling tables

	//Migrate Auth tables
	if err := MigrateAuthModels(db); err != nil {
		log.Printf("[POSTGRES] ERROR migrating auth tables: %v", err)
		return nil, fmt.Errorf("failed to migrate auth tables: %w", err)
	}
	log.Println("[POSTGRES] Auth tables migrated successfully")

	//Configure connection pool to prevent exhaustion under concurrent load.
	log.Println("[POSTGRES] Configuring connection pool...")
	sqlDB, err := db.DB()
	if err != nil {
		log.Printf("[POSTGRES] WARNING: could not configure connection pool: %v", err)
	} else {
		sqlDB.SetMaxOpenConns(25)
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetConnMaxLifetime(5 * time.Minute)
	}

	log.Println("[POSTGRES] PostgresStorage initialization complete")
	return &PostgresStorage{db: db}, nil
}

// maskConnString masks the password in a PostgreSQL connection string for safe logging
func maskConnString(connStr string) string {
	if idx := strings.Index(connStr, ":postgres"); idx != -1 {
		return connStr[:20] + "***...***"
	}
	if len(connStr) > 30 {
		return connStr[:30] + "..."
	}
	return connStr
}
