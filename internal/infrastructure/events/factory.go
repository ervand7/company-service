package events

import (
	"fmt"
	"strings"

	appcompany "company-service/internal/application/company"
	"company-service/internal/config"

	"github.com/rs/zerolog"
)

const (
	ProducerLog   = "log"
	ProducerKafka = "kafka"
)

func NewProducer(cfg config.EventsConfig, logger zerolog.Logger) (appcompany.EventProducer, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Producer)) {
	case "", ProducerLog:
		return NewLogProducer(logger), nil
	case ProducerKafka:
		return NewKafkaProducer(cfg.KafkaBrokers, cfg.KafkaTopic, logger), nil
	default:
		return nil, fmt.Errorf("unsupported event producer %q", cfg.Producer)
	}
}
