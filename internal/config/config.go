package config

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port   string
	Tokens []string
	DBPath string
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		// .env file is optional, continue with environment variables
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	tokensStr := os.Getenv("TOKENS")
	var tokens []string
	if tokensStr != "" {
		tokens = strings.Split(tokensStr, ",")
		for i := range tokens {
			tokens[i] = strings.TrimSpace(tokens[i])
		}
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./auth.db"
	}

	return &Config{
		Port:   port,
		Tokens: tokens,
		DBPath: dbPath,
	}, nil
}
