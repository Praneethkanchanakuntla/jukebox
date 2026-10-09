package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/joho/godotenv"
)

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func LoadDatabase() (*DataBaseConnection, error) {
	_ = godotenv.Load()

	cfg := &DataBaseConnection{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnv("DB_PORT", "3306"),
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		Database: os.Getenv("DB_NAME"),
	}

	var missing []string
	if cfg.User == "" {
		missing = append(missing, "DB_USER")
	}
	if cfg.Password == "" {
		missing = append(missing, "DB_PASSWORD")
	}
	if cfg.Database == "" {
		missing = append(missing, "DB_NAME")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing env vars: %s", strings.Join(missing, ", "))
	}
	return cfg, nil
}

func LoadClerkCereds() {
	_ = godotenv.Load()

	clerk_key := os.Getenv("CLERK_SECRET_KEY")
	clerk.SetKey(clerk_key)
}
