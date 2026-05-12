package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	appcompany "company-service/internal/application/company"
	companydomain "company-service/internal/domain/company"

	"github.com/rs/zerolog"
	"github.com/segmentio/kafka-go"
)

type kafkaWriter interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

type KafkaProducer struct {
	writer kafkaWriter
	topic  string
	logger zerolog.Logger
}

type kafkaEvent struct {
	ID         string        `json:"id"`
	Type       string        `json:"type"`
	OccurredAt time.Time     `json:"occurred_at"`
	CompanyID  string        `json:"company_id"`
	Company    *eventCompany `json:"company,omitempty"`
}

type eventCompany struct {
	ID                string                    `json:"id"`
	Name              string                    `json:"name"`
	Description       string                    `json:"description"`
	AmountOfEmployees int                       `json:"amount_of_employees"`
	Registered        bool                      `json:"registered"`
	Type              companydomain.CompanyType `json:"type"`
	CreatedAt         time.Time                 `json:"created_at"`
	UpdatedAt         time.Time                 `json:"updated_at"`
}

func NewKafkaProducer(brokers []string, topic string, logger zerolog.Logger) *KafkaProducer {
	return &KafkaProducer{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        topic,
			Balancer:     &kafka.Hash{},
			RequiredAcks: kafka.RequireAll,
		},
		topic:  topic,
		logger: logger,
	}
}

func (p *KafkaProducer) Publish(ctx context.Context, event appcompany.Event) error {
	payload, err := json.Marshal(toKafkaEvent(event))
	if err != nil {
		return fmt.Errorf("marshal kafka event: %w", err)
	}

	message := kafka.Message{
		Key:   []byte(event.CompanyID.String()),
		Value: payload,
		Time:  event.OccurredAt,
	}
	if err := p.writer.WriteMessages(ctx, message); err != nil {
		return fmt.Errorf("publish kafka event: %w", err)
	}

	p.logger.Info().
		Str("event_id", event.ID.String()).
		Str("event_type", event.Type).
		Str("company_id", event.CompanyID.String()).
		Str("topic", p.topic).
		Msg("company event published to kafka")
	return nil
}

func (p *KafkaProducer) Close(ctx context.Context) error {
	_ = ctx
	if err := p.writer.Close(); err != nil {
		return fmt.Errorf("close kafka writer: %w", err)
	}
	return nil
}

func toKafkaEvent(event appcompany.Event) kafkaEvent {
	result := kafkaEvent{
		ID:         event.ID.String(),
		Type:       event.Type,
		OccurredAt: event.OccurredAt,
		CompanyID:  event.CompanyID.String(),
	}
	if event.Company != nil {
		result.Company = &eventCompany{
			ID:                event.Company.ID.String(),
			Name:              event.Company.Name,
			Description:       event.Company.Description,
			AmountOfEmployees: event.Company.AmountOfEmployees,
			Registered:        event.Company.Registered,
			Type:              event.Company.Type,
			CreatedAt:         event.Company.CreatedAt,
			UpdatedAt:         event.Company.UpdatedAt,
		}
	}

	return result
}
