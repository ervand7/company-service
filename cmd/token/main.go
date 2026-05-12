package main

import (
	"fmt"
	"os"
	"time"

	"company-service/internal/infrastructure/auth"

	"github.com/rs/zerolog"
)

func main() {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		logger.Fatal().Msg("JWT_SECRET is required")
	}

	subject := os.Getenv("JWT_SUBJECT")
	if subject == "" {
		subject = "local-user"
	}

	token, err := auth.NewJWTManager(secret).Generate(subject, time.Hour)
	if err != nil {
		logger.Fatal().Err(err).Str("subject", subject).Msg("failed to generate JWT token")
	}

	logger.Info().Str("subject", subject).Dur("ttl", time.Hour).Msg("generated JWT token")
	fmt.Println(token)
}
