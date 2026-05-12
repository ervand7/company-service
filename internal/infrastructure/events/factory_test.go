package events

import (
	"testing"

	"company-service/internal/config"

	"github.com/rs/zerolog"
)

func TestNewProducerCreatesLogProducerByDefault(t *testing.T) {
	producer, err := NewProducer(config.EventsConfig{}, zerolog.Nop())
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := producer.(*LogProducer); !ok {
		t.Fatalf("expected LogProducer, got %T", producer)
	}
}

func TestNewProducerCreatesKafkaProducer(t *testing.T) {
	producer, err := NewProducer(config.EventsConfig{
		Producer:     ProducerKafka,
		KafkaBrokers: []string{"localhost:9092"},
		KafkaTopic:   "company.events",
	}, zerolog.Nop())
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := producer.(*KafkaProducer); !ok {
		t.Fatalf("expected KafkaProducer, got %T", producer)
	}
}

func TestNewProducerRejectsUnsupportedProducer(t *testing.T) {
	_, err := NewProducer(config.EventsConfig{Producer: "unknown"}, zerolog.Nop())
	if err == nil {
		t.Fatal("expected error")
	}
}
